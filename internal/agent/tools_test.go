package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, tl Tool, args string) string {
	t.Helper()
	out, err := tl.Exec(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("Exec(%s): %v", args, err)
	}
	return out
}

func TestResolveIn(t *testing.T) {
	root := t.TempDir()
	p, err := resolveIn(root, "Study/sea/a.md")
	if err != nil || p != filepath.Join(root, "Study", "sea", "a.md") {
		t.Fatalf("p=%s err=%v", p, err)
	}
	for _, bad := range []string{"../evil.md", "/etc/passwd", "C:\\evil.md", ".", " a/../.."} {
		if _, err := resolveIn(root, bad); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}
}

func TestSearchTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Study", "sea"), 0o755)
	os.MkdirAll(filepath.Join(root, "Resource"), 0o755)
	os.WriteFile(filepath.Join(root, "Study", "sea", "a.md"), []byte("# channel\nchannel 缓冲要点\n"), 0o644)
	os.WriteFile(filepath.Join(root, "Resource", "skip.md"), []byte("channel 在这里\n"), 0o644)
	out := run(t, SearchTool(root, []string{"Resource"}, 512, 20, 5), `{"query":"channel"}`)
	if !strings.Contains(out, "Study/sea/a.md") || !strings.Contains(out, "命中 1 个文件") {
		t.Fatalf("out=%s", out)
	}
	if run(t, SearchTool(root, nil, 512, 20, 5), `{"query":"不存在词xyz"}`) != "无命中" {
		t.Fatal("应报无命中")
	}
}

func TestReadFileTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Study"), 0o755)
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("第%d行", i))
	}
	os.WriteFile(filepath.Join(root, "Study", "a.md"), []byte(strings.Join(lines, "\n")), 0o644)
	tl := ReadFileTool(root, 4)
	out := run(t, tl, `{"path":"Study/a.md"}`)
	if !strings.HasPrefix(out, "1\t第1行") || !strings.Contains(out, "offset=5") {
		t.Fatalf("out=%s", out)
	}
	out = run(t, tl, `{"path":"Study/a.md","offset":5,"limit":3}`)
	if !strings.Contains(out, "5\t第5行") || !strings.Contains(out, "7\t第7行") || strings.Contains(out, "8\t") {
		t.Fatalf("out=%s", out)
	}
	if out = run(t, tl, `{"path":"Study/a.md","offset":99}`); !strings.Contains(out, "越界") {
		t.Fatalf("out=%s", out)
	}
}

func TestListDirTool(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "Study"), 0o755)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("hi"), 0o644)
	out := run(t, ListDirTool(root), `{}`)
	if !strings.Contains(out, "Study/") || !strings.Contains(out, "README.md") {
		t.Fatalf("out=%s", out)
	}
}

func TestWriteFileTool(t *testing.T) {
	root := t.TempDir()
	written := ""
	tl := WriteFileTool(root, func(abs, content string) bool {
		written = abs + "|" + content
		return true
	})
	out := run(t, tl, `{"path":"Study/new.md","content":"# 新内容"}`)
	if out != "已写入 Study/new.md" {
		t.Fatalf("out=%s", out)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "new.md"))
	if string(data) != "# 新内容" || !strings.HasSuffix(written, "|# 新内容") {
		t.Fatalf("落盘=%q written=%q", data, written)
	}
	// 拒绝：不落盘且返回拒绝文本
	tl2 := WriteFileTool(root, func(abs, content string) bool { return false })
	out = run(t, tl2, `{"path":"Study/new.md","content":"改"}`)
	if !strings.Contains(out, "用户拒绝写入") || !strings.Contains(out, "改为文字建议") {
		t.Fatalf("out=%s", out)
	}
	data, _ = os.ReadFile(filepath.Join(root, "Study", "new.md"))
	if string(data) != "# 新内容" {
		t.Fatal("拒绝后不应改盘")
	}
	// 扩展名白名单 / 越界拒绝走 ai.SafePath
	if _, err := tl.Exec(context.Background(), json.RawMessage(`{"path":"a.exe","content":"x"}`)); err == nil {
		t.Fatal(".exe 应被拒绝")
	}
	if _, err := tl.Exec(context.Background(), json.RawMessage(`{"path":"../evil.md","content":"x"}`)); err == nil {
		t.Fatal("越界路径应被拒绝")
	}
}
