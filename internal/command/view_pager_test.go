package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 短文件（≤20 行）查阅时必须整篇直出：分页按终端页高切页，矮窗口下短笔记
// 会被静默切半屏等按键，表现为「显示不完整」
func TestPagerShouldPage短文件不分页(t *testing.T) {
	p := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(p, []byte(strings.Repeat("行\n", 12)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := pagerShouldPage(pagerPaged, func() (string, bool) { return p, true }); got {
		t.Errorf("12 行短文件不应分页，应整篇直出")
	}
}

func TestPagerShouldPage逾行文件分页(t *testing.T) {
	p := filepath.Join(t.TempDir(), "long.md")
	if err := os.WriteFile(p, []byte(strings.Repeat("行\n", 25)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := pagerShouldPage(pagerPaged, func() (string, bool) { return p, true }); !got {
		t.Errorf("25 行逾行文件应按页高分页")
	}
}

func TestPagerShouldPageFull模式与无编辑目标不分页(t *testing.T) {
	if got := pagerShouldPage(pagerFull, func() (string, bool) { return "x.md", true }); got {
		t.Errorf("pagerFull 一律整篇直出")
	}
	if got := pagerShouldPage(pagerPaged, func() (string, bool) { return "", false }); !got {
		t.Errorf("无可编辑目标的 paged 模式仍应按终端页高分页")
	}
}
