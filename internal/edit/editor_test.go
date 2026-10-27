package edit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestResolvePriority(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	t.Setenv("VISUAL", "emacs")
	cfg := &config.Config{UI: config.UI{Editor: "myed"}}
	if got := Resolve(cfg); got != "myed" {
		t.Fatalf("配置编辑器应最优先, got %q", got)
	}
	if got := Resolve(&config.Config{}); got != "vi" {
		t.Fatalf("应回退 $EDITOR, got %q", got)
	}
	t.Setenv("EDITOR", "")
	if got := Resolve(&config.Config{}); got != "emacs" {
		t.Fatalf("应回退 $VISUAL, got %q", got)
	}
}

// 复现 bug：LookPath 解析出的完整路径含空格（如 D:\app\Microsoft VS Code\bin\code.cmd），
// 不应再按空白拆开
func TestSplitEditorWholePathWithSpaces(t *testing.T) {
	ed := filepath.Join(t.TempDir(), "My Editor.exe")
	if err := os.WriteFile(ed, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	lp, args, err := splitEditor(ed)
	if err != nil {
		t.Fatalf("含空格的完整路径应整体解析, err: %v", err)
	}
	if lp != ed {
		t.Fatalf("lp = %q, want %q", lp, ed)
	}
	if len(args) != 0 {
		t.Fatalf("args = %v, want 空", args)
	}
}

// "可执行文件 -w" 这类带参配置：整串找不到时按空白拆成命令 + 参数
func TestSplitEditorCommandWithArgs(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "myed.exe")
	if err := os.WriteFile(exe, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	lp, args, err := splitEditor(exe + " -w")
	if err != nil {
		t.Fatalf("带参编辑器应拆分解析, err: %v", err)
	}
	if lp != exe {
		t.Fatalf("lp = %q, want %q", lp, exe)
	}
	if len(args) != 1 || args[0] != "-w" {
		t.Fatalf("args = %v, want [-w]", args)
	}
}

func TestSplitEditorNotFound(t *testing.T) {
	_, _, err := splitEditor("no-such-editor-xyz")
	if err == nil || !strings.Contains(err.Error(), "no-such-editor-xyz") {
		t.Fatalf("应报编辑器不可用并带上名称, got %v", err)
	}
}

// 兜底链顺序：PATH 里 trae/code 同时可用时应选 trae（PATH 整体替换隔离真机环境）
func TestResolveFallbackTraeFirst(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"trae.cmd", "code.cmd"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	got := Resolve(&config.Config{})
	if !strings.HasSuffix(got, "trae.cmd") {
		t.Fatalf("兜底链应 trae 优先, got %q", got)
	}
}

// VS Code 家族（trae/code）是秒退型 GUI 编辑器，必须注入 --wait 才有「编辑完返回」语义
func TestWaitArgs(t *testing.T) {
	cases := []struct {
		name string
		lp   string
		in   []string
		want []string
	}{
		{"trae命令行包装", `C:\bin\trae.cmd`, []string{"a.md"}, []string{"--wait", "a.md"}},
		{"trae主程序", `D:\app\AI编辑器\TRAE\Trae CN\Trae CN.exe`, []string{"a.md"}, []string{"--wait", "a.md"}},
		{"code家族", `C:\Program Files\Microsoft VS Code\bin\code.cmd`, []string{"a.md"}, []string{"--wait", "a.md"}},
		{"非VSCode系不动", `C:\Windows\notepad.exe`, []string{"a.md"}, []string{"a.md"}},
		{"已带-w不重复", `C:\bin\trae.cmd`, []string{"-w", "a.md"}, []string{"-w", "a.md"}},
		{"已带--wait不重复", `C:\bin\code.cmd`, []string{"--wait", "a.md"}, []string{"--wait", "a.md"}},
	}
	for _, c := range cases {
		if got := waitArgs(c.lp, c.in); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s: waitArgs = %v, want %v", c.name, got, c.want)
		}
	}
}
