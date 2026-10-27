package planview

import (
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
`))
	if err != nil {
		t.Fatal(err)
	}
	return &Model{RM: rm, Root: t.TempDir(), View: viewGlobal, Expanded: map[int]bool{}}
}

func TestGlobalRowsAndToggle(t *testing.T) {
	m := stateFixture(t)
	rows := m.Rows()
	if len(rows) != 2 || rows[0].Kind != rowBranchHead {
		t.Fatalf("收起时应只有主干行: %+v", rows)
	}
	m.Cursor = 0
	m.Toggle() // 展开 Go 语言基础
	if !m.Expanded[0] {
		t.Fatal("应展开")
	}
	rows = m.Rows()
	// 主干 + 分组标题 + 2 知识点 + 主干
	if len(rows) != 5 {
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
	// spring 小节+提示、code 小节+提示、river 小节+1 条笔记、sea 小节+提示 = 8 行
	if len(rows) != 8 {
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
