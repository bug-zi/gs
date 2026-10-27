package edit

import (
	"regexp"
	"strconv"
	"strings"
)

// Buffer 多行文本缓冲;光标以 (逻辑行下标, 行内 rune 下标) 表示。
// want 为 ↑↓ 移动时的粘滞视觉列,-1 表示未激活。
type Buffer struct {
	lines [][]rune
	cur   int
	col   int
	want  int
}

func NewBuffer(s string) *Buffer {
	b := &Buffer{want: -1}
	for _, l := range strings.Split(s, "\n") {
		b.lines = append(b.lines, []rune(l))
	}
	return b
}

func (b *Buffer) Value() string {
	parts := make([]string, len(b.lines))
	for i, l := range b.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (b *Buffer) LineCount() int         { return len(b.lines) }
func (b *Buffer) Cur() (line, col int)   { return b.cur, b.col }
func (b *Buffer) LineRunes(i int) []rune { return b.lines[i] }

func (b *Buffer) resetWant() { b.want = -1 }

// Insert 在光标处插入字符
func (b *Buffer) Insert(rs ...rune) {
	l := append(b.lines[b.cur], make([]rune, len(rs))...)
	copy(l[b.col+len(rs):], l[b.col:])
	copy(l[b.col:], rs)
	b.lines[b.cur] = l
	b.col += len(rs)
	b.resetWant()
}

// Backspace 删除光标前一字符;行首则与上一行合并
func (b *Buffer) Backspace() {
	if b.col > 0 {
		b.lines[b.cur] = append(b.lines[b.cur][:b.col-1], b.lines[b.cur][b.col:]...)
		b.col--
		b.resetWant()
		return
	}
	if b.cur > 0 {
		prev := b.lines[b.cur-1]
		b.lines[b.cur-1] = append(prev, b.lines[b.cur]...)
		b.lines = append(b.lines[:b.cur], b.lines[b.cur+1:]...)
		b.cur--
		b.col = len(prev)
		b.resetWant()
	}
}

// Delete 删除光标处字符;行尾则吞并下一行
func (b *Buffer) Delete() {
	l := b.lines[b.cur]
	if b.col < len(l) {
		b.lines[b.cur] = append(l[:b.col], l[b.col+1:]...)
		b.resetWant()
		return
	}
	if b.cur < len(b.lines)-1 {
		b.lines[b.cur] = append(l, b.lines[b.cur+1]...)
		b.lines = append(b.lines[:b.cur+1], b.lines[b.cur+2:]...)
		b.resetWant()
	}
}

// Break 在光标处断行,不带任何前缀(Enter 的列表续写建立在它之上;粘贴换行直接用它)
func (b *Buffer) Break() {
	l := b.lines[b.cur]
	head := append([]rune{}, l[:b.col]...)
	rest := append([]rune{}, l[b.col:]...)
	b.lines[b.cur] = head
	b.lines = append(b.lines[:b.cur+1], append([][]rune{rest}, b.lines[b.cur+1:]...)...)
	b.cur++
	b.col = 0
	b.resetWant()
}

func (b *Buffer) Left() {
	if b.col > 0 {
		b.col--
	} else if b.cur > 0 {
		b.cur--
		b.col = len(b.lines[b.cur])
	}
	b.resetWant()
}

func (b *Buffer) Right() {
	if b.col < len(b.lines[b.cur]) {
		b.col++
	} else if b.cur < len(b.lines)-1 {
		b.cur++
		b.col = 0
	}
	b.resetWant()
}

func (b *Buffer) Top() {
	b.cur, b.col = 0, 0
	b.resetWant()
}

func (b *Buffer) Bottom() {
	b.cur = len(b.lines) - 1
	b.col = len(b.lines[b.cur])
	b.resetWant()
}

var (
	reBullet      = regexp.MustCompile(`^([ \t]*)([-*+])[ \t]+`)
	reOrdered     = regexp.MustCompile(`^([ \t]*)(\d+)([.)])[ \t]+`)
	reBulletOnly  = regexp.MustCompile(`^([ \t]*)([-*+])[ \t]*$`)
	reOrderedOnly = regexp.MustCompile(`^([ \t]*)(\d+)([.)])[ \t]*$`)
)

// Enter 换行:列表行(无序/有序)自动续写前缀,有序编号递增;
// 空列表项(只有前缀无内容)回车清除前缀退出列表,不插行
func (b *Buffer) Enter() {
	l := b.lines[b.cur]
	head := string(l[:b.col])
	tail := string(l[b.col:])
	prefix := ""
	switch {
	case tail == "" && reBulletOnly.MatchString(head):
		m := reBulletOnly.FindStringSubmatch(head)
		b.lines[b.cur] = []rune(m[1])
		b.col = len(m[1])
		b.resetWant()
		return
	case tail == "" && reOrderedOnly.MatchString(head):
		m := reOrderedOnly.FindStringSubmatch(head)
		b.lines[b.cur] = []rune(m[1])
		b.col = len(m[1])
		b.resetWant()
		return
	case reBullet.MatchString(head):
		m := reBullet.FindStringSubmatch(head)
		prefix = m[1] + m[2] + " "
	case reOrdered.MatchString(head):
		m := reOrdered.FindStringSubmatch(head)
		n, _ := strconv.Atoi(m[2])
		prefix = m[1] + strconv.Itoa(n+1) + m[3] + " "
	}
	b.Break() // 光标归新行首
	if prefix != "" {
		b.Insert([]rune(prefix)...)
	}
}

// Up 光标上移一个显示行(软换行片段也算一行),视觉列粘滞
func (b *Buffer) Up(width int) {
	rows := Layout(b, width)
	i := CursorVRow(b, width)
	if i == 0 {
		b.col = 0
		b.resetWant()
		return
	}
	b.step(rows, i, -1)
}

// Down 光标下移一个显示行;已是最后一显示行则到逻辑行尾
func (b *Buffer) Down(width int) {
	rows := Layout(b, width)
	i := CursorVRow(b, width)
	if i >= len(rows)-1 {
		b.col = len(b.lines[b.cur])
		b.resetWant()
		return
	}
	b.step(rows, i, +1)
}

// step 沿显示行 ±1 移动,保持视觉列(片段内显示偏移)粘滞
func (b *Buffer) step(rows []VRow, i, d int) {
	if b.want < 0 {
		cur := rows[i]
		b.want = DispCol(b.lines[b.cur], b.col) - DispCol(b.lines[cur.Line], cur.Start)
	}
	t := rows[i+d]
	lr := b.lines[t.Line]
	base := DispCol(lr, t.Start)
	col := idxAtDispCol(lr, base+b.want)
	if col > t.End {
		col = t.End
	}
	if col < t.Start {
		col = t.Start
	}
	b.cur, b.col = t.Line, col
}

// Home 移到当前显示行首(片段行首即逻辑行中段)
func (b *Buffer) Home(width int) {
	rows := Layout(b, width)
	r := rows[CursorVRow(b, width)]
	b.cur, b.col = r.Line, r.Start
	b.resetWant()
}

// End 移到当前显示行尾(含幻影空片段行)
func (b *Buffer) End(width int) {
	rows := Layout(b, width)
	r := rows[CursorVRow(b, width)]
	b.cur, b.col = r.Line, r.End
	b.resetWant()
}
