package structure

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/bugzi/gs/internal/workspace"
)

// Node 介绍缓存条目
type Node struct {
	Path string `toml:"path"` // 相对项目根的正斜杠路径
	Desc string `toml:"desc"`
}

// StructureCache .gs/structures/<项目名>.toml：AI 生成的节点介绍
type StructureCache struct {
	Generated string `toml:"generated"`
	Nodes     []Node `toml:"nodes"`
}

// StructurePath 项目介绍缓存文件路径
func StructurePath(ws *workspace.Workspace, projectName string) string {
	return filepath.Join(ws.Root, ".gs", "structures", projectName+".toml")
}

// LoadStructureCache 不存在返回空缓存；解析失败返回 error（调用方按无缓存处理）
func LoadStructureCache(path string) (*StructureCache, error) {
	c := &StructureCache{}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, nil
	}
	if err := toml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Find 按 path 精确查找介绍
func (c *StructureCache) Find(path string) string {
	for _, n := range c.Nodes {
		if n.Path == path {
			return n.Desc
		}
	}
	return ""
}

// Merge 合并生成结果（同 path 覆盖），按 path 排序并刷新时间戳
func (c *StructureCache) Merge(nodes []Node) {
	m := map[string]string{}
	for _, n := range c.Nodes {
		m[n.Path] = n.Desc
	}
	for _, n := range nodes {
		m[n.Path] = n.Desc
	}
	paths := make([]string, 0, len(m))
	for p := range m {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	c.Nodes = c.Nodes[:0]
	for _, p := range paths {
		c.Nodes = append(c.Nodes, Node{Path: p, Desc: m[p]})
	}
	c.Generated = time.Now().In(workspace.CST).Format(time.RFC3339)
}

func (c *StructureCache) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(c); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(path, []byte(b.String()))
}
