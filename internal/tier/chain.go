// Package tier 四站流转执行器（sea/ship/river 文件迁移、出处链改写）。纯逻辑，不 import ui。
package tier

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

const chainPrefix = "> 出处："

// chainRel 从「> 出处：X」行提取 X；非出处行返回 ""
func chainRel(line string) string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, chainPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(t, chainPrefix))
}

// walkMD 遍历 root 下全部 .md（跳过点开头的子目录），对每个文件调用 fn
func walkMD(root string, fn func(path string, data []byte)) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		fn(path, data)
		return nil
	})
}

// CountChainRefs 全库出处行指向 rel 的次数
func CountChainRefs(root, rel string) int {
	n := 0
	walkMD(root, func(_ string, data []byte) {
		for _, line := range strings.Split(string(data), "\n") {
			if chainRel(line) == rel {
				n++
			}
		}
	})
	return n
}

// RewriteSourceChains 全库把出处行中的 oldRel 改写为 newRel（行内容精确匹配），
// 逐文件原子写；返回被改写的文件数。
func RewriteSourceChains(root, oldRel, newRel string) (int, error) {
	changed := 0
	var firstErr error
	walkMD(root, func(path string, data []byte) {
		lines := strings.Split(string(data), "\n")
		hit := false
		for i, line := range lines {
			if chainRel(line) == oldRel {
				lines[i] = chainPrefix + newRel
				hit = true
			}
		}
		if !hit {
			return
		}
		if err := workspace.WriteFileAtomic(path, []byte(strings.Join(lines, "\n"))); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		changed++
	})
	return changed, firstErr
}
