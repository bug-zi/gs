package command

import (
	"io"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

// Ctx 每个命令执行时拿到的一切
type Ctx struct {
	WS   *workspace.Workspace
	Cfg  *config.Config
	Out  io.Writer
	Args []string  // 命令名之后的参数
	Cmds []Command // 由 repl 注入，供 /help 列表用

	AI       *AIState // AI 对话状态，repl 注入并在命令间共享
	NextMode string   // 命令可请求切换 REPL 模式（"ai" 进入子模式，"" 主模式）
}

type Command struct {
	Name  string
	Alias []string
	Short string
	Subs  []string // Tab 补全的二级子命令（可选项）
	Run   func(ctx *Ctx) error
}

func hasAlias(c Command, name string) bool {
	for _, a := range c.Alias {
		if a == name {
			return true
		}
	}
	return false
}

// Find 按名称或别名查找命令
func Find(cmds []Command, name string) *Command {
	for i := range cmds {
		if cmds[i].Name == name || hasAlias(cmds[i], name) {
			return &cmds[i]
		}
	}
	return nil
}
