package planview

import (
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/ui"
)

// window 视口窗口：不超过 avail 全显；超出则以光标为中心滑动，贴边取齐
func window(n, cur, avail int) (lo, hi int) {
	if avail <= 0 || n <= avail {
		return 0, n
	}
	lo = cur - avail/2
	if lo < 0 {
		lo = 0
	}
	if lo+avail > n {
		lo = n - avail
	}
	return lo, lo + avail
}

// renderRow 单行渲染：超宽截断；光标行整行反白；小标题加粗；提示行灰
func renderRow(r row, selected bool, width int) string {
	if runewidth.StringWidth(r.Text) > width {
		r.Text = runewidth.Truncate(r.Text, width, "…")
	}
	switch {
	case selected:
		return "\x1b[7m" + r.Text + "\x1b[27m"
	case r.Kind == rowGroupHead || r.Kind == rowSectionHead:
		return "\x1b[1m" + r.Text + "\x1b[22m"
	case r.Kind == rowHint:
		return ui.Dim(r.Text)
	default:
		return r.Text
	}
}

// viewTitle 各视图标题行
func viewTitle(m *Model) string {
	switch m.View {
	case viewGlobal:
		done, total := 0, 0
		for bi := range m.RM.Branches {
			d, t := m.progress(bi)
			done, total = done+d, total+t
		}
		pct := 0
		if total > 0 {
			pct = done * 100 / total
		}
		return fmt.Sprintf("\x1b[1mGo 学习路线图\x1b[22m　%d/%d (%d%%)", done, total, pct)
	case viewBranch:
		b := &m.RM.Branches[m.Branch]
		done, total := m.progress(m.Branch)
		return fmt.Sprintf("\x1b[1m%s\x1b[22m　%d/%d", b.Title, done, total)
	default:
		p := m.curPoint()
		return fmt.Sprintf("\x1b[1m%s\x1b[22m　%s　%s", p.Title, roadmap.Glyph(p.Status), m.RM.PathOf(roadmap.PointRef{Branch: m.Branch, Point: m.Point}))
	}
}

// viewHint 各视图键位提示
func viewHint(m *Model) string {
	switch m.View {
	case viewGlobal:
		return ui.Dim("↑↓/jk 移动　Tab 展开/收起　Enter 进入　d/s/t 标记　Esc/q 退出")
	case viewBranch:
		return ui.Dim("↑↓/jk 移动　Enter 看笔记　d/s/t 标记　Esc 返回　q 退出")
	default:
		return ui.Dim("↑↓ 选笔记　Enter/o 打开　d/s/t 标记状态　Esc 返回　q 退出")
	}
}

// drawFrame 重绘整帧：标题 + 键位提示 + 视口行（光标反白）+ 错误行；
// 写完光标移回帧首，供下次整体重绘覆盖
func drawFrame(out io.Writer, m *Model) {
	rows := m.Rows()
	avail := 0
	if h, ok := ui.PageSize(); ok {
		avail = h - 5
	}
	lo, hi := window(len(rows), m.Cursor, avail)
	width := ui.TermWidth(90)
	if width > 100 {
		width = 100
	}
	var lines []string
	if lo > 0 {
		lines = append(lines, ui.Dim(fmt.Sprintf("　…（上方还有 %d 行）", lo)))
	}
	for i := lo; i < hi; i++ {
		lines = append(lines, renderRow(rows[i], i == m.Cursor, width))
	}
	if hi < len(rows) {
		lines = append(lines, ui.Dim(fmt.Sprintf("　…（下方还有 %d 行）", len(rows)-hi)))
	}
	var b strings.Builder
	b.WriteString("\r\x1b[J")
	b.WriteString(viewTitle(m) + "\r\n")
	b.WriteString(viewHint(m) + "\r\n")
	for _, l := range lines {
		b.WriteString(l + "\r\n")
	}
	if m.Err != "" {
		b.WriteString(ui.Err(m.Err) + "\r\n")
	}
	back := 2 + len(lines) // 上移行数 = 标题+提示+选项，差一会使旧帧残留堆叠
	if m.Err != "" {
		back++ // 错误行也占一行
	}
	fmt.Fprintf(&b, "\x1b[%dA\r", back)
	fmt.Fprint(out, b.String())
}
