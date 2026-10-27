package diffx

import (
	"strings"
	"testing"
)

func TestOps(t *testing.T) {
	ops := Ops([]string{"a", "b", "c"}, []string{"a", "B", "c"})
	var got strings.Builder
	for _, op := range ops {
		switch op.Type {
		case OpEqual:
			got.WriteRune('=')
		case OpAdd:
			got.WriteRune('+')
		case OpDel:
			got.WriteRune('-')
		}
	}
	if got.String() != "=-+=" {
		t.Fatalf("ops 序列错误: %s", got.String())
	}
}

func TestOpsEmpty(t *testing.T) {
	ops := Ops(nil, []string{"x", "y"})
	if len(ops) != 2 || ops[0].Type != OpAdd {
		t.Fatalf("ops = %+v", ops)
	}
}

func TestRender(t *testing.T) {
	out := Render("a\nb\nc\nd\ne\nf\ng\nh\n", "a\nb\nX\nd\ne\nf\ng\nh\n")
	if !strings.Contains(out, "X") {
		t.Fatalf("应包含新增行: %s", out)
	}
	if !strings.Contains(out, "b") {
		t.Fatalf("应包含上下文行: %s", out)
	}
}

func TestRenderCompress(t *testing.T) {
	// 10 行相同应被压缩为省略提示
	var oldS strings.Builder
	for i := 0; i < 10; i++ {
		oldS.WriteString("same\n")
	}
	out := Render(oldS.String(), oldS.String())
	if !strings.Contains(out, "10 行相同") {
		t.Fatalf("应压缩等值块: %s", out)
	}
}
