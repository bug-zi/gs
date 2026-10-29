package edit

import "testing"

func TestUndoRedo(t *testing.T) {
	b := NewBuffer("ab")
	b.Bottom() // 光标 (0,2)
	b.PushUndo()
	b.Insert([]rune("c")...)
	if !b.Undo() {
		t.Fatal("undo 应成功")
	}
	if v := b.Value(); v != "ab" {
		t.Fatalf("undo 后: %q", v)
	}
	if _, col := b.Cur(); col != 2 {
		t.Fatalf("undo 应恢复光标列, got %d", col)
	}
	if !b.Redo() {
		t.Fatal("redo 应成功")
	}
	if v := b.Value(); v != "abc" {
		t.Fatalf("redo 后: %q", v)
	}
}

func TestUndoRedoEmptyStack(t *testing.T) {
	b := NewBuffer("x")
	if b.Undo() || b.Redo() {
		t.Fatal("空栈 undo/redo 应返回 false")
	}
}

func TestRedoClearedOnChange(t *testing.T) {
	b := NewBuffer("a")
	b.Bottom() // 光标行尾
	b.PushUndo()
	b.Insert('b')
	b.Undo()
	b.PushUndo()
	b.Insert('c') // 新变更清空 redo
	if b.Redo() {
		t.Fatal("新变更后 redo 栈应清空")
	}
	if v := b.Value(); v != "ac" {
		t.Fatalf("值: %q", v)
	}
}

func TestUndoLimit(t *testing.T) {
	b := NewBuffer("")
	for i := 0; i < undoLimit+10; i++ {
		b.PushUndo()
		b.Insert('x')
	}
	n := 0
	for b.Undo() {
		n++
	}
	if n != undoLimit {
		t.Fatalf("undo 步数应封顶 %d, got %d", undoLimit, n)
	}
}

func TestUndoSnapshotIsolation(t *testing.T) {
	b := NewBuffer("abc")
	b.PushUndo()
	// 连续插入迫行走 append 原地扩容路径,验证快照不被别名污染
	for i := 0; i < 50; i++ {
		b.Insert('x')
	}
	b.Undo()
	if v := b.Value(); v != "abc" {
		t.Fatalf("快照遭别名污染: %q", v)
	}
}
