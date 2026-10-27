package diffx

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type OpType int

const (
	OpEqual OpType = iota
	OpAdd
	OpDel
)

type Op struct {
	Type OpType
	Text string
}

var (
	addStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // 绿
	delStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // 红
	dimStyle = lipgloss.NewStyle().Faint(true)
)

// Ops 行级 LCS diff；规模过大时退化为“前缀+后缀+整体替换”
func Ops(a, b []string) []Op {
	if len(a)*len(b) > 1_000_000 {
		return naiveOps(a, b)
	}
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max2(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []Op
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, Op{OpEqual, a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, Op{OpDel, a[i]})
			i++
		default:
			ops = append(ops, Op{OpAdd, b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, Op{OpDel, a[i]})
	}
	for ; j < len(b); j++ {
		ops = append(ops, Op{OpAdd, b[j]})
	}
	return ops
}

func naiveOps(a, b []string) []Op {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	t := 0
	for t < len(a)-n && t < len(b)-n && a[len(a)-1-t] == b[len(b)-1-t] {
		t++
	}
	var ops []Op
	for _, l := range a[:n] {
		ops = append(ops, Op{OpEqual, l})
	}
	for _, l := range a[n : len(a)-t] {
		ops = append(ops, Op{OpDel, l})
	}
	for _, l := range b[n : len(b)-t] {
		ops = append(ops, Op{OpAdd, l})
	}
	for _, l := range a[len(a)-t:] {
		ops = append(ops, Op{OpEqual, l})
	}
	return ops
}

func max2(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Render 渲染为彩色 diff 文本；连续相同行超过 4 行压缩显示
func Render(oldContent, newContent string) string {
	ops := Ops(splitLines(oldContent), splitLines(newContent))
	var out strings.Builder
	eqStart := 0
	for idx := 0; idx <= len(ops); idx++ {
		runEnded := idx == len(ops) || ops[idx].Type != OpEqual
		if !runEnded {
			continue
		}
		run := idx - eqStart
		if run > 4 {
			fmt.Fprintf(&out, "%s\n", dimStyle.Render(fmt.Sprintf("  …（%d 行相同）", run)))
		} else {
			for k := eqStart; k < idx; k++ {
				fmt.Fprintf(&out, "%s\n", dimStyle.Render("  "+ops[k].Text))
			}
		}
		if idx < len(ops) {
			switch ops[idx].Type {
			case OpDel:
				fmt.Fprintf(&out, "%s\n", delStyle.Render("- "+ops[idx].Text))
			case OpAdd:
				fmt.Fprintf(&out, "%s\n", addStyle.Render("+ "+ops[idx].Text))
			}
		}
		eqStart = idx + 1
	}
	return out.String()
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
