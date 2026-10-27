package planview

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestWindow(t *testing.T) {
	if lo, hi := window(10, 3, 20); lo != 0 || hi != 10 {
		t.Fatalf("不超限应全显: %d-%d", lo, hi)
	}
	lo, hi := window(100, 50, 10)
	if hi-lo != 10 || !(lo <= 50 && 50 < hi) {
		t.Fatalf("应光标居中滑动: %d-%d", lo, hi)
	}
	lo, hi = window(100, 95, 10)
	if lo != 90 || hi != 100 {
		t.Fatalf("贴底取齐: %d-%d", lo, hi)
	}
	lo, hi = window(100, 2, 10)
	if lo != 0 || hi != 10 {
		t.Fatalf("贴顶取齐: %d-%d", lo, hi)
	}
}

func TestRenderRowCursor(t *testing.T) {
	r := row{Kind: rowPoint, Selectable: true, Text: "  [ ] 变量与常量"}
	sel := renderRow(r, true, 80)
	if !strings.HasPrefix(sel, "\x1b[7m") || !strings.HasSuffix(sel, "\x1b[27m") {
		t.Fatalf("光标行应反白: %q", sel)
	}
	head := renderRow(row{Kind: rowGroupHead, Text: "语法核心"}, false, 80)
	if !strings.Contains(head, "语法核心") {
		t.Fatalf("分组行渲染: %q", head)
	}
	hint := renderRow(row{Kind: rowHint, Text: "  （无）"}, false, 80)
	if !strings.Contains(stripANSI(hint), "（无）") {
		t.Fatalf("提示行渲染: %q", hint)
	}
}

func TestViewTitle(t *testing.T) {
	m := stateFixture(t)
	m.Normalize()
	if got := viewTitle(m); !strings.Contains(stripANSI(got), "Go 学习路线图") {
		t.Fatalf("全局树标题: %q", got)
	}
	m.Branch, m.View = 1, viewBranch
	if got := viewTitle(m); !strings.Contains(stripANSI(got), "并发编程") {
		t.Fatalf("主干页标题: %q", got)
	}
	m.Point, m.View = 0, viewPoint
	if got := viewTitle(m); !strings.Contains(stripANSI(got), "atomic") {
		t.Fatalf("详情页标题: %q", got)
	}
}

func TestViewHint(t *testing.T) {
	m := stateFixture(t)
	if h := viewHint(m); !strings.Contains(h, "Tab") {
		t.Fatalf("全局树提示应含 Tab: %q", h)
	}
	m.View = viewPoint
	if h := viewHint(m); !strings.Contains(h, "d/s/t") {
		t.Fatalf("详情页提示应含 d/s/t: %q", h)
	}
}

// assertFrameCursorReturn 校验整帧重绘的光标归位：帧体每行以 \r\n 结尾，
// 收尾上移行数必须等于写入行数；少一行则旧帧顶部清不掉，每次重绘标题逐行堆叠
func assertFrameCursorReturn(t *testing.T, out string) {
	t.Helper()
	written := strings.Count(out, "\r\n")
	i := strings.LastIndex(out, "\x1b[")
	if i < 0 {
		t.Fatalf("帧尾缺少光标归位序列: %q", out)
	}
	var n int
	if _, err := fmt.Sscanf(out[i:], "\x1b[%dA", &n); err != nil {
		t.Fatalf("无法解析归位序列: %q", out[i:])
	}
	if n != written {
		t.Fatalf("帧体写入 %d 行但仅上移 %d 行，重绘会残留旧帧", written, n)
	}
}

func TestDrawFrameCursorReturn(t *testing.T) {
	m := stateFixture(t)
	var buf bytes.Buffer
	drawFrame(&buf, m)
	assertFrameCursorReturn(t, buf.String())

	m.Err = "测试错误"
	buf.Reset()
	drawFrame(&buf, m)
	assertFrameCursorReturn(t, buf.String())
}
