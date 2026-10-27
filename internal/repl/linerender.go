package repl

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// buildInputLine 输入行帧：\r 起始 + 提示符 + 光标前 + 反白光标字符 + 光标后 + ghost（仅行尾时）。
// 调用方负责先输出 "\r\x1b[J" 清旧帧。
func buildInputLine(prompt string, st LineState, ghost string) string {
	line, _ := buildInputLineFit(prompt, st, ghost, 1<<30)
	return line
}

// buildInputLineFit 按可用列数 avail 裁剪输入行（readline 式水平滚动），
// 整行渲染宽度永不超限，返回渲染行与光标的显示列。
// avail 为提示符之后的文本区列数（含反白光标字符）。
func buildInputLineFit(prompt string, st LineState, ghost string, avail int) (string, int) {
	if avail < 1 {
		avail = 1
	}
	lo, hi, _ := inputWindow(st.Runes, st.Pos, avail)
	var b strings.Builder
	b.WriteString("\r")
	b.WriteString(prompt)
	before := string(st.Runes[lo:st.Pos])
	b.WriteString(before)
	b.WriteString("\x1b[7m") // 反白光标字符
	b.WriteString(st.UnderCursor())
	b.WriteString("\x1b[27m")
	b.WriteString(string(st.Runes[st.Pos:hi]))
	if ghost != "" && st.Pos == st.Len() {
		// ghost 按剩余列截断，放不下则不显示
		rest := avail - runewidth.StringWidth(before) - 1
		if rest > 0 {
			if g := truncateByWidth(ghost, rest); g != "" {
				b.WriteString("\x1b[2m") // 灰色 ghost 建议
				b.WriteString(g)
				b.WriteString("\x1b[0m")
			}
		}
	}
	return b.String(), runewidth.StringWidth(prompt) + runewidth.StringWidth(before)
}

// inputWindow 在 avail 列内选取包含光标的可见窗口 [lo,hi)，startCol 为窗口
// 起始显示列。宽字符跨界时整个移出窗口，保证不渲染半个字符。
func inputWindow(rs []rune, pos, avail int) (lo, hi, startCol int) {
	prefix := make([]int, len(rs)+1) // prefix[i] = rs[:i] 的显示宽度
	for i, r := range rs {
		prefix[i+1] = prefix[i] + runewidth.RuneWidth(r)
	}
	cursorCol := prefix[pos]
	cw := 1 // 光标处字符宽（反白渲染时占的列数）
	if pos < len(rs) {
		cw = runewidth.RuneWidth(rs[pos])
	}
	if avail < cw {
		avail = cw
	}
	// 光标贴窗口右缘时滚动：起点 = 光标列 + 光标字符宽 - avail
	startCol = cursorCol + cw - avail
	if startCol < 0 {
		startCol = 0
	}
	lo = 0
	for lo < pos && prefix[lo+1] <= startCol {
		lo++
	}
	if prefix[lo] < startCol { // 窗口起点落在宽字符中间：整个移出窗口
		lo++
		startCol = prefix[lo]
	}
	afterBudget := avail - (cursorCol - startCol) - cw
	hi = pos
	for hi < len(rs) && prefix[hi+1]-cursorCol <= afterBudget {
		hi++
	}
	return lo, hi, startCol
}

// truncateByWidth 按显示宽度截断字符串，宽字符放不下整字丢弃
func truncateByWidth(s string, max int) string {
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > max {
			break
		}
		w += rw
		b.WriteRune(r)
	}
	return b.String()
}

// buildPanelLines 候选面板行：提示在最上，高亮项带 ❯
func buildPanelLines(cands []string, hint string, sel int) []string {
	var lines []string
	if hint != "" {
		lines = append(lines, "\x1b[2m"+hint+"\x1b[0m")
	}
	for i, c := range cands {
		if i == sel {
			lines = append(lines, "\x1b[7m❯ "+c+"\x1b[27m")
		} else {
			lines = append(lines, "  \x1b[2m"+c+"\x1b[0m")
		}
	}
	return lines
}
