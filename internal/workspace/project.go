package workspace

import (
	"os"
	"path/filepath"
)

type Project struct {
	Name   string // 目录名
	Path   string // 绝对路径
	Rel    string // 相对 Root 的正斜杠路径
	Manual bool   // 来自注册表（自动发现未命中）
}

// Projects 有效项目列表 = 自动发现 ∪ 注册表；gs 自身仅在注册后出现
func (w *Workspace) Projects() []Project {
	patterns := []string{"Study/Project/*", "Project/*", "Study/*"}
	seen := map[string]bool{}
	var out []Project
	add := func(abs string, manual bool) {
		if seen[abs] {
			return
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return
		}
		name := filepath.Base(abs)
		// gs 自身：未注册则隐藏；注册表里的 gs 条目按 manual 直通
		if name == "gs" && !manual && !w.gsRegistered() {
			return
		}
		// 自动发现仍要求项目标志；注册表条目无条件纳入。
		// 去重标记放在校验之后：未通过自动发现的目录不占位，注册表仍可补录
		if !manual && !isProjectDir(abs) {
			return
		}
		seen[abs] = true
		out = append(out, Project{Name: name, Path: abs, Rel: w.Rel(abs), Manual: manual})
	}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(filepath.Join(w.Root, filepath.FromSlash(pat)))
		for _, m := range matches {
			add(m, false)
		}
	}
	if reg, err := LoadRegistry(w.RegistryPath()); err == nil {
		for _, e := range reg.Projects {
			add(filepath.Join(w.Root, filepath.FromSlash(e.Rel)), true)
		}
	}
	return out
}

// gsRegistered 注册表中是否存在目录名为 gs 的条目
func (w *Workspace) gsRegistered() bool {
	reg, err := LoadRegistry(w.RegistryPath())
	if err != nil {
		return false
	}
	for _, e := range reg.Projects {
		if filepath.Base(filepath.FromSlash(e.Rel)) == "gs" {
			return true
		}
	}
	return false
}

func isProjectDir(dir string) bool {
	for _, marker := range []string{"go.mod", "README.md", "README", ".git"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}
