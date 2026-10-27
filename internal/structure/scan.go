// Package structure 项目结构纯逻辑：框架树扫描、介绍缓存、笔记关联、git 统计
package structure

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry 框架树节点
type Entry struct {
	Name  string // 文件或目录名
	Rel   string // 相对项目根的正斜杠路径
	IsDir bool
}

// 参与框架树展示的文件类型（其余忽略降噪）
var scanExts = map[string]bool{
	".go": true, ".md": true, ".mod": true, ".sum": true,
	".yaml": true, ".yml": true, ".json": true, ".toml": true,
	".html": true, ".css": true, ".js": true,
}

// Scan 列出 dir 下一层节点：目录在前、同类按名排序。
// prefix 为 dir 相对项目根的正斜杠路径（根传 ""），Rel 据此拼接。
// 跳过隐藏项、exclude 段名与无关文件类型。
func Scan(dir, prefix string, exclude []string) []Entry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	ex := map[string]bool{}
	for _, e := range exclude {
		ex[e] = true
	}
	var out []Entry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		rel := name
		if prefix != "" {
			rel = prefix + "/" + name
		}
		if e.IsDir() {
			if ex[name] {
				continue
			}
			out = append(out, Entry{Name: name, Rel: rel, IsDir: true})
			continue
		}
		if !scanExts[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		out = append(out, Entry{Name: name, Rel: rel, IsDir: false})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out
}
