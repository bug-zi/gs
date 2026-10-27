package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type AI struct {
	BaseURL     string  `toml:"base_url"`
	APIKey      string  `toml:"api_key"`
	Model       string  `toml:"model"`
	Temperature float64 `toml:"temperature"`
}

type Scan struct {
	Exclude   []string `toml:"exclude"`
	MaxFileKB int64    `toml:"max_file_kb"`
}

type UI struct {
	Editor string `toml:"editor"`
}

type Project struct {
	Opener string `toml:"opener"` // 打开源文件的 IDE 命令
}

type Config struct {
	AI      AI      `toml:"ai"`
	Scan    Scan    `toml:"scan"`
	UI      UI      `toml:"ui"`
	Project Project `toml:"project"`
}

func defaultConfig() Config {
	return Config{
		Scan: Scan{
			Exclude:   []string{".git", ".codegraph", ".agents", "Resource", "node_modules", "_docx_render"},
			MaxFileKB: 512,
		},
		Project: Project{Opener: ""}, // 空 = 跟随 [ui] editor 解析链（trae 默认值曾指向失效的包装脚本）
	}
}

// Load 全局(home)为基底，工作区(root)覆盖非空字段；环境变量 GS_API_KEY 优先级最高。
// home 为空时用 os.UserHomeDir()。
func Load(root, home string) (*Config, error) {
	cfg := defaultConfig()
	if home == "" {
		h, err := os.UserHomeDir()
		if err == nil {
			home = h
		}
	}
	mergeFile(&cfg, filepath.Join(home, ".gs", "config.toml"))
	mergeFile(&cfg, filepath.Join(root, ".gs", "config.toml"))
	if v := os.Getenv("GS_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	return &cfg, nil
}

func mergeFile(cfg *Config, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var c Config
	if toml.Unmarshal(data, &c) != nil {
		return
	}
	if c.AI.BaseURL != "" {
		cfg.AI.BaseURL = c.AI.BaseURL
	}
	if c.AI.APIKey != "" {
		cfg.AI.APIKey = c.AI.APIKey
	}
	if c.AI.Model != "" {
		cfg.AI.Model = c.AI.Model
	}
	if c.AI.Temperature != 0 {
		cfg.AI.Temperature = c.AI.Temperature
	}
	if len(c.Scan.Exclude) > 0 {
		cfg.Scan.Exclude = c.Scan.Exclude
	}
	if c.Scan.MaxFileKB != 0 {
		cfg.Scan.MaxFileKB = c.Scan.MaxFileKB
	}
	if c.UI.Editor != "" {
		cfg.UI.Editor = c.UI.Editor
	}
	if c.Project.Opener != "" {
		cfg.Project.Opener = c.Project.Opener
	}
}
