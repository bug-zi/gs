package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/planview"
	"github.com/bugzi/gs/internal/roadmap"
)

func TestScanUnbound(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"Study/sea/主题A", "Study/sea/主题B", "Study/code", "Study/river"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{
		"Study/sea/主题A/已挂.md", "Study/sea/主题A/未挂.md", "Study/sea/主题B/另.md",
		"Study/code/demo.go", "Study/code/readme.md", "Study/river/无主.md",
	} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rm, err := roadmap.Parse([]byte("version = 2\n\n[[branch]]\ntitle = \"主\"\n\n  [[branch.point]]\n  title = \"点\"\n  sea = [\"Study/sea/主题A/已挂.md\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := scanUnbound(root, rm, planview.TierSea)
	want := []string{"Study/sea/主题A/未挂.md", "Study/sea/主题B/另.md"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("sea 未归类 = %+v", got)
	}
	code := scanUnbound(root, rm, planview.TierCode)
	if len(code) != 1 || code[0] != "Study/code/demo.go" {
		t.Fatalf("code 未归类 = %+v（应只认 .go）", code)
	}
	river := scanUnbound(root, rm, planview.TierRiver)
	if len(river) != 1 || river[0] != "Study/river/无主.md" {
		t.Fatalf("river 未归类 = %+v", river)
	}
}

func TestScanUnboundMissingDir(t *testing.T) {
	rm, err := roadmap.Parse([]byte("version = 2\n\n[[branch]]\ntitle = \"主\"\n\n  [[branch.point]]\n  title = \"点\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := scanUnbound(t.TempDir(), rm, planview.TierSpring); len(got) != 0 {
		t.Fatalf("目录缺失应计 0: %+v", got)
	}
}
