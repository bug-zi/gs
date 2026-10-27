package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestHotKeywords(t *testing.T) {
	root := t.TempDir()
	cfg, _ := config.Load(root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/logs/261024.md", "- 学习了 context 超时\n- 继续走读 context 源码\n")
	w("Study/logs/261023.md", "- 复习 slice 与 map\n")
	w("Study/river/Go并发与互斥锁.md", "# x\n")
	w("Study/sea/clip/Go序列化与反序列化.md", "# x\n")
	w("Study/spring/Context超时控制总结.md", "# x\n")

	kw := HotKeywords(root, cfg, 8)
	joined := strings.Join(kw, " ")
	for _, want := range []string{"context", "并发", "互斥锁", "序列化"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("热词缺 %q: %v", want, kw)
		}
	}
	for _, k := range kw {
		if k == "Go" {
			t.Fatalf("Go 前缀不应作为独立热词: %v", kw)
		}
	}
}
