// Package planview /plan 交互式树状浏览器：纯状态机 + 整帧渲染 + raw 浏览循环。
// raw 单键复用 termkey；本包不 import repl/command。
package planview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/bugzi/gs/internal/roadmap"
)

type viewKind int

const (
	viewGlobal viewKind = iota // 全局树：主干 + 展开的知识点
	viewBranch                 // 主干页：单主干知识点列表
	viewPoint                  // 知识点详情：spring/river/ship/sea 清单
)

type rowKind int

const (
	rowBranchHead  rowKind = iota // 主干行（可选）
	rowGroupHead                  // 分组小标题（不可选）
	rowPoint                      // 知识点行（可选）
	rowSectionHead                // 详情页小节标题（不可选）
	rowNote                       // 笔记路径行（可选，可打开）
	rowHint                       // 提示行（不可选）
)

type row struct {
	Kind       rowKind
	Branch     int
	Point      int
	Note       string // rowNote：绑定路径
	Selectable bool
	Text       string
}

// Model 浏览器状态；RM 原地变更，落盘经 Hooks.Save 由壳层回调
type Model struct {
	RM       *roadmap.Roadmap
	Root     string                // 工作区根（失效判断用）
	Exists   func(rel string) bool // nil = os.Stat 默认实现（测试注入）
	View     viewKind
	Branch   int
	Point    int
	Cursor   int
	Expanded map[int]bool
	Err      string // 底部错误行（落盘失败等），按键后清除
}

// Rows 依当前视图计算可见行
func (m *Model) Rows() []row {
	switch m.View {
	case viewGlobal:
		return m.globalRows()
	case viewBranch:
		return m.branchRows()
	default:
		return m.pointRows()
	}
}

func (m *Model) globalRows() []row {
	var rows []row
	for bi := range m.RM.Branches {
		b := &m.RM.Branches[bi]
		mark := "▸ "
		if open(bi, m.Expanded) {
			mark = "▾ "
		}
		text := mark + b.Title
		if done, total := m.progress(bi); total > 0 {
			text = leader(text, fmt.Sprintf("%d/%d", done, total))
		}
		rows = append(rows, row{Kind: rowBranchHead, Branch: bi, Selectable: true, Text: text})
		if !open(bi, m.Expanded) {
			continue
		}
		rows = appendPointRows(rows, b, bi, "    ")
	}
	return rows
}

func (m *Model) branchRows() []row {
	b := &m.RM.Branches[m.Branch]
	return appendPointRows(nil, b, m.Branch, "  ")
}

// appendPointRows 追加知识点行；分组标签变化时插入小标题行（同组连续只出一次）
func appendPointRows(rows []row, b *roadmap.Branch, bi int, indent string) []row {
	prev := ""
	for pi := range b.Points {
		p := &b.Points[pi]
		if p.Group != "" && p.Group != prev {
			rows = append(rows, row{Kind: rowGroupHead, Branch: bi, Text: indent + p.Group})
			prev = p.Group
		} else if p.Group == "" {
			prev = ""
		}
		rows = append(rows, row{Kind: rowPoint, Branch: bi, Point: pi, Selectable: true,
			Text: indent + "  " + roadmap.Glyph(p.Status) + " " + p.Title + star(p.Tags)})
	}
	return rows
}

func (m *Model) pointRows() []row {
	p := m.curPoint()
	var rows []row
	rows = append(rows, row{Kind: rowSectionHead, Text: "spring · 我的总结 ×0/1"})
	if p.Spring == "" {
		rows = append(rows, row{Kind: rowHint, Text: "  （尚未创建，/spring add " + p.Title + " 可创建并绑定）"})
	} else {
		rows = append(rows, m.noteRow(p.Spring))
	}
	rows = append(rows, row{Kind: rowSectionHead, Text: "code · 演示代码 ×0/1"})
	if p.Code == "" {
		rows = append(rows, row{Kind: rowHint, Text: "  （尚未创建，/code add " + p.Title + " 可创建并绑定）"})
	} else {
		rows = append(rows, m.noteRow(p.Code))
	}
	rows = append(rows, row{Kind: rowSectionHead, Text: fmt.Sprintf("river · 沉淀 ×%d", len(p.River))})
	for _, rel := range p.River {
		rows = append(rows, m.noteRow(rel))
	}
	if len(p.River) == 0 {
		rows = append(rows, row{Kind: rowHint, Text: "  （无；/river move 入河后自动挂载）"})
	}
	rows = append(rows, row{Kind: rowSectionHead, Text: fmt.Sprintf("ship · 加工中 ×%d", len(p.Ship))})
	for _, rel := range p.Ship {
		rows = append(rows, m.noteRow(rel))
	}
	if len(p.Ship) == 0 {
		rows = append(rows, row{Kind: rowHint, Text: "  （无；/ship move 装船后开始加工）"})
	}
	rows = append(rows, row{Kind: rowSectionHead, Text: fmt.Sprintf("sea · 来源 ×%d", len(p.Sea))})
	for _, rel := range p.Sea {
		rows = append(rows, m.noteRow(rel))
	}
	if len(p.Sea) == 0 {
		rows = append(rows, row{Kind: rowHint, Text: "  （无）"})
	}
	return rows
}

func (m *Model) noteRow(rel string) row {
	text := "  " + rel
	if !m.exists(rel) {
		text += "　[失效]"
	}
	return row{Kind: rowNote, Note: rel, Selectable: true, Text: text}
}

func (m *Model) exists(rel string) bool {
	if m.Exists != nil {
		return m.Exists(rel)
	}
	_, err := os.Stat(filepath.Join(m.Root, filepath.FromSlash(rel)))
	return err == nil
}

// Normalize 光标夹取与可选行校正：越界拉回、落在不可选行时移到最近可选行
func (m *Model) Normalize() {
	rows := m.Rows()
	if len(rows) == 0 {
		m.Cursor = 0
		return
	}
	if m.Cursor >= len(rows) {
		m.Cursor = len(rows) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if rows[m.Cursor].Selectable {
		return
	}
	for i := m.Cursor; i < len(rows); i++ {
		if rows[i].Selectable {
			m.Cursor = i
			return
		}
	}
	for i := m.Cursor; i >= 0; i-- {
		if rows[i].Selectable {
			m.Cursor = i
			return
		}
	}
}

// Move 光标移动；落在不可选行（分组/小节标题）时沿移动方向找最近可选行，移到头即停
func (m *Model) Move(delta int) {
	rows := m.Rows()
	c := m.Cursor + delta
	if c < 0 || c >= len(rows) {
		return
	}
	m.Cursor = c
	if rows[c].Selectable {
		return
	}
	for i := c + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].Selectable {
			m.Cursor = i
			return
		}
	}
	m.Normalize() // 移动方向无可选行时反向兜底
}

// Toggle 全局树主干行展开/收起；其余情况无效
func (m *Model) Toggle() {
	if m.View != viewGlobal {
		return
	}
	if r := m.rowAt(m.Cursor); r != nil && r.Kind == rowBranchHead {
		m.Expanded[r.Branch] = !m.Expanded[r.Branch]
	}
}

// Action Enter 的语义结果
type Action int

const (
	ActNone Action = iota
	ActIntoBranch
	ActIntoPoint
	ActOpenNote
)

// Enter 进入：主干行→主干页；知识点行→详情页；笔记行→打开。返回动作与笔记路径
func (m *Model) Enter() (Action, string) {
	r := m.rowAt(m.Cursor)
	if r == nil || !r.Selectable {
		return ActNone, ""
	}
	switch {
	case m.View == viewGlobal && r.Kind == rowBranchHead:
		m.Branch, m.View, m.Cursor = r.Branch, viewBranch, 0
		m.Normalize()
		return ActIntoBranch, ""
	case m.View == viewGlobal && r.Kind == rowPoint:
		m.Branch, m.Point, m.View, m.Cursor = r.Branch, r.Point, viewPoint, 0
		m.Normalize()
		return ActIntoPoint, ""
	case m.View == viewBranch && r.Kind == rowPoint:
		m.Point, m.View, m.Cursor = r.Point, viewPoint, 0
		m.Normalize()
		return ActIntoPoint, ""
	case m.View == viewPoint && r.Kind == rowNote:
		return ActOpenNote, r.Note
	}
	return ActNone, ""
}

// Back 返回上一层；全局树返回 false（顶层，调用方退出浏览器）
func (m *Model) Back() bool {
	switch m.View {
	case viewPoint:
		m.View, m.Cursor = viewBranch, 0
		m.Normalize()
		return true
	case viewBranch:
		m.View, m.Cursor = viewGlobal, 0
		m.Normalize()
		return true
	}
	return false
}

// Mark 状态标记：详情页恒定命中当前知识点，其余视图命中光标知识点行。
// 返回命中的知识点与是否命中。
func (m *Model) Mark(status string) (roadmap.PointRef, bool) {
	if m.View == viewPoint {
		ref := roadmap.PointRef{Branch: m.Branch, Point: m.Point}
		_ = m.RM.SetStatus(ref, status)
		return ref, true
	}
	if r := m.rowAt(m.Cursor); r != nil && r.Kind == rowPoint {
		ref := roadmap.PointRef{Branch: r.Branch, Point: r.Point}
		_ = m.RM.SetStatus(ref, status)
		return ref, true
	}
	return roadmap.PointRef{}, false
}

// Focus 全局树中定位关键词主干：展开并移动光标到其主干行；找不到不动
func (m *Model) Focus(kw string) {
	kw = strings.ToLower(kw)
	for bi := range m.RM.Branches {
		if !strings.Contains(strings.ToLower(m.RM.Branches[bi].Title), kw) {
			continue
		}
		m.Expanded[bi] = true
		for i, r := range m.Rows() {
			if r.Kind == rowBranchHead && r.Branch == bi {
				m.Cursor = i
				return
			}
		}
		return
	}
}

func (m *Model) rowAt(i int) *row {
	rows := m.Rows()
	if i < 0 || i >= len(rows) {
		return nil
	}
	return &rows[i]
}

func (m *Model) curPoint() *roadmap.Point {
	return &m.RM.Branches[m.Branch].Points[m.Point]
}

func (m *Model) progress(bi int) (done, total int) {
	for pi := range m.RM.Branches[bi].Points {
		total++
		if m.RM.Branches[bi].Points[pi].Status == roadmap.StatusDone {
			done++
		}
	}
	return done, total
}

func open(bi int, expanded map[int]bool) bool { return expanded[bi] }

func star(tags []string) string {
	for _, tg := range tags {
		if strings.Contains(tg, "高频") {
			return " *"
		}
	}
	return ""
}

// leader 「title ···· progress」，目标总显示宽 40（中点至少 3 个）
func leader(title, progress string) string {
	w := runewidth.StringWidth(title) + runewidth.StringWidth(progress)
	dots := 40 - w - 2
	if dots < 3 {
		dots = 3
	}
	return title + " " + strings.Repeat("·", dots) + " " + progress
}
