package command

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

// tierDirs 层名 → 目录
var tierDirs = map[string]func(*workspace.Workspace) string{
	"sea":    (*workspace.Workspace).SeaDir,
	"ship":   (*workspace.Workspace).ShipDir,
	"river":  (*workspace.Workspace).RiverDir,
	"spring": (*workspace.Workspace).SpringDir,
	"code":   (*workspace.Workspace).CodeDir,
}

// tierExts 层名 → 条目扩展名（code 层是 .go 演示代码，其余层为 .md 笔记）
var tierExts = map[string]string{
	"sea": ".md", "ship": ".md", "river": ".md", "spring": ".md", "code": ".go",
}

// ListTierFiles 返回指定层（sea/river/spring/code）下全部条目的工作区相对路径（正斜杠），
// 按层过滤扩展名（tierExts）
func ListTierFiles(ws *workspace.Workspace, tier string) ([]string, error) {
	dirFn, ok := tierDirs[tier]
	if !ok {
		return nil, errors.New("未知层: " + tier)
	}
	ext := tierExts[tier]
	var out []string
	err := filepath.WalkDir(dirFn(ws), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 层目录缺失视为空
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ext) {
			return nil
		}
		rel, err := filepath.Rel(ws.Root, path)
		if err != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}

// ListKnowledge 返回知识库全部 .md（sea → ship → river → spring 顺序）
func ListKnowledge(ws *workspace.Workspace) ([]string, error) {
	var out []string
	for _, tier := range []string{"sea", "ship", "river", "spring"} {
		files, err := ListTierFiles(ws, tier)
		if err != nil {
			return nil, err
		}
		out = append(out, files...)
	}
	return out, nil
}
