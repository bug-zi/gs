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

func TestUnbindAndBindShip(t *testing.T) {
	rm := opsFixture(t)
	ref := PointRef{1, 0} // atomic
	if !rm.UnbindList(ref, RiverList, "Study/river/atomic.md") {
		t.Fatal("应命中 river 绑定")
	}
	if rm.UnbindList(ref, RiverList, "Study/river/atomic.md") {
		t.Fatal("二次卸载应未命中（幂等）")
	}
	if len(rm.PointAt(ref).River) != 0 {
		t.Fatalf("river 应为空: %+v", rm.PointAt(ref).River)
	}
	rm.BindShip(ref, "Study/ship/atomic.md")
	rm.BindShip(ref, "Study/ship/atomic.md") // 幂等
	if got := rm.PointAt(ref).Ship; len(got) != 1 || got[0] != "Study/ship/atomic.md" {
		t.Fatalf("ship = %+v", got)
	}
	rm.UnbindSpring(ref)
	if rm.PointAt(ref).Spring != "" {
		t.Fatal("spring 应清空")
	}
}

func TestMoveBinding(t *testing.T) {
	rm := opsFixture(t)
	src, dst := PointRef{1, 0}, PointRef{0, 0} // atomic → slice 底层结构与扩容
	if !rm.MoveBinding(src, RiverList, "Study/river/atomic.md", dst) {
		t.Fatal("移动应命中")
	}
	if rm.MoveBinding(src, RiverList, "Study/river/atomic.md", dst) {
		t.Fatal("二次移动应未命中（原子已无此绑定）")
	}
	if len(rm.PointAt(src).River) != 0 {
		t.Fatal("源 river 应为空")
	}
	if got := rm.PointAt(dst).River; len(got) != 1 || got[0] != "Study/river/atomic.md" {
		t.Fatalf("目标 river = %+v", got)
	}
}

func TestIsBoundAndOwner(t *testing.T) {
	rm := opsFixture(t)
	if !rm.IsBound("Study/river/atomic.md") {
		t.Fatal("river/atomic.md 应已绑定")
	}
	if rm.IsBound("Study/sea/无主.md") {
		t.Fatal("无主文件不应判定为已绑定")
	}
	ref, ok := rm.OwnerOf("Study/river/atomic.md")
	if !ok || ref != (PointRef{1, 0}) {
		t.Fatalf("OwnerOf = %+v ok=%v", ref, ok)
	}
}

func TestRenamePoint(t *testing.T) {
	rm := opsFixture(t)
	ref := PointRef{0, 0}
	// 约定路径跟随
	rm.PointAt(ref).Spring = "Study/spring/slice 底层结构与扩容.md"
	oldSpring, err := rm.RenamePoint(ref, "slice 扩容")
	if err != nil {
		t.Fatal(err)
	}
	if oldSpring != "Study/spring/slice 底层结构与扩容.md" {
		t.Fatalf("oldSpring = %q", oldSpring)
	}
	if got := rm.PointAt(ref).Spring; got != "Study/spring/slice 扩容.md" {
		t.Fatalf("spring 字段应跟随: %q", got)
	}
	// 非约定路径不动
	rm.PointAt(ref).Spring = "Study/spring/自定义.md"
	if oldSpring, err := rm.RenamePoint(ref, "slice 扩容2"); err != nil || oldSpring != "" {
		t.Fatalf("非约定路径 oldSpring=%q err=%v", oldSpring, err)
	}
	if got := rm.PointAt(ref).Spring; got != "Study/spring/自定义.md" {
		t.Fatalf("非约定 spring 不应动: %q", got)
	}
	// 全树重名报错
	if _, err := rm.RenamePoint(ref, "atomic"); err == nil {
		t.Fatal("重名应报错")
	}
	// 空标题报错
	if _, err := rm.RenamePoint(ref, "  "); err == nil {
		t.Fatal("空标题应报错")
	}
}

func TestMergePoints(t *testing.T) {
	rm := opsFixture(t)
	keep, absorb := PointRef{0, 1}, PointRef{1, 0} // map 底层 ← atomic
	kp, ap := rm.PointAt(keep), rm.PointAt(absorb)
	ap.Spring = "Study/spring/atomic.md"
	kp.Status = StatusDoing
	kp.Tags = []string{"高频"}
	ap.Tags = []string{"高频", "难"}
	ap.Note = "备注"
	rm.MergePoints(keep, absorb)
	if got := rm.PointAt(keep).Sea; len(got) != 0 {
		t.Fatalf("sea 应为空: %+v", got)
	}
	if got := rm.PointAt(keep).River; len(got) != 1 || got[0] != "Study/river/atomic.md" {
		t.Fatalf("river 应并入: %+v", got)
	}
	if got := rm.PointAt(keep).Spring; got != "Study/spring/atomic.md" {
		t.Fatalf("spring 空则补: %q", got)
	}
	if got := rm.PointAt(keep).Status; got != StatusDoing {
		t.Fatalf("状态应取高: %q", got)
	}
	if got := rm.PointAt(keep).Tags; len(got) != 2 || got[0] != "高频" || got[1] != "难" {
		t.Fatalf("tags 并集去重: %+v", got)
	}
	if got := rm.PointAt(keep).Note; got != "备注" {
		t.Fatalf("note 拼接: %q", got)
	}
	if len(rm.Branches[1].Points) != 0 {
		t.Fatal("absorb 应被删除")
	}
}

func TestSplitMoveDelete(t *testing.T) {
	rm := opsFixture(t)
	src := PointRef{0, 0}
	np, err := rm.SplitPoint(src, "slice 扩容实战")
	if err != nil {
		t.Fatal(err)
	}
	if np != (PointRef{0, 1}) {
		t.Fatalf("新点应插正后方: %+v", np)
	}
	np1 := rm.PointAt(np)
	if np1.Status != StatusTodo || np1.Group != "语法核心" || np1.Title != "slice 扩容实战" {
		t.Fatalf("新点字段: %+v", np1)
	}
	if _, err := rm.SplitPoint(src, "slice 底层结构与扩容"); err == nil {
		t.Fatal("与自身同名应报错")
	}
	// 换分组
	rm.SetGroup(np, "实战")
	if got := rm.PointAt(np).Group; got != "实战" {
		t.Fatalf("group = %q", got)
	}
	// 换主干
	rm.MovePointToBranch(np, 1)
	if last := len(rm.Branches[1].Points) - 1; rm.Branches[1].Points[last].Title != "slice 扩容实战" {
		t.Fatalf("应挪到目标主干末尾: %+v", rm.Branches[1].Points[last])
	}
	// 删除：非空拒绝、空点可删
	np = PointRef{1, len(rm.Branches[1].Points) - 1}
	if err := rm.DeletePoint(PointRef{1, 0}); err == nil { // atomic 有 river 绑定
		t.Fatal("非空点应拒绝删除")
	}
	if err := rm.DeletePoint(np); err != nil {
		t.Fatal(err)
	}
	if len(rm.Branches[1].Points) != 1 {
		t.Fatalf("删除后并发编程应剩 1 点: %d", len(rm.Branches[1].Points))
	}
}

func rmForDelete(t *testing.T) *Roadmap {
	t.Helper()
	rm, err := Parse([]byte("version = 2\n\n" +
		"[[branch]]\ntitle = \"并发\"\n\n" +
		"  [[branch.point]]\n  title = \"select\"\n" +
		"  river = [\"Study/river/01-并发/01-s.md\"]\n" +
		"  spring = \"Study/spring/select.md\"\n" +
		"  sea = [\"Study/sea/x.md\"]\n\n" +
		"[[branch]]\ntitle = \"语言\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

func TestOwnersOf(t *testing.T) {
	rm := rmForDelete(t)
	got := rm.OwnersOf("Study/sea/x.md")
	if len(got) != 1 || got[0] != (PointRef{Branch: 0, Point: 0}) {
		t.Fatalf("OwnersOf = %v", got)
	}
	if _, ok := rm.OwnerOf("Study/sea/x.md"); !ok {
		t.Fatal("OwnerOf 应命中")
	}
	if _, ok := rm.OwnerOf("Study/sea/none.md"); ok {
		t.Fatal("未绑定路径不应命中")
	}
}

func TestDetachPoint(t *testing.T) {
	rm := rmForDelete(t)
	ref := PointRef{Branch: 0, Point: 0}
	river := rm.DetachPoint(ref)
	if len(river) != 1 || river[0] != "Study/river/01-并发/01-s.md" {
		t.Fatalf("DetachPoint river = %v", river)
	}
	if !rm.PointEmpty(ref) {
		t.Fatal("清空后 PointEmpty 应为真")
	}
	if err := rm.DeletePoint(ref); err != nil {
		t.Fatalf("清空后 DeletePoint 应通过: %v", err)
	}
}

func TestDeleteBranch(t *testing.T) {
	rm := rmForDelete(t)
	if err := rm.DeleteBranch(0); err == nil {
		t.Fatal("非空主干应拒绝")
	}
	empty := rmForDelete(t)
	empty.DetachPoint(PointRef{Branch: 0, Point: 0})
	if err := empty.DeletePoint(PointRef{Branch: 0, Point: 0}); err != nil {
		t.Fatal(err)
	}
	if err := empty.DeleteBranch(0); err != nil {
		t.Fatalf("空主干应可删: %v", err)
	}
	if len(empty.Branches) != 1 || empty.Branches[0].Title != "语言" {
		t.Fatal("删除后剩一个主干")
	}
	if err := empty.DeleteBranch(0); err == nil {
		t.Fatal("最后一个主干应拒绝")
	}
}
