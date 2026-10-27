package complete

// Cmd 补全源里的一条命令（与 command.Command 解耦的简单结构）
type Cmd struct {
	Name  string
	Alias []string
	Subs  []string
	Hint  string // 参数提示（面板顶部灰字），如 <关键词|日期>
}

// Tree 补全树
type Tree []Cmd

// BuildTree 由命令表构建补全树
func BuildTree(cmds []Cmd) Tree { return Tree(cmds) }
