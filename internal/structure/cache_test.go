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
