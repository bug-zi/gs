package roadmap

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// ColorFn 状态行上色回调：返回包装后的行；nil 表示不上色。
// 由壳层注入（lipgloss），本包不 import ui，保持纯逻辑可独立测试。
type ColorFn func(status, line string) string

// RenderOpts 静态渲染选项（浏览器不可用时的降级通道）
type RenderOpts struct {
	Title     string  // 树标题（如「Go 学习路线图」）
	Progress  string  // 树顶进度段，空则不显示
	Next      string  // 树顶断点摘要行，空则不显示
	BranchIdx []int   // 只渲染这些主干；nil = 全部
	Color     ColorFn // nil = 不上色
}

// Glyph 状态符号：done [x] / doing [~] / 其他 [ ]
func Glyph(status string) string {
	switch status {
	case StatusDone:
		return "[x]"
	case StatusDoing:
		return "[~]"
	default:
		return "[ ]"
	}
}

// Render 静态树：主干行挂 done/total（空主干不挂）；分组标签变化时输出小标题行；
// 「高频」尾随 *；主干过滤只输出指定主干。
func Render(rm *Roadmap, o RenderOpts) string {
	var b strings.Builder
	if o.Progress != "" {
		b.WriteString(leaderLine(o.Title, o.Progress, 44) + "\n")
	} else {
		b.WriteString(o.Title + "\n")
	}
	if o.Next != "" {
		b.WriteString(o.Next + "\n")
	}
	b.WriteString(strings.Repeat("═", 44) + "\n")

	ids := o.BranchIdx
	if ids == nil {
		ids = make([]int, len(rm.Branches))
		for i := range rm.Branches {
			ids[i] = i
		}
	}
	for k, bi := range ids {
		branch := &rm.Branches[bi]
		bg, bc := "├─ ", "│  "
		if k == len(ids)-1 {
			bg, bc = "└─ ", "   "
		}
		line := bg + branch.Title
		if done, total := rm.BranchProgress(bi); total > 0 {
			line = leaderLine(line, fmt.Sprintf("%d/%d", done, total), 40)
		}
		b.WriteString(line + "\n")
		prevGroup := ""
		for pi := range branch.Points {
			p := &branch.Points[pi]
			if p.Group != "" && p.Group != prevGroup {
				b.WriteString(bc + "▸ " + p.Group + "\n")
				prevGroup = p.Group
			} else if p.Group == "" {
				prevGroup = ""
			}
			ng := "├─ "
			if pi == len(branch.Points)-1 {
				ng = "└─ "
			}
			nl := bc + "   " + ng + Glyph(p.Status) + " " + p.Title
			if hasHighFreq(p.Tags) {
				nl += " *"
			}
			if o.Color != nil {
				nl = o.Color(p.Status, nl)
			}
			b.WriteString(nl + "\n")
		}
	}
	return b.String()
}

// leaderLine 「title ···· progress」三段行，目标总显示宽 w（中点至少 3 个）
func leaderLine(title, progress string, w int) string {
	dots := w - runewidth.StringWidth(title) - runewidth.StringWidth(progress) - 2
	if dots < 3 {
		dots = 3
	}
	return title + " " + strings.Repeat("·", dots) + " " + progress
}

// hasHighFreq tags 含「高频」即视为高频
func hasHighFreq(tags []string) bool {
	for _, tg := range tags {
		if strings.Contains(tg, "高频") {
			return true
		}
	}
	return false
}
