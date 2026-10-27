// Package agent 实现 gs AI 模式的工具调用循环：调模型 → 执行工具 → 回填结果，
// 直到模型给出最终回答或达到步数上限。本包不含 UI，展示与审批由调用方经 Hooks/构造参数注入。
package agent

import (
	"context"
	"encoding/json"

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
