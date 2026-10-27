package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/ai"
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

func TestMaybeSaveAIChatSkipsSaved(t *testing.T) {
	ws := wsForTest(t)
	st := &AIState{Hist: []ai.Message{{Role: "user", Content: "q"}}, Saved: true}
	var out strings.Builder
	ctx := &Ctx{WS: ws, Out: &out, AI: st}
	if err := MaybeSaveAIChat(ctx); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(ws.SeaDir(), "ai"))
	if len(entries) != 0 {
		t.Fatal("已存档过的会话不应重复存档")
	}
}

func TestMaybeSaveAIChatSkipsEmpty(t *testing.T) {
	ws := wsForTest(t)
	var out strings.Builder
	ctx := &Ctx{WS: ws, Out: &out, AI: &AIState{}}
	if err := MaybeSaveAIChat(ctx); err != nil {
		t.Fatal(err)
	}
}
