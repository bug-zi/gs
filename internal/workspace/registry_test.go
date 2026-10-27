package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryLoadMissing(t *testing.T) {
	r, err := LoadRegistry(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Projects) != 0 {
		t.Fatalf("不存在的注册表应为空: %v", r.Projects)
	}
}

func TestRegistryAddHasRel(t *testing.T) {
	root := t.TempDir()
	ws := &Workspace{Root: root}
	r, _ := LoadRegistry(ws.RegistryPath())
	if err := r.Add(ws.RegistryPath(), "Project/gs"); err != nil {
		t.Fatal(err)
	}
	// 重复 Add 幂等
	if err := r.Add(ws.RegistryPath(), "Project/gs"); err != nil {
		t.Fatal(err)
	}
	r2, err := LoadRegistry(ws.RegistryPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Projects) != 1 || r2.Projects[0].Rel != "Project/gs" {
		t.Fatalf("注册表内容不符: %+v", r2.Projects)
	}
	if !r2.HasRel("Project/gs") || r2.HasRel("Project/x") {
		t.Fatal("HasRel 判定错误")
	}
	if !strings.Contains(r2.Projects[0].Added, "20") {
		t.Fatalf("added 应为日期: %q", r2.Projects[0].Added)
	}
}
