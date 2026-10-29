package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestRenderMDHeadingNoPrefix(t *testing.T) {
	out := stripANSI(RenderMD("## 3. 为什么要把完整的读—改—写放在锁内\n\n正文。\n\n```go\nmu.Lock()\n```\n"))
	if strings.Contains(out, "#") {
		t.Fatalf("渲染后不应残留 # 标记: %q", out)
	}
	for _, want := range []string{"为什么要把完整的读—改—写放在锁内", "正文。", "mu.Lock()"} {
		if !strings.Contains(out, want) {
			t.Fatalf("内容丢失 %q: %q", want, out)
		}
	}
}

func TestRenderMD(t *testing.T) {
	out := RenderMD("# 标题\n\n- 条目一\n")
	if strings.TrimSpace(out) == "" {
		t.Fatal("渲染结果不应为空")
	}
	if !strings.Contains(out, "条目一") {
		t.Fatalf("内容丢失: %q", out)
	}
}

func TestHighlightWords(t *testing.T) {
	got := HighlightWords("context 超时与 context 取消", []string{"context"})
	// 全部命中且上色（ANSI 包裹不影响子串计数）
	if strings.Count(got, "context") != 2 {
		t.Fatalf("got %q", got)
	}
	if HighlightWords("无命中", []string{"zzz"}) != "无命中" {
		t.Fatal("无命中应原样返回")
	}
	// 片段顺序保持：剥离 ANSI 后应与原文一致（命中在行中也不得颠倒）
	if s := stripANSI(HighlightWords("- 并发普通记录一行", []string{"并发"})); s != "- 并发普通记录一行" {
		t.Fatalf("片段顺序被破坏: %q", s)
	}
}

func TestRenderGo(t *testing.T) {
	out := RenderGo("package main\n\nfunc main() {}\n")
	if !strings.Contains(out, "package") || !strings.Contains(out, "\x1b[") {
		t.Fatalf("应带 ANSI 高亮输出源码: %q", out)
	}
}

func TestSemanticHelpers(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256) // 测试环境非 TTY，lipgloss 默认退化为无色，强制 256 色
	defer lipgloss.SetColorProfile(termenv.Ascii)
	if s := Gold("gs"); !strings.Contains(s, "220") || !strings.Contains(s, "gs") {
		t.Fatalf("Gold 应含 220 与原文: %q", s)
	}
	if s := Label("目标"); !strings.Contains(s, "179") {
		t.Fatalf("Label 应含 179: %q", s)
	}
	if s := OK("已保存"); !strings.Contains(s, "144") {
		t.Fatalf("OK 应含 144: %q", s)
	}
}
