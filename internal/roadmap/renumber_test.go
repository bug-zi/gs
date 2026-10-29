package roadmap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripNumPrefix(t *testing.T) {
	cases := map[string]string{
		"5 中间件Middleware":  "中间件Middleware",
		"2.1 前置知识补充":       "前置知识补充",
		"1-3 整理：五级跳的演进时间线": "整理：五级跳的演进时间线",
		"03.1-切片陷阱汇总":      "切片陷阱汇总",
		"05-slice底层结构与扩容":  "slice底层结构与扩容",
		"12、超时控制":          "超时控制",
		"select":           "select", // 无前缀原样
		"7 ":               "7",      // 全被剥掉回退原名（去 .md 后）
	}
	for in, want := range cases {
		if got := StripNumPrefix(in); got != want {
			t.Errorf("StripNumPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextDupSeq(t *testing.T) {
	const pn = 3
	cases := []struct {
		existing []string
		want     int
	}{
		{nil, 0},
		{[]string{"Study/river/03-首篇.md"}, 1},
		{[]string{"Study/river/03-首篇.md", "Study/river/03.1-第二篇.md"}, 2},
		{[]string{"Study/river/03.1-a.md", "Study/river/03.3-c.md"}, 4},
		{[]string{"Study/river/13-别的点.md"}, 0}, // 其他点不干扰
	}
	for _, c := range cases {
		if got := NextDupSeq(c.existing, pn); got != c.want {
			t.Errorf("NextDupSeq(%v, %d) = %d, want %d", c.existing, pn, got, c.want)
		}
	}
}

func TestRiverFileName(t *testing.T) {
	if got := RiverFileName(3, 0, "变量与常量"); got != "03-变量与常量.md" {
		t.Errorf("首篇 = %q", got)
	}
	if got := RiverFileName(3, 2, "变量与常量"); got != "03.2-变量与常量.md" {
		t.Errorf("同点第 2 篇 = %q", got)
	}
}

func TestBuildRenumberPlan(t *testing.T) {
	rm := &Roadmap{Version: Version, Branches: []Branch{
		{Title: "语言基础", Points: []Point{
			{Title: "变量", River: []string{"Study/river/01-变量.md"}},
			{Title: "切片", River: []string{
				"Study/river/01-旧目录/02-切片底层.md", // 主干序已变（该点现处 branch 0 point 1 → 应为 02-）
				"Study/river/01-旧目录/02.1-切片陷阱.md",
			}},
		}},
		{Title: "并发编程", Points: []Point{
			{Title: "select", River: []string{"Study/river/并发/select.md"}}, // 无号名 → 01-select.md
		}},
	}}
	existing := []string{
		"Study/river/01-变量.md",
		"Study/river/01-旧目录/02-切片底层.md",
		"Study/river/01-旧目录/02.1-切片陷阱.md",
		"Study/river/并发/select.md",
		"Study/river/孤儿.md",           // 无绑定 → 99
		"Study/river/99-未挂载/已在收容区.md", // 已在 99 → 不动
	}
	plan := BuildRenumberPlan(rm, existing)

	wantTo := map[string]string{
		"Study/river/01-变量.md":            "Study/river/01-语言基础/01-变量.md",
		"Study/river/01-旧目录/02-切片底层.md":   "Study/river/01-语言基础/02-切片底层.md",
		"Study/river/01-旧目录/02.1-切片陷阱.md": "Study/river/01-语言基础/02.1-切片陷阱.md",
		"Study/river/并发/select.md":        "Study/river/02-并发编程/01-select.md",
		"Study/river/孤儿.md":               "Study/river/99-未挂载/孤儿.md",
	}
	got := map[string]string{}
	for _, m := range plan.Moves {
		got[m.From] = m.To
	}
	for _, m := range plan.Orphans {
		got[m.From] = m.To
	}
	if len(got) != len(wantTo) {
		t.Fatalf("计划条目数 = %d, want %d\nplan = %+v", len(got), len(wantTo), plan)
	}
	for from, to := range wantTo {
		if got[from] != to {
			t.Errorf("%s → %s, want %s", from, got[from], to)
		}
	}
}

func TestBuildUnmountMoves(t *testing.T) {
	rels := []string{
		"Study/river/01-a/01-x.md",
		"Study/river/99-未挂载/dup.md",
	}
	moves := BuildUnmountMoves(rels, []string{"dup.md"})
	if len(moves) != 2 {
		t.Fatalf("moves = %v", moves)
	}
	if moves[0].To != "Study/river/99-未挂载/01-x.md" {
		t.Fatalf("moves[0] = %v", moves[0])
	}
	if moves[1].To != "Study/river/99-未挂载/dup.1.md" {
		t.Fatalf("撞名应顺延 .1，got %v", moves[1])
	}
}

func TestListUnmounted(t *testing.T) {
	root := t.TempDir()
	if got := ListUnmounted(root); got != nil {
		t.Fatalf("目录缺失应返回 nil，got %v", got)
	}
	dir := filepath.Join(root, RiverDirRel, UnmountedDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n"), 0o644)
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)
	got := ListUnmounted(root)
	if len(got) != 1 || got[0] != "a.md" {
		t.Fatalf("ListUnmounted = %v（子目录不应计入）", got)
	}
}
