package repl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/command"
	"github.com/bugzi/gs/internal/complete"
	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/seaport"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

type REPL struct {
	ws         *workspace.Workspace
	cfg        *config.Config
	cmds       []command.Command
	out        io.Writer
	mode       string             // "" 主模式;"ai" AI 子模式
	aiState    *command.AIState   // AI 会话状态
	portRes    chan seaport.Result // seaport 后台导入结果(有任务时非 nil)
	portCancel context.CancelFunc // 取消后台导入
}

func New(ws *workspace.Workspace, cfg *config.Config, out io.Writer) *REPL {
	return &REPL{ws: ws, cfg: cfg, cmds: command.Build(), out: out}
}

func (r *REPL) banner() {
	fmt.Fprintf(r.out, "gs · Go 学习工作台 · %s\n", r.ws.Root)
	data, _ := os.ReadFile(r.ws.StateFile())
	st := command.ParseState(string(data))
	for _, kv := range st.Breakpoints {
		switch kv.Key {
		case "当前优先模块":
			fmt.Fprintf(r.out, "当前断点: %s\n", kv.Value)
		case "目标":
			fmt.Fprintf(r.out, "目标: %s\n", kv.Value)
		}
	}
	if dates, err := r.ws.ListLogs(); err == nil && len(dates) > 0 {
		last := dates[0]
		today := time.Now().In(workspace.CST)
		tag := ""
		if last.Format("20060102") == today.Format("20060102") {
			tag = " (今天)"
		}
		fmt.Fprintf(r.out, "上次学习: %s%s · 连续 %d 天\n", last.Format("01-02"), tag, r.ws.Streak())
	}
	fmt.Fprintln(r.out, ui.Dim("输入 /help 查看命令；/ 即弹候选，Tab/→ 接受灰色建议，↑↓ 历史，/exit 退出"))
	if n := command.InboxCount(r.ws); n > 0 {
		fmt.Fprintf(r.out, ui.Dim("sea/inbox 有 %d 个待导入文件，/sea import 处理\n"), n)
	}
}

// Run 阻塞运行 REPL；返回时用户已退出
func (r *REPL) Run() error {
	r.banner()
	defer func() {
		if r.portCancel != nil {
			r.portCancel()
		}
	}()
	r.portCheck()
	home, _ := os.UserHomeDir()
	_ = os.MkdirAll(filepath.Join(home, ".gs"), 0o755)
	hist := LoadHistory(filepath.Join(home, ".gs", "history"))
	reader := NewReader("gs> ", complete.BuildTree(toCompleteCmds(r.cmds)), hist)
	defer reader.Close()
	for {
		if r.mode == "ai" {
			reader.SetPrompt("ai> ")
		} else {
			reader.SetPrompt("gs> ")
		}
		line, err := reader.Read()
		if errors.Is(err, ErrEscBack) {
			if r.mode == "ai" {
				r.backFromAI()
			}
			continue
		}
		if err != nil {
			fmt.Fprintln(r.out) // Ctrl-D 换行
			return nil
		}
		r.drainPort()
		if r.exec(line) {
			return nil
		}
	}
}

func (r *REPL) newCtx(args []string) *command.Ctx {
	return &command.Ctx{WS: r.ws, Cfg: r.cfg, Out: r.out, Args: args, Cmds: r.cmds, AI: r.aiState}
}

func (r *REPL) aiChat(input string) {
	if r.aiState == nil {
		fmt.Fprintln(r.out, ui.Err("AI 未初始化，请先执行 /AI"))
		return
	}
	ctx := r.newCtx(nil)
	if err := command.AITurn(ctx, r.aiState, input); err != nil {
		fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
	}
}

// backFromAI 返回主模式；有未存档会话时询问存档（Esc 与 /back 共用）
func (r *REPL) backFromAI() {
	ctx := r.newCtx(nil)
	ctx.AI = r.aiState
	if err := command.MaybeSaveAIChat(ctx); err != nil {
		fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
	}
	r.mode = ""
	fmt.Fprintln(r.out, ui.Dim("已返回主模式"))
}

// exec 返回 true 表示要退出
func (r *REPL) exec(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if !strings.HasPrefix(line, "/") {
		if r.mode == "ai" {
			r.aiChat(line)
		} else {
			fmt.Fprintln(r.out, ui.Dim("命令以 / 开头，输入 /help 查看命令列表"))
		}
		return false
	}
	fields := strings.Fields(strings.TrimPrefix(line, "/"))
	name := strings.ToLower(fields[0])
	args := fields[1:]
	if r.mode == "ai" {
		switch name {
		case "back":
			r.backFromAI()
		case "save":
			if r.aiState == nil {
				fmt.Fprintln(r.out, ui.Err("AI 未初始化，请先执行 /AI"))
				return false
			}
			path, err := command.SaveAIChat(r.ws, r.aiState)
			if err != nil {
				fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
				return false
			}
			fmt.Fprintf(r.out, "已存档 %s（/sea 可查阅）\n", path)
		case "exit", "q", "quit":
			ctx := r.newCtx(nil)
			ctx.AI = r.aiState
			_ = command.MaybeSaveAIChat(ctx)
			fmt.Fprintln(r.out, "再见，学习顺利。")
			return true
		default:
			fmt.Fprintln(r.out, ui.Dim("AI 模式：直接输入问题，/save 存档，/back 或 Esc 返回主模式"))
		}
		return false
	}
	cmd := command.Find(r.cmds, name)
	if cmd == nil {
		var names []string
		for _, c := range r.cmds {
			names = append(names, c.Name)
			names = append(names, c.Alias...)
		}
		msg := fmt.Sprintf("未知命令 /%s", name)
		if s := suggest(names, name); s != "" {
			msg += fmt.Sprintf("，是否想输入 /%s？", s)
		}
		fmt.Fprintln(r.out, ui.Err(msg))
		return false
	}
	if cmd.Name == "exit" {
		fmt.Fprintln(r.out, "再见，学习顺利。")
		return true
	}
	ctx := r.newCtx(args)
	if err := cmd.Run(ctx); err != nil {
		if errors.Is(err, command.ErrCancelled) {
			return false // 用户取消选择（Esc 等），静默返回提示符
		}
		fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
	}
	if ctx.AI != nil {
		r.aiState = ctx.AI
	}
	if ctx.NextMode != "" {
		r.mode = ctx.NextMode
	}
	return false
}

// toCompleteCmds 命令表 → 补全树（附参数提示表）
func toCompleteCmds(cmds []command.Command) []complete.Cmd {
	out := make([]complete.Cmd, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, complete.Cmd{Name: c.Name, Alias: c.Alias, Subs: c.Subs, Hint: hintFor(c.Name)})
	}
	return out
}

// portCheck 启动时检查海港:有待导入文件则后台分类导入,不阻塞进 REPL
func (r *REPL) portCheck() {
	_ = seaport.Ensure(r.ws)
	files, skipped := seaport.Scan(r.ws)
	if len(skipped) > 0 {
		fmt.Fprintf(r.out, ui.Dim("seaport 有 %d 个非文本文件未处理(仅支持 .md/.txt)\n"), len(skipped))
	}
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(r.out, ui.Dim("seaport 有 %d 个文件,后台导入中…\n"), len(files))
	var cc seaport.ChatClient
	if c, err := ai.NewClient(r.cfg); err == nil {
		c.Temperature = 0 // 分类求稳定,不跟随会话温度
		cc = c
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.portCancel = cancel
	r.portRes = make(chan seaport.Result, 1)
	go func() {
		r.portRes <- seaport.Import(ctx, r.ws, cc)
	}()
}

// drainPort 非阻塞收割后台导入结果;有结果则在提示符前打印
func (r *REPL) drainPort() {
	if r.portRes == nil {
		return
	}
	select {
	case res := <-r.portRes:
		seaport.Print(r.out, res)
	default:
	}
}

// hintFor 各命令的参数提示（面板顶部灰字）
func hintFor(name string) string {
	switch name {
	case "logs":
		return "<关键词|日期>"
	case "sea", "river", "spring", "search":
		return "<关键词>"
	case "show":
		return "<主题|edit>"
	case "project":
		return "<名称|edit>"
	}
	return ""
}
