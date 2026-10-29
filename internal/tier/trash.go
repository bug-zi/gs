package tier

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

// TrashDirRel 回收站根（工作区相对）。位于 .gs/ 下，不在 Study/、Project/ 内容区，
// 搜索经默认 scan.exclude 的 ".gs" 排除。
const TrashDirRel = ".gs/trash"

const trashReadme = `# gs 回收站

此处存放 gs 删除机制移入的条目（软删除，不自动清理）。

目录结构：<删除时间戳>-<序号>/<原工作区相对路径>。

找回方法：把对应文件/目录 mv 回原相对路径即可，例如

    mv .gs/trash/20261028-180000-1/Study/river/a.md Study/river/a.md

条目删除时绑定已清；找回后可重新挂载（/plan 浏览器 a 键）。
`

// MoveToTrash 软删除：把 rel（文件或目录）移入 <root>/.gs/trash/<YYYYMMDD-HHMMSS>-<n>/<rel>，
// 保留原相对结构。同秒多次删除以 n 递增防碰撞；首次使用时写 README 找回说明。
// 返回回收站内工作区相对路径。
func MoveToTrash(root, rel string, now time.Time) (string, error) {
	src := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("源不存在: %s", rel)
	}
	stamp := now.In(workspace.CST).Format("20060102-150405")
	for n := 1; ; n++ {
		dstRel := fmt.Sprintf("%s/%s-%d/%s", TrashDirRel, stamp, n, rel)
		dst := filepath.Join(root, filepath.FromSlash(dstRel))
		if _, err := os.Lstat(dst); err == nil {
			continue // 同秒同路径已占用，顺延序号
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return "", err
		}
		if err := os.Rename(src, dst); err != nil {
			return "", err
		}
		writeTrashReadme(root)
		return dstRel, nil
	}
}

// writeTrashReadme 首次使用时写找回说明；已存在则跳过（失败不影响删除本身）
func writeTrashReadme(root string) {
	p := filepath.Join(root, filepath.FromSlash(TrashDirRel), "README.md")
	if _, err := os.Stat(p); err == nil {
		return
	}
	_ = workspace.WriteFileAtomic(p, []byte(trashReadme))
}
