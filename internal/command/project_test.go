package command

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/structure"
	"github.com/bugzi/gs/internal/workspace"
)

var cmdAnsiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI 剥离 ANSI 转义序列，供渲染输出断言用
func stripANSI(s string) string { return cmdAnsiRe.ReplaceAllString(s, "") }

func projectFixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	ws := wsForTest(t)
	p := filepath.Join(ws.Root, "Study", "Project", "demo")
	if err := os.MkdirAll(filepath.Join(p, "docs", "log"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "README.md"), []byte("# demo 项目\n\n一个演示项目\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "docs", "log", "260101.md"), []byte("- 跑通 main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestProjectNoMatch(t *testing.T) {
	ws := projectFixture(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, Args: []string{"不存在"}}
	if err := newProject().Run(ctx); err == nil {
		t.Fatal("无匹配项目应报错")
	}
}

func TestBuildProjectLogText(t *testing.T) {
	ws := projectFixture(t) // 含 Study/Project/demo：go.mod、README、docs/log/260101.md
	p := workspace.Project{Name: "demo", Path: filepath.Join(ws.Root, "Study", "Project", "demo")}
	text := buildProjectLogText(p, structure.GitInfo{Available: true, Created: "2025-12-01", LastCommit: "2026-01-05", Total: 10, Recent30: 2})
	for _, want := range []string{"2025-12-01", "2026-01-05", "共 10 次", "近 30 天 2 次", "2026-01-01", "1 篇"} {
		if !strings.Contains(stripANSI(text), want) {
			t.Fatalf("日志汇总缺 %q: %s", want, stripANSI(text))
		}
	}
	// 无 git：目录时间降级
	text2 := buildProjectLogText(p, structure.GitInfo{})
	if !strings.Contains(stripANSI(text2), "目录时间") {
		t.Fatalf("无 git 应降级显示目录时间: %s", stripANSI(text2))
	}
}

func TestOpenInIDEMissingCommand(t *testing.T) {
	ws := projectFixture(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	cfg.Project.Opener = "gs-definitely-not-exist-cmd"
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := openInIDE(ctx, "x.go"); err == nil {
		t.Fatal("命令不存在应报错")
	} else if !strings.Contains(err.Error(), "opener") {
		t.Fatalf("报错应提示 opener 配置: %v", err)
	}
}

func TestResolveProject(t *testing.T) {
	ps := []workspace.Project{
		{Name: "HDU-KillCourse", Path: "/x/k", Rel: "Study/Project/HDU-KillCourse"},
		{Name: "gs", Path: "/x/g", Rel: "Project/gs"},
	}
	if p := resolveProject(ps, "kill"); p == nil || p.Name != "HDU-KillCourse" {
		t.Fatalf("关键词应命中: %+v", p)
	}
	if p := resolveProject(ps, "GS"); p == nil || p.Name != "gs" {
		t.Fatal("匹配应大小写不敏感")
	}
	if resolveProject(ps, "不存在") != nil {
		t.Fatal("未命中应返回 nil")
	}
}

func TestProjectOptions(t *testing.T) {
	// 无 gs：尾部出现加入提示项
	opts, prompt := projectOptions([]workspace.Project{
		{Name: "demo", Path: "/x", Rel: "Study/Project/demo"},
	})
	if !prompt {
		t.Fatal("gs 未在列表时应显示提示项")
	}
	last := stripANSI(opts[len(opts)-1].Label)
	if !strings.Contains(last, "加入项目列表") {
		t.Fatalf("尾项应为提示项: %q", last)
	}
	// 有 gs：无提示项，标签带「本工具」
	opts2, prompt2 := projectOptions([]workspace.Project{
		{Name: "gs", Path: "/x", Rel: "Project/gs"},
	})
	if prompt2 {
		t.Fatal("gs 在列表时不应显示提示项")
	}
	if l := stripANSI(opts2[0].Label); !strings.Contains(l, "本工具") {
		t.Fatalf("gs 标签应含本工具: %q", l)
	}
	// 手动项目标签
	opts3, _ := projectOptions([]workspace.Project{
		{Name: "misc", Path: "/x", Rel: "Study/misc", Manual: true},
	})
	if l := stripANSI(opts3[0].Label); !strings.Contains(l, "手动") {
		t.Fatalf("手动项目标签应含「手动」: %q", l)
	}
}
