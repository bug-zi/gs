package edit

import (
	"bytes"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bugzi/gs/internal/ui"
)

const wrapIndent = "  " // 软换行延续片段缩进标记

var cursorStyle = lipgloss.NewStyle().Reverse(true)

// editor 内联编辑 tea 模型;buffer/wrap 为纯逻辑,本文件只做按键分流、滚动与渲染
type editor struct {
	buf     *Buffer
	path    string
	crlf    bool
	hadNL   bool // 原文件以换行结尾,保存时补回(md 文件惯例)
	initial string
	width   int
	height  int // 编辑区可见显示行数
	offset  int // 顶部显示行序号(全文档显示行坐标系)
	err     error
	confirm bool
}

func newEditor(path string, data []byte) editor {
	norm := normalizeCRLF(data)
	e := editor{
		buf:     NewBuffer(norm),
		path:    path,
		crlf:    bytes.Contains(data, []byte("\r\n")),
		hadNL:   strings.HasSuffix(norm, "\n"),
		initial: norm,
	}
	e.buf.Bottom() // 打开既有文件时光标落在文档末,续写体验与旧 textarea 一致
	return e
}

func (e editor) Init() tea.Cmd { return nil }

func (e editor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height - 3 // 标题行 + 空行 + 确认/错误行余量
		if e.height < 1 {
			e.height = 1
		}
		e.clampOffset()
		return e, nil
	case tea.KeyMsg:
		switch {
		case msg.Paste: // bracketed paste(默认启用),整段插入
			e.paste(string(msg.Runes))
			e.followCursor()
			return e, nil
		}
		switch msg.Type {
		case tea.KeyCtrlS:
			content := e.buf.Value()
			if e.hadNL && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			if err := saveFile(e.path, content, e.crlf); err != nil {
				e.err = err
				return e, nil
			}
			return e, tea.Quit
		case tea.KeyEsc:
			if e.confirm {
				e.confirm = false
				return e, nil
			}
			if e.buf.Value() != e.initial {
				e.confirm = true
				return e, nil
			}
			return e, tea.Quit
		}
		if e.confirm { // 确认态吞掉其余按键
			switch strings.ToLower(msg.String()) {
			case "y":
				return e, tea.Quit
			case "n":
				e.confirm = false
			}
			return e, nil
		}
		switch msg.Type {
		case tea.KeyRunes, tea.KeySpace:
			rs := msg.Runes
			if msg.Type == tea.KeySpace && len(rs) == 0 {
				rs = []rune{' '}
			}
			e.buf.Insert(rs...)
		case tea.KeyEnter:
			e.buf.Enter()
		case tea.KeyBackspace:
			e.buf.Backspace()
		case tea.KeyDelete:
			e.buf.Delete()
		case tea.KeyLeft:
			e.buf.Left()
		case tea.KeyRight:
			e.buf.Right()
		case tea.KeyUp:
			e.buf.Up(e.width)
		case tea.KeyDown:
			e.buf.Down(e.width)
		case tea.KeyHome:
			e.buf.Home(e.width)
		case tea.KeyEnd:
			e.buf.End(e.width)
		case tea.KeyPgUp:
			e.offset -= e.height
			e.clampOffset()
			return e, nil
		case tea.KeyPgDown:
			e.offset += e.height
			e.clampOffset()
			return e, nil
		default:
			return e, nil
		}
		e.followCursor()
		return e, nil
	}
	return e, nil
}

// paste 粘贴文本;换行用 Break,不走列表续写
func (e *editor) paste(s string) {
	for i, p := range strings.Split(normalizeCRLF([]byte(s)), "\n") {
		if i > 0 {
			e.buf.Break()
		}
		if p != "" {
			e.buf.Insert([]rune(p)...)
		}
	}
}

// clampOffset 把 offset 夹回合法范围
func (e *editor) clampOffset() {
	if e.width < 1 || e.height < 1 {
		return
	}
	n := e.rowCount()
	if n < e.height {
		n = e.height
	}
	if e.offset > n-e.height {
		e.offset = n - e.height
	}
	if e.offset < 0 {
		e.offset = 0
	}
}

func (e *editor) rowCount() int { return len(Layout(e.buf, e.width)) }

// followCursor 滚动保证光标显示行可见
func (e *editor) followCursor() {
	if e.width < 1 || e.height < 1 {
		return
	}
	i := CursorVRow(e.buf, e.width)
	if i < e.offset {
		e.offset = i
	} else if i >= e.offset+e.height {
		e.offset = i - e.height + 1
	}
}

func (e editor) View() string {
	var b strings.Builder
	line, col := e.buf.Cur()
	mod := ""
	if e.buf.Value() != e.initial {
		mod = " · 已修改"
	}
	fmt.Fprintf(&b, "编辑 %s · 行 %d,列 %d · %s · %s%s\n\n",
		ui.Path(e.path), line+1, DispCol(e.buf.LineRunes(line), col)+1,
		ui.Dim("Ctrl+S 保存退出"), ui.Dim("Esc 退出"), ui.Dim(mod))
	rows := Layout(e.buf, e.width)
	curRow := CursorVRow(e.buf, e.width)
	end := e.offset + e.height
	if end > len(rows) {
		end = len(rows)
	}
	if e.offset < len(rows) {
		for i := e.offset; i < end; i++ {
			r := rows[i]
			if i == curRow {
				b.WriteString(e.cursorRowStr(r))
			} else {
				b.WriteString(e.rowText(r))
			}
			b.WriteByte('\n')
		}
	}
	if e.confirm {
		b.WriteString("\n有未保存修改，放弃并退出？(y/N)")
	}
	if e.err != nil {
		fmt.Fprintf(&b, "\n保存失败: %v", e.err)
	}
	return b.String()
}

// rowText 普通显示行;延续片段(逻辑行中段起)加缩进标记
func (e editor) rowText(r VRow) string {
	lr := e.buf.LineRunes(r.Line)
	s := string(lr[r.Start:r.End])
	if DispCol(lr, r.Start) > 0 {
		s = wrapIndent + s
	}
	return s
}

// cursorRowStr 光标所在显示行:光标字符反显(行尾/幻影行为反显空格)
func (e editor) cursorRowStr(r VRow) string {
	line, col := e.buf.Cur()
	lr := e.buf.LineRunes(line)
	s := ""
	if DispCol(lr, r.Start) > 0 {
		s = wrapIndent
	}
	if col < r.Start || col > r.End {
		return s + string(lr[r.Start:r.End])
	}
	if col < r.End {
		return s + string(lr[r.Start:col]) + cursorStyle.Render(string(lr[col])) + string(lr[col+1:r.End])
	}
	// col == End:光标落在行末空格(含幻影行)
	return s + string(lr[r.Start:col]) + cursorStyle.Render(" ")
}
