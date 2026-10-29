package edit

import "strings"

// selPos 选区端点:逻辑行下标 + 行内 rune 下标
type selPos struct {
	line, col int
}

// normSel 归一化选区:起点≤终点(行序优先,行内列序)
func normSel(a, b selPos) (start, end selPos) {
	if a.line < b.line || (a.line == b.line && a.col <= b.col) {
		return a, b
	}
	return b, a
}

// selText 选区文本:起终行间以 \n 拼接(不含结尾换行)
func selText(lines [][]rune, start, end selPos) string {
	if start.line == end.line {
		return string(lines[start.line][start.col:end.col])
	}
	parts := []string{string(lines[start.line][start.col:])}
	for i := start.line + 1; i < end.line; i++ {
		parts = append(parts, string(lines[i]))
	}
	parts = append(parts, string(lines[end.line][:end.col]))
	return strings.Join(parts, "\n")
}

// delSel 删除选区:首尾行拼接、中间行移除;光标落选区起点
func delSel(b *Buffer, start, end selPos) {
	head := append([]rune{}, b.lines[start.line][:start.col]...)
	tail := append([]rune{}, b.lines[end.line][end.col:]...)
	b.lines[start.line] = append(head, tail...)
	n := end.line - start.line
	if n > 0 {
		copy(b.lines[start.line+1:], b.lines[start.line+1+n:])
		b.lines = b.lines[:len(b.lines)-n]
	}
	b.cur, b.col = start.line, start.col
	b.resetWant()
}
