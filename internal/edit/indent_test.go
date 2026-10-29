package edit

import "testing"

func TestIndentOutdent(t *testing.T) {
	b := NewBuffer("- 甲")
	b.Indent()
	if v := b.Value(); v != "  - 甲" {
		t.Fatalf("无序缩进 2: %q", v)
	}
	if _, col := b.Cur(); col != 2 {
		t.Fatalf("光标随缩进: %d", col)
	}
	b.Outdent()
	if v := b.Value(); v != "- 甲" {
		t.Fatalf("反缩进: %q", v)
	}

	o := NewBuffer("10. 点")
	o.Indent()
	if v := o.Value(); v != "    10. 点" {
		t.Fatalf("有序前缀宽 4: %q", v)
	}

	p := NewBuffer("普通行")
	p.Indent()
	if v := p.Value(); v != "  普通行" {
		t.Fatalf("非列表行插 2 空格: %q", v)
	}
	p.Outdent()
	if v := p.Value(); v != "普通行" {
		t.Fatalf("非列表反缩进: %q", v)
	}

	q := NewBuffer("甲")
	q.Outdent() // 顶格无空白,no-op 不崩
	if v := q.Value(); v != "甲" {
		t.Fatalf("顶格反缩进 no-op: %q", v)
	}
}
