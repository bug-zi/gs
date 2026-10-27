package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestRunSearchCmd(t *testing.T) {
	ws := wsForTest(t)
	if err := os.WriteFile(filepath.Join(ws.LogsDir(), "261024.md"), []byte("- 学习了 context 超时\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, Args: []string{"context"}}
	if err := newSearch().Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Study/logs/261024.md") {
		t.Fatalf("缺文件路径: %q", out.String())
	}
}

func TestRunSearchNoArgs(t *testing.T) {
	ws := wsForTest(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := newSearch().Run(ctx); err == nil {
		t.Fatal("无参数应报用法错误")
	}
}
