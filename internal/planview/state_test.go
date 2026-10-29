package planview

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

func stateFixture(t *testing.T) *Model {
	t.Helper()
	rm, err := roadmap.Parse([]byte(`version = 2

[[branch]]
title = "语言基础"

  [[branch.point]]
  title = "变量与常量"
  group = "语法核心"

  [[branch.point]]
  title = "slice 底层结构与扩容"
  group = "语法核心"
  status = "done"
  tags = ["高频"]

[[branch]]
title = "并发编程"

  [[branch.point]]
  title = "atomic"
  river = ["Study/river/atomic.md"]
  ship = ["Study/ship/atomic.md"]
`))
	if err != nil {
		t.Fatal(err)
	}
	return &Model{RM: rm, Root: t.TempDir(), View: viewGlobal, Expanded: map[int]bool{}}
}

func TestGlobalRowsAndToggle(t *testing.T) {
	m := stateFixture(t)
	m.refreshUnbound() // 待归类区计数缓存（Unbound 未注入则计 0）
	rows := m.Rows()
	// 2 主干 + 尾部待归类区行
	if len(rows) != 3 || rows[0].Kind != rowBranchHead {
		t.Fatalf("收起时应只有主干行+待归类区行: %+v", rows)
	}
	m.Cursor = 0
	m.Toggle() // 展开 Go 语言基础
	if !m.Expanded[0] {
		t.Fatal("应展开")
	}
	rows = m.Rows()
	// 主干 + 分组标题 + 2 知识点 + 主干 + 待归类区行
	if len(rows) != 6 {
		t.Fatalf("展开后行数 = %d\n%+v", len(rows), rows)
	}
	if rows[1].Kind != rowGroupHead || rows[1].Text != "    语法核心" {
		t.Fatalf("分组标题行不符: %+v", rows[1])
	}
	m.Toggle() // 光标仍在主干行 → 收起
	if m.Expanded[0] {
		t.Fatal("应收起")
	}
}

func TestMoveSkipsUnselectable(t *testing.T) {
	m := stateFixture(t)
	m.Expanded[0] = true
	m.Normalize() // 光标 0 = 主干行（可选）
	m.Move(1)     // → 应跳过分组标题停在第一个知识点
	rows := m.Rows()
	if rows[m.Cursor].Kind != rowPoint {
		t.Fatalf("应停在知识点行，got %+v", rows[m.Cursor])
	}
	if rows[m.Cursor].Point != 0 {
		t.Fatalf("应为第一个知识点: %+v", rows[m.Cursor])
	}
	m.Move(-1)
	if m.Cursor != 0 {
		t.Fatalf("上移应回主干行，cursor=%d", m.Cursor)
	}
}

func TestEnterAndBack(t *testing.T) {
	m := stateFixture(t)
	m.Expanded[1] = true
	m.Focus("并发") // 定位并发编程并展开
	if !m.Expanded[1] {
		t.Fatal("Focus 应展开目标主干")
	}
	// 光标在并发编程主干行，Enter → 主干页
	m.Normalize()
	act, _ := m.Enter()
	if act != ActIntoBranch || m.View != viewBranch {
		t.Fatalf("act=%d view=%d", act, m.View)
	}
	if m.Branch != 1 {
		t.Fatalf("Branch = %d", m.Branch)
	}
	// 主干页第一行可能是分组标题（atomic 有组）→ Enter 前先 Normalize
	m.Normalize()
	if act, _ = m.Enter(); act != ActIntoPoint || m.View != viewPoint {
		t.Fatalf("act=%d view=%d", act, m.View)
	}
	// 详情页 Back → 主干页 → 全局树 → 再 Back 应 false
	if !m.Back() || m.View != viewBranch {
		t.Fatal("详情页 Back 应回主干页")
	}
	if !m.Back() || m.View != viewGlobal {
		t.Fatal("主干页 Back 应回全局树")
	}
	if m.Back() {
		t.Fatal("全局树 Back 应返回 false（退出信号）")
	}
}

func TestPointRowsAndOpenNote(t *testing.T) {
	m := stateFixture(t)
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	rows := m.Rows()
	// spring 小节+提示、code 小节+提示、river 小节+1 条笔记、ship 小节+1 条笔记、sea 小节+提示 = 10 行
	if len(rows) != 10 {
		t.Fatalf("详情页行数 = %d\n%+v", len(rows), rows)
	}
	if !strings.Contains(rows[0].Text, "spring") || rows[1].Selectable {
		t.Fatalf("spring 未建应为不可选提示行: %+v", rows[1])
	}
	if !strings.Contains(rows[2].Text, "code") || rows[3].Selectable {
		t.Fatalf("code 未建应为不可选提示行: %+v", rows[3])
	}
	// 光标定位到 river 笔记行，Enter → OpenNote
	for i, r := range rows {
		if r.Kind == rowNote {
			m.Cursor = i
			break
		}
	}
	act, rel := m.Enter()
	if act != ActOpenNote || rel != "Study/river/atomic.md" {
		t.Fatalf("act=%d rel=%q", act, rel)
	}
}

func TestPointRowsShipSection(t *testing.T) {
	m := stateFixture(t)
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	rows := m.Rows()
	// 段顺序固定：spring / code / river / ship / sea
	var heads []string
	for _, r := range rows {
		if r.Kind == rowSectionHead {
			heads = append(heads, r.Text)
		}
	}
	want := []string{"spring", "code", "river", "ship", "sea"}
	if len(heads) != len(want) {
		t.Fatalf("小节数 = %d：%+v", len(heads), heads)
	}
	for i, w := range want {
		if !strings.Contains(heads[i], w) {
			t.Errorf("第 %d 段 = %q, want 含 %q", i, heads[i], w)
		}
	}
	if !strings.Contains(heads[3], "×1") {
		t.Errorf("ship 段应计 1 篇：%q", heads[3])
	}
	// ship 笔记行可选且可打开（Note 保留全路径，显示文本已去层前缀）
	found := false
	for _, r := range rows {
		if r.Kind == rowNote && r.Note == "Study/ship/atomic.md" {
			found = r.Selectable
		}
	}
	if !found {
		t.Fatal("ship 笔记行应可选")
	}
}

// TestPointRowsShortPath 详情页笔记行显示去 Study/<层>/ 前缀；Note 仍为全路径
func TestPointRowsShortPath(t *testing.T) {
	m := stateFixture(t)
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	rows := m.Rows()
	for _, r := range rows {
		if r.Kind != rowNote {
			continue
		}
		if strings.Contains(r.Text, "Study/") {
			t.Errorf("详情页笔记行应去层前缀: %+v", r)
		}
		if !strings.HasSuffix(strings.TrimSuffix(r.Text, "　[失效]"), filepath.Base(r.Note)) {
			t.Errorf("显示文本应保留文件名: text=%q note=%q", r.Text, r.Note)
		}
	}
}

// TestSeaShippedDim sea 条目已装船（ship 同名）或已入河（去编号同名）应灰显，未装船不灰。
// sea 原件带源编号（1.2.0 xxx）、装船/入河后改名或领号（02.1-xxx），编号归一后仍应命中。
func TestSeaShippedDim(t *testing.T) {
	m := stateFixture(t)
	p := m.RM.PointAt(roadmap.PointRef{Branch: 1, Point: 0})
	p.Ship = []string{"Study/ship/已装船.md", "Study/ship/1.3.0 装船改名.md"}
	p.River = []string{"Study/river/01-已入河.md", "Study/river/01-Go 语言基础/02.1-入河领号.md"}
	p.Sea = []string{
		"Study/sea/已装船.md",
		"Study/sea/已入河.md",
		"Study/sea/Go语言基础/1.2.0 装船改名.md",
		"Study/sea/Go语言基础/1.2.1 入河领号.md",
		"Study/sea/新材料.md",
	}
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	rows := m.Rows()
	dim := map[string]bool{}
	for _, r := range rows {
		if r.Kind == rowNote && r.Tier == TierSea {
			dim[r.Note] = r.Dim
		}
	}
	if !dim["Study/sea/已装船.md"] {
		t.Error("ship 同名已装船应灰显")
	}
	if !dim["Study/sea/已入河.md"] {
		t.Error("river 去编号同名已入河应灰显")
	}
	if !dim["Study/sea/Go语言基础/1.2.0 装船改名.md"] {
		t.Error("源编号与 ship 改名归一后应灰显")
	}
	if !dim["Study/sea/Go语言基础/1.2.1 入河领号.md"] {
		t.Error("源编号与 river 领号归一后应灰显")
	}
	if dim["Study/sea/新材料.md"] {
		t.Error("未装船不应灰显")
	}
}

// TestSeaRowsShippedLast 详情页 sea 清单排序与 /sea 选择器同一约定：
// 未消费（未标灰）在前、已消费（标灰）在后，各自按名升序，与绑定顺序无关。
func TestSeaRowsShippedLast(t *testing.T) {
	m := stateFixture(t)
	p := m.RM.PointAt(roadmap.PointRef{Branch: 1, Point: 0})
	p.Ship = []string{"Study/ship/函数.md", "Study/ship/多返回值.md"}
	p.Sea = []string{
		"Study/sea/Go语言基础/1.3.0 函数.md",
		"Study/sea/Go语言基础/1.8.2 匿名函数.md",
		"Study/sea/Go语言基础/1.8.5 多返回值.md",
		"Study/sea/Go语言基础/1.8.3 闭包场景.md",
	}
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	var got []string
	for _, r := range m.Rows() {
		if r.Kind == rowNote && r.Tier == TierSea {
			got = append(got, r.Note)
		}
	}
	want := []string{
		"Study/sea/Go语言基础/1.8.2 匿名函数.md",
		"Study/sea/Go语言基础/1.8.3 闭包场景.md",
		"Study/sea/Go语言基础/1.3.0 函数.md",
		"Study/sea/Go语言基础/1.8.5 多返回值.md",
	}
	if len(got) != len(want) {
		t.Fatalf("sea 行数 = %d, want %d：%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 行 = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestMoveWrap 光标循环回绕：末行 ↓ 应回顶（落最近可选行），首行 ↑ 底到底
func TestMoveWrap(t *testing.T) {
	m := stateFixture(t)
	m.Branch, m.View = 0, viewBranch // 分支 0：分组标题 + 两个知识点行
	m.Normalize()
	rows := m.Rows()

	m.Cursor = len(rows) - 1
	m.Move(1)
	if !rows[m.Cursor].Selectable {
		t.Fatalf("回绕后应落在可选行: cursor=%d", m.Cursor)
	}
	if m.Cursor >= len(rows)-1 {
		t.Fatalf("末行 ↓ 应回绕到顶部区域: cursor=%d", m.Cursor)
	}

	m.Cursor = 0
	m.Move(-1)
	if !rows[m.Cursor].Selectable {
		t.Fatalf("反向回绕后应落在可选行: cursor=%d", m.Cursor)
	}
	if m.Cursor != len(rows)-1 {
		t.Fatalf("首行 ↑ 应回绕到末行: cursor=%d, want %d", m.Cursor, len(rows)-1)
	}

	// 首个可选行 ↑ 也应回绕到底（详情页顶部是笔记行，上邻小节标题不可选）
	m.Cursor = 1
	m.Move(-1)
	if m.Cursor != len(rows)-1 {
		t.Fatalf("首个可选行 ↑ 应回绕到末行: cursor=%d, want %d", m.Cursor, len(rows)-1)
	}
}

// TestTierColor 五层点缀色齐备且互不相同
func TestTierColor(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range tierOrder {
		c := tc.Color()
		if c == "" {
			t.Errorf("%s 层应有点缀色", tc.Label())
		}
		if seen[c] {
			t.Errorf("%s 层颜色重复: %q", tc.Label(), c)
		}
		seen[c] = true
	}
}

func TestPointRowsCode失效标注(t *testing.T) {
	m := stateFixture(t)
	m.RM.PointAt(roadmap.PointRef{Branch: 1, Point: 0}).Code = "Study/code/atomic.go"
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	m.Exists = func(rel string) bool { return false } // 注入失效
	rows := m.Rows()
	note := rows[3]
	if note.Kind != rowNote || !strings.Contains(note.Text, "[失效]") {
		t.Fatalf("code 绑定失效应标注: %+v", note)
	}
	m.Cursor = 3
	act, rel := m.Enter()
	if act != ActOpenNote || rel != "Study/code/atomic.go" {
		t.Fatalf("act=%d rel=%q", act, rel)
	}
}

func TestMark(t *testing.T) {
	m := stateFixture(t)
	m.Expanded[0] = true
	m.Normalize()
	m.Move(1) // 第一个知识点
	ref, ok := m.Mark("done")
	if !ok || ref != (roadmap.PointRef{Branch: 0, Point: 0}) {
		t.Fatalf("Mark = %+v %v", ref, ok)
	}
	if m.RM.PointAt(roadmap.PointRef{Branch: 0, Point: 0}).Status != "done" {
		t.Fatalf("状态未变更: %+v", m.RM.PointAt(roadmap.PointRef{Branch: 0, Point: 0}))
	}
	// 详情页：恒定命中当前知识点
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	if ref, ok := m.Mark("doing"); !ok || ref != (roadmap.PointRef{Branch: 1, Point: 0}) {
		t.Fatalf("Mark = %+v %v", ref, ok)
	}
	if got := m.RM.PointAt(roadmap.PointRef{Branch: 1, Point: 0}).Status; got != "doing" {
		t.Fatalf("status = %s", got)
	}
}

func TestNoteRowInvalidMarked(t *testing.T) {
	m := stateFixture(t)
	m.Branch, m.Point, m.View = 1, 0, viewPoint
	m.Exists = func(string) bool { return false } // 注入：全部失效
	rows := m.Rows()
	for _, r := range rows {
		if r.Kind == rowNote && !strings.Contains(r.Text, "[失效]") {
			t.Fatalf("失效笔记应标注: %+v", r)
		}
	}
}

func TestTierAndPointAtCursor(t *testing.T) {
	m := stateFixture(t)
	m.View, m.Branch, m.Point = viewPoint, 1, 0 // atomic 详情页（river/ship 各 1 条）
	rows := m.Rows()
	var riverIdx int
	for i, r := range rows {
		if r.Kind == rowNote && r.Tier == TierRiver {
			riverIdx = i
			break
		}
	}
	m.Cursor = riverIdx
	if tier, ok := m.TierAtCursor(); !ok || tier != TierRiver {
		t.Fatalf("TierAtCursor = %v %v", tier, ok)
	}
	if rel := rows[m.Cursor].Note; rel != "Study/river/atomic.md" {
		t.Fatalf("note = %q", rel)
	}
	m.View = viewBranch
	m.Branch, m.Cursor = 1, 0
	m.Normalize() // 跳过分组标题行到知识点行
	if ref, ok := m.PointAtCursor(); !ok || ref != (roadmap.PointRef{Branch: 1, Point: 0}) {
		t.Fatalf("PointAtCursor = %+v %v", ref, ok)
	}
}

func TestUnboundView(t *testing.T) {
	m := stateFixture(t)
	m.Unbound = func(t2 Tier) []string {
		if t2 == TierSea {
			return []string{"Study/sea/主题/无主.md"}
		}
		return nil
	}
	m.refreshUnbound()
	// 全局树尾应有待归类区行
	rows := m.Rows()
	last := rows[len(rows)-1]
	if last.Kind != rowUnboundHead {
		t.Fatalf("全局树尾应为待归类区行: %+v", last)
	}
	// Enter 进入待归类区
	m.Cursor = len(rows) - 1
	m.Enter()
	if m.View != viewUnbound {
		t.Fatalf("应进入 viewUnbound: %v", m.View)
	}
	rows = m.Rows()
	// sea 节标题+1 文件 + 其余 4 节各 标题+提示 = 2 + 8
	if len(rows) != 10 {
		t.Fatalf("待归类区行数 = %d\n%+v", len(rows), rows)
	}
	if rows[0].Kind != rowSectionHead || rows[0].Tier != TierSea {
		t.Fatalf("首行应为 sea 节标题: %+v", rows[0])
	}
	if rows[1].Kind != rowNote || rows[1].Note != "Study/sea/主题/无主.md" {
		t.Fatalf("第二行应为无主文件: %+v", rows[1])
	}
	// Back 回全局树
	if !m.Back() || m.View != viewGlobal {
		t.Fatal("Back 应回全局树")
	}
}

func TestGlobalRowCountUpdated(t *testing.T) {
	m := stateFixture(t)
	m.Unbound = func(Tier) []string { return nil }
	m.refreshUnbound()
	rows := m.Rows()
	if rows[len(rows)-1].Kind != rowUnboundHead {
		t.Fatal("无未挂载时也应有待归类区行（计 0）")
	}
}
