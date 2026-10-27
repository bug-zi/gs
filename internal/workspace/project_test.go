package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestProjects(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string, markers ...string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, m := range markers {
			if err := os.WriteFile(filepath.Join(p, m), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("Study/Project/HDU-KillCourse", "go.mod")
	mk("Project/fileforge", "README.md")
	mk("Study/7days-golang", "go.mod")
	mk("Study/logs")           // 无标志文件，不应算项目
	mk("Project/gs", "go.mod") // gs 自身排除

	ws := &Workspace{Root: root}
	ps := ws.Projects()
	var names []string
	for _, p := range ps {
		names = append(names, p.Name)
	}
	for _, want := range []string{"HDU-KillCourse", "fileforge", "7days-golang"} {
		if !sliceContains(names, want) {
			t.Fatalf("缺项目 %q: %v", want, names)
		}
	}
	if sliceContains(names, "gs") || sliceContains(names, "logs") {
		t.Fatalf("不应包含 gs/logs: %v", names)
	}
}

func TestProjectsRegistryGS(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string, markers ...string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, m := range markers {
			if err := os.WriteFile(filepath.Join(p, m), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("Project/gs", "go.mod")
	ws := &Workspace{Root: root}

	// 未注册：gs 隐藏（现有行为）
	for _, p := range ws.Projects() {
		if p.Name == "gs" {
			t.Fatal("未注册时 gs 不应出现")
		}
	}

	// 注册后：gs 出现
	reg, _ := LoadRegistry(ws.RegistryPath())
	if err := reg.Add(ws.RegistryPath(), "Project/gs"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range ws.Projects() {
		if p.Name == "gs" {
			found = true
		}
	}
	if !found {
		t.Fatal("注册后 gs 应出现在列表")
	}
}

func TestProjectsManualEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Study", "misc"), 0o755); err != nil {
		t.Fatal(err)
	}
	ws := &Workspace{Root: root}
	reg, _ := LoadRegistry(ws.RegistryPath())
	if err := reg.Add(ws.RegistryPath(), "Study/misc"); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range ws.Projects() {
		if p.Name == "misc" && p.Manual {
			found = true
		}
	}
	if !found {
		t.Fatal("注册表条目应作为手动项目出现（无标志文件也纳入）")
	}
}
