# AI 模式 Agent 化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 gs 的 `/AI` 从单轮 chat 升级为 Agent 轮：模型可自主调用 4 个工具（检索/读文件/列目录/受确认写文件）多步完成查询与改写。

**Architecture:** `internal/ai` 扩展 OpenAI tools 协议（`ChatWithTools`，流式拼装 tool_calls）；新建 `internal/agent` 纯逻辑包承载循环与工具；`command/ai.go` 只留 UI 壳（步骤行、流式打印、写审批）。设计正本：`docs/project/全局/2026-10-26-AI模式Agent化-design.md`（含全部决策记录）。

**Tech Stack:** Go 标准库 + 既有 `internal/search`（新增 `SearchFiltered` 入口）、`internal/workspace.WriteFileAtomic`、`internal/ai.SafePath`、`internal/ui`/`diffx`。无新依赖。

**纪律约束（覆盖本计划所有任务）：**

1. **禁止 AI 执行任何 git 命令**（项目 CLAUDE.md 硬性规则）。各任务末尾的「提交点」是写给开发者的建议，由开发者自行提交。
2. TDD：每个任务先写失败测试、跑红、再实现、跑绿。
3. 无需生成 designs-specs：本计划的 spec 即 `2026-10-26-AI模式Agent化-design.md`，二者共同构成实施依据。
4. 完成后在 `docs/log/<YYMMDD>.md` 追加开发日志（每小节标题下附 `> 创建于 YYYY-MM-DD HH:MM` 时间行，用 `date` 取实际时间）。

---

### Task 1: `internal/search` 抽出无 config 依赖的 `SearchFiltered`

**Files:**
- Modify: `internal/search/search.go:24-71`

agent 包不 import config（spec 已定），搜索参数由调用方展开传入。

- [ ] **Step 1: 跑既有测试确认基线**

Run: `go test ./internal/search/ -v`
Expected: 全部 PASS（记录用例名，重构后须保持）

- [ ] **Step 2: 重构 Search 为委托**

`search.Search` 改为薄委托，新增 `SearchFiltered`；`excluded` 改收 `[]string`。替换 `search.go` 中三个相关函数（`funcRe`、`Hit`、`FileResult`、`searchFile` 均不动）：

```go
// Search 多词 AND：文件内全部词命中才收录；行级任一词命中即为 Hit
func Search(root string, cfg *config.Config, query string) ([]FileResult, []string, error) {
	return SearchFiltered(root, cfg.Scan.Exclude, cfg.Scan.MaxFileKB, query)
}

// SearchFiltered 与 Search 相同，但直接接收排除目录与大小上限（供无 config 依赖的调用方使用）
func SearchFiltered(root string, exclude []string, maxFileKB int64, query string) ([]FileResult, []string, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, nil, nil
	}
	var results []FileResult
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excluded(exclude, root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".md" && ext != ".go" {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > maxFileKB*1024 {
			return nil
		}
		if fr := searchFile(path, root, words); fr != nil {
			results = append(results, *fr)
		}
		return nil
	})
	return results, words, err
}

func excluded(exclude []string, root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		for _, ex := range exclude {
			if seg == ex {
				return true
			}
		}
	}
	return false
}
```

- [ ] **Step 3: 跑既有测试确认不回归**

Run: `go build ./... && go test ./internal/search/ -v`
Expected: PASS（与 Step 1 相同用例全绿）

- [ ] **提交点（开发者执行）**：`refactor: search 抽出无 config 依赖的 SearchFiltered`

---

### Task 2: `internal/ai` tools 协议扩展（ChatWithTools）

**Files:**
- Modify: `internal/ai/client.go`（Message/chatReq/chatChunk/Chat 重构，新增 Tool/ToolCall/ChatWithTools）
- Test: `internal/ai/ai_test.go`（追加用例）

- [ ] **Step 1: 写失败测试**

在 `ai_test.go` 末尾追加（imports 需补 `context` 已有，无需新增包）：

```go
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
```

- [ ] **Step 2: 跑测试确认编译失败**

Run: `go test ./internal/ai/ -run TestChatWithTools -v`
Expected: 编译错误 `undefined: Tool` / `c.ChatWithTools is undefined`

- [ ] **Step 3: 实现**

`client.go` 改动 4 处。

(a) 类型区（`Message` 替换 + 新增两个类型）：

```go
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // role:"tool" 回填用
}

// Tool OpenAI function 定义，Parameters 为 JSON Schema 原样透传
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
```

(b) `chatReq` 与 `chatChunk`：

```go
type chatReq struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	Stream      bool      `json:"stream"`
	Tools       []Tool    `json:"tools,omitempty"`
}

// chatChunk 兼容两种载荷：非 200 的 JSON 错误体（error 字段）与流式增量（choices[].delta）
type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`

	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
```

(c) `Chat` 改为委托，删除其原函数体，原整体超时/看门狗/SSE 循环逻辑全部移入 `ChatWithTools`：

```go
// Chat 无工具便捷形式，等价于 ChatWithTools(msgs, nil, nil)
func (c *Client) Chat(ctx context.Context, msgs []Message) (string, error) {
	m, err := c.ChatWithTools(ctx, msgs, nil, nil)
	if err != nil {
		return "", err
	}
	return m.Content, nil
}

// toolCallAcc 流式增量中单个 tool_call 的拼装缓冲（按 index 对齐）
type toolCallAcc struct{ id, name, args strings.Builder }

// ChatWithTools 调用 chat/completions（流式）并支持 tools。
// content 增量实时回调 onDelta（可为 nil）；tool_calls 增量按 index 静默拼装。
// 返回的 assistant 消息可能只有 content、只有 tool_calls，或两者兼有。
func (c *Client) ChatWithTools(ctx context.Context, msgs []Message, tools []Tool, onDelta func(string)) (Message, error) {
```

函数体 = 原 `Chat` 函数体，做如下替换：

1. `body, err := json.Marshal(chatReq{...})` 一行加 `Tools: tools`
2. 所有 `return "", err` / `return "", fmt.Errorf...` / `return "", errors.New...` 改为返回 `Message{}, ...`
3. `var out strings.Builder` 改为：

```go
	var content strings.Builder
	var accs []*toolCallAcc
```

4. 原累加分支 `if len(chunk.Choices) > 0 { out.WriteString(...) }` 替换为：

```go
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			content.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(delta.Content)
			}
		}
		for _, tc := range delta.ToolCalls {
			for len(accs) <= tc.Index {
				accs = append(accs, &toolCallAcc{})
			}
			acc := accs[tc.Index]
			if tc.ID != "" {
				acc.id.WriteString(tc.ID)
			}
			if tc.Function.Name != "" {
				acc.name.WriteString(tc.Function.Name)
			}
			acc.args.WriteString(tc.Function.Arguments)
		}
```

5. 末尾组装与空判定（替换原 `if out.Len() == 0 {...} return out.String(), nil`）：

```go
	msg := Message{Role: "assistant", Content: content.String()}
	for i, acc := range accs {
		if acc.name.Len() == 0 {
			continue
		}
		id := acc.id.String()
		if id == "" {
			id = fmt.Sprintf("call_%d", i) // 个别端点后续分片不重复 id
		}
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{ID: id, Name: acc.name.String(), Arguments: acc.args.String()})
	}
	if msg.Content == "" && len(msg.ToolCalls) == 0 {
		return Message{}, errors.New("AI 响应缺少内容")
	}
	return msg, nil
}
```

看门狗、空闲超时、错误分支、scanner 逻辑逐行保留不动。

- [ ] **Step 4: 跑全部 ai 包测试**

Run: `go test ./internal/ai/ -v`
Expected: 新增 2 用例 PASS，既有 5 用例（含 `TestChatStreamsSSE` 走新委托路径）全绿

- [ ] **提交点（开发者执行）**：`feat(ai): ChatWithTools 支持 tools 协议与流式 tool_calls 拼装`

---

### Task 3: `internal/agent` 循环骨架

**Files:**
- Create: `internal/agent/agent.go`
- Test: `internal/agent/agent_test.go`

- [ ] **Step 1: 写失败测试（fake client 驱动）**

```go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ai"
)

// fakeClient 按次序回放脚本化响应，记录每次调用的入参
type fakeClient struct {
	resps    []ai.Message
	calls    int
	gotTools [][]ai.Tool
	lastMsgs []ai.Message
}

func (f *fakeClient) ChatWithTools(ctx context.Context, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (ai.Message, error) {
	f.calls++
	f.gotTools = append(f.gotTools, tools)
	f.lastMsgs = msgs
	if len(f.resps) == 0 {
		return ai.Message{}, errors.New("脚本耗尽")
	}
	m := f.resps[0]
	f.resps = f.resps[1:]
	return m, nil
}

func toolCallResp(name, args string) ai.Message {
	return ai.Message{Role: "assistant", ToolCalls: []ai.ToolCall{{ID: "t1", Name: name, Arguments: args}}}
}

func stubTool(name string, fn func(args json.RawMessage) (string, error)) Tool {
	return Tool{
		Def:  ai.Tool{Name: name},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) { return fn(args) },
	}
}

func TestRunDirectAnswer(t *testing.T) {
	fc := &fakeClient{resps: []ai.Message{{Role: "assistant", Content: "最终回答"}}}
	ag := &Agent{Client: fc}
	final, all, err := ag.Run(context.Background(), []ai.Message{{Role: "user", Content: "q"}}, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if final.Content != "最终回答" || len(all) != 2 {
		t.Fatalf("final=%+v len(all)=%d", final, len(all))
	}
	if fc.calls != 1 {
		t.Fatalf("calls=%d", fc.calls)
	}
}

func TestRunLoopWithTools(t *testing.T) {
	fc := &fakeClient{resps: []ai.Message{
		toolCallResp("probe", `{"x":1}`),
		{Role: "assistant", Content: "完成"},
	}}
	var executed []string
	ag := &Agent{Client: fc, Tools: []Tool{stubTool("probe", func(args json.RawMessage) (string, error) {
		executed = append(executed, string(args))
		return "工具结果", nil
	})}}
	var steps []string
	hooks := Hooks{OnToolCall: func(name, args string) { steps = append(steps, name+" "+args) }}
	final, all, err := ag.Run(context.Background(), []ai.Message{{Role: "user", Content: "q"}}, hooks)
	if err != nil {
		t.Fatal(err)
	}
	if final.Content != "完成" {
		t.Fatalf("final=%+v", final)
	}
	if len(executed) != 1 || executed[0] != `{"x":1}` {
		t.Fatalf("executed=%v", executed)
	}
	if len(steps) != 1 || steps[0] != `probe {"x":1}` {
		t.Fatalf("steps=%v", steps)
	}
	// all 应为 user + assistant(tool_calls) + tool + assistant(content)
	if len(all) != 4 || all[2].Role != "tool" || all[2].ToolCallID != "t1" || all[2].Content != "工具结果" {
		t.Fatalf("all = %+v", all)
	}
}

func TestRunUnknownTool(t *testing.T) {
	fc := &fakeClient{resps: []ai.Message{
		toolCallResp("nope", `{}`),
		{Role: "assistant", Content: "ok"},
	}}
	ag := &Agent{Client: fc}
	_, all, err := ag.Run(context.Background(), []ai.Message{{Role: "user", Content: "q"}}, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(all[2].Content, "未知工具 nope") {
		t.Fatalf("tool 结果 = %q", all[2].Content)
	}
}

func TestRunToolErrorContinues(t *testing.T) {
	fc := &fakeClient{resps: []ai.Message{
		toolCallResp("bad", `{}`),
		{Role: "assistant", Content: "已绕过"},
	}}
	ag := &Agent{Client: fc, Tools: []Tool{stubTool("bad", func(args json.RawMessage) (string, error) {
		return "", fmt.Errorf("磁盘炸了")
	})}}
	_, all, err := ag.Run(context.Background(), []ai.Message{{Role: "user", Content: "q"}}, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(all[2].Content, "工具执行失败: 磁盘炸了") {
		t.Fatalf("tool 结果 = %q", all[2].Content)
	}
	if fc.calls != 2 {
		t.Fatalf("工具失败应继续循环，calls=%d", fc.calls)
	}
}

func TestRunMaxStepsForceSummary(t *testing.T) {
	fc := &fakeClient{}
	for i := 0; i < 12; i++ {
		fc.resps = append(fc.resps, toolCallResp("probe", `{}`))
	}
	fc.resps = append(fc.resps, ai.Message{Role: "assistant", Content: "总结"})
	ag := &Agent{Client: fc, Tools: []Tool{stubTool("probe", func(args json.RawMessage) (string, error) {
		return "结果", nil
	})}}
	final, _, err := ag.Run(context.Background(), []ai.Message{{Role: "user", Content: "q"}}, Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	if final.Content != "总结" {
		t.Fatalf("final=%+v", final)
	}
	// 最后一次调用 tools 应为 nil（强制总结不允许再调工具），且最后一条消息是强制总结提示
	if fc.gotTools[len(fc.gotTools)-1] != nil {
		t.Fatalf("总结调用不应携带 tools")
	}
	last := fc.lastMsgs[len(fc.lastMsgs)-1]
	if last.Role != "user" || !strings.Contains(last.Content, "步数上限") {
		t.Fatalf("强制总结消息 = %+v", last)
	}
}
```

注意：测试里 Hooks 是 `Run` 的第三参（实现中 `Agent` 结构体不含 Hooks 字段，见 Step 3）。

- [ ] **Step 2: 跑测试确认编译失败**

Run: `go test ./internal/agent/ -v`
Expected: 编译错误（包不存在 / `undefined: Agent`）

- [ ] **Step 3: 实现 agent.go**

```go
// Package agent 实现 gs AI 模式的工具调用循环：调模型 → 执行工具 → 回填结果，
// 直到模型给出最终回答或达到步数上限。本包不含 UI，展示与审批由调用方经 Hooks/构造参数注入。
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bugzi/gs/internal/ai"
)

// ChatClient Agent 依赖的模型客户端最小接口；*ai.Client 天然满足
type ChatClient interface {
	ChatWithTools(ctx context.Context, msgs []ai.Message, tools []ai.Tool, onDelta func(string)) (ai.Message, error)
}

type Tool struct {
	Def  ai.Tool // name / description / JSON Schema
	Exec func(ctx context.Context, args json.RawMessage) (string, error)
}

type Hooks struct {
	OnToolCall func(name, argsJSON string) // 每次工具执行前回调（步骤行展示）
	OnDelta    func(delta string)          // 透传模型 content 增量（流式展示）
}

type Agent struct {
	Client   ChatClient
	Tools    []Tool
	MaxSteps int // ≤0 时取默认 12
}

const forceSummaryMsg = "已达到单轮工具调用步数上限，请基于以上已获取的信息直接给出最终回答，不要再调用工具。"

// Run 执行一轮 Agent 循环。返回最终 assistant 消息与本轮产生的全部消息
// （含 assistant/tool 中间消息），历史取舍由调用方决定。传入 msgs 不会被修改。
func (a *Agent) Run(ctx context.Context, msgs []ai.Message, hooks Hooks) (ai.Message, []ai.Message, error) {
	maxSteps := a.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 12
	}
	toolDefs := make([]ai.Tool, len(a.Tools))
	byName := make(map[string]Tool, len(a.Tools))
	for i, t := range a.Tools {
		toolDefs[i] = t.Def
		byName[t.Def.Name] = t
	}
	for step := 0; step < maxSteps; step++ {
		m, err := a.Client.ChatWithTools(ctx, msgs, toolDefs, hooks.OnDelta)
		if err != nil {
			return ai.Message{}, nil, err
		}
		msgs = append(msgs, m)
		if len(m.ToolCalls) == 0 {
			return m, msgs, nil
		}
		for _, tc := range m.ToolCalls {
			result := execTool(ctx, byName, tc, hooks)
			msgs = append(msgs, ai.Message{Role: "tool", Content: result, ToolCallID: tc.ID})
		}
	}
	// 步数兜底：不带工具强制总结
	msgs = append(msgs, ai.Message{Role: "user", Content: forceSummaryMsg})
	m, err := a.Client.ChatWithTools(ctx, msgs, nil, hooks.OnDelta)
	if err != nil {
		return ai.Message{}, nil, err
	}
	msgs = append(msgs, m)
	return m, msgs, nil
}

// execTool 执行单个工具调用；失败不中断循环，错误文本作为 tool 结果交模型自纠
func execTool(ctx context.Context, byName map[string]Tool, tc ai.ToolCall, hooks Hooks) string {
	t, ok := byName[tc.Name]
	if !ok {
		return "未知工具 " + tc.Name
	}
	if hooks.OnToolCall != nil {
		hooks.OnToolCall(tc.Name, tc.Arguments)
	}
	out, err := t.Exec(ctx, json.RawMessage(tc.Arguments))
	if err != nil {
		return "工具执行失败: " + err.Error()
	}
	if out == "" {
		out = "（无输出）"
	}
	return out
}
```

- [ ] **Step 4: 跑测试**

Run: `go test ./internal/agent/ -v`
Expected: 5 用例全 PASS

- [ ] **提交点（开发者执行）**：`feat(agent): 工具调用循环骨架（步数兜底/错误自纠）`

---

### Task 4: `internal/agent` 四个工具与路径安全

**Files:**
- Create: `internal/agent/tools.go`
- Test: `internal/agent/tools_test.go`

- [ ] **Step 1: 写失败测试**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, tl Tool, args string) string {
	t.Helper()
	out, err := tl.Exec(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Exec(%s): %v", args, err)
	}
	return out
}

func TestResolveIn(t *testing.T) {
	root := t.TempDir()
	p, err := resolveIn(root, "Study/sea/a.md")
	if err != nil || p != filepath.Join(root, "Study", "sea", "a.md") {
		t.Fatalf("p=%s err=%v", p, err)
	}
	for _, bad := range []string{"../evil.md", "/etc/passwd", "C:\\evil.md", ".", " a/../.."} {
		if _, err := resolveIn(root, bad); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}
}

func TestSearchTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Study", "sea"), 0o755)
	os.MkdirAll(filepath.Join(root, "Resource"), 0o755)
	os.WriteFile(filepath.Join(root, "Study", "sea", "a.md"), []byte("# channel\nchannel 缓冲要点\n"), 0o644)
	os.WriteFile(filepath.Join(root, "Resource", "skip.md"), []byte("channel 在这里\n"), 0o644)
	out := run(t, SearchTool(root, []string{"Resource"}, 512, 20, 5), `{"query":"channel"}`)
	if !strings.Contains(out, "Study/sea/a.md") || !strings.Contains(out, "命中 1 个文件") {
		t.Fatalf("out=%s", out)
	}
	if run(t, SearchTool(root, nil, 512, 20, 5), `{"query":"不存在词xyz"}`) != "无命中" {
		t.Fatal("应报无命中")
	}
}

func TestReadFileTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Study"), 0o755)
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("第%d行", i))
	}
	os.WriteFile(filepath.Join(root, "Study", "a.md"), []byte(strings.Join(lines, "\n")), 0o644)
	tl := ReadFileTool(root, 4)
	out := run(t, tl, `{"path":"Study/a.md"}`)
	if !strings.HasPrefix(out, "1\t第1行") || !strings.Contains(out, "offset=5") {
		t.Fatalf("out=%s", out)
	}
	out = run(t, tl, `{"path":"Study/a.md","offset":5,"limit":3}`)
	if !strings.Contains(out, "5\t第5行") || !strings.Contains(out, "7\t第7行") || strings.Contains(out, "8\t") {
		t.Fatalf("out=%s", out)
	}
	if out = run(t, tl, `{"path":"Study/a.md","offset":99}`); !strings.Contains(out, "越界") {
		t.Fatalf("out=%s", out)
	}
}

func TestListDirTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Study"), 0o755)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("hi"), 0o644)
	out := run(t, ListDirTool(root), `{}`)
	if !strings.Contains(out, "Study/") || !strings.Contains(out, "README.md") {
		t.Fatalf("out=%s", out)
	}
}

func TestWriteFileTool(t *testing.T) {
	root := t.TempDir()
	written := ""
	tl := WriteFileTool(root, func(abs, content string) bool {
		written = abs + "|" + content
		return true
	})
	out := run(t, tl, `{"path":"Study/new.md","content":"# 新内容"}`)
	if out != "已写入 Study/new.md" {
		t.Fatalf("out=%s", out)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "new.md"))
	if string(data) != "# 新内容" || !strings.HasSuffix(written, "|# 新内容") {
		t.Fatalf("落盘=%q written=%q", data, written)
	}
	// 拒绝：不落盘且返回拒绝文本
	tl2 := WriteFileTool(root, func(abs, content string) bool { return false })
	out = run(t, tl2, `{"path":"Study/new.md","content":"改"}`)
	if !strings.Contains(out, "用户拒绝写入") || !strings.Contains(out, "改为文字建议") {
		t.Fatalf("out=%s", out)
	}
	data, _ = os.ReadFile(filepath.Join(root, "Study", "new.md"))
	if string(data) != "# 新内容" {
		t.Fatal("拒绝后不应改盘")
	}
	// 扩展名白名单 / 越界拒绝走 ai.SafePath
	if _, err := tl.Exec(context.Background(), json.RawMessage(`{"path":"a.exe","content":"x"}`)); err == nil {
		t.Fatal(".exe 应被拒绝")
	}
	if _, err := tl.Exec(context.Background(), json.RawMessage(`{"path":"../evil.md","content":"x"}`)); err == nil {
		t.Fatal("越界路径应被拒绝")
	}
}
```

- [ ] **Step 2: 跑测试确认编译失败**

Run: `go test ./internal/agent/ -run TestResolveIn -v`
Expected: `undefined: resolveIn`

- [ ] **Step 3: 实现 tools.go**

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/search"
	"github.com/bugzi/gs/internal/workspace"
)

// resolveIn 校验 rel 为 root 内相对路径（拒绝绝对路径与 .. 穿越），返回绝对路径
func resolveIn(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || clean == "." {
		return "", fmt.Errorf("路径必须是工作区相对路径: %s", rel)
	}
	return filepath.Join(root, clean), nil
}

const searchParams = `{"type":"object","properties":{"query":{"type":"string","description":"检索词，多个词为 AND 关系"}},"required":["query"]}`

// SearchTool 检索工具：包装 search.SearchFiltered，输出截断为 maxFiles 个文件、每文件 maxHits 条命中
func SearchTool(root string, exclude []string, maxFileKB int64, maxFiles, maxHits int) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "search_workspace",
			Description: "在整个学习工作区内检索 .md 与 .go 文件（多个词为 AND 关系），返回命中文件、小节与命中行。",
			Parameters:  json.RawMessage(searchParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			results, _, err := search.SearchFiltered(root, exclude, maxFileKB, in.Query)
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "无命中", nil
			}
			total := len(results)
			if total > maxFiles {
				results = results[:maxFiles]
			}
			var b strings.Builder
			for _, r := range results {
				fmt.Fprintf(&b, "【%s】\n", r.Path)
				hits := r.Hits
				if len(hits) > maxHits {
					hits = hits[:maxHits]
				}
				for _, h := range hits {
					fmt.Fprintf(&b, "  L%d（%s）\n", h.Line, h.Unit)
					for _, c := range h.Context {
						fmt.Fprintf(&b, "    %s\n", strings.TrimSpace(c))
					}
				}
			}
			fmt.Fprintf(&b, "\n（命中 %d 个文件）", total)
			return strings.TrimSpace(b.String()), nil
		},
	}
}

const readFileParams = `{"type":"object","properties":{"path":{"type":"string","description":"工作区相对路径（正斜杠）"},"offset":{"type":"integer","description":"起始行号（1 起），默认 1"},"limit":{"type":"integer","description":"读取行数"}},"required":["path"]}`

// ReadFileTool 读文件工具：行号标注，默认前 defaultLines 行，offset/limit 分段续读
func ReadFileTool(root string, defaultLines int) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "read_file",
			Description: "读取工作区内一个文本文件，带行号；大文件用 offset/limit 分段读取。",
			Parameters:  json.RawMessage(readFileParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			p, err := resolveIn(root, in.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			if in.Offset < 1 {
				in.Offset = 1
			}
			if in.Limit <= 0 {
				in.Limit = defaultLines
			}
			if in.Offset > len(lines) {
				return fmt.Sprintf("%s 共 %d 行，offset=%d 越界", in.Path, len(lines), in.Offset), nil
			}
			hi := in.Offset - 1 + in.Limit
			if hi > len(lines) {
				hi = len(lines)
			}
			var b strings.Builder
			for i := in.Offset - 1; i < hi; i++ {
				fmt.Fprintf(&b, "%d\t%s\n", i+1, lines[i])
			}
			if hi < len(lines) {
				fmt.Fprintf(&b, "…（共 %d 行，已显示 %d-%d，可用 offset=%d 续读）", len(lines), in.Offset, hi, hi+1)
			}
			return b.String(), nil
		},
	}
}

const listDirParams = `{"type":"object","properties":{"path":{"type":"string","description":"工作区相对路径，默认根目录"}}}`

// ListDirTool 列目录工具：单层，目录带 / 后缀，文件带大小
func ListDirTool(root string) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "list_dir",
			Description: "列出工作区内某个目录的一层内容（不递归）。",
			Parameters:  json.RawMessage(listDirParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			p := root
			if strings.TrimSpace(in.Path) != "" {
				var err error
				if p, err = resolveIn(root, in.Path); err != nil {
					return "", err
				}
			}
			entries, err := os.ReadDir(p)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for i, e := range entries {
				if i == 200 {
					fmt.Fprintf(&b, "…（还有 %d 项未列出）\n", len(entries)-200)
					break
				}
				if e.IsDir() {
					fmt.Fprintf(&b, "%s/\n", e.Name())
					continue
				}
				size := ""
				if info, err := e.Info(); err == nil {
					size = fmt.Sprintf("（%d B）", info.Size())
				}
				fmt.Fprintf(&b, "%s%s\n", e.Name(), size)
			}
			return strings.TrimRight(b.String(), "\n"), nil
		},
	}
}

const writeFileParams = `{"type":"object","properties":{"path":{"type":"string","description":"工作区相对路径（正斜杠），仅允许 .md/.go/.toml"},"content":{"type":"string","description":"完整的新文件内容"}},"required":["path","content"]}`

// WriteFileTool 写文件工具：经 approve 审批后原子落盘；路径与扩展名校验复用 ai.SafePath
func WriteFileTool(root string, approve func(absPath, content string) bool) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "write_file",
			Description: "写入工作区文件（覆盖或新建）。content 必须是完整新内容；调用后用户会看到确认请求，拒绝后不要再尝试写该文件。",
			Parameters:  json.RawMessage(writeFileParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			p, err := ai.SafePath(root, in.Path)
			if err != nil {
				return "", err
			}
			if approve != nil && !approve(p, in.Content) {
				return "用户拒绝写入 " + in.Path + "，不要再尝试写入该文件，改为文字建议。", nil
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			if err := workspace.WriteFileAtomic(p, []byte(strings.ReplaceAll(in.Content, "\r\n", "\n"))); err != nil {
				return "", err
			}
			return "已写入 " + in.Path, nil
		},
	}
}
```

注意：`ai.SafePath` 保留在 `internal/ai/actions.go`（`/review`、`/project` 仍在用，勿删）。

- [ ] **Step 4: 跑测试**

Run: `go test ./internal/agent/ -v`
Expected: Task 3 的 5 用例 + 本任务 5 用例全 PASS

- [ ] **提交点（开发者执行）**：`feat(agent): search/read/list/write 四工具与路径安全`

---

### Task 5: `command/ai.go` 改造（Agent 壳 + 审批 UI + 新系统提示）

**Files:**
- Modify: `internal/command/ai.go:27-39`（系统提示）、`:98-123`（AITurn，替换为 Agent 版；applyActions/applyOne/truncateLines 保留供 `/review`、`/project` 使用）
- Create: `internal/command/ai_agent_test.go`

- [ ] **Step 1: 写失败测试**

```go
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
	return &AIState{Client: c}, &workspace.Workspace{Root: t.TempDir()}, cfg
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
	if !strings.Contains(out.String(), "直接回答") {
		t.Fatalf("应流式输出回答: %s", out.String())
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
	if !strings.Contains(out.String(), "文件内容是 hello") {
		t.Fatalf("应流式输出最终回答: %s", out.String())
	}
	if len(st.Hist) != 2 {
		t.Fatalf("Hist 应只存两条，不含工具中间消息: %+v", st.Hist)
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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run 'TestAITurn|TestCompactArgs' -v`
Expected: 编译错误 `undefined: compactArgs`（AITurn 已存在但行为不同：直答用例可能因旧 JSON 动作路径而失败）

- [ ] **Step 3: 实现**

`internal/command/ai.go` 改动：

(a) imports 增加 `"github.com/bugzi/gs/internal/agent"`（其余不变）。

(b) `aiSystemTpl` 整体替换：

```go
const aiSystemTpl = `你是 gs 学习工作台的 AI 助手，帮助用户整理 Go 学习笔记、维护学习计划、解答 Go 问题。

工作区根目录：%s
当前断点：
%s
阶段清单：
%s

目录结构：
- Study/logs/ 每日学习日志（YYMMDD.md）
- Study/sea/ 输入层：收集的资料与笔记
- Study/river/ 加工层：自己改写的理解
- Study/spring/ 输出层：产出与分享
- Study/plan/ 学习状态与路线图

可用工具：
- search_workspace：按关键词检索工作区 .md/.go 文件（多词 AND）
- read_file：按行读取工作区内文件（path 为工作区相对路径，正斜杠）
- list_dir：列出目录内容
- write_file：写入文件（content 为完整新内容；调用后用户会看到确认请求）

行为准则：
1. 涉及学习资料、笔记内容、学习进度的问题，先用工具检索和阅读再回答；不确定的内容不臆造。
2. 只在用户明确要求改动文件时调用 write_file，并在调用前用一句话说明要写什么。
3. 回答使用中文，简洁教学式。`
```

(c) `AITurn` 整体替换（`applyActions`、`applyOne`、`truncateLines`、`expandAtFile`、`ensureAIState`、`ensureSystem` 均保留不动）：

```go
// AITurn 一轮 Agent 对话：@文件 展开 → Agent 循环（流式展示/步骤行/写审批）→ 历史只存 user+最终回答
func AITurn(ctx *Ctx, st *AIState, input string) error {
	st.ensureSystem(ctx.WS)
	user := expandAtFile(ctx.WS, input)
	msgs := append([]ai.Message{{Role: "system", Content: st.System}}, st.Hist...)
	msgs = append(msgs, ai.Message{Role: "user", Content: user})

	fmt.Fprintln(ctx.Out, ui.Dim("（AI 处理中，可自主检索工作区…）"))
	approve := func(absPath, content string) bool { return approveWrite(ctx, absPath, content) }
	ag := &agent.Agent{
		Client: st.Client,
		Tools: []agent.Tool{
			agent.SearchTool(ctx.WS.Root, ctx.Cfg.Scan.Exclude, ctx.Cfg.Scan.MaxFileKB, 20, 5),
			agent.ReadFileTool(ctx.WS.Root, 400),
			agent.ListDirTool(ctx.WS.Root),
			agent.WriteFileTool(ctx.WS.Root, approve),
		},
	}
	final, _, err := ag.Run(context.Background(), msgs, agent.Hooks{
		OnToolCall: func(name, argsJSON string) {
			fmt.Fprintf(ctx.Out, "%s\n", ui.Dim("· "+name+" "+compactArgs(argsJSON)))
		},
		OnDelta: func(delta string) { fmt.Fprint(ctx.Out, delta) },
	})
	if err != nil {
		return err
	}
	if final.Content != "" {
		fmt.Fprintln(ctx.Out)
	}
	st.Hist = append(st.Hist,
		ai.Message{Role: "user", Content: user},
		ai.Message{Role: "assistant", Content: final.Content})
	if len(st.Hist) > 16 {
		st.Hist = st.Hist[len(st.Hist)-16:]
	}
	return nil
}

// approveWrite 写文件审批：展示 diff/新建预览后询问；取消（Esc 等）视为拒绝
func approveWrite(ctx *Ctx, absPath, content string) bool {
	rel := ctx.WS.Rel(absPath)
	current, err := os.ReadFile(absPath)
	switch {
	case err == nil:
		fmt.Fprintf(ctx.Out, "\n%s\n%s\n", ui.Path(rel), truncateLines(diffx.Render(string(current), content), 40))
	case os.IsNotExist(err):
		fmt.Fprintf(ctx.Out, "将创建 %s：\n%s\n", ui.Path(rel), truncateLines(strings.ReplaceAll(content, "\r\n", "\n"), 15))
	default:
		fmt.Fprintf(ctx.Out, "%s（读取现状失败：%s）\n", ui.Path(rel), err)
	}
	choice := "应用"
	opts := []menuOption{option("应用", "应用"), option("跳过", "跳过")}
	if err := selectFrom(ctx, "AI 请求写入 "+rel, opts, &choice); err != nil {
		return false
	}
	if choice == "应用" {
		fmt.Fprintf(ctx.Out, "%s\n", ui.Dim("已确认写入 "+rel))
	}
	return choice == "应用"
}

// compactArgs 步骤行里的参数展示：去首尾空白，过长截断
func compactArgs(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 100 {
		s = s[:100] + "…"
	}
	return s
}
```

注意：`st.Client` 类型是 `*ai.Client`，作为 `agent.ChatClient` 接口值直接传入即可（方法集满足）。

- [ ] **Step 4: 跑测试**

Run: `go test ./internal/command/ -v`
Expected: 新增 3 用例 PASS；既有 command 测试全绿（`/review`、`/project`、aisave 路径未动）

- [ ] **提交点（开发者执行）**：`feat(ai): /AI 升级为 Agent 轮（工具步骤行/流式回答/写审批）`

---

### Task 6: 全量验证与手动验收

**Files:** 无新增（验证 + 交付）

- [ ] **Step 1: 全量构建与测试**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全部通过，0 失败

- [ ] **Step 2: 交叉检查遗留引用**

Run: `grep -rn "ai.SafePath\|ai.SplitReply\|applyActions" internal/ --include="*.go" | grep -v _test.go`
Expected: `actions.go` 定义、`review.go:111`/`project.go:576` 调用、`ai.go` 中 `applyActions` 定义与 `review.go:116` 调用——不得出现 `AITurn` 内调用 `SplitReply` 的残留。

- [ ] **Step 3: 手动验收（由开发者在真实终端执行）**

1. `go run ./cmd/gs` → `/AI` 进入子模式
2. 输入「我最近三天在学什么？从日志里找证据」→ 应看到暗色步骤行（`· search_workspace …`、`· read_file …`）后给出基于日志的回答
3. 输入「什么是 goroutine 泄漏」（纯知识问答）→ 无工具行，直接流式回答
4. 输入「帮我在 Study/spring 建一个 test.md 内容写测试」→ 应出现写入确认菜单（应用/跳过）；选「跳过」→ AI 不落盘并转为文字建议；再试选「应用」→ 文件生成
5. Esc 取消写入确认 → 视为拒绝，不落盘
6. `/back` → 存档询问照常

- [ ] **Step 4: 开发日志与归档（AI 执行）**

1. `date +"%Y-%m-%d %H:%M"` 取实际时间，在 `docs/log/<YYMMDD>.md` 追加本轮日志（每小节标题下附 `> 创建于 …` 时间行）
2. 提醒开发者提交后，把 `docs/project/新功能开发区.md` 本轮条目按轮归档（若本轮在该区有登记）
3. spec 与 plan 落地后按 README.md 机制移入 `docs/project/全局/archive/`

---

## 自审记录（已执行）

- **Spec 覆盖**：tools 协议（Task 2）、循环/步数兜底/错误自纠（Task 3）、四工具+路径安全+审批（Task 4）、壳/系统提示/Hist/流式/步骤行（Task 5）、测试三层（Task 2/3/4/5 各自用例）——spec 全节有着落；`/review`、aisave 零改动（Task 5 保留 applyActions/applyOne，Step 4 既有测试护航）。
- **占位符扫描**：无 TBD/「适当处理」；所有代码步骤给出完整代码。
- **类型一致性**：`ChatWithTools(ctx, msgs, tools, onDelta) (ai.Message, error)` 在 ai/agent/command 三层一致；`agent.Tool{Def, Exec}`、`WriteFileTool(root, approve func(absPath, content string) bool)`、`Hooks{OnToolCall, OnDelta}` 与 spec 一致。
