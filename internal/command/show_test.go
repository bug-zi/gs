package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/workspace"
)

func showFixture(t *testing.T) (*workspace.Workspace, *config.Config) {
	t.Helper()
	ws := wsForTest(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/river/Go并发与互斥锁.md", "# 并发\n\ncontext 传递取消信号。\n")
	w("Study/sea/clip/context文章剪藏.md", "# 剪藏\n\ncontext 最佳实践。\n")
	w("Study/sea/interview/context面试题.md", "# 面试题\n\n1. context 的取消机制？\n")
	w("Study/Project/demo/use.go", "package main\n\nfunc Use() {\n\t_ = context.Background()\n}\n")
	w("Study/logs/261024.md", "- 学习了 context 超时\n")
	w("Study/Project/demo/doc/interview-questions.md", "# 面试题\n\n1. context 的取消机制？\n")
	return ws, cfg
}

func TestBuildShowViewScan(t *testing.T) {
	ws, cfg := showFixture(t)
	v := buildShowView(ws, cfg, index.New(), "context")
	if v.Topic != "context" || v.Indexed {
		t.Fatalf("v = %+v", v)
	}
	if len(v.Sections) != 5 {
		t.Fatalf("应有五节, got %d", len(v.Sections))
	}
	if v.Sections[1].Title != "知识" {
		t.Fatalf("第二节标题应为知识, got %s", v.Sections[1].Title)
	}
	if !strings.Contains(v.Sections[1].Body, "Go并发与互斥锁.md") {
		t.Fatalf("知识节应含河流条目: %s", v.Sections[1].Body)
	}
	if !strings.Contains(v.Sections[1].Body, "context文章剪藏.md") {
		t.Fatalf("知识节应含大海补充条目: %s", v.Sections[1].Body)
	}
	if strings.Contains(v.Sections[1].Body, "context面试题.md") {
		t.Fatalf("面试题正本不应落入知识节: %s", v.Sections[1].Body)
	}
	if !strings.Contains(v.Sections[0].Body, "知识 2") {
		t.Fatalf("概览应统计知识 2: %s", v.Sections[0].Body)
	}
	if !strings.Contains(v.Sections[2].Body, "Study/Project/demo/use.go") {
		t.Fatalf("代码节: %s", v.Sections[2].Body)
	}
	if !strings.Contains(v.Sections[3].Body, "学习了 context 超时") {
		t.Fatalf("日志节: %s", v.Sections[3].Body)
	}
	if !strings.Contains(v.Sections[4].Body, "context面试题.md") {
		t.Fatalf("面试节: %s", v.Sections[4].Body)
	}
}

func TestBuildShowViewIndexed(t *testing.T) {
	ws, cfg := showFixture(t)
	ix := &index.Index{Topics: []index.Topic{{
		Name:    "context",
		Created: "2026-10-25",
		Files: []index.FileRef{
			{Path: "Study/river/Go并发与互斥锁.md", Hint: "重点讲取消"},
			{Path: "Study/Project/demo/use.go", Hint: ""},
		},
		Logs:      []string{"261024"},
		Interview: "Study/Project/demo/doc/interview-questions.md",
	}}}
	v := buildShowView(ws, cfg, ix, "context")
	if !v.Indexed {
		t.Fatal("应识别为索引视图")
	}
	if !strings.Contains(v.Sections[1].Body, "重点讲取消") {
		t.Fatalf("笔记节应含演示提示: %s", v.Sections[1].Body)
	}
}

func TestShowSave(t *testing.T) {
	ws, cfg := showFixture(t)
	ix := index.New()
	v := buildShowView(ws, cfg, ix, "context")
	if err := showSave(ws, v); err != nil {
		t.Fatal(err)
	}
	got, err := index.Load(ws.IndexPath())
	if err != nil {
		t.Fatal(err)
	}
	if got.Find("context") == nil {
		t.Fatal("沉淀后索引应有该主题")
	}
}
