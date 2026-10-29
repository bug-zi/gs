package edit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

func keyRunes(rs ...rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: rs} }

func newTestEditor(t *testing.T, content string, width int) editor {
	t.Helper()
	e := newEditor(filepath.Join(t.TempDir(), "a.md"), []byte(content))
	e.width, e.height = width, 10
	return e
}

func typeString(e editor, s string) editor {
	for _, r := range s {
		m, _ := e.Update(keyRunes(r))
		e = m.(editor)
	}
	return e
}

func TestEditorOpensAtTop(t *testing.T) {
	e := newTestEditor(t, "第一行\n第二行\n", 40)
	line, col := e.buf.Cur()
	if line != 0 || col != 0 {
		t.Fatalf("编辑模式初始光标应在首行首列, got (%d,%d)", line, col)
	}
}

func TestEditorTypeBulletList(t *testing.T) {
	e := newTestEditor(t, "", 40)
	e = typeString(e, "- 变量声明:var")
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	e = m.(editor)
	e = typeString(e, "Go语言") // "- " 前缀已自动续写,无需手打
	if v := e.buf.Value(); v != "- 变量声明:var\n- Go语言" {
		t.Fatalf("按键流建列表: %q", v)
	}
	// Enter 后自动续写,不必手打 "- "
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	e = m.(editor)
	if v := e.buf.Value(); v != "- 变量声明:var\n- Go语言\n- " {
		t.Fatalf("自动续写: %q", v)
	}
}

func TestEditorUpArrowRegression(t *testing.T) {
	// 复刻用户场景(width=12 使 L1 软换行):
	// 下方行连按 ↑ 不得跳进更上一条逻辑行
	e := newTestEditor(t, "- 第一条\n- Go语言中很多\n", 12)
	e.buf.Bottom() // 测的是文末续写后向上移动,先归位文末
	e = typeString(e, "- 第三条")
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyUp})
	e = m.(editor)
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyUp})
	e = m.(editor)
	m, _ = e.Update(keyRunes('X'))
	e = m.(editor)
	lines := strings.Split(e.buf.Value(), "\n")
	if !strings.Contains(lines[1], "X") {
		t.Fatalf("X 应落在 L1(软换行片段行), got %q", lines)
	}
	if strings.Contains(lines[0], "X") {
		t.Fatalf("X 不得落进 L0(textarea 旧缺陷), got %q", lines)
	}
}

func TestEditorPasteNoContinuation(t *testing.T) {
	e := newTestEditor(t, "- a", 40)
	e.buf.Bottom() // 粘贴落在行尾
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune("\n- b")})
	e = m.(editor)
	if v := e.buf.Value(); v != "- a\n- b" {
		t.Fatalf("粘贴不走列表续写: %q", v)
	}
}

func TestEditorEscConfirm(t *testing.T) {
	e := newTestEditor(t, "hi", 40)
	e = typeString(e, "!")
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyEsc}) // 有改动 → 进确认
	e = m.(editor)
	if !e.confirm || !strings.Contains(e.View(), "放弃") {
		t.Fatalf("Esc 应进确认态")
	}
	m, _ = e.Update(keyRunes('n')) // n 取消
	e = m.(editor)
	if e.confirm {
		t.Fatalf("n 应退出确认态")
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	e = m.(editor)
	_, cmd := e.Update(keyRunes('y'))
	if cmd == nil {
		t.Fatalf("确认态 y 应产生 quit cmd")
	}
	e2 := newTestEditor(t, "hi", 40) // 未改动 → Esc 直接退出
	_, cmd2 := e2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd2 == nil {
		t.Fatalf("未改动 Esc 应产生 quit cmd")
	}
}

func TestCtrlSSavesWithoutExit(t *testing.T) {
	e := newTestEditor(t, "hi", 40)
	p := e.path
	e.buf.Bottom()
	e = typeString(e, "!")
	m, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	e = m.(editor)
	if cmd != nil {
		t.Fatal("Ctrl+S 只保存不退出")
	}
	if e.err != nil {
		t.Fatalf("保存失败: %v", e.err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "hi!" {
		t.Fatalf("文件: %q", data)
	}
	if !strings.Contains(e.View(), "已保存") {
		t.Fatalf("状态行应显示已保存: %q", e.View())
	}
	// 保存后 initial 对齐:Esc 直退不再确认
	_, cmd = e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("已保存后 Esc 应直退")
	}
}

func TestCtrlQExitsAfterSave(t *testing.T) {
	e := newTestEditor(t, "hi", 40)
	p := e.path
	e.buf.Bottom()
	e = typeString(e, "!")
	m, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	e = m.(editor)
	if cmd == nil {
		t.Fatal("Ctrl+Q 应保存并退出")
	}
	if e.err != nil {
		t.Fatalf("保存失败: %v", e.err)
	}
	data, _ := os.ReadFile(p)
	if string(data) != "hi!" {
		t.Fatalf("文件: %q", data)
	}
}

func TestUndoRedoKeys(t *testing.T) {
	e := newTestEditor(t, "ab", 40)
	e.buf.Bottom() // 光标行尾
	e.buf.PushUndo()
	e = typeString(e, "c")
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	e = m.(editor)
	if v := e.buf.Value(); v != "ab" {
		t.Fatalf("Ctrl+Z 撤销: %q", v)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	e = m.(editor)
	if v := e.buf.Value(); v != "abc" {
		t.Fatalf("Ctrl+R 重做: %q", v)
	}
}

func TestEditorCtrlSSavesCRLF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(p, []byte("a\r\nb\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newEditor(p, []byte("a\r\nb\r\n"))
	e.width, e.height = 40, 10
	e.buf.Bottom() // 光标在文档末(尾空行),追加 X
	e = typeString(e, "X")
	m, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	e = m.(editor)
	if cmd != nil {
		t.Fatal("Ctrl+S 只保存不退出")
	}
	if e.err != nil {
		t.Fatalf("保存不应报错: %v", e.err)
	}
	data, _ := os.ReadFile(p)
	if !bytes.Equal(data, []byte("a\r\nb\r\nX\r\n")) {
		t.Fatalf("CRLF 应原样还原: %q", data)
	}
}

func TestTypingReplacesSelection(t *testing.T) {
	e := newTestEditor(t, "abcdef", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	e = m.(editor)
	e = typeString(e, "X")
	if v := e.buf.Value(); v != "Xbcdef" {
		t.Fatalf("输入替换选区: %q", v)
	}
	if e.selOn {
		t.Fatal("编辑后选区应取消")
	}
}

func TestSelectionExtendAndDelete(t *testing.T) {
	e := newTestEditor(t, "abcdef\nghijkl", 40)
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyShiftRight},
		{Type: tea.KeyShiftRight},
		{Type: tea.KeyShiftDown},
	} {
		m, _ := e.Update(k)
		e = m.(editor)
	}
	if !e.selOn {
		t.Fatal("Shift+移动应激活选区")
	}
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	e = m.(editor)
	if v := e.buf.Value(); v != "ijkl" {
		t.Fatalf("跨行删选区: %q", v)
	}
}

func TestPasteReplacesSelection(t *testing.T) {
	e := newTestEditor(t, "abcdef", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	e = m.(editor)
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune("XY")})
	e = m.(editor)
	if v := e.buf.Value(); v != "XYbcdef" {
		t.Fatalf("粘贴替换选区: %q", v)
	}
}

func TestEscCancelsSelection(t *testing.T) {
	e := newTestEditor(t, "hi", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	e = m.(editor)
	if !e.selOn {
		t.Fatal("Shift+→ 应激活选区")
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	e = m.(editor)
	if e.selOn {
		t.Fatal("Esc 应仅取消选区")
	}
	if _, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEsc}); cmd == nil {
		t.Fatal("再次 Esc 才进入退出流(未改动直退)")
	}
}

func TestSelectionRender(t *testing.T) {
	e := newTestEditor(t, "abcdef", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyShiftRight})
	e = m.(editor)
	if !strings.Contains(e.View(), "\x1b[48;5;238m") {
		t.Fatalf("选区行应有暗底高亮: %q", e.View())
	}
}

func TestCtrlXCutNoSelectionNoop(t *testing.T) {
	e := newTestEditor(t, "abcdef", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	e = m.(editor)
	if v := e.buf.Value(); v != "abcdef" {
		t.Fatalf("无选区 Ctrl+X 应无操作: %q", v)
	}
}

func TestYankRotation(t *testing.T) {
	e := newTestEditor(t, "xx", 40)
	e.buf.Bottom()
	e.kills.push("一")
	e.kills.push("二")
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	e = m.(editor)
	if v := e.buf.Value(); v != "xx二" {
		t.Fatalf("最近条目: %q", v)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	e = m.(editor)
	if v := e.buf.Value(); v != "xx一" {
		t.Fatalf("连按轮换到更早: %q", v)
	}
	e = typeString(e, "!") // 其他按键打断轮换态
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	e = m.(editor)
	if v := e.buf.Value(); v != "xx一!二" {
		t.Fatalf("打断后重新从最近贴: %q", v)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	e = m.(editor)
	if v := e.buf.Value(); v != "xx一!" {
		t.Fatalf("yank 应可一步撤销: %q", v)
	}
}

func TestSearchFlow(t *testing.T) {
	e := newTestEditor(t, "Go语言\ngo 好用\nGO 语言", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	e = m.(editor)
	if !e.searching {
		t.Fatal("Ctrl+F 应进查找态")
	}
	e = typeString(e, "go")
	if len(e.matches) != 3 {
		t.Fatalf("忽略大小写 3 处: %v", e.matches)
	}
	if v := e.buf.Value(); v != "Go语言\ngo 好用\nGO 语言" {
		t.Fatalf("查找态输入不得改动正文: %q", v)
	}
	if !strings.Contains(e.View(), "查找") {
		t.Fatalf("查找态应显示查找行: %q", e.View())
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	e = m.(editor)
	if e.searching {
		t.Fatal("Enter 应退出查找态")
	}
	if line, col := e.buf.Cur(); line != 1 || col != 0 {
		t.Fatalf("Enter 跳光标后首个匹配: (%d,%d)", line, col)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyF3})
	e = m.(editor)
	if line, _ := e.buf.Cur(); line != 2 {
		t.Fatalf("F3 下一处: %d", line)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyF15}) // WT 的 Shift+F3
	e = m.(editor)
	if line, _ := e.buf.Cur(); line != 1 {
		t.Fatalf("Shift+F3 上一处: %d", line)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	e = m.(editor)
	if len(e.matches) != 3 {
		t.Fatal("Esc 退出输入态应保留高亮")
	}
	e = typeString(e, "Z") // 编辑后匹配重算(仍 3 处)
	if len(e.matches) != 3 {
		t.Fatalf("编辑后重算: %v", e.matches)
	}
}

func TestSearchBackspace(t *testing.T) {
	e := newTestEditor(t, "abc", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	e = m.(editor)
	e = typeString(e, "abc")
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	e = m.(editor)
	if e.q != "ab" || len(e.matches) != 1 {
		t.Fatalf("退格改词重算: q=%q matches=%v", e.q, e.matches)
	}
}

func TestAltMoveKeys(t *testing.T) {
	e := newTestEditor(t, "- 一\n- 二\n- 三", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: true})
	e = m.(editor)
	if v := e.buf.Value(); v != "- 二\n- 一\n- 三" {
		t.Fatalf("Alt+↓ 下移: %q", v)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyUp, Alt: true})
	e = m.(editor)
	if v := e.buf.Value(); v != "- 一\n- 二\n- 三" {
		t.Fatalf("Alt+↑ 上移: %q", v)
	}
}

func TestAltMoveBoundaryNoSpuriousUndo(t *testing.T) {
	e := newTestEditor(t, "a\nb", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyUp, Alt: true}) // 首行上移 no-op
	e = m.(editor)
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	e = m.(editor)
	if v := e.buf.Value(); v != "a\nb" {
		t.Fatalf("边界 no-op 不得产生空撤销步: %q", v)
	}
}

func TestTabIndentKeys(t *testing.T) {
	e := newTestEditor(t, "- 甲", 40)
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyTab})
	e = m.(editor)
	if v := e.buf.Value(); v != "  - 甲" {
		t.Fatalf("Tab 缩进: %q", v)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	e = m.(editor)
	if v := e.buf.Value(); v != "- 甲" {
		t.Fatalf("Shift+Tab 反缩进: %q", v)
	}
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyTab})
	e = m.(editor)
	m, _ = e.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	e = m.(editor)
	if v := e.buf.Value(); v != "- 甲" {
		t.Fatalf("缩进应一步撤销: %q", v)
	}
}

func TestEditorViewShowsIndentMarker(t *testing.T) {
	// "- Go语言中很多" 显示宽 14,width=12 → 折为 "- Go语言中很" + "多"
	e := newTestEditor(t, "- Go语言中很多", 12)
	rows := strings.Split(e.View(), "\n")
	found := false
	for _, r := range rows {
		if strings.HasPrefix(r, "  多") { // 延续片段有 2 空格缩进
			found = true
		}
	}
	if !found {
		t.Fatalf("软换行片段应有缩进标记: %q", rows)
	}
}

func TestTypingDropsControlRunes(t *testing.T) {
	// Ctrl+Space 等按键经终端上报为 NUL rune;入内容会让文件被判二进制(VS Code 打不开)
	e := newTestEditor(t, "", 40)
	m, _ := e.Update(keyRunes('a', 0, 'b'))
	e = m.(editor)
	if v := e.buf.Value(); v != "ab" {
		t.Fatalf("控制字符应被丢弃: %q", v)
	}
}

func TestPasteDropsControlRunes(t *testing.T) {
	e := newTestEditor(t, "x", 40)
	e.buf.Bottom()
	m, _ := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune("a\x00b")})
	e = m.(editor)
	if v := e.buf.Value(); v != "xab" {
		t.Fatalf("粘贴中的控制字符应被丢弃: %q", v)
	}
}
