package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/structure"
)

func TestParseDescJSON(t *testing.T) {
	nodes, err := parseDescJSON(`[{"path":"main.go","desc":"入口"},{"path":"web/","desc":"前端"}]`)
	if err != nil || len(nodes) != 2 || nodes[0].Path != "main.go" {
		t.Fatalf("裸数组解析失败: %+v %+v", nodes, err)
	}
	nodes, err = parseDescJSON("```json\n[{\"path\":\"a\",\"desc\":\"b\"}]\n```")
	if err != nil || len(nodes) != 1 {
		t.Fatalf("围栏包裹应可解析: %+v %+v", nodes, err)
	}
	if _, err := parseDescJSON("不是 JSON"); err == nil {
		t.Fatal("无数组应报错")
	}
	if _, err := parseDescJSON(`[{"path":"","desc":"b"}]`); err == nil {
		t.Fatal("空 path 应报错")
	}
}

func TestFileHeadBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 100)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	budget := 50
	head := fileHead(path, 40, &budget)
	if len(head) != 50 || budget != 0 {
		t.Fatalf("应按预算截断: len=%d budget=%d", len(head), budget)
	}
	if fileHead(filepath.Join(t.TempDir(), "nope"), 10, &budget) != "" {
		t.Fatal("读不到的文件应返回空")
	}
}

func TestRenderTree(t *testing.T) {
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
	write("main.go")
	write("web/a.go")
	write("web/b.go")

	cache := &structure.StructureCache{}
	cache.Merge([]structure.Node{
		{Path: "main.go", Desc: "入口"},
		{Path: "web", Desc: "前端页面"},
		{Path: "web/a.go", Desc: "路由"},
	})
	matched := map[string][]structure.Note{
		"main.go": {{Abs: "/x/main.go串讲.md", Display: "main.go串讲.md"}},
	}
	out := stripANSI(renderTree("demo", "", root, nil, cache, matched))
	for _, want := range []string{
		"框架树 · demo", "```", "├── web/", "└── main.go", "入口", "·笔1",
		"│   ├── a.go", "│   └── b.go", "路由",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("树渲染缺 %q:\n%s", want, out)
		}
	}
	// 子目录视图：标题带路径，只含该子树，不含根层文件
	out2 := stripANSI(renderTree("demo", "web", root, nil, cache, matched))
	if !strings.Contains(out2, "框架树 · demo　web/") {
		t.Fatalf("子目录标题应带路径:\n%s", out2)
	}
	if !strings.Contains(out2, "├── a.go") || strings.Contains(out2, "main.go") {
		t.Fatalf("子目录视图应只含子树:\n%s", out2)
	}
}
