package edit

import rw "github.com/mattn/go-runewidth"

// Segment 是一行按显示宽度切出的显示片段
type Segment struct {
	Runes   []rune
	Wrapped bool // true = 软换行延续片段(渲染加缩进标记)
}

// WrapRunes 把 line 按显示宽度 width 切成显示片段。
// 纯字符切分(不做断词),保证:每片段显示宽 ≤ width;
// 空行返回单个未换行片段;整行恰填满 width 时末尾补空片段给光标落脚。
func WrapRunes(line []rune, width int) []Segment {
	if width < 1 {
		width = 1
	}
	var segs []Segment
	w, start := 0, 0
	for i, r := range line {
		cw := rw.RuneWidth(r)
		if w+cw > width {
			segs = append(segs, Segment{Runes: line[start:i]})
			start = i
			w = 0
		}
		w += cw
	}
	segs = append(segs, Segment{Runes: line[start:]})
	if w == width {
		segs = append(segs, Segment{})
	}
	for i := range segs {
		segs[i].Wrapped = i > 0
	}
	return segs
}

// VRow 描述一个显示行:逻辑行 Line 的 rune 区间 [Start,End);
// Start>0 即软换行延续片段(渲染加缩进)
type VRow struct {
	Line  int
	Start int
	End   int
}

// Layout 计算整个缓冲在 width 下的显示行布局
func Layout(b *Buffer, width int) []VRow {
	var rows []VRow
	for i := 0; i < b.LineCount(); i++ {
		lr := b.LineRunes(i)
		off := 0
		for _, s := range WrapRunes(lr, width) {
			rows = append(rows, VRow{Line: i, Start: off, End: off + len(s.Runes)})
			off += len(s.Runes)
		}
	}
	return rows
}

// CursorVRow 光标所在显示行序号(全文档显示行坐标系)
func CursorVRow(b *Buffer, width int) int {
	rows := Layout(b, width)
	line, col := b.Cur()
	first, last, hit := -1, -1, -1
	for i, r := range rows {
		if r.Line != line {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
		if col >= r.Start && col < r.End {
			hit = i
		}
	}
	if hit >= 0 {
		return hit
	}
	if last >= 0 {
		return last // col 在行尾/空行/幻影行 → 该逻辑行最后一个显示行
	}
	return 0
}

// DispCol rune 下标 → 行内显示列
func DispCol(line []rune, idx int) int {
	return rw.StringWidth(string(line[:idx]))
}

// idxAtDispCol 行内显示列 → rune 下标(不超过该列的最大下标)
func idxAtDispCol(line []rune, dcol int) int {
	w := 0
	for i, r := range line {
		cw := rw.RuneWidth(r)
		if w+cw > dcol {
			return i
		}
		w += cw
	}
	return len(line)
}
