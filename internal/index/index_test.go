package index

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topics.toml")
	ix := &Index{Topics: []Topic{{
		Name:      "context",
		Created:   "2026-10-25",
		Files:     []FileRef{{Path: "Study/notes/GPT/a.md", Hint: "重点讲超时"}},
		Logs:      []string{"261021"},
		Interview: "Study/Project/x/doc/interview-questions.md",
	}}}
	if err := ix.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tp := got.Find("Context") // 大小写不敏感
	if tp == nil || tp.Name != "context" || len(tp.Files) != 1 || tp.Files[0].Hint != "重点讲超时" {
		t.Fatalf("Find 结果 = %+v", tp)
	}
	if got.Find("nope") != nil {
		t.Fatal("未登记主题应返回 nil")
	}
}

func TestLoadMissing(t *testing.T) {
	ix, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil {
		t.Fatalf("缺文件应返回空索引而非错误: %v", err)
	}
	if len(ix.Topics) != 0 {
		t.Fatal("应为空索引")
	}
}

func TestCountAndPrunePaths(t *testing.T) {
	ix := New()
	ix.Topics = []Topic{
		{Name: "a", Files: []FileRef{{Path: "x.md"}, {Path: "y.md"}}},
		{Name: "b", Files: []FileRef{{Path: "x.md"}}},
	}
	if n := ix.CountPaths(map[string]bool{"x.md": true}); n != 2 {
		t.Fatalf("CountPaths = %d, want 2", n)
	}
	if n := ix.PrunePaths(map[string]bool{"x.md": true}); n != 2 {
		t.Fatalf("PrunePaths = %d, want 2", n)
	}
	if len(ix.Topics[0].Files) != 1 || ix.Topics[0].Files[0].Path != "y.md" {
		t.Fatalf("a 组应只剩 y.md，got %v", ix.Topics[0].Files)
	}
	if len(ix.Topics[1].Files) != 0 {
		t.Fatalf("b 组应清空，got %v", ix.Topics[1].Files)
	}
	if n := ix.CountPaths(map[string]bool{"x.md": true}); n != 0 {
		t.Fatal("清除后 CountPaths 应为 0")
	}
}
