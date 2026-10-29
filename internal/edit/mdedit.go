package edit

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bugzi/gs/internal/ui"
)

const wrapIndent = "  " // 软换行延续片段缩进标记

var cursorStyle = lipgloss.NewStyle().Reverse(true)

var selStyle = lipgloss.NewStyle().Background(lipgloss.Color("238"))

var matchStyle = lipgloss.NewStyle().Background(lipgloss.Color("236"))

// editor 内联编辑 tea 模型;buffer/wrap 为纯逻辑,本文件只做按键分流、滚动与渲染
type editor struct {
	buf        *Buffer
	path       string
	crlf       bool
	hadNL      bool // 原文件以换行结尾,保存时补回(md 文件惯例)
	initial    string
	width      int
	height     int       // 编辑区可见显示行数
	offset     int       // 顶部显示行序号(全文档显示行坐标系)
	savedAt    string    // 最近保存时刻 HH:MM,下次编辑前显示
	kills      *killRing // 删除内容留存(Ctrl+Y 回贴)
	yankBase   *snapshot // 本轮 yank 起点,轮换时先恢复
	yankIdx    int       // 当前轮换偏移(0=最近)
	yankActive bool      // 上一动作是否为 Ctrl+Y(轮换窗口)
	searching  bool      // 查找输入态
	q          string    // 查询词
	matches    []match   // 当前全部匹配
	mi         int       // 当前匹配下标
	selOn      bool      // 选区激活
	selA       selPos    // 选区锚点(另一端为光标)
	err        error
	confirm    bool
}

func newEditor(path string, data []byte) editor {
	norm := normalizeCRLF(data)
	e := editor{
		buf:     NewBuffer(norm),
		path:    path,
		crlf:    bytes.Contains(data, []byte("\r\n")),
		hadNL:   strings.HasSuffix(norm, "\n"),
		initial: norm,
		kills:   &killRing{},
	}
	e.buf.Top() // 默认光标停在首行首列
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
		if msg.Type != tea.KeyCtrlY {
			e.yankActive = false
		}
		if e.searching { // 查找输入态截流全部按键
			switch msg.Type {
			case tea.KeyEsc:
				e.searching = false
			case tea.KeyEnter:
				e.searching = false
				e.jumpNextMatch()
			case tea.KeyBackspace:
				if rs := []rune(e.q); len(rs) > 0 {
					e.q = string(rs[:len(rs)-1])
					e.recalcMatches()
				}
			case tea.KeyRunes, tea.KeySpace:
				rs := msg.Runes
				if msg.Type == tea.KeySpace && len(rs) == 0 {
					rs = []rune{' '}
				}
				e.q += string(rs)
				e.recalcMatches()
			}
			return e, nil
		}
		switch msg.Type {
		case tea.KeyCtrlS:
			content := e.saveContent()
			if err := saveFile(e.path, content, e.crlf); err != nil {
				e.err = err
				return e, nil
			}
			e.markSaved(content)
			return e, nil
		case tea.KeyCtrlQ:
			content := e.saveContent()
			if err := saveFile(e.path, content, e.crlf); err != nil {
				e.err = err
				return e, nil
			}
			e.markSaved(content)
			return e, tea.Quit
		case tea.KeyCtrlL:
			return e, tea.ClearScreen
		case tea.KeyEsc:
			if e.selOn {
				e.selOn = false
				return e, nil
			}
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
			e.editPrep()
			e.buf.Insert(dropCtrl(rs)...)
		case tea.KeyEnter:
			e.editPrep()
			e.buf.Enter()
		case tea.KeyBackspace, tea.KeyDelete:
			if e.selOn {
				e.editPrep() // 选区:仅删选区
				break
			}
			e.buf.PushUndo()
			if msg.Type == tea.KeyBackspace {
				e.buf.Backspace()
			} else {
				e.buf.Delete()
			}
		case tea.KeyCtrlY:
			snap := e.buf.snap()
			if e.yankActive && e.yankBase != nil {
				e.buf.restore(*e.yankBase)
				e.yankIdx++
			} else {
				e.yankBase = &snap
				e.yankIdx = 0
			}
			if txt, ok := e.kills.yank(e.yankIdx); ok {
				e.yankActive = true
				e.buf.PushUndo()
				e.buf.Insert([]rune(txt)...)
			}
		case tea.KeyCtrlF:
			e.searching = true
			return e, nil
		case tea.KeyF3, tea.KeyF15: // F15=WT 的 Shift+F3
			if msg.Type == tea.KeyF15 {
				e.jumpMatch(-1)
			} else {
				e.jumpMatch(+1)
			}
			return e, nil
		case tea.KeyCtrlZ:
			e.buf.Undo()
		case tea.KeyCtrlR:
			e.buf.Redo()
		case tea.KeyCtrlC:
			if s, t, ok := e.selRange(); ok {
				e.copyOut(selText(e.buf.lines, s, t))
			} else {
				line, _ := e.buf.Cur()
				e.copyOut(string(e.buf.LineRunes(line)))
			}
			return e, nil
		case tea.KeyCtrlX:
			if s, t, ok := e.selRange(); ok {
				txt := selText(e.buf.lines, s, t)
				e.copyOut(txt)
				e.buf.PushUndo()
				e.kills.push(txt)
				delSel(e.buf, s, t)
				e.selOn = false
			} // 无选区:无操作(设计决策)
		// 本版 bubbletea 以独立键类型承载 Shift 修饰(无 Shift 字段):
		// KeyShiftLeft/Right/Up/Down/Home/End;WT 的 Shift+F3 映射为 KeyF15
		case tea.KeyShiftLeft:
			e.move(true, e.buf.Left)
		case tea.KeyShiftRight:
			e.move(true, e.buf.Right)
		case tea.KeyShiftUp:
			e.move(true, func() { e.buf.Up(e.width) })
		case tea.KeyShiftDown:
			e.move(true, func() { e.buf.Down(e.width) })
		case tea.KeyShiftHome:
			e.move(true, func() { e.buf.Home(e.width) })
		case tea.KeyShiftEnd:
			e.move(true, func() { e.buf.End(e.width) })
		case tea.KeyLeft:
			e.move(false, e.buf.Left)
		case tea.KeyRight:
			e.move(false, e.buf.Right)
		case tea.KeyUp:
			if msg.Alt {
				e.selOn = false
				if e.buf.CanMove(true) {
					e.buf.PushUndo()
					e.buf.MoveLine(true)
					e.recalcMatches()
				}
				e.followCursor()
				return e, nil
			}
			e.move(false, func() { e.buf.Up(e.width) })
		case tea.KeyDown:
			if msg.Alt {
				e.selOn = false
				if e.buf.CanMove(false) {
					e.buf.PushUndo()
					e.buf.MoveLine(false)
					e.recalcMatches()
				}
				e.followCursor()
				return e, nil
			}
			e.move(false, func() { e.buf.Down(e.width) })
		case tea.KeyTab, tea.KeyShiftTab:
			outd := msg.Type == tea.KeyShiftTab
			e.selOn = false
			if outd {
				if e.buf.CanOutdent() {
					e.buf.PushUndo()
					e.buf.Outdent()
				}
			} else {
				e.buf.PushUndo()
				e.buf.Indent()
			}
		case tea.KeyHome:
			e.move(false, func() { e.buf.Home(e.width) })
		case tea.KeyEnd:
			e.move(false, func() { e.buf.End(e.width) })
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
		e.recalcMatches()
		e.followCursor()
		return e, nil
	}
	return e, nil
}

// paste 粘贴文本;换行用 Break,不走列表续写
func (e *editor) paste(s string) {
	e.editPrep()
	for i, p := range strings.Split(normalizeCRLF([]byte(s)), "\n") {
		if i > 0 {
			e.buf.Break()
		}
		if p != "" {
			e.buf.Insert(dropCtrl([]rune(p))...)
		}
	}
}

// dropCtrl 丢弃 C0 控制字符与 DEL:终端会把 Ctrl+Space 等按键上报为 NUL rune,
// 入内容会让文件被判为二进制(编辑器打不开)
func dropCtrl(rs []rune) []rune {
	out := make([]rune, 0, len(rs))
	for _, r := range rs {
		if r >= 0x20 && r != 0x7f {
			out = append(out, r)
		}
	}
	return out
}

// selRange 当前选区归一化;未激活 ok=false
func (e editor) selRange() (selPos, selPos, bool) {
	if !e.selOn {
		return selPos{}, selPos{}, false
	}
	line, col := e.buf.Cur()
	s, t := normSel(e.selA, selPos{line, col})
	return s, t, true
}

// move 光标移动并维护选区:extend=true 延伸(首动定锚),否则取消选区
func (e *editor) move(extend bool, f func()) {
	if extend {
		if !e.selOn {
			line, col := e.buf.Cur()
			e.selA = selPos{line, col}
			e.selOn = true
		}
	} else {
		e.selOn = false
	}
	f()
}

// editPrep 一次可撤销编辑的前置:压栈;若有选区则删选区并入 kill ring
func (e *editor) editPrep() {
	e.buf.PushUndo()
	if s, t, ok := e.selRange(); ok {
		e.kills.push(selText(e.buf.lines, s, t))
		delSel(e.buf, s, t)
		e.selOn = false
	}
}

// copyOut 经 OSC52 写系统剪贴板(不支持 OSC52 的终端静默无效)
func (e editor) copyOut(s string) {
	os.Stdout.WriteString(osc52(s))
}

// recalcMatches 内容变更后重算;mi 定位到光标处或其后首个匹配,否则 0
func (e *editor) recalcMatches() {
	e.matches = findMatches(e.buf.lines, e.q)
	e.mi = 0
	line, col := e.buf.Cur()
	for i, m := range e.matches {
		if m.line > line || (m.line == line && m.col >= col) {
			e.mi = i
			return
		}
	}
}

func (e *editor) gotoMatch(i int) {
	m := e.matches[i]
	e.buf.cur, e.buf.col = m.line, m.col
	e.buf.resetWant()
	e.followCursor()
}

// jumpMatch 沿方向跳相邻匹配(回绕);无匹配不动
func (e *editor) jumpMatch(d int) {
	n := len(e.matches)
	if n == 0 {
		return
	}
	e.mi = ((e.mi+d)%n + n) % n
	e.gotoMatch(e.mi)
}

// jumpNextMatch 跳光标之后首个匹配,无则回绕首个
func (e *editor) jumpNextMatch() {
	if len(e.matches) == 0 {
		return
	}
	line, col := e.buf.Cur()
	for i, m := range e.matches {
		if m.line > line || (m.line == line && m.col > col) {
			e.mi = i
			e.gotoMatch(i)
			return
		}
	}
	e.mi = 0
	e.gotoMatch(0)
}

// saveContent 生成待保存内容(md 惯例补结尾换行)
func (e editor) saveContent() string {
	content := e.buf.Value()
	if e.hadNL && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content
}

// markSaved 对齐 initial(保存后 Esc 直退)并记录保存时刻
func (e *editor) markSaved(content string) {
	e.initial = content
	e.savedAt = time.Now().Format("15:04")
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
	} else if e.savedAt != "" {
		mod = " · 已保存 " + e.savedAt
	}
	fmt.Fprintf(&b, "编辑 %s · 行 %d,列 %d · %s · %s · %s%s\n\n",
		ui.Path(e.path), line+1, DispCol(e.buf.LineRunes(line), col)+1,
		ui.Dim("Ctrl+S 保存"), ui.Dim("Ctrl+Q 保存退出"), ui.Dim("Esc 退出"), ui.Dim(mod))
	if e.searching {
		fmt.Fprintf(&b, "%s %s · %d 处匹配 · %s\n\n",
			ui.Dim("查找"), e.q, len(e.matches), ui.Dim("Enter 下一个 · Esc 退出"))
	}
	rows := Layout(e.buf, e.width)
	curRow := CursorVRow(e.buf, e.width)
	end := e.offset + e.height
	if end > len(rows) {
		end = len(rows)
	}
	if e.offset < len(rows) {
		for i := e.offset; i < end; i++ {
			r := rows[i]
			use := i == curRow
			if !use && e.selOn {
				if s, t, ok := e.selRange(); ok {
					if _, _, hit := selSpan(r, s, t); hit {
						use = true
					}
				}
			}
			if use {
				b.WriteString(e.rowStyled(r))
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

// selSpan 本显示行上选区覆盖区间 [lo,hi);无覆盖 ok=false
func selSpan(r VRow, s, t selPos) (int, int, bool) {
	if r.Line < s.line || r.Line > t.line {
		return 0, 0, false
	}
	lo, hi := r.Start, r.End
	if r.Line == s.line && s.col > lo {
		lo = s.col
	}
	if r.Line == t.line && t.col < hi {
		hi = t.col
	}
	return lo, hi, hi > lo
}

// span 行内一段上样式区间
type span struct {
	lo, hi int
	st     lipgloss.Style
}

// rowStyled 渲染需上样式的一行:查找匹配(当前匹配反显)/选区暗底,光标格最优先
func (e editor) rowStyled(r VRow) string {
	_, col := e.buf.Cur()
	lr := e.buf.LineRunes(r.Line)
	var spans []span
	if s, t, ok := e.selRange(); ok {
		if a, c, hit := selSpan(r, s, t); hit {
			spans = append(spans, span{a, c, selStyle})
		}
	}
	for i, m := range e.matches {
		if m.line != r.Line || m.end <= r.Start || m.col >= r.End {
			continue
		}
		lo, hi := m.col, m.end
		if lo < r.Start {
			lo = r.Start
		}
		if hi > r.End {
			hi = r.End
		}
		st := matchStyle
		if i == e.mi {
			st = cursorStyle
		}
		spans = append(spans, span{lo, hi, st})
	}
	var b strings.Builder
	if DispCol(lr, r.Start) > 0 {
		b.WriteString(wrapIndent)
	}
	for i := r.Start; i <= r.End; i++ {
		if i == col {
			if col < r.End {
				b.WriteString(cursorStyle.Render(string(lr[col])))
			} else {
				b.WriteString(cursorStyle.Render(" "))
			}
			continue
		}
		s := ""
		if i < r.End {
			s = string(lr[i])
		}
		for _, sp := range spans { // 后入者覆盖:选区先入,匹配后入
			if i >= sp.lo && i < sp.hi {
				s = sp.st.Render(string(lr[i]))
			}
		}
		b.WriteString(s)
	}
	return b.String()
}
