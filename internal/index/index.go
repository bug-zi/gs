package index

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/bugzi/gs/internal/workspace"
)

type FileRef struct {
	Path string `toml:"path"`
	Hint string `toml:"hint"`
}

type Topic struct {
	Name      string    `toml:"name"`
	Created   string    `toml:"created"`
	Files     []FileRef `toml:"files"`
	Logs      []string  `toml:"logs"`
	Interview string    `toml:"interview"`
}

type Index struct {
	Topics []Topic `toml:"topics"`
}

// New 返回空索引
func New() *Index { return &Index{} }

// Load 文件不存在时返回空索引
func Load(path string) (*Index, error) {
	ix := New()
	data, err := os.ReadFile(path)
	if err != nil {
		return ix, nil
	}
	if err := toml.Unmarshal(data, ix); err != nil {
		return nil, err
	}
	return ix, nil
}

// Find 按名称查找主题（大小写不敏感）
func (ix *Index) Find(name string) *Topic {
	for i := range ix.Topics {
		if strings.EqualFold(ix.Topics[i].Name, name) {
			return &ix.Topics[i]
		}
	}
	return nil
}

// Save 目录不存在则创建，原子写
func (ix *Index) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(ix); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(path, []byte(b.String()))
}
