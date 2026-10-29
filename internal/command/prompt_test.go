package command

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ui"
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
	// 光标行金箭头 + 微底亮字，且只标在首项
	selRow := ui.ANSIGold + "❯" + ui.ANSIReset + " " + ui.ANSISelBg + ui.ANSIBright + "用 trae 打开"
	if !strings.Contains(buf.String(), selRow) {
		t.Fatalf("选中项应金箭头微底: %q", buf.String())
	}
	if strings.Count(buf.String(), ui.ANSISelBg) != 1 {
		t.Fatalf("应只有一行选中底: %q", buf.String())
	}
	// 移动光标后 ❯ 跟随
	buf.Reset()
	drawSelectFrame(&buf, "t", opts, &selectState{sel: 2})
	if !strings.Contains(buf.String(), ui.ANSISelBg+ui.ANSIBright+"返回") {
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
	if !strings.Contains(buf.String(), ui.ANSISelBg+ui.ANSIBright+"返回") {
		t.Fatalf("光标应跟随到末项: %q", buf.String())
	}
}

func TestDrawSelectFrameDim(t *testing.T) {
	opts := []menuOption{
		{Label: "已装船条目", Value: "a", Dim: true},
		{Label: "新材料", Value: "b"},
	}
	var buf bytes.Buffer
	drawSelectFrame(&buf, "选择条目", opts, &selectState{sel: 1})
	s := buf.String()
	if !strings.Contains(s, ui.Dim("已装船条目")) {
		t.Fatalf("Dim 项应与提示行同款阴影色: %q", s)
	}
	if !strings.Contains(s, ui.ANSISelBg+ui.ANSIBright+"新材料") {
		t.Fatalf("光标行应保持反白高亮不受 Dim 影响: %q", s)
	}
}

// TestDrawSelectFrameDimSelected 已装船条目被选中：微底提示位置但保持灰字，不亮白
func TestDrawSelectFrameDimSelected(t *testing.T) {
	opts := []menuOption{{Label: "已装船条目", Value: "a", Dim: true}}
	var buf bytes.Buffer
	drawSelectFrame(&buf, "t", opts, &selectState{sel: 0})
	s := buf.String()
	if strings.Contains(s, ui.ANSIBright) {
		t.Fatalf("已装船条目选中不应亮白高亮: %q", s)
	}
	if !strings.Contains(s, ui.ANSISelBg) || !strings.Contains(s, ui.ANSIFaint) {
		t.Fatalf("选中应微底且保持灰字: %q", s)
	}
}

func TestPlainOptionsTextDim(t *testing.T) {
	opts := []menuOption{
		{Label: "已装船", Value: "a", Dim: true},
		{Label: "新材料", Value: "b"},
	}
	s := plainOptionsText("选择条目", opts)
	if !strings.Contains(s, ui.Dim("已装船")) {
		t.Fatalf("降级列表 Dim 项应与提示行同款阴影色: %q", s)
	}
	if !strings.Contains(s, "2. 新材料") {
		t.Fatalf("编号列表应完整: %q", s)
	}
	if !strings.HasSuffix(s, "输入编号选择（空/0 取消）> ") {
		t.Fatalf("应保留输入提示: %q", s)
	}
}
