package edit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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

func TestEditorCtrlSSavesCRLF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(p, []byte("a\r\nb\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newEditor(p, []byte("a\r\nb\r\n"))
	e.width, e.height = 40, 10
	e = typeString(e, "X") // 光标在文档末(尾空行),追加 X
	m, cmd := e.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatalf("Ctrl+S 应产生 quit cmd")
	}
	if m.(editor).err != nil {
		t.Fatalf("保存不应报错: %v", m.(editor).err)
	}
	data, _ := os.ReadFile(p)
	if !bytes.Equal(data, []byte("a\r\nb\r\nX\r\n")) {
		t.Fatalf("CRLF 应原样还原: %q", data)
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
