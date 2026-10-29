package command

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func TestSaveAIChat(t *testing.T) {
	ws := wsForTest(t)
	st := &AIState{Hist: []ai.Message{
		{Role: "user", Content: "goroutine 泄漏的常见原因有哪些？请详细说明"},
		{Role: "assistant", Content: "常见原因：channel 阻塞、for-range 退出未 return……"},
	}}
	path, err := SaveAIChat(ws, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "Study/sea/ai/") || !strings.HasSuffix(path, ".md") {
		t.Fatalf("path = %s", path)
	}
	data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "> 存档自 /AI 会话") || !strings.Contains(s, "goroutine 泄漏") {
		t.Fatalf("标题引用行缺失: %s", s)
	}
	if !strings.Contains(s, "## 问") || !strings.Contains(s, "channel 阻塞") {
		t.Fatalf("问答内容缺失: %s", s)
	}
}

func TestSaveAIChatEmpty(t *testing.T) {
	ws := wsForTest(t)
	if _, err := SaveAIChat(ws, &AIState{}); err == nil {
		t.Fatal("空会话应报错")
	}
}

func TestChatTitle(t *testing.T) {
	if got := chatTitle("first line\nsecond line"); got != "first line second line" {
		t.Fatalf("换行应转空格: %q", got)
	}
	if got := chatTitle(strings.Repeat("好", 40)); got != strings.Repeat("好", 30) {
		t.Fatalf("应截取前 30 字: %d 字", len([]rune(got)))
	}
	if got := chatTitle("   \n  "); got != "AI 问答" {
		t.Fatalf("空白应回退默认标题: %q", got)
	}
}

func TestTurnArchiveBody(t *testing.T) {
	b := turnArchiveBody("标题", "怎么问", "怎么答")
	if !strings.Contains(b, "> 存档自 /AI 会话") || !strings.Contains(b, "标题") {
		t.Fatalf("引用行缺失: %s", b)
	}
	if !strings.Contains(b, "## 问\n\n怎么问") || !strings.Contains(b, "## 答\n\n怎么答") {
		t.Fatalf("问答正文缺失: %s", b)
	}
}

// fakeChat 分类测试注入的最小 ChatClient
type fakeChat struct {
	reply string
	err   error
}

func (f *fakeChat) Chat(ctx context.Context, msgs []ai.Message) (string, error) {
	return f.reply, f.err
}

// roadmapForClassifyFixture 两知识点最小路线图（变量与常量 / map）
func roadmapForClassifyFixture() *roadmap.Roadmap {
	return &roadmap.Roadmap{
		Version: 2,
		Branches: []roadmap.Branch{{
			Title: "Go 语言基础",
			Points: []roadmap.Point{
				{Title: "变量与常量", Group: "语法核心"},
				{Title: "map"},
			},
		}},
	}
}

func TestParsePointChoice(t *testing.T) {
	if got, err := parsePointChoice(`{"知识点":"map"}`); err != nil || got != "map" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if got, err := parsePointChoice("```json\n{\"知识点\":\"变量与常量\"}\n```"); err != nil || got != "变量与常量" {
		t.Fatalf("应容忍围栏: got=%q err=%v", got, err)
	}
	if _, err := parsePointChoice("没有 JSON"); err == nil {
		t.Fatal("无 JSON 应报错")
	}
	if _, err := parsePointChoice("{bad json}"); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
}

func TestClassifyPoint(t *testing.T) {
	rm := roadmapForClassifyFixture()
	cands := pointCandidates(rm)
	if len(cands) != 2 {
		t.Fatalf("候选数应为 2: %d", len(cands))
	}
	cc := &fakeChat{reply: `前缀杂文本 {"知识点":"map"} 后缀`}
	if got := classifyPoint(context.Background(), cc, cands, "问", "答"); got != "map" {
		t.Fatalf("应返回候选原话标题: %q", got)
	}
	cc = &fakeChat{reply: `{"知识点":"不存在的知识点"}`}
	if got := classifyPoint(context.Background(), cc, cands, "问", "答"); got != "" {
		t.Fatalf("无效候选应返回空: %q", got)
	}
	cc = &fakeChat{err: errors.New("网络错误")}
	if got := classifyPoint(context.Background(), cc, cands, "问", "答"); got != "" {
		t.Fatalf("AI 失败应返回空: %q", got)
	}
	if got := classifyPoint(context.Background(), cc, nil, "问", "答"); got != "" {
		t.Fatalf("空候选应直接返回空: %q", got)
	}
}

// writeRoadmapFileForTest 把最小路线图写到 ws 的 roadmap.toml
func writeRoadmapFileForTest(t *testing.T, ws *workspace.Workspace) {
	t.Helper()
	toml := "version = 2\n\n[[branch]]\n  title = \"Go 语言基础\"\n\n  [[branch.point]]\n    title = \"变量与常量\"\n    group = \"语法核心\"\n\n  [[branch.point]]\n    title = \"map\"\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveTurnToSeaBound(t *testing.T) {
	ws := wsForTest(t)
	writeRoadmapFileForTest(t, ws)
	var out strings.Builder
	ctx := &Ctx{WS: ws, Out: &out}
	cc := &fakeChat{reply: `{"知识点":"变量与常量"}`}
	if err := archiveTurnToSea(ctx, cc, "iota 是什么", "iota 是常量计数器"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(ws.SeaDir(), "ai"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("sea/ai 应有一个条目: %v %v", entries, err)
	}
	data, _ := os.ReadFile(filepath.Join(ws.SeaDir(), "ai", entries[0].Name()))
	if !strings.Contains(string(data), "## 问\n\niota 是什么") || !strings.Contains(string(data), "iota 是常量计数器") {
		t.Fatalf("问答正文缺失: %s", data)
	}
	rmData, _ := os.ReadFile(ws.RoadmapFile())
	if !strings.Contains(string(rmData), `"Study/sea/ai/`) {
		t.Fatalf("roadmap 应有 sea 绑定: %s", rmData)
	}
	if !strings.Contains(out.String(), "已存入") || !strings.Contains(out.String(), "绑定：变量与常量") {
		t.Fatalf("结果行缺失: %s", out.String())
	}
}

func TestArchiveTurnToSeaFallback(t *testing.T) {
	ws := wsForTest(t)
	writeRoadmapFileForTest(t, ws)
	before, _ := os.ReadFile(ws.RoadmapFile())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Out: &out}
	cc := &fakeChat{reply: `{"知识点":""}`} // AI 认为没有合适的知识点
	if err := archiveTurnToSea(ctx, cc, "问", "答"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(ws.SeaDir(), "未分类"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("未分类应有一个条目: %v %v", entries, err)
	}
	after, _ := os.ReadFile(ws.RoadmapFile())
	if string(before) != string(after) {
		t.Fatal("未命中时 roadmap 不应被改写")
	}
	if strings.Contains(out.String(), "绑定") {
		t.Fatalf("未命中不应出现绑定字样: %s", out.String())
	}
}

func TestArchiveTurnToSeaNoRoadmap(t *testing.T) {
	ws := wsForTest(t) // 不写 roadmap.toml
	var out strings.Builder
	ctx := &Ctx{WS: ws, Out: &out}
	cc := &fakeChat{reply: `{"知识点":"变量与常量"}`}
	if err := archiveTurnToSea(ctx, cc, "问", "答"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), "未分类")); err != nil {
		t.Fatalf("roadmap 缺失应落未分类: %v", err)
	}
}
