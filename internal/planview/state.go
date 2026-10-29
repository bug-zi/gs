// Package planview /plan 交互式树状浏览器：纯状态机 + 整帧渲染 + raw 浏览循环。
// raw 单键复用 termkey；本包不 import repl/command。
package planview

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/ui"
)

type viewKind int

const (
	viewGlobal  viewKind = iota // 全局树：主干 + 展开的知识点
	viewBranch                  // 主干页：单主干知识点列表
	viewPoint                   // 知识点详情：spring/river/ship/sea 清单
	viewUnbound                 // 待归类区：五类未挂载文件
)

// Tier 绑定类型：详情页五清单与待归类区共用；TierNone 表示行不归属任何清单
type Tier int

const (
	TierNone Tier = iota
	TierSea
	TierShip
	TierRiver
	TierSpring
	TierCode
)

// tierOrder 待归类区分组顺序
var tierOrder = []Tier{TierSea, TierShip, TierRiver, TierSpring, TierCode}

// Label 清单标题文案（详情页节标题与待归类区分组共用前缀）
func (t Tier) Label() string {
	switch t {
	case TierSea:
		return "sea · 来源"
	case TierShip:
		return "ship · 加工中"
	case TierRiver:
		return "river · 沉淀"
	case TierSpring:
		return "spring · 我的总结"
	case TierCode:
		return "code · 演示代码"
	}
	return ""
}

// Color 层点缀色（低饱和，与主题金赭沙同亮度带），小节标题按层区分
func (t Tier) Color() string {
	switch t {
	case TierSea:
		return ui.ANSISea
	case TierShip:
		return ui.ANSIShip
	case TierRiver:
		return ui.ANSIRiver
	case TierSpring:
		return ui.ANSISpring
	case TierCode:
		return ui.ANSICode
	}
	return ""
}

// dir 层在 Study/ 下的目录名（笔记行去前缀显示用）
func (t Tier) dir() string {
	switch t {
	case TierSea:
		return "sea"
	case TierShip:
		return "ship"
	case TierRiver:
		return "river"
	case TierSpring:
		return "spring"
	case TierCode:
		return "code"
	}
	return ""
}

type rowKind int

const (
	rowBranchHead  rowKind = iota // 主干行（可选）
	rowGroupHead                  // 分组小标题（不可选）
	rowPoint                      // 知识点行（可选）
	rowSectionHead                // 详情页小节标题（不可选）
	rowNote                       // 笔记路径行（可选，可打开）
	rowHint                       // 提示行（不可选）
	rowUnboundHead                // 待归类区入口行（全局树尾，可选）
)

type row struct {
	Kind       rowKind
	Branch     int
	Point      int
	Note       string // rowNote：绑定路径
	Tier       Tier   // 行归属的绑定清单；TierNone = 无归属
	Dim        bool   // rowNote：灰显（sea 已装船/入河，出处已消费）
	Selectable bool
	Text       string
}

// Model 浏览器状态；RM 原地变更，落盘经 Hooks.Save 由壳层回调
type Model struct {
	RM           *roadmap.Roadmap
	Root         string                // 工作区根（失效判断用）
	Exists       func(rel string) bool // nil = os.Stat 默认实现（测试注入）
	Unbound      func(Tier) []string   // 壳层注入：未挂载扫描（nil = 恒空）
	unboundCache map[Tier][]string     // 未挂载快照：进入视图/每次落盘后刷新
	View         viewKind
	Branch       int
	Point        int
	Cursor       int
	Expanded     map[int]bool
	Err          string // 底部错误行（落盘失败等），按键后清除
}

// refreshUnbound 重扫未挂载清单（壳层回调，代价 = 五目录一次遍历）
func (m *Model) refreshUnbound() {
	m.unboundCache = map[Tier][]string{}
	if m.Unbound == nil {
		return
	}
	for _, t := range tierOrder {
		m.unboundCache[t] = m.Unbound(t)
	}
}

// Rows 依当前视图计算可见行
func (m *Model) Rows() []row {
	switch m.View {
	case viewGlobal:
		return m.globalRows()
	case viewBranch:
		return m.branchRows()
	case viewUnbound:
		return m.unboundRows()
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
	// 待归类区：无主文件集中地（反查计算视图，非物理目录）
	n := 0
	for _, t := range tierOrder {
		n += len(m.unboundCache[t])
	}
	text := "▸ 待归类区"
	if n > 0 {
		text = leader(text, fmt.Sprintf("%d", n))
	}
	rows = append(rows, row{Kind: rowUnboundHead, Selectable: true, Text: text})
	return rows
}

// unboundRows 待归类区：五类未挂载文件分组列出
func (m *Model) unboundRows() []row {
	var rows []row
	for _, t := range tierOrder {
		files := m.unboundCache[t]
		rows = append(rows, row{Kind: rowSectionHead, Tier: t,
			Text: fmt.Sprintf("%s · 未归类 ×%d", t.Label(), len(files))})
		for _, rel := range files {
			rows = append(rows, m.noteRow(t, rel))
		}
		if len(files) == 0 {
			rows = append(rows, row{Kind: rowHint, Tier: t, Text: "  （无）"})
		}
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
			Text: indent + "  " + roadmap.Glyph(p.Status) + " " + p.Title})
	}
	return rows
}

func (m *Model) pointRows() []row {
	p := m.curPoint()
	var rows []row
	rows = append(rows, row{Kind: rowSectionHead, Tier: TierSpring, Text: TierSpring.Label() + " ×0/1"})
	if p.Spring == "" {
		rows = append(rows, row{Kind: rowHint, Tier: TierSpring, Text: "  （尚未创建，/spring add " + p.Title + " 可创建并绑定）"})
	} else {
		rows = append(rows, m.noteRow(TierSpring, p.Spring))
	}
	rows = append(rows, row{Kind: rowSectionHead, Tier: TierCode, Text: TierCode.Label() + " ×0/1"})
	if p.Code == "" {
		rows = append(rows, row{Kind: rowHint, Tier: TierCode, Text: "  （尚未创建，/code add " + p.Title + " 可创建并绑定）"})
	} else {
		rows = append(rows, m.noteRow(TierCode, p.Code))
	}
	rows = append(rows, row{Kind: rowSectionHead, Tier: TierRiver, Text: fmt.Sprintf("%s ×%d", TierRiver.Label(), len(p.River))})
	for _, rel := range p.River {
		rows = append(rows, m.noteRow(TierRiver, rel))
	}
	if len(p.River) == 0 {
		rows = append(rows, row{Kind: rowHint, Tier: TierRiver, Text: "  （无；/ship move 入河后自动挂载）"})
	}
	rows = append(rows, row{Kind: rowSectionHead, Tier: TierShip, Text: fmt.Sprintf("%s ×%d", TierShip.Label(), len(p.Ship))})
	for _, rel := range p.Ship {
		rows = append(rows, m.noteRow(TierShip, rel))
	}
	if len(p.Ship) == 0 {
		rows = append(rows, row{Kind: rowHint, Tier: TierShip, Text: "  （无；/ship copy 装船后开始加工）"})
	}
	rows = append(rows, row{Kind: rowSectionHead, Tier: TierSea, Text: fmt.Sprintf("%s ×%d", TierSea.Label(), len(p.Sea))})
	for _, rel := range sortedSea(p) {
		r := m.noteRow(TierSea, rel)
		r.Dim = seaShipped(p, rel)
		rows = append(rows, r)
	}
	if len(p.Sea) == 0 {
		rows = append(rows, row{Kind: rowHint, Tier: TierSea, Text: "  （无）"})
	}
	return rows
}

// noteRow 笔记行：Note 保留全路径供打开/卸载；显示文本去 Study/<层>/ 前缀（小节标题已标明层）
func (m *Model) noteRow(t Tier, rel string) row {
	text := "  " + strings.TrimPrefix(rel, "Study/"+t.dir()+"/")
	if !m.exists(rel) {
		text += "　[失效]"
	}
	return row{Kind: rowNote, Note: rel, Tier: t, Selectable: true, Text: text}
}

// seaShipped 大海条目是否已装船/入河：同名文件已挂在本知识点 ship，或 river 去编号后同名。
// 两边都做编号归一（StripNumPrefix）：sea 原件常带源编号（1.2.0 xxx），装船/入河后往往
// 改名去号或领新号（02-xxx），按原始基名比对会漏标；归一后同名即视为同一出处。
// 与 /sea 选择器的已装船灰显同一约定，出处已消费则弱化显示。
func seaShipped(p *roadmap.Point, seaRel string) bool {
	base := roadmap.StripNumPrefix(filepath.Base(seaRel))
	for _, lists := range [][]string{p.Ship, p.River} {
		for _, rel := range lists {
			if roadmap.StripNumPrefix(filepath.Base(rel)) == base {
				return true
			}
		}
	}
	return false
}

// sortedSea sea 清单排序：未消费在前、已消费（已装船/入河标灰）在后，各自按名升序——
// 与 /sea 选择器 sortShippedFirst 同一约定，原材料一眼可辨。
func sortedSea(p *roadmap.Point) []string {
	sea := append([]string(nil), p.Sea...)
	sort.Slice(sea, func(i, j int) bool {
		di, dj := seaShipped(p, sea[i]), seaShipped(p, sea[j])
		if di != dj {
			return !di
		}
		return sea[i] < sea[j]
	})
	return sea
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

// Move 光标移动，循环回绕（底→下键回顶、顶→上键到底）：整圈依次探测，落第一个可选行；
// 沿途不可选行（分组/小节标题）自动跳过；整表无可选行则不动
func (m *Model) Move(delta int) {
	rows := m.Rows()
	n := len(rows)
	if n == 0 {
		return
	}
	c := m.Cursor
	for step := 0; step < n; step++ {
		c = ((c+delta)%n + n) % n
		if rows[c].Selectable {
			m.Cursor = c
			return
		}
	}
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
	ActIntoUnbound
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
	case m.View == viewGlobal && r.Kind == rowUnboundHead:
		m.refreshUnbound()
		m.View, m.Cursor = viewUnbound, 0
		m.Normalize()
		return ActIntoUnbound, ""
	case r.Kind == rowNote && (m.View == viewPoint || m.View == viewUnbound):
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
	case viewUnbound:
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

// PointAtCursor 光标所指知识点行（全局树/主干页）
func (m *Model) PointAtCursor() (roadmap.PointRef, bool) {
	r := m.rowAt(m.Cursor)
	if r != nil && r.Kind == rowPoint {
		return roadmap.PointRef{Branch: r.Branch, Point: r.Point}, true
	}
	return roadmap.PointRef{}, false
}

// curRef 详情页当前知识点
func (m *Model) curRef() roadmap.PointRef {
	return roadmap.PointRef{Branch: m.Branch, Point: m.Point}
}

// TierAtCursor 光标所在清单节（详情页与待归类区的行都带 Tier）
func (m *Model) TierAtCursor() (Tier, bool) {
	r := m.rowAt(m.Cursor)
	if r != nil && r.Tier != TierNone {
		return r.Tier, true
	}
	return TierNone, false
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

// leader 「title ···· progress」，目标总显示宽 40（中点至少 3 个）
func leader(title, progress string) string {
	w := runewidth.StringWidth(title) + runewidth.StringWidth(progress)
	dots := 40 - w - 2
	if dots < 3 {
		dots = 3
	}
	return title + " " + strings.Repeat("·", dots) + " " + progress
}
