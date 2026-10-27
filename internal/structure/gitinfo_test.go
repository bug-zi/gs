package structure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitStatsStubbed(t *testing.T) {
	orig := GitRun
	defer func() { GitRun = orig }()
	GitRun = func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "log":
			if len(args) >= 2 && args[1] == "--reverse" {
				return "2025-09-01\n2025-09-02\n", nil
			}
			return "2026-10-20\n", nil
		case "rev-list":
			if len(args) >= 2 && args[1] == "--count" {
				if len(args) >= 4 && args[2] == "--since=30 days ago" {
					return "5\n", nil
				}
				return "42\n", nil
			}
		}
		return "", os.ErrNotExist
	}
	info := GitStats("any")
	if !info.Available || info.Created != "2025-09-01" || info.LastCommit != "2026-10-20" ||
		info.Total != 42 || info.Recent30 != 5 {
		t.Fatalf("GitStats 结果不符: %+v", info)
	}
}

func TestGitStatsUnavailable(t *testing.T) {
	orig := GitRun
	defer func() { GitRun = orig }()
	GitRun = func(dir string, args ...string) (string, error) {
		return "", os.ErrPermission
	}
	if info := GitStats("any"); info.Available {
		t.Fatal("git 失败应 Available=false")
	}
}

func TestGoStatsAndLogDates(t *testing.T) {
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
	mk("a.go", "package a\n\nfunc f() {}\n")
	mk("sub/b.go", "package b\n")
	mk("c.txt", "not go")
	files, lines := GoStats(root)
	if files != 2 || lines != 4 {
		t.Fatalf("GoStats 应为 2 文件 4 行: %d %d", files, lines)
	}
	mk("docs/log/260101.md", "x")
	mk("docs/log/251231.md", "x")
	mk("docs/log/随手记.md", "x") // 非日志名过滤
	dates := ProjectLogDates(root)
	if len(dates) != 2 || dates[0].Raw != "260101" || dates[0].Display != "2026-01-01" {
		t.Fatalf("ProjectLogDates 结果不符: %+v", dates)
	}
}
