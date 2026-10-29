package command

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func tierDelFixture(t *testing.T) (*workspace.Workspace, *Ctx) {
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
	rmToml := "version = 2\n\n" +
		"[[branch]]\ntitle = \"并发编程\"\n\n" +
		"  [[branch.point]]\n  title = \"select\"\n" +
		"  river = [\"Study/river/01-并发编程/01-首篇.md\"]\n" +
		"  spring = \"Study/spring/select.md\"\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	mk("Study/river/01-并发编程/01-首篇.md", "# 首篇\n")
	mk("Study/spring/select.md", "# select\n\n> 出处：Study/river/01-并发编程/01-首篇.md\n")
	ix := index.New()
	ix.Topics = append(ix.Topics, index.Topic{Name: "t", Files: []index.FileRef{
		{Path: "Study/river/01-并发编程/01-首篇.md"},
		{Path: "Study/sea/keep.md"},
	}})
	if err := ix.Save(ws.IndexPath()); err != nil {
		t.Fatal(err)
	}
	return ws, &Ctx{WS: ws, Out: &bytes.Buffer{}}
}

const delRiverRel = "Study/river/01-并发编程/01-首篇.md"

func TestApplyTierDeleteRiver(t *testing.T) {
	ws, ctx := tierDelFixture(t)
	if err := applyTierDelete(ctx, "river", delRiverRel); err != nil {
		t.Fatal(err)
	}
	// 文件已入回收站
	trashed := ""
	err := filepath.WalkDir(filepath.Join(ws.Root, ".gs", "trash"),
		func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() == "01-首篇.md" {
				trashed = p
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if trashed == "" {
		t.Fatal("回收站中找不到已删条目")
	}
	// 绑定已清
	rm2, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	if len(rm2.Branches[0].Points[0].River) != 0 {
		t.Fatal("river 绑定未清")
	}
	// topics 已清理
	ix, _ := index.Load(ws.IndexPath())
	if n := ix.CountPaths(map[string]bool{delRiverRel: true}); n != 0 {
		t.Fatal("topics 条目未清理")
	}
	// 出处链原文保留（只警告不改写）
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "spring", "select.md"))
	if !strings.Contains(string(data), "> 出处："+delRiverRel) {
		t.Fatal("出处链不应改写")
	}
	// 提示包含失效警告与 topics 清理数
	out := ctx.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "出处链引用 1 处已失效") {
		t.Fatalf("缺出处链警告: %s", out)
	}
	if !strings.Contains(out, "topics.toml 清除 1 条") {
		t.Fatalf("缺 topics 清理提示: %s", out)
	}
}

func TestPrintTierDeletePreview(t *testing.T) {
	_, ctx := tierDelFixture(t)
	printTierDeletePreview(ctx, "river", delRiverRel)
	out := ctx.Out.(*bytes.Buffer).String()
	for _, want := range []string{
		"删除 river 条目",
		"绑定清理",
		"并发编程 › select",
		"topics.toml 将清除 1 条",
		"出处链引用 1 处将失效",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("预览缺 %q:\n%s", want, out)
		}
	}
}

func TestTierCmdsHaveDelSub(t *testing.T) {
	for _, c := range []Command{newSea(), newShip(), newRiver(), newSpring(), newCode()} {
		found := false
		for _, s := range c.Subs {
			if s == "del" {
				found = true
			}
		}
		if !found {
			t.Fatalf("/%s 缺 del 子命令", c.Name)
		}
	}
}
