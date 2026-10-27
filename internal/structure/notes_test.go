package structure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchNotes(t *testing.T) {
	goFiles := []Entry{
		{Name: "main.go", Rel: "main.go"},
		{Name: "config.go", Rel: "internal/config/config.go"},
	}
	notes := []Note{
		{Abs: "/x/main.go-context串讲.md", Display: "main.go-context串讲.md"}, // 完整名命中
		{Abs: "/x/main函数分析.md", Display: "main函数分析.md"},                   // 基名 main(4) 命中
		{Abs: "/x/配置config解析.md", Display: "配置config解析.md"},               // 基名 config 命中
		{Abs: "/x/蹲课实现.md", Display: "蹲课实现.md"},                           // 不含任何源文件名
		{Abs: "/x/go笔记.md", Display: "go笔记.md"},                           // 短基名（len<4）不命中 main
	}
	m := MatchNotes(goFiles, notes)
	if len(m["main.go"]) != 2 {
		t.Fatalf("main.go 应命中 2 篇: %+v", m["main.go"])
	}
	if len(m["internal/config/config.go"]) != 1 {
		t.Fatalf("config.go 应命中 1 篇: %+v", m["internal/config/config.go"])
	}
}

func TestMatchNotesCaseInsensitive(t *testing.T) {
	goFiles := []Entry{{Name: "Main.go", Rel: "Main.go"}}
	notes := []Note{{Abs: "/x/MAIN.GO 串讲.md", Display: "MAIN.GO 串讲.md"}}
	if len(MatchNotes(goFiles, notes)["Main.go"]) != 1 {
		t.Fatal("匹配应大小写不敏感")
	}
}

func TestReverseResolve(t *testing.T) {
	goFiles := []Entry{
		{Name: "main.go", Rel: "main.go"},
		{Name: "web.go", Rel: "web/web.go"},
	}
	if rel, ok := ReverseResolve("/x/main.go-context串讲.md", goFiles); !ok || rel != "main.go" {
		t.Fatalf("应唯一反解 main.go: %q %v", rel, ok)
	}
	if _, ok := ReverseResolve("/x/蹲课实现.md", goFiles); ok {
		t.Fatal("无 .go 名应反解失败")
	}
	// 歧义：两个同名文件都不给结果
	amb := append(goFiles, Entry{Name: "main.go", Rel: "cmd/main.go"})
	if _, ok := ReverseResolve("/x/main.go串讲.md", amb); ok {
		t.Fatal("多命中应反解失败")
	}
}

func TestCollectAndWalkGoFiles(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("docs/study/a.md", "A")
	mk("docs/study/sub/b.md", "B")
	mk("main.go", "package main")
	mk("internal/x.go", "package x")
	mk(".git/y.go", "package y") // 隐藏目录跳过
	goFiles := WalkGoFiles(root, nil)
	if len(goFiles) != 2 {
		t.Fatalf("应收集 2 个 go 文件: %+v", goFiles)
	}
	projNotes, seaNotes := CollectNotes(root, filepath.Join(root, "sea", "demo"))
	if len(projNotes) != 2 {
		t.Fatalf("项目内笔记应 2 篇: %+v", projNotes)
	}
	if len(seaNotes) != 0 {
		t.Fatalf("sea 目录不存在应返回空: %+v", seaNotes)
	}
}
