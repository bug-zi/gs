package seaport

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/workspace"
)

// portFixture 最小工作区:Study/logs+plan(workspace 判定)+ sea 两个主题目录 + seaport
func portFixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{
		"Study/logs", "Study/plan",
		"Study/sea/Go语言基础", "Study/sea/GPT",
		"seaport",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &workspace.Workspace{Root: root}
}

func TestDir(t *testing.T) {
	ws := portFixture(t)
	want := filepath.Join(ws.Root, "seaport")
	if got := Dir(ws); got != want {
		t.Fatalf("Dir = %s, want %s", got, want)
	}
}

func TestEnsureCreatesDir(t *testing.T) {
	ws := portFixture(t)
	if err := os.RemoveAll(Dir(ws)); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(ws); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(Dir(ws)); err != nil || !info.IsDir() {
		t.Fatal("seaport 目录应被创建")
	}
}

func TestScanFiltersExtAndSubdirs(t *testing.T) {
	ws := portFixture(t)
	for _, name := range []string{"闭包.md", "笔记.TXT", "图.png", "小抄.txt"} {
		if err := os.WriteFile(filepath.Join(Dir(ws), name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(Dir(ws), "子目录"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, skipped := Scan(ws)
	if len(files) != 3 {
		t.Fatalf("files = %v, want 3 个(md/txt 不分大小写)", files)
	}
	if len(skipped) != 1 || skipped[0] != "图.png" {
		t.Fatalf("skipped = %v, want [图.png]", skipped)
	}
}

func TestScanMissingDir(t *testing.T) {
	ws := portFixture(t)
	if err := os.RemoveAll(Dir(ws)); err != nil {
		t.Fatal(err)
	}
	files, skipped := Scan(ws)
	if files != nil || skipped != nil {
		t.Fatalf("目录缺失应返回空:%v %v", files, skipped)
	}
}

func TestCandidates(t *testing.T) {
	ws := portFixture(t)
	got := Candidates(ws)
	// sea 下两个主题目录 + 兜底"未分类"始终在列
	want := map[string]bool{"Go语言基础": true, "GPT": true, Fallback: true}
	if len(got) != len(want) {
		t.Fatalf("Candidates = %v, want %v", got, want)
	}
	for _, c := range got {
		if !want[c] {
			t.Fatalf("Candidates = %v, 多出 %q", got, c)
		}
	}
}

func TestParseClassify(t *testing.T) {
	m, err := parseClassify("```json\n{\"分类\":{\"闭包.md\":\"Go语言基础\"}}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if m["闭包.md"] != "Go语言基础" {
		t.Fatalf("m = %v", m)
	}
	if _, err := parseClassify("抱歉我无法回答"); err == nil {
		t.Fatal("无 JSON 应报错")
	}
}

type fakeChat struct {
	reply string
	err   error
}

func (f fakeChat) Chat(ctx context.Context, msgs []ai.Message) (string, error) {
	return f.reply, f.err
}

func putPort(t *testing.T, ws *workspace.Workspace, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(Dir(ws), name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportMovesByClass(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "闭包.md", "# 闭包\nGo 闭包捕获变量……")
	res := Import(context.Background(), ws, fakeChat{reply: `{"分类":{"闭包.md":"Go语言基础"}}`})
	if res.Note != "" {
		t.Fatalf("Note = %s", res.Note)
	}
	if len(res.Items) != 1 || res.Items[0].Err != "" {
		t.Fatalf("Items = %+v", res.Items)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包.md")); err != nil {
		t.Fatal("应迁移到 Go语言基础/", err)
	}
	if _, err := os.Stat(filepath.Join(Dir(ws), "闭包.md")); !os.IsNotExist(err) {
		t.Fatal("seaport 原件应消失")
	}
}

func TestImportUnknownDirFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, fakeChat{reply: `{"分类":{"a.md":"不存在的目录"}}`})
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal("目录不匹配应进未分类:", err)
	}
	if res.Items[0].Err != "" {
		t.Fatalf("未分类是正常迁移,不应有 Err: %+v", res.Items)
	}
}

func TestImportParseFailFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, fakeChat{reply: "我分不出来"})
	if res.Note == "" {
		t.Fatal("应有失败说明 Note")
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestImportChatErrFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, fakeChat{err: errors.New("网络断开")})
	if !strings.Contains(res.Note, "网络断开") {
		t.Fatalf("Note = %s", res.Note)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestImportNilClientFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, nil)
	if !strings.Contains(res.Note, "AI 未配置") {
		t.Fatalf("Note = %s", res.Note)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestImportNameConflictSuffix(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "闭包.md", "内容")
	if err := os.MkdirAll(filepath.Join(ws.SeaDir(), "Go语言基础"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包.md"), []byte("旧的"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Import(context.Background(), ws, fakeChat{reply: `{"分类":{"闭包.md":"Go语言基础"}}`})
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包-2.md")); err != nil {
		t.Fatal("重名应加后缀:", err)
	}
	old, _ := os.ReadFile(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包.md"))
	if string(old) != "旧的" {
		t.Fatal("已存在文件不应被覆盖")
	}
	if res.Items[0].Err != "" {
		t.Fatalf("Items = %+v", res.Items)
	}
}

func TestImportCtxCancelledStops(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "1")
	putPort(t, ws, "b.md", "2")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := Import(ctx, ws, fakeChat{reply: `{"分类":{"a.md":"GPT","b.md":"GPT"}}`})
	if _, err := os.Stat(filepath.Join(Dir(ws), "a.md")); err != nil {
		t.Fatal("取消后文件应留在 seaport:", err)
	}
	if res.Note == "" {
		t.Fatal("应有取消说明")
	}
}

func TestImportEmpty(t *testing.T) {
	ws := portFixture(t)
	res := Import(context.Background(), ws, nil)
	if len(res.Items) != 0 {
		t.Fatalf("Items = %+v", res.Items)
	}
}

func TestPrint(t *testing.T) {
	var out strings.Builder
	Print(&out, Result{
		Items: []Item{
			{File: "闭包.md", Dest: "Study/sea/Go语言基础/闭包.md"},
			{File: "随手.md", Dest: "Study/sea/未分类/随手.md"},
			{File: "锁.md", Err: "被占用"},
		},
		Note: "",
	})
	s := out.String()
	for _, want := range []string{"seaport 导入", "Go语言基础", "未分类", "锁.md 导入失败:被占用"} {
		if !strings.Contains(s, want) {
			t.Fatalf("输出缺 %q:\n%s", want, s)
		}
	}
}

func TestPrintNoteAndEmpty(t *testing.T) {
	var out strings.Builder
	Print(&out, Result{Note: "AI 未配置,全部放入未分类"})
	if !strings.Contains(out.String(), "AI 未配置") {
		t.Fatalf("输出 = %s", out.String())
	}
}
