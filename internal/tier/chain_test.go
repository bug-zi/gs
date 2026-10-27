package tier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chainFixture(t *testing.T) string {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Study/river/a.md", "# A\n\n> 出处：Study/sea/x.md\n\n正文\n")
	write("Study/river/b.md", "# B\n\n> 出处：Study/sea/y.md\n")
	write("Study/spring/c.md", "> 出处：Study/sea/x.md")     // 文件尾无换行
	write("Study/sea/x.md", "# X\n> 出处：Study/sea/x.md\n") // 自引用仅验证匹配
	write(".gs/topics.toml", "topic = 1\n")               // 点目录跳过
	return root
}

func TestCountAndRewriteSourceChains(t *testing.T) {
	root := chainFixture(t)
	if n := CountChainRefs(root, "Study/sea/x.md"); n != 3 {
		t.Fatalf("CountChainRefs = %d, want 3", n)
	}
	changed, err := RewriteSourceChains(root, "Study/sea/x.md", "Study/ship/x.md")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 3 {
		t.Fatalf("changed = %d, want 3", changed)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "river", "a.md"))
	if !strings.Contains(string(data), "> 出处：Study/ship/x.md") {
		t.Fatalf("改写未生效，文件内容：\n%s", data)
	}
	if n := CountChainRefs(root, "Study/sea/x.md"); n != 0 {
		t.Fatalf("改写后仍有残留引用 %d 处", n)
	}
	if changed, _ := RewriteSourceChains(root, "Study/sea/none.md", "Study/ship/none.md"); changed != 0 {
		t.Fatalf("无关路径 changed = %d, want 0", changed)
	}
}
