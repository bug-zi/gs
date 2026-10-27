package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bugzi/gs/internal/agent"
	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/edit/diffx"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// AIState 一个 REPL 会话内共享的 AI 上下文
type AIState struct {
	Client *ai.Client
	Hist   []ai.Message
	System string
	Saved  bool // 会话已存档过，避免重复询问
}

var atFileRe = regexp.MustCompile(`@([^\s]+)`)

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

// truncateLines 超过 n 行时截断并提示
func truncateLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n…（截断）"
}

// ensureAIState 惰性创建并注入 ctx.AI；未配置时返回友好错误
func ensureAIState(ctx *Ctx) (*AIState, error) {
	if ctx.AI != nil {
		return ctx.AI, nil
	}
	c, err := ai.NewClient(ctx.Cfg)
	if err != nil {
		return nil, err
	}
	st := &AIState{Client: c}
	ctx.AI = st
	return st, nil
}

// ensureSystem 补全系统提示（需要 ws 信息，首次对话时构建）
func (st *AIState) ensureSystem(ws *workspace.Workspace) {
	if st.System != "" {
		return
	}
	data, _ := os.ReadFile(ws.StateFile())
	state := ParseState(string(data))
	var bp, ph strings.Builder
	for _, kv := range state.Breakpoints {
		fmt.Fprintf(&bp, "- %s：%s\n", kv.Key, kv.Value)
	}
	for i, p := range state.Phases {
		fmt.Fprintf(&ph, "%d. %s\n", i+1, p)
	}
	st.System = fmt.Sprintf(aiSystemTpl, ws.Root, bp.String(), ph.String())
}

// expandAtFile 把 @路径 替换为文件内容引用
func expandAtFile(ws *workspace.Workspace, input string) string {
	return atFileRe.ReplaceAllStringFunc(input, func(m string) string {
		rel := strings.TrimPrefix(m, "@")
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		data, err := os.ReadFile(p)
		if err != nil {
			return m + "（文件读取失败）"
		}
		content := string(data)
		if len(content) > 8000 {
			content = content[:8000] + "\n…（截断）"
		}
		return fmt.Sprintf("\n--- 文件 %s ---\n%s\n--- 引用结束 ---\n", rel, content)
	})
}

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

// applyActions 逐条 diff 预览并确认写入
func applyActions(ctx *Ctx, actions []ai.Action) error {
	fmt.Fprintf(ctx.Out, ui.Dim("AI 提出了 %d 个文件修改\n"), len(actions))
	applyAll := false
	for i, a := range actions {
		path, err := ai.SafePath(ctx.WS.Root, a.File)
		if err != nil {
			fmt.Fprintf(ctx.Out, "%s\n", ui.Err("跳过越界文件 "+a.File))
			continue
		}
		choice := "应用"
		if !applyAll {
			opts := []menuOption{
				option("应用", "应用"),
				option("跳过", "跳过"),
				option("应用剩余全部", "全部"),
				option("放弃剩余", "放弃"),
			}
			label := fmt.Sprintf("[%d/%d] %s（%s）", i+1, len(actions), a.File, a.Action)
			if err := selectFrom(ctx, label, opts, &choice); err != nil {
				return err
			}
		}
		switch choice {
		case "跳过":
			continue
		case "放弃":
			fmt.Fprintln(ctx.Out, "已放弃剩余修改")
			return nil
		case "全部":
			applyAll = true
		}
		if err := applyOne(ctx, path, a); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx *Ctx, path string, a ai.Action) error {
	current, err := os.ReadFile(path)
	switch {
	case a.Action == "create":
		if err == nil {
			return fmt.Errorf("%s 已存在，不能 create", ctx.WS.Rel(path))
		}
		fmt.Fprintf(ctx.Out, "将创建 %s：\n%s\n", ui.Path(ctx.WS.Rel(path)),
			truncateLines(strings.ReplaceAll(a.Content, "\r\n", "\n"), 15))
	case err == nil:
		fmt.Fprintf(ctx.Out, "\n%s\n%s", ui.Path(ctx.WS.Rel(path)), diffx.Render(string(current), a.Content))
	case os.IsNotExist(err):
		return fmt.Errorf("%s 不存在，不能 edit", ctx.WS.Rel(path))
	default:
		return err
	}
	if err := workspace.WriteFileAtomic(path, []byte(a.Content)); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s\n", ui.Dim("已写入 "+ctx.WS.Rel(path)))
	return nil
}

func newAI() Command {
	return Command{
		Name:  "ai",
		Alias: []string{"AI"},
		Short: "AI 对话：/AI 问题 单发，/AI 进入子模式，@路径 引用文件",
		Run: func(ctx *Ctx) error {
			st, err := ensureAIState(ctx)
			if err != nil {
				return err
			}
			if len(ctx.Args) == 0 {
				ctx.NextMode = "ai"
				fmt.Fprintln(ctx.Out, ui.Dim("已进入 AI 模式，/back 返回，@路径 可引用文件"))
				return nil
			}
			return AITurn(ctx, st, strings.Join(ctx.Args, " "))
		},
	}
}
