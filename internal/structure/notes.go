package structure

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Note 一篇项目相关学习笔记
type Note struct {
	Abs     string // 绝对路径
	Display string // 列表展示用路径
}

// CollectNotes 聚合两处笔记来源：项目内 docs/study 与笔记海 seaDir（Study/sea/<项目名>）
func CollectNotes(projectPath, seaDir string) (projectNotes, seaNotes []Note) {
	projectNotes = walkNotes(filepath.Join(projectPath, "docs", "study"), "docs/study/")
	seaNotes = walkNotes(seaDir, "")
	return
}

func walkNotes(root, displayPrefix string) []Note {
	var out []Note
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		out = append(out, Note{Abs: p, Display: displayPrefix + filepath.ToSlash(rel)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Display < out[j].Display })
	return out
}

// WalkGoFiles 递归收集项目全部 .go 文件（跳过隐藏目录与 exclude 段名）
func WalkGoFiles(projectPath string, exclude []string) []Entry {
	ex := map[string]bool{}
	for _, e := range exclude {
		ex[e] = true
	}
	var out []Entry
	filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || ex[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".go") {
			return nil
		}
		rel, err := filepath.Rel(projectPath, p)
		if err != nil {
			return nil
		}
		out = append(out, Entry{Name: d.Name(), Rel: filepath.ToSlash(rel), IsDir: false})
		return nil
	})
	return out
}

// MatchNotes 命名约定关联：笔记名（去 .md、大小写不敏感）包含源文件完整名，
// 或包含其去扩展名基名（基名长度 ≥4，避免 go 这类短名误撞）。
// 返回 go 文件 Rel -> 关联笔记列表。
func MatchNotes(goFiles []Entry, notes []Note) map[string][]Note {
	out := map[string][]Note{}
	for _, n := range notes {
		stem := strings.ToLower(strings.TrimSuffix(filepath.Base(n.Abs), ".md"))
		for _, g := range goFiles {
			if g.IsDir {
				continue
			}
			name := strings.ToLower(g.Name)
			bare := strings.TrimSuffix(name, filepath.Ext(name))
			if strings.Contains(stem, name) || (len(bare) >= 4 && strings.Contains(stem, bare)) {
				out[g.Rel] = append(out[g.Rel], n)
			}
		}
	}
	return out
}

var goNameRe = regexp.MustCompile(`(?i)([a-z0-9_\-]+\.go)`)

// ReverseResolve 从笔记名反解源文件：提取其中的 .go 文件名，
// 在 goFiles 中按名唯一命中才返回其 Rel
func ReverseResolve(noteAbs string, goFiles []Entry) (string, bool) {
	stem := strings.TrimSuffix(filepath.Base(noteAbs), ".md")
	matches := goNameRe.FindAllString(stem, -1)
	if len(matches) == 0 {
		return "", false
	}
	name := strings.ToLower(matches[0])
	var hits []string
	for _, g := range goFiles {
		if !g.IsDir && strings.ToLower(g.Name) == name {
			hits = append(hits, g.Rel)
		}
	}
	if len(hits) == 1 {
		return hits[0], true
	}
	return "", false
}
