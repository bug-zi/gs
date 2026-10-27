package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func fixture(t *testing.T) (root string, cfg *config.Config) {
	t.Helper()
	root = t.TempDir()
	cfg, _ = config.Load(root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/notes/GPT/a.md", "# A标题\n\ncontext 用于跨协程传值。\n\n## 小节二\ncontext 超时控制。\n")
	w("Study/code.go", "package main\n\nfunc Foo() {\n\tctx := context.Background()\n}\n")
	w("Resource/hidden.md", "context 不应被扫到\n")
	return root, cfg
}

func TestSearchANDAndExclusion(t *testing.T) {
	root, cfg := fixture(t)
	// 双词 AND：两词都在才收录
	results, words, err := Search(root, cfg, "context 超时")
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 {
		t.Fatalf("words = %v", words)
	}
	var paths []string
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	want := "Study/notes/GPT/a.md"
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %v, want [%s]", paths, want)
	}
	// Resource 已被排除；a.md 中两个命中单元分别为标题与小节二
	if results[0].Hits[0].Unit != "A标题" {
		t.Fatalf("Unit = %q", results[0].Hits[0].Unit)
	}
	last := results[0].Hits[len(results[0].Hits)-1]
	if last.Unit != "小节二" {
		t.Fatalf("末命中 Unit = %q", last.Unit)
	}
}

func TestSearchGoFuncUnit(t *testing.T) {
	root, cfg := fixture(t)
	results, _, err := Search(root, cfg, "Background")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != "Study/code.go" {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Hits[0].Unit != "Foo" {
		t.Fatalf("go 命中 Unit = %q", results[0].Hits[0].Unit)
	}
	if results[0].Hits[0].Line != 4 {
		t.Fatalf("Line = %d", results[0].Hits[0].Line)
	}
	if len(results[0].Hits[0].Context) == 0 {
		t.Fatal("Context 不应为空")
	}
}

func TestSearchNoHit(t *testing.T) {
	root, cfg := fixture(t)
	results, _, err := Search(root, cfg, "不存在的词")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %+v", results)
	}
}
