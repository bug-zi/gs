package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/structure"
	"github.com/bugzi/gs/internal/workspace"
)

// buildTreeProject 建临时项目结构：main.go + web/{a.go,b.go} + web/api/c.go
func buildTreeProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir := func(rel string) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkdir("web")
	mkdir("web/api")
	write("main.go")
	write("web/a.go")
	write("web/b.go")
	write("web/api/c.go")
	return root
}

func TestFlattenTreeCollapsedByDefault(t *testing.T) {
	root := buildTreeProject(t)
	cache := &structure.StructureCache{}
	cache.Merge([]structure.Node{
		{Path: "web", Desc: "前端页面"},
		{Path: "web/a.go", Desc: "路由"},
	})
	matched := map[string][]structure.Note{
		"main.go": {{Abs: "/x.md", Display: "x.md"}},
	}
	rows := flattenTree("", root, nil, map[string]bool{}, cache, matched)
	// 默认全折叠：只有根层 2 行，web 不展开
	if len(rows) != 2 {
		t.Fatalf("默认应只有根层 2 行，得 %d:\n%+v", len(rows), rows)
	}
	if rows[0].rel != "web" || !rows[0].isDir || rows[0].expanded {
		t.Fatalf("首行应为折叠的 web 目录: %+v", rows[0])
	}
	if rows[0].head != "├── ▸ web/" || rows[0].desc != "前端页面" {
		t.Fatalf("web 行不符: %+v", rows[0])
	}
	if rows[1].head != "└──   main.go" || rows[1].notes != 1 {
		t.Fatalf("main.go 行不符: %+v", rows[1])
	}
}

func TestFlattenTreeExpandRecursive(t *testing.T) {
	root := buildTreeProject(t)
	cache := &structure.StructureCache{}
	cache.Merge([]structure.Node{{Path: "web/a.go", Desc: "路由"}})
	expanded := map[string]bool{"web": true, "web/api": true}
	rows := flattenTree("", root, nil, expanded, cache, map[string][]structure.Note{})
	// web 展开 → api(展开,目录优先)/a.go/b.go，api 展开 → c.go，加 main.go 共 6 行
	if len(rows) != 6 {
		t.Fatalf("展开后应 6 行，得 %d:\n%+v", len(rows), rows)
	}
	if rows[1].head != "│   ├── ▾ api/" {
		t.Fatalf("api 行不符: %+v", rows[1])
	}
	if rows[2].head != "│   │   └──   c.go" {
		t.Fatalf("c.go 行不符: %+v", rows[2])
	}
	if rows[3].head != "│   ├──   a.go" || rows[3].desc != "路由" {
		t.Fatalf("a.go 行不符: %+v", rows[3])
	}
}

func TestTreeWindow(t *testing.T) {
	// 不超页高全显
	if lo, hi := treeWindow(3, 1, 10); lo != 0 || hi != 3 {
		t.Fatalf("短列表应全显: %d %d", lo, hi)
	}
	// 居中滑动
	if lo, hi := treeWindow(100, 50, 10); lo != 45 || hi != 55 {
		t.Fatalf("应围绕光标居中: %d %d", lo, hi)
	}
	// 贴顶
	if lo, hi := treeWindow(100, 2, 10); lo != 0 || hi != 10 {
		t.Fatalf("贴顶应从 0 起: %d %d", lo, hi)
	}
	// 贴底
	if lo, hi := treeWindow(100, 99, 10); lo != 90 || hi != 100 {
		t.Fatalf("贴底应收尾对齐: %d %d", lo, hi)
	}
	// avail<=0 不分页
	if lo, hi := treeWindow(100, 99, 0); lo != 0 || hi != 100 {
		t.Fatalf("avail=0 应全显: %d %d", lo, hi)
	}
}

func TestRenderRowStyles(t *testing.T) {
	r := treeRow{head: "├── ▸ web/", desc: "前端页面", notes: 2}
	if s := renderRow(r, true, 100); !strings.Contains(s, "\x1b[7m") || strings.Contains(s, "\x1b[2m") {
		t.Fatalf("光标行应整行反白且无内嵌灰: %q", s)
	}
	if s := renderRow(r, false, 100); strings.Contains(s, "\x1b[7m") || !strings.Contains(s, "前端页面") || !strings.Contains(s, "·笔2") {
		t.Fatalf("普通行应含介绍/笔记且无反白: %q", s)
	}
	// 超宽降级素文本截断
	if s := renderRow(r, true, 5); strings.Contains(s, "\x1b[7m") {
		t.Fatalf("超宽行应为素文本截断: %q", s)
	}
}

func TestParentDir(t *testing.T) {
	for in, want := range map[string]string{
		"":         "",
		"web":      "",
		"web/api":  "web",
		"a/b/c.go": "a/b",
	} {
		if got := parentDir(in); got != want {
			t.Fatalf("parentDir(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestDrawTreeFrameCursorReturn(t *testing.T) {
	ctx := &Ctx{Out: &bytes.Buffer{}}
	rows := []treeRow{
		{rel: "web", isDir: true, head: "├── ▸ web/"},
		{rel: "main.go", head: "└──   main.go"},
	}
	drawTreeFrame(ctx, workspace.Project{Name: "gs"}, &treeView{}, rows)
	assertFrameCursorReturn(t, ctx.Out.(*bytes.Buffer).String())
}
