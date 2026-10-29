package tier

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "Study", "sea", "g", "a.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(root, "Study/sea/g/a.md", "Study/ship/a.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("源文件应已不存在，err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Study", "ship", "a.md"))
	if err != nil || string(data) != "# A\n" {
		t.Fatalf("目标内容异常 data=%q err=%v", data, err)
	}
	// 目标已存在报错
	if err := os.WriteFile(filepath.Join(root, "Study", "ship", "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(root, "Study/sea/g/a.md", "Study/ship/a.md"); err == nil {
		t.Fatal("目标已存在应报错")
	}
}

func TestCopyFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "Study", "sea", "g", "a.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(root, "Study/sea/g/a.md", "Study/ship/a.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("sea 原件应保留，err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Study", "ship", "a.md"))
	if err != nil || string(data) != "# A\n" {
		t.Fatalf("副本内容异常 data=%q err=%v", data, err)
	}
	if err := CopyFile(root, "Study/sea/g/a.md", "Study/ship/a.md"); err == nil {
		t.Fatal("目标已存在应报错")
	}
}
