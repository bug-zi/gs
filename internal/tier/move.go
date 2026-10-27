package tier

import (
	"fmt"
	"os"
	"path/filepath"
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
