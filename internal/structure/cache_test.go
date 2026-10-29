package structure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/workspace"
)

func TestStructureCacheRoundtrip(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	path := StructurePath(ws, "demo")
	c := &StructureCache{}
	c.Merge([]Node{{Path: "main.go", Desc: "入口"}, {Path: "internal", Desc: "核心"}})
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	c2, err := LoadStructureCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Find("main.go") != "入口" || c2.Find("internal") != "核心" || c2.Find("nope") != "" {
		t.Fatalf("Find 结果不符: %+v", c2.Nodes)
	}
	if !strings.HasPrefix(c2.Generated, "20") {
		t.Fatalf("generated 应有时间戳: %q", c2.Generated)
	}
}

func TestStructureCacheMergeOverwrite(t *testing.T) {
	c := &StructureCache{}
	c.Merge([]Node{{Path: "main.go", Desc: "旧"}, {Path: "a.go", Desc: "A"}})
	c.Merge([]Node{{Path: "main.go", Desc: "新"}})
	if c.Find("main.go") != "新" || c.Find("a.go") != "A" || len(c.Nodes) != 2 {
		t.Fatalf("合并覆盖结果不符: %+v", c.Nodes)
	}
	// 按 path 排序，便于 diff 稳定
	if c.Nodes[0].Path != "a.go" {
		t.Fatalf("节点应按 path 排序: %+v", c.Nodes)
	}
}

func TestStructureCacheLoadMissingAndBroken(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	c, err := LoadStructureCache(StructurePath(ws, "nope"))
	if err != nil || len(c.Nodes) != 0 {
		t.Fatalf("不存在的缓存应返回空: %+v %+v", c, err)
	}
	broken := StructurePath(ws, "broken")
	if err := os.MkdirAll(filepath.Dir(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("[[[不是 toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadStructureCache(broken); err == nil {
		t.Fatal("损坏缓存应报错")
	}
}

func TestStructureCacheMissing(t *testing.T) {
	c := &StructureCache{}
	c.Merge([]Node{{Path: "main.go", Desc: "入口"}})
	targets := []Entry{
		{Name: "main.go", Rel: "main.go"},
		{Name: "web", Rel: "web", IsDir: true},
		{Name: "a.go", Rel: "web/a.go"},
	}
	missing := c.Missing(targets)
	if len(missing) != 2 || missing[0].Rel != "web" || missing[1].Rel != "web/a.go" {
		t.Fatalf("应只含缓存缺失节点且保持顺序: %+v", missing)
	}
}

func TestStructureCacheClearSubtree(t *testing.T) {
	c := &StructureCache{}
	c.Merge([]Node{
		{Path: "internal", Desc: "a"},
		{Path: "internal/command", Desc: "b"},
		{Path: "internal/command/x.go", Desc: "c"},
		{Path: "internalx.go", Desc: "d"},
		{Path: "web", Desc: "e"},
	})
	c.ClearSubtree("internal")
	for _, p := range []string{"internal", "internal/command", "internal/command/x.go"} {
		if c.Find(p) != "" {
			t.Fatalf("%s 应被清除", p)
		}
	}
	if c.Find("internalx.go") != "d" || c.Find("web") != "e" {
		t.Fatalf("子树外节点不应受影响: %+v", c.Nodes)
	}
	c.ClearSubtree("")
	if len(c.Nodes) != 0 {
		t.Fatalf("空前缀应清空全部: %+v", c.Nodes)
	}
}
