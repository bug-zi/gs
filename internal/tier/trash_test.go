package tier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

var trashNow = time.Date(2026, 10, 28, 18, 0, 0, 0, workspace.CST)

func trashMk(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMoveToTrash(t *testing.T) {
	root := t.TempDir()
	trashMk(t, root, "Study/river/a.md", "# A\n")
	dst, err := MoveToTrash(root, "Study/river/a.md", trashNow)
	if err != nil {
		t.Fatal(err)
	}
	want := ".gs/trash/20261028-180000-1/Study/river/a.md"
	if dst != want {
		t.Fatalf("dst = %s, want %s", dst, want)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dst))); err != nil {
		t.Fatalf("回收站文件缺失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "a.md")); !os.IsNotExist(err) {
		t.Fatal("原文件应已不在")
	}
}

func TestMoveToTrashCollisionBumpsSeq(t *testing.T) {
	root := t.TempDir()
	trashMk(t, root, ".gs/trash/20261028-180000-1/Study/river/a.md", "旧\n")
	trashMk(t, root, "Study/river/a.md", "# A\n")
	dst, err := MoveToTrash(root, "Study/river/a.md", trashNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dst, ".gs/trash/20261028-180000-2/") {
		t.Fatalf("碰撞应顺延序号 2，got %s", dst)
	}
}

func TestMoveToTrashMissingSource(t *testing.T) {
	root := t.TempDir()
	if _, err := MoveToTrash(root, "Study/river/none.md", trashNow); err == nil {
		t.Fatal("源缺失应报错")
	}
}

func TestMoveToTrashDir(t *testing.T) {
	root := t.TempDir()
	trashMk(t, root, "Study/river/01-x/keep.md", "# K\n")
	dst, err := MoveToTrash(root, "Study/river/01-x", trashNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dst), "keep.md")); err != nil {
		t.Fatalf("目录应整体迁入: %v", err)
	}
}

func TestMoveToTrashReadmeOnce(t *testing.T) {
	root := t.TempDir()
	trashMk(t, root, "Study/river/a.md", "# A\n")
	trashMk(t, root, "Study/river/b.md", "# B\n")
	if _, err := MoveToTrash(root, "Study/river/a.md", trashNow); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(root, ".gs", "trash", "README.md")
	data1, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("首次删除应写 README: %v", err)
	}
	if _, err := MoveToTrash(root, "Study/river/b.md", trashNow); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(readme)
	if string(data1) != string(data2) {
		t.Fatal("README 已存在时不应改写")
	}
}
