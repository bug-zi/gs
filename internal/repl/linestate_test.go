package repl

import "testing"

func TestLineStateInsertAndMove(t *testing.T) {
	s := NewLineState()
	for _, r := range []rune("abc") {
		s.Insert(r)
	}
	if s.Text() != "abc" || s.Pos != 3 {
		t.Fatalf("插入: %q pos=%d", s.Text(), s.Pos)
	}
	s.Left() // 光标移到 c 前（b 后）
	s.Insert('X')
	if s.Text() != "abXc" || s.Pos != 3 {
		t.Fatalf("中插: %q pos=%d", s.Text(), s.Pos)
	}
	s.Backspace()
	if s.Text() != "abc" || s.Pos != 2 {
		t.Fatalf("退格: %q pos=%d", s.Text(), s.Pos)
	}
	s.Delete()
	if s.Text() != "ab" {
		t.Fatalf("Delete: %q", s.Text())
	}
	s.Left()
	s.Left()
	if s.Pos != 0 {
		t.Fatalf("左移越界 pos=%d", s.Pos)
	}
	s.Backspace() // 行首退格应无害
	s.Left()
	if s.Pos != 0 {
		t.Fatal("行首左移应无害")
	}
}

func TestFromTextAndHelpers(t *testing.T) {
	s := FromText("并发")
	if s.Len() != 2 || s.Pos != 2 {
		t.Fatalf("len=%d pos=%d", s.Len(), s.Pos)
	}
	if s.UnderCursor() != " " {
		t.Fatalf("行尾光标字符应为空格: %q", s.UnderCursor())
	}
	if s.AfterCursor() != "" {
		t.Fatalf("AfterCursor = %q", s.AfterCursor())
	}
	s.Left()
	if s.UnderCursor() != "发" || s.Before() != "并" || s.AfterCursor() != "" {
		t.Fatalf("光标在「发」上: under=%q before=%q after=%q", s.UnderCursor(), s.Before(), s.AfterCursor())
	}
}

func TestInsertText(t *testing.T) {
	s := FromText("/lo")
	s.InsertText("gs")
	if s.Text() != "/logs" || s.Pos != 5 {
		t.Fatalf("ghost 接受: %q pos=%d", s.Text(), s.Pos)
	}
}
