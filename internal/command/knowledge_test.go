package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListKnowledge(t *testing.T) {
	ws := wsForTest(t)
	w := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/sea/clip/context文章.md", "# a\n")
	w("Study/river/Go并发笔记.md", "# b\n")
	w("Study/spring/context理解.md", "# c\n")
	w("Study/logs/261024.md", "# 日志不算知识库\n") // 干扰项

	files, err := ListKnowledge(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files = %v", files)
	}
	for _, f := range files {
		if filepath.IsAbs(f) || !strings.HasPrefix(f, "Study/") || strings.Contains(f, `\`) {
			t.Fatalf("应为 Study/ 下正斜杠相对路径: %s", f)
		}
	}
}

func TestListKnowledgeEmpty(t *testing.T) {
	ws := wsForTest(t) // 三层目录均不存在
	files, err := ListKnowledge(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("files = %v", files)
	}
}

func TestListTierFiles(t *testing.T) {
	ws := wsForTest(t)
	mk := func(rel string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/sea/clip/a.md")
	mk("Study/sea/interview/b.md")
	mk("Study/river/c.md")
	mk("Study/spring/d.md")
	mk("Study/spring/子/notes.md")
	mk("Study/logs/261024.md")
	mk("Study/code/e.go")
	mk("Study/code/notes.md") // 干扰项：code 层不收 .md

	seaFiles, err := ListTierFiles(ws, "sea")
	if err != nil {
		t.Fatal(err)
	}
	if len(seaFiles) != 2 {
		t.Fatalf("sea = %v", seaFiles)
	}
	springFiles, err := ListTierFiles(ws, "spring")
	if err != nil {
		t.Fatal(err)
	}
	if len(springFiles) != 2 {
		t.Fatalf("spring = %v", springFiles)
	}
	codeFiles, err := ListTierFiles(ws, "code")
	if err != nil {
		t.Fatal(err)
	}
	if len(codeFiles) != 1 || codeFiles[0] != "Study/code/e.go" {
		t.Fatalf("code = %v", codeFiles)
	}
	if _, err := ListTierFiles(ws, "ocean"); err == nil {
		t.Fatal("未知层应报错")
	}
}
