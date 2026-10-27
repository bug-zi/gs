package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Scan.MaxFileKB != 512 {
		t.Fatalf("默认 MaxFileKB = %d", cfg.Scan.MaxFileKB)
	}
	if len(cfg.Scan.Exclude) == 0 {
		t.Fatal("默认 Exclude 不应为空")
	}
}

func TestProjectOpener(t *testing.T) {
	home := t.TempDir()
	cfg, err := Load(t.TempDir(), home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Opener != "" {
		t.Fatalf("默认 opener 应为空（跟随编辑器解析链），得到 %q", cfg.Project.Opener)
	}
	if err := os.MkdirAll(filepath.Join(home, ".gs"), 0o755); err != nil {
		t.Fatal(err)
	}
	tomlSrc := "[project]\nopener = \"code\"\n"
	if err := os.WriteFile(filepath.Join(home, ".gs", "config.toml"), []byte(tomlSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg2, err := Load(t.TempDir(), home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Project.Opener != "code" {
		t.Fatalf("opener 应被覆盖为 code，得到 %q", cfg2.Project.Opener)
	}
}

func TestLoadMerge(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	mk := func(dir, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, ".gs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".gs", "config.toml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(home, "[ai]\nmodel = \"m-home\"\nbase_url = \"https://home\"\n")
	mk(root, "[ai]\nmodel = \"m-root\"\n[scan]\nmax_file_kb = 64\n")
	t.Setenv("GS_API_KEY", "sk-test")

	cfg, err := Load(root, home)
	if err != nil {
		t.Fatal(err)
	}
	// 工作区覆盖全局的同名字段，未覆盖字段保留全局值
	if cfg.AI.Model != "m-root" || cfg.AI.BaseURL != "https://home" {
		t.Fatalf("merge 错误: %+v", cfg.AI)
	}
	if cfg.Scan.MaxFileKB != 64 {
		t.Fatalf("MaxFileKB = %d", cfg.Scan.MaxFileKB)
	}
	if cfg.AI.APIKey != "sk-test" {
		t.Fatalf("GS_API_KEY 未生效")
	}
}
