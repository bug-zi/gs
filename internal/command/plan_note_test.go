package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

// fakeEditor 造一个立即退出的临时 .cmd 编辑器并注入 EDITOR。
// Windows 下 openExternal 对 .cmd 经 cmd /c 启动，@exit 0 立即返回不阻塞测试；
// 同时兜底防止测试误触真实 TRAE/code。
func fakeEditor(t *testing.T) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "fakeedit.cmd")
	if err := os.WriteFile(p, []byte("@exit 0\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", p)
	t.Setenv("VISUAL", "")
}

// muteStdin 把 os.Stdin 换成空设备：pager 降级为行读取后立即 EOF，查阅循环跑一轮即退出
func muteStdin(t *testing.T) {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = old; _ = f.Close() })
}

func writeNote(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 全局原则：≤20 行的文件在 CLI 查阅/操作，>20 行转交外部编辑器（TRAE）查阅+编辑
func TestOpenPlanNote逾行文件转交外部编辑器(t *testing.T) {
	fakeEditor(t)
	muteStdin(t)
	root := t.TempDir()
	writeNote(t, root, "Study/spring/long.md", strings.Repeat("内容行\n", 25))

	var out bytes.Buffer
	ctx := &Ctx{WS: &workspace.Workspace{Root: root}, Cfg: &config.Config{}, Out: &out}
	if err := openPlanNote(ctx, "Study/spring/long.md"); err != nil {
		t.Fatalf("openPlanNote: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "长文件") {
		t.Errorf("逾行文件应入口转交外部编辑器，实际输出：\n%s", got)
	}
	if strings.Contains(got, "[e]编辑") {
		t.Errorf("逾行文件不应进入 CLI 查阅循环，实际输出：\n%s", got)
	}
}

func TestOpenPlanNote短文件CLI查阅(t *testing.T) {
	fakeEditor(t)
	muteStdin(t)
	root := t.TempDir()
	writeNote(t, root, "Study/spring/short.md", "标记短文件内容\n"+strings.Repeat("行\n", 14))

	var out bytes.Buffer
	ctx := &Ctx{WS: &workspace.Workspace{Root: root}, Cfg: &config.Config{}, Out: &out}
	if err := openPlanNote(ctx, "Study/spring/short.md"); err != nil {
		t.Fatalf("openPlanNote: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "[e]编辑") {
		t.Errorf("短文件应进入 CLI 查阅循环，实际输出：\n%s", got)
	}
	if !strings.Contains(got, "标记短文件内容") {
		t.Errorf("短文件内容应被渲染，实际输出：\n%s", got)
	}
	if strings.Contains(got, "长文件") {
		t.Errorf("短文件不应转交外部编辑器，实际输出：\n%s", got)
	}
}

func TestOpenPlanNoteGo文件CLI查阅(t *testing.T) {
	fakeEditor(t)
	muteStdin(t)
	root := t.TempDir()
	writeNote(t, root, "Study/code/short.go", "package main\n\nfunc main() {}\n")

	var out bytes.Buffer
	ctx := &Ctx{WS: &workspace.Workspace{Root: root}, Cfg: &config.Config{}, Out: &out}
	if err := openPlanNote(ctx, "Study/code/short.go"); err != nil {
		t.Fatalf("openPlanNote: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "package") {
		t.Errorf(".go 文件内容应被高亮渲染，实际输出：\n%s", got)
	}
	if strings.Contains(got, "长文件") {
		t.Errorf("短 .go 文件不应转交外部编辑器，实际输出：\n%s", got)
	}
}
