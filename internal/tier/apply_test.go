package tier

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

func TestApplyRenumber(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/river/并发/select.md", "s")
	mk("Study/river/孤儿.md", "o")

	plan := &roadmap.RenumberPlan{
		Moves:   []roadmap.FileMove{{From: "Study/river/并发/select.md", To: "Study/river/02-并发编程/01-select.md"}},
		Orphans: []roadmap.FileMove{{From: "Study/river/孤儿.md", To: "Study/river/99-未挂载/孤儿.md"}},
	}
	if err := ApplyRenumber(root, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "02-并发编程", "01-select.md")); err != nil {
		t.Fatalf("归位失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "99-未挂载", "孤儿.md")); err != nil {
		t.Fatalf("收容失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "并发")); !os.IsNotExist(err) {
		t.Fatal("空目录应被清理")
	}
	// 幂等：重跑不报错
	if err := ApplyRenumber(root, plan); err != nil {
		t.Fatalf("重跑应幂等: %v", err)
	}
}
