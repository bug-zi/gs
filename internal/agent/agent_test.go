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
		t.Fatal("总结调用不应携带 tools")
	}
	last := fc.lastMsgs[len(fc.lastMsgs)-1]
	if last.Role != "user" || !strings.Contains(last.Content, "步数上限") {
		t.Fatalf("强制总结消息 = %+v", last)
	}
}
