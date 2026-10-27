package structure

import (
	"os"
	"path/filepath"
	"testing"
)

func treeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkDir := func(rel string) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mkFile := func(rel string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkFile("main.go")
	mkFile("go.mod")
	mkFile("README.md")
	mkFile("logo.png") // 非关注类型，应过滤
	mkDir("internal/config")
	mkFile("internal/config/config.go")
	mkDir("web")
	mkDir(".git")         // 隐藏目录恒跳过
	mkDir("node_modules") // 在 exclude 中，应跳过
	return root
}

func TestScanTopLevel(t *testing.T) {
	root := treeFixture(t)
	entries := Scan(root, "", []string{"node_modules"})
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	for _, want := range []string{"main.go", "go.mod", "README.md", "internal", "web"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("缺节点 %q: %v", want, names)
		}
	}
	for _, banned := range []string{"logo.png", ".git", "node_modules"} {
		for _, n := range names {
			if n == banned {
				t.Fatalf("不应包含 %q: %v", banned, names)
			}
		}
	}
	// 目录在前
	if !entries[0].IsDir {
		t.Fatalf("首项应为目录: %+v", entries[0])
	}
}

func TestScanSubdirRel(t *testing.T) {
	root := treeFixture(t)
	entries := Scan(filepath.Join(root, "internal"), "internal", []string{"node_modules"})
	if len(entries) != 1 || entries[0].Name != "config" || !entries[0].IsDir || entries[0].Rel != "internal/config" {
		t.Fatalf("子目录扫描结果不符: %+v", entries)
	}
}

func TestScanMissingDir(t *testing.T) {
	if entries := Scan(filepath.Join(t.TempDir(), "nope"), "", nil); entries != nil {
		t.Fatalf("不存在的目录应返回 nil: %v", entries)
	}
}
