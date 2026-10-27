package tier

import (
	"os"
	"path/filepath"

	"github.com/bugzi/gs/internal/roadmap"
)

// ApplyRenumber 执行重排计划：逐条迁移文件（目标目录自动创建），迁移后清理 river 下变空的
// 一级目录。幂等：From 不存在的条目跳过，可中断重跑。
func ApplyRenumber(root string, plan *roadmap.RenumberPlan) error {
	all := append(append([]roadmap.FileMove{}, plan.Moves...), plan.Orphans...)
	for _, m := range all {
		src := filepath.Join(root, filepath.FromSlash(m.From))
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // 已迁移过（重跑）
		}
		if err := MoveFile(root, m.From, m.To); err != nil {
			return err
		}
	}
	cleanupEmptyRiverDirs(root)
	return nil
}

// cleanupEmptyRiverDirs 删除 river 下变空的一级子目录（不动 river 根与散件）
func cleanupEmptyRiverDirs(root string) {
	riverDir := filepath.Join(root, filepath.FromSlash(roadmap.RiverDirRel))
	entries, err := os.ReadDir(riverDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(riverDir, e.Name())
		if sub, err := os.ReadDir(p); err == nil && len(sub) == 0 {
			os.Remove(p)
		}
	}
}
