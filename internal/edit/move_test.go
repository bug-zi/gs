package edit

import "testing"

func TestMoveLinePlain(t *testing.T) {
	b := NewBuffer("a\nb\nc")
	b.cur = 1
	if !b.MoveLine(true) || b.Value() != "b\na\nc" {
		t.Fatalf("上移: %q", b.Value())
	}
	if line, _ := b.Cur(); line != 0 {
		t.Fatalf("光标跟随: %d", line)
	}
}

func TestMoveLineEdges(t *testing.T) {
	b := NewBuffer("a\nb")
	if b.MoveLine(true) {
		t.Fatal("首行不可上移")
	}
	b.cur = 1
	if b.MoveLine(false) {
		t.Fatal("末行不可下移")
	}
}

func TestMoveListItemBlock(t *testing.T) {
	b := NewBuffer("- 一\n  续行\n- 二\n- 三")
	b.cur = 0
	if !b.MoveLine(false) || b.Value() != "- 二\n- 一\n  续行\n- 三" {
		t.Fatalf("列表项块下移: %q", b.Value())
	}
	b.cur = 0
	if !b.MoveLine(false) || b.Value() != "- 一\n  续行\n- 二\n- 三" {
		t.Fatalf("再下移: %q", b.Value())
	}
	b.cur = 3
	if !b.MoveLine(true) || b.Value() != "- 一\n  续行\n- 三\n- 二" {
		t.Fatalf("尾块上移: %q", b.Value())
	}
}

func TestMoveOrderedKeepsNumber(t *testing.T) {
	b := NewBuffer("1. 甲\n2. 乙")
	b.cur = 1
	if !b.MoveLine(true) || b.Value() != "2. 乙\n1. 甲" {
		t.Fatalf("有序编号应原样: %q", b.Value())
	}
}

func TestCanMove(t *testing.T) {
	b := NewBuffer("a\nb")
	if b.CanMove(true) {
		t.Fatal("首行 CanMove(上) 应 false")
	}
	b = NewBuffer("a\nb\nc")
	b.cur = 1
	if !b.CanMove(true) || !b.CanMove(false) {
		t.Fatal("中间行上下应均可移")
	}
}
