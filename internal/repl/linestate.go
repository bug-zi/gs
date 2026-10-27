package repl

// LineState 输入行的可编辑状态：rune 序列 + 光标位置（0..len）
type LineState struct {
	Runes []rune
	Pos   int
}

// NewLineState 空行
func NewLineState() LineState { return LineState{Runes: []rune{}} }

// FromText 以既有文本建行，光标在末尾
func FromText(text string) LineState {
	rs := []rune(text)
	return LineState{Runes: rs, Pos: len(rs)}
}

// Insert 在光标处插入一个 rune，光标后移
func (s *LineState) Insert(r rune) {
	s.Runes = append(s.Runes, 0)
	copy(s.Runes[s.Pos+1:], s.Runes[s.Pos:])
	s.Runes[s.Pos] = r
	s.Pos++
}

// InsertText 在光标处插入一串 rune（ghost 接受用）
func (s *LineState) InsertText(text string) {
	for _, r := range text {
		s.Insert(r)
	}
}

// Backspace 删除光标前一个 rune
func (s *LineState) Backspace() {
	if s.Pos == 0 {
		return
	}
	s.Runes = append(s.Runes[:s.Pos-1], s.Runes[s.Pos:]...)
	s.Pos--
}

// Delete 删除光标处一个 rune
func (s *LineState) Delete() {
	if s.Pos < len(s.Runes) {
		s.Runes = append(s.Runes[:s.Pos], s.Runes[s.Pos+1:]...)
	}
}

// Left 光标左移
func (s *LineState) Left() {
	if s.Pos > 0 {
		s.Pos--
	}
}

// Right 光标右移
func (s *LineState) Right() {
	if s.Pos < len(s.Runes) {
		s.Pos++
	}
}

// Len 行内 rune 数
func (s *LineState) Len() int { return len(s.Runes) }

// Text 整行文本
func (s *LineState) Text() string { return string(s.Runes) }

// Before 光标前文本
func (s *LineState) Before() string { return string(s.Runes[:s.Pos]) }

// UnderCursor 光标处 rune（空行返回空格）
func (s *LineState) UnderCursor() string {
	if s.Pos < len(s.Runes) {
		return string(s.Runes[s.Pos])
	}
	return " "
}

// AfterCursor 光标后文本（不含光标处字符）
func (s *LineState) AfterCursor() string {
	if s.Pos < len(s.Runes) {
		return string(s.Runes[s.Pos+1:])
	}
	return ""
}
