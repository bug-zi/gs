package command

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/structure"
	"github.com/bugzi/gs/internal/workspace"
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

func TestFilterSubtreeAndSplitBatches(t *testing.T) {
	entries := []structure.Entry{
		{Rel: "main.go"},
		{Rel: "internal", IsDir: true},
		{Rel: "internalx.go"},
		{Rel: "internal/command", IsDir: true},
		{Rel: "internal/command/a.go"},
	}
	sub := filterSubtree(entries, "internal")
	var rels []string
	for _, e := range sub {
		rels = append(rels, e.Rel)
	}
	if !reflect.DeepEqual(rels, []string{"internal", "internal/command", "internal/command/a.go"}) {
		t.Fatalf("子树过滤应含自身与后代、排除 internalx.go: %v", rels)
	}
	dirs, byDir := splitBatches(sub)
	if !reflect.DeepEqual(dirs, []string{"", "internal", "internal/command"}) {
		t.Fatalf("批次目录应自上而下（子树根归根批次）: %v", dirs)
	}
	if len(byDir[""]) != 1 || byDir[""][0].Rel != "internal" {
		t.Fatalf("子树根应归入根批次: %+v", byDir[""])
	}
	if len(byDir["internal"]) != 1 || byDir["internal"][0].Rel != "internal/command" {
		t.Fatalf("按父目录分组不符: %+v", byDir["internal"])
	}
	dirs2, _ := splitBatches(entries)
	if !reflect.DeepEqual(dirs2, []string{"", "internal", "internal/command"}) {
		t.Fatalf("根批次应最前: %v", dirs2)
	}
}

func TestGenTreeDescriptionsFaultTolerant(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "a"), 0o755)
	os.MkdirAll(filepath.Join(root, "b"), 0o755)
	os.WriteFile(filepath.Join(root, "a", "x.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b", "y.go"), []byte("package b\n"), 0o644)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		content := "完全不是 JSON" // 第 1 批（根）坏回复
		switch calls {
		case 2:
			content = `[{"path":"a","desc":"A 包"},{"path":"a/x.go","desc":"A 文件"}]`
		case 3:
			content = `[{"path":"b","desc":"B 包"},{"path":"b/y.go","desc":"B 文件"}]`
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", strconv.Quote(content))
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, err := ai.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Ctx{WS: &workspace.Workspace{Root: t.TempDir()}, Cfg: cfg, Out: &out, AI: &AIState{Client: c}}
	p := workspace.Project{Name: "demo", Path: root}
	cache := &structure.StructureCache{}

	failed, err := genTreeDescriptions(ctx, p, cache, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(failed, []string{""}) {
		t.Fatalf("失败批次应被记录（根批次为空串）: %v", failed)
	}
	if calls != 3 {
		t.Fatalf("单批失败后应继续其余批次，共 3 次调用: %d", calls)
	}
	if cache.Find("a") != "A 包" || cache.Find("a/x.go") != "A 文件" || cache.Find("b/y.go") != "B 文件" {
		t.Fatalf("成功批次应入缓存: %+v", cache.Nodes)
	}
	// 断点续传：无缺失时不再调 AI
	if _, err := genTreeDescriptions(ctx, p, cache, ""); err != nil || calls != 3 {
		t.Fatalf("无缺失不应再调用: calls=%d err=%v", calls, err)
	}
	// 子树范围：a 子树无缺失时也不调
	if _, err := genTreeDescriptions(ctx, p, cache, "a"); err != nil || calls != 3 {
		t.Fatalf("子树无缺失不应调用: calls=%d", calls)
	}
}
