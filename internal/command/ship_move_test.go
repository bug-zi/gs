package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func shipMoveFixture(t *testing.T) (*workspace.Workspace, *Ctx, string) {
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
	mk("Study/sea/g/a.md", "# A\n\n> 出处：Study/sea/src.md\n\n正文\n")
	mk("Study/river/r.md", "# R\n\n> 出处：Study/sea/g/a.md\n")
	rmToml := "version = 2\n\n[[branch]]\ntitle = \"并发编程\"\n\n  [[branch.point]]\n  title = \"select\"\n  sea = [\"Study/sea/g/a.md\"]\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &Ctx{WS: ws, Out: &bytes.Buffer{}}
	return ws, ctx, "Study/sea/g/a.md"
}

func TestMoveToShip(t *testing.T) {
	ws, ctx, src := shipMoveFixture(t)
	dst, err := moveToShip(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if dst != "Study/ship/a.md" {
		t.Fatalf("dst = %s", dst)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "ship", "a.md")); err != nil {
		t.Fatalf("ship 文件缺失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "sea", "g", "a.md")); !os.IsNotExist(err) {
		t.Fatal("sea 原文件应已不在")
	}
	rm, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	p := rm.Branches[0].Points[0]
	if len(p.Ship) != 1 || p.Ship[0] != "Study/ship/a.md" {
		t.Fatalf("Ship 绑定未迁移: %v", p.Ship)
	}
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "river", "r.md"))
	if !strings.Contains(string(data), "> 出处：Study/ship/a.md") {
		t.Fatalf("出处链未改写：\n%s", data)
	}
}

func TestListTierFilesShip(t *testing.T) {
	ws, _, _ := shipMoveFixture(t)
	if err := os.MkdirAll(filepath.Join(ws.Root, "Study", "ship"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.Root, "Study", "ship", "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := ListTierFiles(ws, "ship")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "Study/ship/x.md" {
		t.Fatalf("files = %v", files)
	}
}
