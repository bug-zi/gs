package command

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func renumberFixture(t *testing.T) (*workspace.Workspace, *Ctx) {
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
	rmToml := "version = 2\n\n[[branch]]\ntitle = \"并发编程\"\n\n  [[branch.point]]\n  title = \"select\"\n  river = [\"Study/river/并发/select.md\"]\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	mk("Study/river/并发/select.md", "# select\n\n> 出处：Study/sea/s.md\n")
	mk("Study/river/孤儿.md", "# 孤儿\n")
	mk(".gs/topics.toml", "[[topics]]\nname = \"演示\"\ncreated = \"2026-10-27\"\n\n[[topics.files]]\npath = \"Study/river/并发/select.md\"\nhint = \"\"\n")
	return ws, &Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"renumber"}}
}

func TestRunPlanRenumberEndToEnd(t *testing.T) {
	ws, ctx := renumberFixture(t)
	plan, err := buildRenumberPreview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("应产出重排计划")
	}
	if err := applyRenumberPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"Study/river/01-并发编程/01-select.md", "Study/river/99-未挂载/孤儿.md"} {
		if _, err := os.Stat(filepath.Join(ws.Root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s 缺失: %v", rel, err)
		}
	}
	rm, _ := roadmap.Load(ws.RoadmapFile())
	if got := rm.Branches[0].Points[0].River; len(got) != 1 || got[0] != "Study/river/01-并发编程/01-select.md" {
		t.Fatalf("River 绑定未改写: %v", got)
	}
	ix, _ := index.Load(ws.IndexPath())
	found := false
	for _, tp := range ix.Topics {
		for _, f := range tp.Files {
			if f.Path == "Study/river/01-并发编程/01-select.md" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("topics.toml 未改写")
	}
	// 幂等：重跑应报告无需重排（preview 返回 nil plan）
	ctx2 := &Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"renumber"}}
	plan2, err := buildRenumberPreview(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if plan2 != nil {
		t.Fatalf("重跑应无需重排，仍得计划：%+v", plan2)
	}
}

func TestRewriteTopicsNoFileNoop(t *testing.T) {
	ws := wsForTest(t)
	n, err := rewriteTopics(ws, []roadmap.FileMove{{From: "a", To: "b"}})
	if err != nil || n != 0 {
		t.Fatalf("无索引文件应 no-op: n=%d err=%v", n, err)
	}
	if _, err := os.Stat(ws.IndexPath()); !os.IsNotExist(err) {
		t.Fatal("不应创建 topics.toml")
	}
}
