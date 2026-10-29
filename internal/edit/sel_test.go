package edit

import "testing"

func TestNormSel(t *testing.T) {
	s, e := normSel(selPos{1, 3}, selPos{0, 2})
	if s != (selPos{0, 2}) || e != (selPos{1, 3}) {
		t.Fatalf("跨行归一: %v %v", s, e)
	}
	s, e = normSel(selPos{2, 5}, selPos{2, 1})
	if s != (selPos{2, 1}) || e != (selPos{2, 5}) {
		t.Fatalf("同行归一: %v %v", s, e)
	}
}

func TestSelTextAndDel(t *testing.T) {
	b := NewBuffer("hello 世界\nsecond\nthird")
	start, end := selPos{0, 2}, selPos{2, 3}
	if got := selText(b.lines, start, end); got != "llo 世界\nsecond\nthi" {
		t.Fatalf("selText: %q", got)
	}
	delSel(b, start, end)
	if v := b.Value(); v != "herd" {
		t.Fatalf("delSel: %q", v)
	}
	if line, col := b.Cur(); line != 0 || col != 2 {
		t.Fatalf("光标应落选区起点: (%d,%d)", line, col)
	}
}

func TestDelSelSameLine(t *testing.T) {
	b := NewBuffer("abcdef")
	delSel(b, selPos{0, 2}, selPos{0, 4})
	if v := b.Value(); v != "abef" {
		t.Fatalf("同行删除: %q", v)
	}
}
