package command

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/notes"
	"github.com/bugzi/gs/internal/workspace"
)

func notesTestTime() time.Time { return time.Date(2026, 10, 29, 14, 30, 0, 0, workspace.CST) }

func TestNotesRouteAdd(t *testing.T) {
	ws := wsForTest(t)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out, Args: []string{"slice、map", "channel 可以看作引用类型"}}
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	ns, err := notes.Load(ws.Root)
	if err != nil || len(ns) != 1 {
		t.Fatalf("应录入一条便签: %+v %v", ns, err)
	}
	if ns[0].Text != "slice、map channel 可以看作引用类型" {
		t.Fatalf("Args 应拼接: %q", ns[0].Text)
	}
	if !strings.Contains(out.String(), "已记录") {
		t.Fatalf("应回显确认: %s", out.String())
	}
}

func TestNotesRouteListEmptyAndNonEmpty(t *testing.T) {
	ws := wsForTest(t)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(out.String()), "暂无便签") {
		t.Fatalf("空应提示: %s", out.String())
	}
	if err := notes.Add(ws.Root, "甲", notesTestTime()); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[1]") || !strings.Contains(out.String(), "甲") {
		t.Fatalf("应列出便签: %s", out.String())
	}
}

func TestNotesRouteEmptyText(t *testing.T) {
	ws := wsForTest(t)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out, Args: []string{"   "}}
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(out.String()), "用法") {
		t.Fatalf("空文本应提示用法: %s", out.String())
	}
}

// moveNoteCore move 的非交互核心：原文追加目标 + 从缓存移除（可直测）
func TestMoveNoteCore(t *testing.T) {
	ws := wsForTest(t)
	target := filepath.Join(ws.Root, "Study", "spring", "切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片\n\n总结\n"), 0o644)
	if err := notes.Add(ws.Root, "扩容翻倍", notesTestTime()); err != nil {
		t.Fatal(err)
	}
	ns, _ := notes.Load(ws.Root)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if err := moveNoteCore(ctx, ns, 0, "Study/spring/切片.md"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "扩容翻倍") {
		t.Fatalf("目标应含便签原文: %q", data)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 0 {
		t.Fatalf("缓存应清空: %+v", rest)
	}
}

func TestBuildTidyPrompt(t *testing.T) {
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("Study/river/01-基础/03-切片.md", "# 切片\n")
	mk("Study/spring/切片.md", "# 切片总结\n")
	ns := []notes.Note{{Time: notesTestTime(), Text: "扩容翻倍"}}
	prompt, err := buildTidyPrompt(ws, ns)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Study/river/01-基础/03-切片.md",
		"Study/spring/切片.md",
		"1. 扩容翻倍",
		"placements",
		"不得新建文件",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("提示词缺 %q:\n%s", want, prompt)
		}
	}
}

func TestBuildTidyPromptNoCandidates(t *testing.T) {
	ws := wsForTest(t)
	if _, err := buildTidyPrompt(ws, []notes.Note{{Time: notesTestTime(), Text: "x"}}); err == nil {
		t.Fatal("无候选应报错")
	}
}

func TestExtractJSON(t *testing.T) {
	if got := extractJSON("```json\n{\"a\":1}\n```"); got != `{"a":1}` {
		t.Fatalf("围栏应剥离: %q", got)
	}
	if got := extractJSON(` {"a":1} `); got != `{"a":1}` {
		t.Fatalf("空白应去除: %q", got)
	}
}

func TestTidyPreviewApply(t *testing.T) {
	ws := wsForTest(t)
	target := filepath.Join(ws.Root, "Study", "river", "01-基础", "03-切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片\n\n原文\n"), 0o644)
	ns := []notes.Note{{Time: notesTestTime(), Text: "扩容翻倍"}}
	notes.Save(ws.Root, ns)
	plan, err := notes.ParsePlanJSON(
		`{"placements":[{"note":1,"target":"Study/river/01-基础/03-切片.md","content":"# 切片\n\n原文\n\n扩容翻倍\n"}]}`, ns)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	asked := ""
	if err := tidyPreviewApply(ctx, ns, plan, func(title string) bool { asked = title; return true }); err != nil {
		t.Fatal(err)
	}
	if asked == "" {
		t.Fatal("应弹出确认")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "# 切片\n\n原文\n\n扩容翻倍\n" {
		t.Fatalf("目标未更新: %q", data)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 0 {
		t.Fatalf("缓存应清空: %+v", rest)
	}
	if !strings.Contains(stripANSI(out.String()), "归位 1 条") {
		t.Fatalf("应汇报结果: %s", out.String())
	}
}

func TestTidyNoPlacementKeepsNotes(t *testing.T) {
	ws := wsForTest(t)
	ns := []notes.Note{{Time: notesTestTime(), Text: "无处可去的便签"}}
	notes.Save(ws.Root, ns)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if err := tidyPreviewApply(ctx, ns, notes.Plan{}, func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 1 {
		t.Fatalf("空方案应原样保留: %+v", rest)
	}
	if !strings.Contains(stripANSI(out.String()), "没有找到合适的归宿") {
		t.Fatalf("应提示保留: %s", out.String())
	}
}

func TestRunNotesTidyBadJSON(t *testing.T) {
	st, ws, cfg := newTestAIState(t, "data: {\"choices\":[{\"delta\":{\"content\":\"不是JSON\"}}]}\n\n")
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("Study/river/a.md", "# A\n")
	notes.Add(ws.Root, "甲", notesTestTime())
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, AI: st}
	if err := runNotesTidy(ctx); err == nil {
		t.Fatal("解析失败应报错")
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 1 {
		t.Fatalf("解析失败便签不得动: %+v", rest)
	}
}

func TestRunNotesTidyEndToEnd(t *testing.T) {
	planContent := `{"placements":[{"note":1,"target":"Study/river/a.md","content":"# A\n\n融入甲\n"}]}`
	ev, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"content": planContent}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	st, ws, cfg := newTestAIState(t, "data: "+string(ev)+"\n\n")
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("Study/river/a.md", "# A\n")
	notes.Add(ws.Root, "甲", notesTestTime())
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, AI: st}
	// 注入确认桩：模拟用户选「应用」
	tidyAsk = func(ctx *Ctx, title string) bool { return true }
	defer func() { tidyAsk = nil }()
	if err := runNotesTidy(ctx); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "river", "a.md"))
	if string(data) != "# A\n\n融入甲\n" {
		t.Fatalf("目标未更新: %q", data)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 0 {
		t.Fatalf("缓存应清空: %+v", rest)
	}
}
