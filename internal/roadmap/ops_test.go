package roadmap

import (
	"strings"
	"testing"
)

func opsFixture(t *testing.T) *Roadmap {
	t.Helper()
	rm, err := Parse([]byte(opsTOML))
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

const opsTOML = `version = 2

[[branch]]
title = "语言基础"

  [[branch.point]]
  title = "slice 底层结构与扩容"
  group = "语法核心"
  status = "done"
  tags = ["高频"]

  [[branch.point]]
  title = "map 底层与负载因子"
  group = "语法核心"

  [[branch.point]]
  title = "iface/eface 底层"
  group = "方法与接口"

[[branch]]
title = "并发编程"

  [[branch.point]]
  title = "atomic"
  group = "同步原语"
  river = ["Study/river/atomic.md"]
`

func TestMatchPoints(t *testing.T) {
	rm := opsFixture(t)
	got := rm.MatchPoints("map")
	if len(got) != 1 || got[0].Branch != 0 || got[0].Point != 1 {
		t.Fatalf("MatchPoints(map) = %+v", got)
	}
	if got := rm.MatchPoints("mutex"); len(got) != 0 {
		t.Fatalf("应零匹配: %+v", got)
	}
	if got := rm.MatchPoints("ATOMIC"); len(got) != 1 { // 大小写不敏感
		t.Fatalf("MatchPoints(ATOMIC) = %+v", got)
	}
}

func TestMatchBranchesAndPath(t *testing.T) {
	rm := opsFixture(t)
	if got := rm.MatchBranches("并发"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("MatchBranches = %+v", got)
	}
	if got := rm.PathOf(PointRef{1, 0}); got != "并发编程 › atomic" {
		t.Fatalf("PathOf = %s", got)
	}
}

func TestProgressAndSetStatus(t *testing.T) {
	rm := opsFixture(t)
	if rm.TotalPoints() != 4 || rm.DonePoints() != 1 {
		t.Fatalf("进度: %d/%d", rm.DonePoints(), rm.TotalPoints())
	}
	done, total := rm.BranchProgress(0)
	if done != 1 || total != 3 {
		t.Fatalf("BranchProgress(0) = %d/%d", done, total)
	}
	if err := rm.SetStatus(PointRef{1, 0}, StatusDoing); err != nil {
		t.Fatal(err)
	}
	if rm.PointAt(PointRef{1, 0}).Status != StatusDoing {
		t.Fatal("状态未变更")
	}
	if err := rm.SetStatus(PointRef{1, 0}, "paused"); err == nil {
		t.Fatal("非法状态应报错")
	}
}

func TestAdd(t *testing.T) {
	rm := opsFixture(t)
	if err := rm.AddBranch("并发编程"); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("主干重名应报错: %v", err)
	}
	// v2 全局查重：跨主干也不允许重名
	if err := rm.AddPoint(1, "atomic", "同步原语", nil); err == nil || !strings.Contains(err.Error(), "唯一") {
		t.Fatalf("跨主干重名应报错: %v", err)
	}
	if err := rm.AddPoint(1, "sync.Cond", "同步原语", []string{"高频"}); err != nil {
		t.Fatal(err)
	}
	p := rm.PointAt(PointRef{1, 1})
	if p.Status != StatusTodo || p.Group != "同步原语" || len(p.Tags) != 1 {
		t.Fatalf("新知识点 = %+v", p)
	}
	if err := rm.AddPoint(1, "  ", "", nil); err == nil {
		t.Fatal("空标题应报错")
	}
}

func TestBind(t *testing.T) {
	rm := opsFixture(t)
	ref := PointRef{1, 0}
	rm.BindSpring(ref, "Study/spring/atomic.md")
	rm.BindSpring(ref, "Study/spring/atomic2.md") // 覆盖（0/1 语义）
	if rm.PointAt(ref).Spring != "Study/spring/atomic2.md" {
		t.Fatalf("spring = %q", rm.PointAt(ref).Spring)
	}
	rm.BindRiver(ref, "Study/river/a.md")
	rm.BindRiver(ref, "Study/river/a.md") // 幂等
	rm.BindCode(ref, "Study/code/atomic.go")
	rm.BindCode(ref, "Study/code/atomic2.go") // 覆盖（0/1 语义）
	if rm.PointAt(ref).Code != "Study/code/atomic2.go" {
		t.Fatalf("code = %q", rm.PointAt(ref).Code)
	}
	rm.BindSea(ref, "Study/sea/a.md")
	p := rm.PointAt(ref)
	// fixture 原有 1 条 river，追加去重后共 2 条
	if len(p.River) != 2 || len(p.Sea) != 1 {
		t.Fatalf("绑定 = %+v", p)
	}
}
