package command

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func riverMoveFixture(t *testing.T) (*workspace.Workspace, *Ctx, *roadmap.Roadmap) {
	t.Helper()
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 分支 0「语言基础」占位，分支 1「并发编程」（点位号 02 文件夹），point 下标 2 为目标（点位号 03）
	rmToml := "version = 2\n\n" +
		"[[branch]]\ntitle = \"语言基础\"\n\n  [[branch.point]]\n  title = \"变量与常量\"\n\n" +
		"[[branch]]\ntitle = \"并发编程\"\n\n" +
		"  [[branch.point]]\n  title = \"goroutine\"\n\n" +
		"  [[branch.point]]\n  title = \"channel\"\n\n" +
		"  [[branch.point]]\n  title = \"select\"\n" +
		"  river = [\"Study/river/02-并发编程/03-首篇.md\"]\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	mk("Study/river/02-并发编程/03-首篇.md", "# 首篇\n")
	mk("Study/ship/b.md", "# B\n\n> 出处：Study/sea/x.md\n")
	rm, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	return ws, &Ctx{WS: ws, Out: &bytes.Buffer{}}, rm
}

func TestRiverLandingBoundNumbering(t *testing.T) {
	ws, ctx, rm := riverMoveFixture(t)
	ref := roadmap.PointRef{Branch: 1, Point: 2}
	dst, err := riverLanding(ctx, rm, "Study/ship/b.md", ref, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "Study/river/02-并发编程/03.1-b.md"
	if dst != want {
		t.Fatalf("dst = %s, want %s", dst, want)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, filepath.FromSlash(want))); err != nil {
		t.Fatalf("目标缺失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "ship", "b.md")); !os.IsNotExist(err) {
		t.Fatal("ship 原文件应已不在")
	}
}

func TestRiverLandingUnboundShelter(t *testing.T) {
	ws, ctx, _ := riverMoveFixture(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/ship/u.md", "# U\n")
	dst, err := riverLanding(ctx, nil, "Study/ship/u.md", roadmap.PointRef{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if dst != "Study/river/99-未挂载/u.md" {
		t.Fatalf("dst = %s", dst)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "river", "99-未挂载", "u.md")); err != nil {
		t.Fatalf("应落入收容区: %v", err)
	}
}

func TestRiverLandingCollisionBumpsDup(t *testing.T) {
	ws, ctx, rm := riverMoveFixture(t)
	// 预置同名目标已存在 → 篇序应顺延到 .2
	if err := os.MkdirAll(filepath.Join(ws.Root, "Study", "river", "02-并发编程"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.Root, "Study", "river", "02-并发编程", "03.1-b.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst, err := riverLanding(ctx, rm, "Study/ship/b.md", roadmap.PointRef{Branch: 1, Point: 2}, true)
	if err != nil {
		t.Fatal(err)
	}
	if dst != "Study/river/02-并发编程/03.2-b.md" {
		t.Fatalf("dst = %s, want 03.2", dst)
	}
}
