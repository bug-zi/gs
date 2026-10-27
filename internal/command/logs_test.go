package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

func wsForTest(t *testing.T) *workspace.Workspace {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Study/logs", "Study/plan"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &workspace.Workspace{Root: root}
}

func TestAppendEntry(t *testing.T) {
	ws := wsForTest(t)
	today := time.Now().In(workspace.CST)
	if err := AppendEntry(ws, today, "跑通了CAS登录"); err != nil {
		t.Fatal(err)
	}
	if err := AppendEntry(ws, today, "看了GC三色标记"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(ws.LogsDir(), workspace.LogFileName(today)))
	if err != nil {
		t.Fatal(err)
	}
	want := "- 跑通了CAS登录\n- 看了GC三色标记\n"
	if string(data) != want {
		t.Fatalf("got %q want %q", data, want)
	}
}

func TestEntryTooLong(t *testing.T) {
	s40 := strings.Repeat("学", 40)
	s41 := s40 + "习"
	if EntryTooLong(s40) {
		t.Fatal("40字不应超限")
	}
	if !EntryTooLong(s41) {
		t.Fatal("41字应超限")
	}
}

func TestFilterLines(t *testing.T) {
	content := "- 确认空版本提示\n- 继续走读登录模块\n- 建立学习总览\n"
	got := FilterLines(content, "登录")
	if len(got) != 1 || !strings.Contains(got[0], "登录") {
		t.Fatalf("got %v", got)
	}
}

func TestViewFormat(t *testing.T) {
	ws := wsForTest(t)
	yesterday := time.Now().In(workspace.CST).AddDate(0, 0, -1)
	if err := AppendEntry(ws, yesterday, "复习了map底层"); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	shown, err := viewLog(&out, ws, yesterday, false)
	if err != nil || !shown {
		t.Fatalf("viewLog shown=%v err=%v", shown, err)
	}
	if !strings.Contains(out.String(), "复习了map底层") {
		t.Fatalf("输出缺内容: %q", out.String())
	}
}

func TestViewByKeywordHeadingRendered(t *testing.T) {
	ws := wsForTest(t)
	if err := os.WriteFile(filepath.Join(ws.LogsDir(), "261025.md"), []byte("## 并发小节\n- 并发普通记录一行\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := viewByKeyword(&out, ws, "并发"); err != nil {
		t.Fatal(err)
	}
	got := stripANSI(out.String())
	if strings.Contains(got, "##") {
		t.Fatalf("标题命中行不应残留 ## 前缀: %q", got)
	}
	if !strings.Contains(got, "并发小节") || !strings.Contains(got, "并发普通记录一行") {
		t.Fatalf("标题行与普通行都应输出: %q", got)
	}
}

func TestViewByDayArg(t *testing.T) {
	ws := wsForTest(t)
	if err := os.WriteFile(filepath.Join(ws.LogsDir(), "261024.md"), []byte("- 十月二十四的记录\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	shown, err := viewByDayArg(&out, ws, "261024") // 六位年月日
	if err != nil || !shown {
		t.Fatalf("shown=%v err=%v", shown, err)
	}
	if !strings.Contains(out.String(), "十月二十四的记录") {
		t.Fatalf("六位日期应命中: %s", out.String())
	}
	out.Reset()
	shown, err = viewByDayArg(&out, ws, "1024") // 四位月日
	if err != nil || !shown || !strings.Contains(out.String(), "十月二十四的记录") {
		t.Fatalf("四位日期应命中: shown=%v err=%v out=%s", shown, err, out.String())
	}
	out.Reset()
	shown, _ = viewByDayArg(&out, ws, "999999")
	if !shown || !strings.Contains(out.String(), "没有 999999 的日志") {
		t.Fatalf("无日志应提示: %s", out.String())
	}
}

// logs 专项规则：日志文档不论行数都在 CLI 查阅，不转交外部编辑器（显示策略优先于长短分流）
func TestRunDayPager长日志仍CLI查阅(t *testing.T) {
	fakeEditor(t)
	muteStdin(t)
	ws := wsForTest(t)
	day := time.Now().In(workspace.CST).AddDate(0, 0, -1)
	body := "标记长日志首行\n" + strings.Repeat("记录行\n", 24) // 25 行 > maxCliLines
	if err := os.WriteFile(filepath.Join(ws.LogsDir(), workspace.LogFileName(day)), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: &config.Config{}, Out: &out}
	if err := runDayPager(ctx, day); err != nil {
		t.Fatalf("runDayPager: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "长文件") {
		t.Errorf("日志不论行数都应留在 CLI，不应转交外部编辑器: %s", got)
	}
	if !strings.Contains(got, "[e]编辑") {
		t.Errorf("长日志应进入 CLI 查阅循环: %s", got)
	}
	if !strings.Contains(got, "标记长日志首行") {
		t.Errorf("日志内容应被渲染: %s", got)
	}
}
