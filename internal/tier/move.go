package tier

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bugzi/gs/internal/workspace"
)

// MoveFile 同盘 rename 迁移；目标目录不存在则创建。rel 为工作区相对正斜杠路径。
func MoveFile(root, srcRel, dstRel string) error {
	src := filepath.Join(root, filepath.FromSlash(srcRel))
	dst := filepath.Join(root, filepath.FromSlash(dstRel))
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("目标已存在: %s", dstRel)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// CopyFile 复制出副本，源文件保留（装船语义：sea 原件不动）；目标目录不存在则创建，
// 内容走原子写落盘。rel 为工作区相对正斜杠路径。
func CopyFile(root, srcRel, dstRel string) error {
	src := filepath.Join(root, filepath.FromSlash(srcRel))
	dst := filepath.Join(root, filepath.FromSlash(dstRel))
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("目标已存在: %s", dstRel)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(dst, data)
}
