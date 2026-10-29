package repl

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

// stripANSI 去除 ANSI 转义序列，便于断言纯文本
func stripANSI(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b {
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
					j++
				}
				if j < len(s) {
					j++
				}
				i = j
				continue
			}
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func TestBuildInputLine(t *testing.T) {
	st := FromText("/lo")
	line := buildInputLine("gs> ", st, "gs")
	plain := strings.TrimPrefix(stripANSI(line), "\r")
	if !strings.HasPrefix(plain, "gs> /lo") || !strings.Contains(plain, "gs") {
		t.Fatalf("line = %q", plain)
	}
	if !strings.Contains(line, "\x1b[7m") {
		t.Fatal("应有反白光标")
	}
	if !strings.Contains(line, "\x1b[2mgs\x1b[0m") {
		t.Fatal("行尾 ghost 应为灰色")
	}
	// 光标不在行尾时 ghost 不显示
	st.Left()
	line = buildInputLine("gs> ", st, "gs")
	if strings.Contains(line, "\x1b[2mgs") {
		t.Fatal("光标不在行尾不应渲染 ghost")
	}
}

func TestBuildPanelLines(t *testing.T) {
	lines := buildPanelLines([]string{"add", "edit"}, "<关键词|日期>", 1)
	if len(lines) != 3 {
		t.Fatalf("提示+2候选 = 3 行, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "<关键词|日期>") {
		t.Fatalf("首行应为提示: %q", lines[0])
	}
	if !strings.Contains(lines[2], "❯") || !strings.Contains(lines[2], "edit") {
		t.Fatalf("高亮项: %q", lines[2])
	}
	if !strings.Contains(lines[2], "\x1b[48;5;236m") || !strings.Contains(lines[2], "\x1b[38;5;220m") {
		t.Fatalf("选中行应含微底与金箭头: %q", lines[2])
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	if strings.Contains(plain, "❯ add") {
		t.Fatal("未选中项不应有 ❯")
	}
}

func TestPlainWidth(t *testing.T) {
	colored := "\x1b[38;5;220m\x1b[1mgs\x1b[0m \x1b[38;5;240m❯\x1b[0m "
	if got := plainWidth(colored); got != 5 {
		t.Fatalf("plainWidth(带色提示符) = %d, want 5", got)
	}
	if got := plainWidth("gs> "); got != 4 {
		t.Fatalf("plainWidth(素提示符) = %d, want 4", got)
	}
}

func TestCursorColumnWideChars(t *testing.T) {
	st := FromText("并发")
	st.Left() // 光标在「发」前
	// 40 列文本区远大于内容：不滚动，光标列 = 「并」宽 2
	_, cur := buildInputLineFit("gs> ", st, "", 40)
	if cur != 4+2 { // gs> =4 列 + 「并」=2 列
		t.Fatalf("宽字符光标列 = %d, want 6", cur)
	}
	if runewidth.StringWidth("并发") != 4 {
		t.Fatal("runewidth 应按宽字符计 2 列")
	}
}

// cjk 生成 n 个汉字的字符串
func cjk(n int) string {
	return strings.Repeat("汉", n)
}

func TestInputWindowShortLine(t *testing.T) {
	lo, hi, start := inputWindow([]rune("abc"), 3, 10)
	if lo != 0 || hi != 3 || start != 0 {
		t.Fatalf("短行不应滚动: lo=%d hi=%d start=%d", lo, hi, start)
	}
}

func TestInputWindowLongLatin(t *testing.T) {
	rs := []rune(strings.Repeat("a", 50))
	lo, hi, start := inputWindow(rs, 50, 10)
	if hi-lo > 10 {
		t.Fatalf("窗口超宽: %d 列内容", hi-lo)
	}
	// 光标必须落在窗口内且渲染总宽 ≤ avail
	if rs[hi-1] != 'a' || lo < 0 || start < 0 {
		t.Fatalf("窗口异常: lo=%d hi=%d start=%d", lo, hi, start)
	}
	if cur := 50 - start; cur > 10 {
		t.Fatalf("光标列 %d 超出 avail 10", cur)
	}
}

func TestInputWindowLongCJK(t *testing.T) {
	rs := []rune(cjk(30))
	lo, hi, start := inputWindow(rs, 30, 10)
	width := 0
	for _, r := range rs[lo:hi] {
		width += runewidth.RuneWidth(r)
	}
	if width > 10 {
		t.Fatalf("CJK 窗口宽 %d 超出 avail 10", width)
	}
	if cur := 60 - start; cur > 10 {
		t.Fatalf("光标列 %d 超出 avail 10", cur)
	}
	if hi != 30 || hi <= lo {
		t.Fatalf("hi 应到行尾: lo=%d hi=%d", lo, hi)
	}
}

func TestInputWindowWideCharBoundary(t *testing.T) {
	// 构造窗口起点落在宽字符中间的场景：avail 与光标列错位
	rs := []rune(cjk(10))
	lo, hi, _ := inputWindow(rs, 5, 7)
	width := 0
	for _, r := range rs[lo:hi] {
		width += runewidth.RuneWidth(r)
	}
	if width > 7 {
		t.Fatalf("窗口宽 %d 超出 avail 7（宽字符被切半）", width)
	}
}

func TestInputWindowCursorMiddle(t *testing.T) {
	rs := []rune(strings.Repeat("a", 40))
	pos := 20
	lo, hi, start := inputWindow(rs, pos, 10)
	if pos < lo || pos > hi {
		t.Fatalf("光标 %d 不在窗口 [%d,%d) 内", pos, lo, hi)
	}
	if cur := pos - start; cur > 10 || cur < 0 {
		t.Fatalf("光标相对列 %d 越界", cur)
	}
}

func TestBuildInputLineFitClips(t *testing.T) {
	// 超长中文行：渲染结果去 ANSI 后显示宽度不得超过 提示符+avail
	st := FromText(cjk(30))
	line, cur := buildInputLineFit("ai> ", st, "", 20)
	plain := strings.TrimPrefix(stripANSI(line), "\r")
	if w := runewidth.StringWidth(plain); w > 4+20 {
		t.Fatalf("渲染宽 %d 超出终端预算 %d", w, 4+20)
	}
	if !strings.Contains(line, "\x1b[7m") {
		t.Fatal("应有反白光标")
	}
	if cur > 4+20 {
		t.Fatalf("光标列 %d 超出预算", cur)
	}
}

func TestTruncateByWidth(t *testing.T) {
	if got := truncateByWidth(cjk(5), 7); got != cjk(3) { // 3 字 = 6 列，第 4 字放不下
		t.Fatalf("truncate = %q, want %q", got, cjk(3))
	}
	if got := truncateByWidth("abc", 10); got != "abc" {
		t.Fatalf("不超宽应原样: %q", got)
	}
	if got := truncateByWidth(cjk(3), 0); got != "" {
		t.Fatalf("0 列应为空: %q", got)
	}
}

func TestAcceptCandidateText(t *testing.T) {
	if got := acceptCandidateText("/se", "sea"); got != "/sea" {
		t.Fatalf("第一词: %q", got)
	}
	if got := acceptCandidateText("/sea e", "edit"); got != "/sea edit" {
		t.Fatalf("第二词: %q", got)
	}
}
