package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/config"
)

// TestChatStreamsSSE Chat 应以流式请求（headers 立即返回），并把 SSE 增量拼成完整回答
func TestChatStreamsSSE(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody, gotAuth, gotPath = string(raw), r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\n")
		fmt.Fprint(w, ": keep-alive\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"，世界\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL + "/v1"
	cfg.AI.Model = "test-model"
	cfg.AI.APIKey = "sk-x"
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "你好，世界" {
		t.Fatalf("out = %q", out)
	}
	if !strings.Contains(gotBody, `"stream":true`) {
		t.Fatalf("请求体应声明流式: %s", gotBody)
	}
	if gotAuth != "Bearer sk-x" || gotPath != "/v1/chat/completions" {
		t.Fatalf("auth=%q path=%q", gotAuth, gotPath)
	}
}

// TestChatStreamErrorPayload 上游在流内返回 error 事件时应透传错误消息
func TestChatStreamErrorPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"error\":{\"message\":\"额度不足\"}}\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	if _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil || !strings.Contains(err.Error(), "额度不足") {
		t.Fatalf("应透传流内错误: %v", err)
	}
}

// TestChatIdleTimeout 连接挂起（长时间无新数据）应在空闲超时内报错，而不是无限等待
func TestChatIdleTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n")
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second) // 之后挂起不发数据
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	c.IdleTimeout = 150 * time.Millisecond
	if _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil || !strings.Contains(err.Error(), "响应超时") {
		t.Fatalf("空闲挂起应报超时: %v", err)
	}
}

func TestChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	if _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("应返回含状态码的错误: %v", err)
	}
}

func TestNewClientRequiresConfig(t *testing.T) {
	if _, err := NewClient(&config.Config{}); err == nil {
		t.Fatal("未配置应报错")
	}
}

func TestExtractActions(t *testing.T) {
	reply := "这是说明文字。\n\n```json\n{\"actions\":[{\"file\":\"Study/notes/a.md\",\"action\":\"edit\",\"content\":\"# 新\"}]}\n```\n完毕"
	prose, actions := SplitReply(reply)
	if len(actions) != 1 || actions[0].File != "Study/notes/a.md" || actions[0].Action != "edit" {
		t.Fatalf("actions = %+v", actions)
	}
	if strings.Contains(prose, "gs-actions") || strings.Contains(prose, "\"actions\"") {
		t.Fatalf("prose 不应含动作块: %s", prose)
	}
	if _, actions2 := SplitReply("普通回答，无动作"); len(actions2) != 0 {
		t.Fatal("无动作块应返回空")
	}
}

func TestSafePath(t *testing.T) {
	root := t.TempDir()
	p, err := SafePath(root, "Study/notes/a.md")
	if err != nil || p != filepath.Join(root, "Study", "notes", "a.md") {
		t.Fatalf("p=%s err=%v", p, err)
	}
	if _, err := SafePath(root, "Study/x.go"); err != nil {
		t.Fatalf(".go 应放行: %v", err)
	}
	for _, bad := range []string{"../evil.md", "/etc/passwd", "a.exe", "a.gs-tmp"} {
		if _, err := SafePath(root, bad); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}
}

// TestChatWithToolsAssemblesToolCalls 流式 tool_calls 分片应按 index 拼装，content 增量回调 onDelta
func TestChatWithToolsAssemblesToolCalls(t *testing.T) {
	var deltas []string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"让我查一下\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"search_workspace\",\"arguments\":\"{\\\"query\\\":\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"golang\\\"}\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"id\":\"call_2\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{}\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	tools := []Tool{{Name: "search_workspace"}, {Name: "read_file"}}
	m, err := c.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, tools,
		func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatal(err)
	}
	if m.Content != "让我查一下" || len(deltas) != 1 || deltas[0] != "让我查一下" {
		t.Fatalf("content=%q deltas=%v", m.Content, deltas)
	}
	if len(m.ToolCalls) != 2 {
		t.Fatalf("tool_calls = %+v", m.ToolCalls)
	}
	tc := m.ToolCalls[0]
	if tc.ID != "call_1" || tc.Name != "search_workspace" || tc.Arguments != `{"query":"golang"}` {
		t.Fatalf("tc0 = %+v", tc)
	}
	if m.ToolCalls[1].ID != "call_2" || m.ToolCalls[1].Arguments != "{}" {
		t.Fatalf("tc1 = %+v", m.ToolCalls[1])
	}
	if !strings.Contains(gotBody, `"tools"`) || !strings.Contains(gotBody, `"search_workspace"`) {
		t.Fatalf("请求体应携带 tools: %s", gotBody)
	}
}

// TestChatWithToolsWireFormat tools 必须按 OpenAI 规范序列化：外层 type:"function"，函数定义嵌在 function 里
func TestChatWithToolsWireFormat(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	tools := []Tool{{Name: "search_workspace", Description: "检索", Parameters: json.RawMessage(`{"type":"object"}`)}}
	if _, err := c.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, tools, nil); err != nil {
		t.Fatal(err)
	}
	var req struct {
		Tools []struct {
			Type     string `json:"type"`
			Name     string `json:"name"`
			Function struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Parameters  json.RawMessage `json:"parameters"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v\n%s", err, gotBody)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("应携带 1 个 tool，实际 %d: %s", len(req.Tools), gotBody)
	}
	entry := req.Tools[0]
	if entry.Type != "function" {
		t.Fatalf("tools[0].type 应为 function，实际 %q: %s", entry.Type, gotBody)
	}
	if entry.Name != "" || entry.Function.Name != "search_workspace" {
		t.Fatalf("函数定义应嵌在 function 里且外层无裸 name: %s", gotBody)
	}
	if entry.Function.Description != "检索" || string(entry.Function.Parameters) != `{"type":"object"}` {
		t.Fatalf("function 内容应原样透传: %s", gotBody)
	}
}

// TestChatWithToolsToolCallWireFormat 第二轮回传的 assistant.tool_calls 必须是 OpenAI 包装格式：
// 每个条目带 type:"function"，name/arguments 嵌在 function 里
func TestChatWithToolsToolCallWireFormat(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	msgs := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "我看一下",
			ToolCalls: []ToolCall{{ID: "call_1", Name: "list_dir", Arguments: `{"path":"Study"}`}}},
		{Role: "tool", Content: "a.md\nb.md", ToolCallID: "call_1"},
	}
	if _, err := c.ChatWithTools(context.Background(), msgs, nil, nil); err != nil {
		t.Fatal(err)
	}
	var req struct {
		Messages []struct {
			Role      string `json:"role"`
			ToolCalls []struct {
				Type     string `json:"type"`
				ID       string `json:"id"`
				Name     string `json:"name"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v\n%s", err, gotBody)
	}
	var asst *struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		Name     string `json:"name"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	for i := range req.Messages {
		if req.Messages[i].Role == "assistant" && len(req.Messages[i].ToolCalls) > 0 {
			asst = &req.Messages[i].ToolCalls[0]
		}
	}
	if asst == nil {
		t.Fatalf("assistant 消息应携带 tool_calls: %s", gotBody)
	}
	if asst.Type != "function" {
		t.Fatalf("tool_calls[0].type 应为 function，实际 %q: %s", asst.Type, gotBody)
	}
	if asst.Name != "" || asst.Function.Name != "list_dir" || asst.Function.Arguments != `{"path":"Study"}` {
		t.Fatalf("name/arguments 应嵌在 function 里且外层无裸 name: %s", gotBody)
	}
	if asst.ID != "call_1" {
		t.Fatalf("tool_call id 应保留: %s", gotBody)
	}
}

// TestChatWithToolsEmptyRejects 既无 content 又无 tool_calls 时报错
func TestChatWithToolsEmptyRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	if _, err := c.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "x"}}, nil, nil); err == nil || !strings.Contains(err.Error(), "缺少内容") {
		t.Fatalf("空响应应报错: %v", err)
	}
}
