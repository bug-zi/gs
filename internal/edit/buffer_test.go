package edit

import "testing"

func TestNewBufferAndValue(t *testing.T) {
	b := NewBuffer("a\n中文\n")
	if v := b.Value(); v != "a\n中文\n" {
		t.Fatalf("Value round-trip: %q", v)
	}
	if b.LineCount() != 3 {
		t.Fatalf("LineCount = %d, want 3", b.LineCount())
	}
	if b := NewBuffer(""); b.LineCount() != 1 {
		t.Fatalf("空文件应为 1 行, got %d", b.LineCount())
	}
}

func TestInsert(t *testing.T) {
	b := NewBuffer("ab")
	b.Bottom()
	b.Insert('中', '文') // 尾部
	if v := b.Value(); v != "ab中文" {
		t.Fatalf("尾部插入: %q", v)
	}
	b2 := NewBuffer("ab")
	b2.Right() // col=1
	b2.Insert('X')
	if v := b2.Value(); v != "aXb" {
		t.Fatalf("中间插入: %q", v)
	}
}

func TestBackspace(t *testing.T) {
	b := NewBuffer("ab\ncd")
	b.Bottom()
	b.Backspace() // 删 d
	if v := b.Value(); v != "ab\nc" {
		t.Fatalf("行内退格: %q", v)
	}
	b.Backspace() // 删 c
	b.Backspace() // 行首,与上行合并
	if v := b.Value(); v != "ab" {
		t.Fatalf("合并上行: %q", v)
	}
	b.Top()
	b.Backspace() // 文档首,无操作
	if v := b.Value(); v != "ab" {
		t.Fatalf("文档首退格应无操作: %q", v)
	}
}

func TestDelete(t *testing.T) {
	b := NewBuffer("ab\ncd")
	b.Delete() // 删 a
	if v := b.Value(); v != "b\ncd" {
		t.Fatalf("行内删除: %q", v)
	}
	b.Right()  // → "b" 行尾
	b.Delete() // 行尾吞并下一行
	if v := b.Value(); v != "bcd" {
		t.Fatalf("行尾吞并行: %q", v)
	}
}

func TestLeftRight(t *testing.T) {
	b := NewBuffer("ab\ncd")
	b.Right()
	b.Right()
	b.Right() // 行尾再右 → 下一行行首
	if l, c := b.Cur(); l != 1 || c != 0 {
		t.Fatalf("跨行右移: line=%d col=%d", l, c)
	}
	b.Left() // 行首再左 → 上一行行尾
	if l, c := b.Cur(); l != 0 || c != 2 {
		t.Fatalf("跨行左移: line=%d col=%d", l, c)
	}
}

func TestEnterPlain(t *testing.T) {
	b := NewBuffer("ab")
	b.Right()
	b.Enter()
	if v := b.Value(); v != "a\nb" {
		t.Fatalf("普通断行: %q", v)
	}
}

func TestEnterContinuesBullet(t *testing.T) {
	b := NewBuffer("- 第一条")
	b.Bottom()
	b.Enter()
	if v := b.Value(); v != "- 第一条\n- " {
		t.Fatalf("无序列表续写: %q", v)
	}
	if _, c := b.Cur(); c != 2 {
		t.Fatalf("光标应在前缀后: col=%d", c)
	}
}

func TestEnterContinuesOrdered(t *testing.T) {
	b := NewBuffer("1. a")
	b.Bottom()
	b.Enter()
	if v := b.Value(); v != "1. a\n2. " {
		t.Fatalf("有序列表续写递增: %q", v)
	}
}

func TestEnterMidLineSplitsWithPrefix(t *testing.T) {
	b := NewBuffer("- abc")
	for i := 0; i < 3; i++ { // col=3,即 "a" 之后
		b.Right()
	}
	b.Enter()
	if v := b.Value(); v != "- a\n- bc" {
		t.Fatalf("行中断行续写: %q", v)
	}
}

func TestEnterOnEmptyBulletExits(t *testing.T) {
	b := NewBuffer("- ")
	b.Bottom()
	b.Enter()
	if v := b.Value(); v != "" {
		t.Fatalf("空列表项回车应清前缀退出: %q", v)
	}
}

func TestUpDownVisualRows(t *testing.T) {
	// width=12 时 L1("- Go语言中很多",显示宽 14)切两段:
	//   seg0 = "- Go语言中很"(12 列, rune 0..8),seg1 = "多"(rune 8..9)
	b := NewBuffer("- 第一条\n- Go语言中很多\n- 第三条")
	b.Bottom() // L2 行尾
	b.Up(12)   // → L1 seg1("多" 片段),钳到片段末尾
	if l, c := b.Cur(); l != 1 || c != 9 {
		t.Fatalf("第一次↑应到 L1 seg1 末尾: line=%d col=%d", l, c)
	}
	b.Up(12) // → L1 seg0(不跳到 L0!);粘滞视觉列 8 → 落在"中"(列 8-9)
	if l, _ := b.Cur(); l != 1 {
		t.Fatalf("第二次↑应仍在 L1(回归 textarea 跳行缺陷): line=%d", l)
	}
	b.Insert('X')
	if got := string(b.LineRunes(1)); got != "- Go语言X中很多" {
		t.Fatalf("X 应落在 seg0 内: %q", got)
	}
}

func TestDownPastWrappedTail(t *testing.T) {
	b := NewBuffer("- Go语言中很多\n- 第二条")
	b.Top()
	b.Down(12) // seg0 → seg1,视觉列 0
	if l, c := b.Cur(); l != 0 || c != 8 {
		t.Fatalf("↓ 应到 L0 的 seg1: line=%d col=%d", l, c)
	}
	b.Down(12) // seg1 → 下一逻辑行,视觉列 0 保持
	if l, c := b.Cur(); l != 1 || c != 0 {
		t.Fatalf("seg1 ↓ 应到 L1 行首: line=%d col=%d", l, c)
	}
}

func TestHomeEndVisualRow(t *testing.T) {
	b := NewBuffer("- Go语言中很多")
	b.End(12) // seg0 显示行尾 = rune 8("- Go语言中很")
	if _, c := b.Cur(); c != 8 {
		t.Fatalf("End 应到显示行尾: col=%d", c)
	}
	b.Right() // → col 9,落在 seg1
	b.Home(12)
	if _, c := b.Cur(); c != 8 {
		t.Fatalf("Home 应回 seg1 行首(rune 8): col=%d", c)
	}
	b.Top()
	b.Home(12)
	if _, c := b.Cur(); c != 0 {
		t.Fatalf("Top 后 Home 应为 0: col=%d", c)
	}
}
