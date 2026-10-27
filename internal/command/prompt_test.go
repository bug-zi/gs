package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestSelectStateMove(t *testing.T) {
	st := &selectState{}
	// 下移环形回绕
	for i := 0; i < 3; i++ {
		st.move(3, 1)
	}
	if st.sel != 0 {
		t.Fatalf("下移三轮应回绕到 0，得 %d", st.sel)
	}
	// 上移回绕到末尾
	st.move(3, -1)
	if st.sel != 2 {
		t.Fatalf("0 上移应回绕到 2，得 %d", st.sel)
	}
	// 大步长回绕
	st2 := &selectState{sel: 1}
	st2.move(4, -5)
	if st2.sel != 0 {
		t.Fatalf("1-5 mod 4 应为 0，得 %d", st2.sel)
	}
}

func TestDrawSelectFrame(t *testing.T) {
	opts := []menuOption{
		{Label: "用 trae 打开", Value: "@open"},
		{Label: "查看相关笔记（2 篇）", Value: "@notes"},
		{Label: "返回", Value: ""},
	}
	var buf bytes.Buffer
	drawSelectFrame(&buf, "web/a.go", opts, &selectState{})
	out := stripANSI(buf.String())
	for _, want := range []string{"web/a.go", "↑↓/jk 选择", "用 trae 打开", "查看相关笔记（2 篇）", "返回"} {
		if !strings.Contains(out, want) {
			t.Fatalf("菜单帧缺 %q:\n%s", want, out)
		}
	}
	// 光标行反白 + ❯，且只标在首项
	if !strings.Contains(buf.String(), "\x1b[7m❯ 用 trae 打开") {
		t.Fatalf("选中项应反白带 ❯: %q", buf.String())
	}
	if strings.Count(buf.String(), "\x1b[7m") != 1 {
		t.Fatalf("应只有一行反白: %q", buf.String())
	}
	// 移动光标后 ❯ 跟随
	buf.Reset()
	drawSelectFrame(&buf, "t", opts, &selectState{sel: 2})
	if !strings.Contains(buf.String(), "\x1b[7m❯ 返回") {
		t.Fatalf("光标应跟随到末项: %q", buf.String())
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

func TestDrawSelectFrameCursorReturn(t *testing.T) {
	opts := []menuOption{
		{Label: "a", Value: "a"},
		{Label: "b", Value: "b"},
		{Label: "c", Value: "c"},
	}
	var buf bytes.Buffer
	drawSelectFrame(&buf, "选择项目", opts, &selectState{})
	assertFrameCursorReturn(t, buf.String())
}

// 移动光标后 ❯ 跟随
func TestDrawSelectFrameCursorFollow(t *testing.T) {
	opts := []menuOption{
		{Label: "a", Value: "a"},
		{Label: "返回", Value: ""},
	}
	var buf bytes.Buffer
	drawSelectFrame(&buf, "t", opts, &selectState{sel: 1})
	if !strings.Contains(buf.String(), "\x1b[7m❯ 返回") {
		t.Fatalf("光标应跟随到末项: %q", buf.String())
	}
}
