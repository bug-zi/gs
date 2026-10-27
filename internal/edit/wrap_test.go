package edit

import "testing"

func TestWrapRunesCJK(t *testing.T) {
	// "中文ab" 显示宽 6;width 4 → [中文][ab]
	segs := WrapRunes([]rune("中文ab"), 4)
	if len(segs) != 2 {
		t.Fatalf("seg 数 = %d, want 2", len(segs))
	}
	if string(segs[0].Runes) != "中文" || segs[0].Wrapped {
		t.Fatalf("seg0 = %q wrapped=%v", segs[0].Runes, segs[0].Wrapped)
	}
	if string(segs[1].Runes) != "ab" || !segs[1].Wrapped {
		t.Fatalf("seg1 = %q wrapped=%v", segs[1].Runes, segs[1].Wrapped)
	}
}

func TestWrapRunesExactWidthAddsPhantom(t *testing.T) {
	// "中文" 宽 4 恰好填满 width 4 → 尾补空片段给光标落脚
	segs := WrapRunes([]rune("中文"), 4)
	if len(segs) != 2 {
		t.Fatalf("seg 数 = %d, want 2(含幻影)", len(segs))
	}
	if len(segs[1].Runes) != 0 || !segs[1].Wrapped {
		t.Fatalf("幻影片段: %q wrapped=%v", segs[1].Runes, segs[1].Wrapped)
	}
}

func TestWrapRunesEmptyLine(t *testing.T) {
	segs := WrapRunes([]rune(""), 80)
	if len(segs) != 1 || segs[0].Wrapped {
		t.Fatalf("空行应单片段未换行: %+v", segs)
	}
}

func TestWrapRunesLongASCII(t *testing.T) {
	// 超长无空格 ASCII 串按 width 硬切
	segs := WrapRunes([]rune("abcdefghij"), 3)
	if len(segs) != 4 {
		t.Fatalf("seg 数 = %d, want 4", len(segs))
	}
	if string(segs[3].Runes) != "j" {
		t.Fatalf("末片段 = %q", segs[3].Runes)
	}
}

func TestWrapRunesWidthClamp(t *testing.T) {
	segs := WrapRunes([]rune("abc"), 0)
	if len(segs) < 1 {
		t.Fatalf("width=0 不应 panic")
	}
}

func TestLayout(t *testing.T) {
	b := NewBuffer("ab\n中文cd")
	rows := Layout(b, 4)
	// 行0:1 段;行1:中文(4)填满后断 → [中文][cd],共 3 显示行
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].Line != 0 || rows[0].Start != 0 || rows[0].End != 2 {
		t.Fatalf("rows[0] = %+v", rows[0])
	}
	if rows[1].Line != 1 || rows[1].Start != 0 || rows[1].End != 2 {
		t.Fatalf("rows[1] = %+v", rows[1])
	}
	if rows[2].Line != 1 || rows[2].Start != 2 || rows[2].End != 4 {
		t.Fatalf("rows[2] = %+v", rows[2])
	}
}

func TestDispColAndIdx(t *testing.T) {
	line := []rune("- 中文") // 显示宽:['-'1][' '1]['中'2]['文'2]
	if DispCol(line, 3) != 4 { // 前 3 rune = "- 中" = 4 列
		t.Fatalf("DispCol(3) = %d, want 4", DispCol(line, 3))
	}
	if idx := idxAtDispCol(line, 3); idx != 2 { // 显示列 3 落在"中"(列2-3)内部 → 取"中"
		t.Fatalf("idxAtDispCol(3) = %d, want 2", idx)
	}
	if idx := idxAtDispCol(line, 99); idx != len(line) {
		t.Fatalf("超出宽应返回行尾: %d", idx)
	}
}
