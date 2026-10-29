package command

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

// newTestAIState httptest SSE 脚本服务 + AIState + 临时工作区
func newTestAIState(t *testing.T, events ...string) (*AIState, *workspace.Workspace, *config.Config) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, e := range events {
			fmt.Fprint(w, e)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, err := ai.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &AIState{Client: c, AskSea: func() bool { return false }}, &workspace.Workspace{Root: t.TempDir()}, cfg
}

func TestAITurnDirectAnswer(t *testing.T) {
	st, ws, cfg := newTestAIState(t, "data: {\"choices\":[{\"delta\":{\"content\":\"直接回答\"}}]}\n\n")
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := AITurn(ctx, st, "你好"); err != nil {
		t.Fatal(err)
	}
	if len(st.Hist) != 2 || st.Hist[0].Role != "user" || st.Hist[1].Role != "assistant" {
		t.Fatalf("Hist 应只追加 user+assistant 两条: %+v", st.Hist)
	}
	if st.Hist[1].Content != "直接回答" {
		t.Fatalf("hist=%q", st.Hist[1].Content)
	}
	if !strings.Contains(stripANSI(out.String()), "直接回答") {
		t.Fatalf("回答应整段渲染输出: %s", out.String())
	}
}

func TestAITurnToolLoop(t *testing.T) {
	st, ws, cfg := newTestAIState(t,
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"path\\\":\\\"Study/sea/a.md\\\"}\"}}]}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\"文件内容是 hello\"}}]}\n\n",
	)
	os.MkdirAll(filepath.Join(ws.Root, "Study", "sea"), 0o755)
	os.WriteFile(filepath.Join(ws.Root, "Study", "sea", "a.md"), []byte("hello"), 0o644)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := AITurn(ctx, st, "读一下 a.md"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "· read_file") {
		t.Fatalf("应打印工具步骤行: %s", out.String())
	}
	if !strings.Contains(stripANSI(out.String()), "文件内容是 hello") {
		t.Fatalf("最终回答应整段渲染输出: %s", out.String())
	}
	if len(st.Hist) != 2 {
		t.Fatalf("Hist 应只存两条，不含工具中间消息: %+v", st.Hist)
	}
}

func TestAITurnSeaPromptYes(t *testing.T) {
	// 确认后触发存档：httptest handler 对分类请求重放同一事件（回复=回答原文），
	// 解析不出 JSON → 兜底未分类
	st, ws, cfg := newTestAIState(t, "data: {\"choices\":[{\"delta\":{\"content\":\"回答内容\"}}]}\n\n")
	asked := false
	st.AskSea = func() bool { asked = true; return true }
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := AITurn(ctx, st, "问"); err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("答毕应触发 sea 询问")
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), "未分类")); err != nil {
		t.Fatalf("应落入未分类: %v", err)
	}
	if !strings.Contains(out.String(), "已存入") {
		t.Fatalf("应打印结果行: %s", out.String())
	}
	if len(st.Hist) != 2 {
		t.Fatalf("存档不应改写 Hist: %+v", st.Hist)
	}
}

func TestHistoryText(t *testing.T) {
	st := &AIState{Hist: []ai.Message{
		{Role: "user", Content: "iota 是什么"},
		{Role: "assistant", Content: "**iota** 是常量计数器"},
		{Role: "user", Content: "slice 怎么扩容"},
		{Role: "assistant", Content: "翻倍增长"},
	}}
	s := historyText(st)
	if !strings.Contains(s, "会话回顾 · 2 轮") {
		t.Fatalf("标题轮数缺失: %s", s)
	}
	if !strings.Contains(s, "1. 问") || !strings.Contains(s, "iota 是什么") || !strings.Contains(s, "**iota** 是常量计数器") {
		t.Fatalf("第 1 轮缺失: %s", s)
	}
	if !strings.Contains(s, "2. 问") || !strings.Contains(s, "翻倍增长") {
		t.Fatalf("第 2 轮缺失: %s", s)
	}
}

func TestAIHistoryViewEmpty(t *testing.T) {
	ws := wsForTest(t)
	var out strings.Builder
	ctx := &Ctx{WS: ws, Out: &out}
	if err := AIHistoryView(ctx, &AIState{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(out.String()), "本轮还没有对话") {
		t.Fatalf("应提示空会话: %s", out.String())
	}
}

func TestCompactArgs(t *testing.T) {
	// "…" 是 3 字节 UTF-8：100 字节截断 + 3 字节省略号 = 103
	if got := compactArgs(strings.Repeat("a", 120)); len(got) != 103 || !strings.HasSuffix(got, "…") {
		t.Fatalf("应截断为 100 字节+省略号: %q", got)
	}
	if got := compactArgs("  {\"query\":\"x\"}  "); got != `{"query":"x"}` {
		t.Fatalf("应去首尾空白: %q", got)
	}
}
