package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// RegistryEntry 手动加入项目列表的条目
type RegistryEntry struct {
	Rel   string `toml:"rel"`   // 相对工作区根的正斜杠路径
	Added string `toml:"added"` // 加入日期 YYYY-MM-DD
	// 预留 status / note 字段，本版不启用不写入
}

// Registry .gs/projects.toml：手动加入的项目注册表
type Registry struct {
	Projects []RegistryEntry `toml:"projects"`
}

// RegistryPath 注册表文件路径
func (w *Workspace) RegistryPath() string {
	return filepath.Join(w.Root, ".gs", "projects.toml")
}

// LoadRegistry 文件不存在返回空注册表
func LoadRegistry(path string) (*Registry, error) {
	r := &Registry{}
	data, err := os.ReadFile(path)
	if err != nil {
		return r, nil
	}
	if err := toml.Unmarshal(data, r); err != nil {
		return nil, err
	}
	return r, nil
}

// HasRel 是否已登记
func (r *Registry) HasRel(rel string) bool {
	for _, e := range r.Projects {
		if e.Rel == rel {
			return true
		}
	}
	return false
}

// Add 幂等登记并保存；added 取东八区今天
func (r *Registry) Add(path, rel string) error {
	if r.HasRel(rel) {
		return nil
	}
	r.Projects = append(r.Projects, RegistryEntry{
		Rel:   rel,
		Added: time.Now().In(CST).Format("2006-01-02"),
	})
	return r.Save(path)
}

func (r *Registry) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(r); err != nil {
		return err
	}
	return WriteFileAtomic(path, []byte(b.String()))
}
