package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/termkey"
)

func TestPagerKeyString(t *testing.T) {
	cases := []struct {
		name string
		key  termkey.Key
		want string
		ok   bool
	}{
		{"字母小写化", termkey.Key{Kind: termkey.KindRune, R: 'Q'}, "q", true},
		{"空格翻页", termkey.Key{Kind: termkey.KindRune, R: ' '}, " ", true},
		{"回车", termkey.Key{Kind: termkey.KindEnter}, "", true},
		{"esc", termkey.Key{Kind: termkey.KindEsc}, "esc", true},
		{"ctrl+c", termkey.Key{Kind: termkey.KindCtrlC}, "ctrl+c", true},
		{"ctrl+d", termkey.Key{Kind: termkey.KindCtrlD}, "ctrl+d", true},
		{"方向键忽略", termkey.Key{Kind: termkey.KindUp}, "", false},
		{"退格忽略", termkey.Key{Kind: termkey.KindBackspace}, "", false},
	}
	for _, c := range cases {
		got, ok := pagerKeyString(c.key)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: pagerKeyString(%v) = (%q, %v)，want (%q, %v)", c.name, c.key, got, ok, c.want, c.ok)
		}
	}
}

// nonDocExternalFixture 非文档短文件 + 坏编辑器配置的 Ctx：
// 坏编辑器使外部链路在 LookPath 处确定性报错，测试不会真调起编辑器
func nonDocExternalFixture(t *testing.T) (*Ctx, string, *strings.Builder) {
	t.Helper()
	ws := wsForTest(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	cfg.UI.Editor = "gs-no-such-editor-xyz"
	out := &strings.Builder{}
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: out}
	path := filepath.Join(ws.Root, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ctx, path, out
}

func TestRunViewPagerNonDocDirectExternal(t *testing.T) {
	ctx, path, out := nonDocExternalFixture(t)
	err := runViewPager(ctx, func() string { return "" }, func() (string, bool) { return path, true })
	if err == nil || !strings.Contains(err.Error(), "编辑器不可用") {
		t.Fatalf("非文档应直开外部编辑器，got %v", err)
	}
	if !strings.Contains(stripANSI(out.String()), "正在打开") {
		t.Fatalf("应先打印打开提示: %s", out.String())
	}
}

func TestRunViewPagerDelNonDocDirectExternal(t *testing.T) {
	ctx, path, _ := nonDocExternalFixture(t)
	err := runViewPagerDel(ctx, func() string { return "" }, func() (string, bool) { return path, true }, nil)
	if err == nil || !strings.Contains(err.Error(), "编辑器不可用") {
		t.Fatalf("非文档应直开外部编辑器，got %v", err)
	}
}
