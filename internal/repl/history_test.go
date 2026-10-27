package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadHistoryMissing(t *testing.T) {
	h := LoadHistory(filepath.Join(t.TempDir(), "不存在"))
	if len(h.Items()) != 0 {
		t.Fatalf("缺失文件应得空历史: %v", h.Items())
	}
}

func TestHistoryPersistAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h := LoadHistory(path)
	h.Add("/logs")
	h.Add("/sea")
	h.Add("/sea") // 相邻重复应忽略
	h.Add("/logs")
	if got := h.Items(); len(got) != 3 {
		t.Fatalf("items = %v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 3 {
		t.Fatalf("文件应只落 3 行: %q", data)
	}
	// 重新加载可见
	if got := LoadHistory(path).Items(); len(got) != 3 || got[0] != "/logs" || got[1] != "/sea" || got[2] != "/logs" {
		t.Fatalf("reload = %v", got)
	}
}

func TestHistorySkipsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	h := LoadHistory(path)
	h.Add("   ")
	if len(h.Items()) != 0 {
		t.Fatal("空白行不应入史")
	}
}
