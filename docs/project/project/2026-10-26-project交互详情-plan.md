# project 模块增强（交互详情 + gs 自纳管）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [2026-10-26-project交互详情-design.md](./2026-10-26-project交互详情-design.md)

**Goal:** `/project` 从静态渲染升级为五块内容交互菜单（README / 框架树 / 学习笔记 / 项目日志 / 升级计划），框架树带 AI 生成介绍缓存并与学习笔记按命名约定联动，gs 自身经 `.gs/projects.toml` 注册表一键纳管。

**Architecture:** 纯逻辑下沉 `internal/structure`（扫描/缓存/笔记关联/git 统计）与 `internal/workspace`（注册表），`internal/command/project.go` 只做壳（菜单循环、huh 交互、pager、TRAE 打开、AI 调用）。每任务独立可编译可测试。

**Tech Stack:** Go 1.27、BurntSushi/toml、charmbracelet huh/lipgloss（既有栈，不新增依赖）。

**执行须知（gs 硬性规则）：**
- **AI 禁止执行任何 git 命令**。各任务末尾的「提交点」由开发者自行 commit，AI 只给出建议信息。
- 文件写入一律走 `workspace.WriteFileAtomic`（临时文件 + rename）。
- 终端 UI 不用 emoji、不用彩亮饱和色；代码注释中文、仅写非直观处。
- 每任务完成判定：`go build ./...` 与 `go test ./...` 全绿（在 `D:\Code\Go\Project\gs` 下执行）。

**执行进度（2026-10-26）：** Task 1–11 代码全部完成；`go build ./...`、`go vet`、`gofmt` 全绿；structure/workspace/config/ai/index 测试全绿。**Task 12 回归被外部阻塞**：并行会话的 `internal/command/plan_roadmap_test.go` 引用未定义的 `runPlan`，待其完成后跑 command 包测试回归 + 人工冒烟。

---

## 文件总览

| 动作 | 文件 | 职责 |
|---|---|---|
| Modify | `internal/config/config.go` | 新增 `[project]` 段（opener） |
| Modify | `internal/config/config_test.go` | opener 默认值与覆盖测试 |
| Create | `internal/workspace/registry.go` | 注册表类型与读写 |
| Create | `internal/workspace/registry_test.go` | 注册表单测 |
| Modify | `internal/workspace/project.go` | Project 增 Manual 字段；Projects() 合并注册表、gs 条件排除 |
| Modify | `internal/workspace/project_test.go` | 注册后 gs 出现的测试 |
| Modify | `internal/workspace/workspace.go` | 无改动（RegistryPath 放 registry.go） |
| Create | `internal/structure/scan.go` | 框架树逐层扫描 |
| Create | `internal/structure/cache.go` | AI 介绍缓存（`.gs/structures/<项目名>.toml`） |
| Create | `internal/structure/notes.go` | 笔记聚合、命名约定关联、反解源文件 |
| Create | `internal/structure/gitinfo.go` | git 统计（可注入）、GoStats 迁入、项目日志日期 |
| Create | `internal/structure/*_test.go` | 上述各文件单测 |
| Modify | `internal/command/project.go` | 壳重写：列表提示项、菜单循环、五块 handler |
| Modify | `internal/command/project_test.go` | 改写受影响测试、新增纯函数测试 |

---

### Task 1: config 新增 `[project]` 段

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: 写失败测试**（追加到 `internal/config/config_test.go`）

```go
func TestProjectOpener(t *testing.T) {
	home := t.TempDir()
	cfg, err := Load(t.TempDir(), home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Opener != "trae" {
		t.Fatalf("默认 opener 应为 trae，得到 %q", cfg.Project.Opener)
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
```

（若该文件缺 `os` / `path/filepath` import 则补上；`Load` 签名为 `Load(root, home string)`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/config/ -run TestProjectOpener -v`
Expected: FAIL（`cfg.Project undefined`）

- [ ] **Step 3: 最小实现**（修改 `internal/config/config.go`）

在 `type UI struct` 之后加：

```go
type Project struct {
	Opener string `toml:"opener"` // 打开源文件的 IDE 命令
}
```

`Config` 结构体加字段：

```go
type Config struct {
	AI      AI      `toml:"ai"`
	Scan    Scan    `toml:"scan"`
	UI      UI      `toml:"ui"`
	Project Project `toml:"project"`
}
```

`defaultConfig()` 返回值加：

```go
		Project: Project{Opener: "trae"},
```

`mergeFile()` 末尾追加：

```go
	if c.Project.Opener != "" {
		cfg.Project.Opener = c.Project.Opener
	}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/config/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 全绿

- [ ] **Step 6: 提交点（开发者执行）**

建议信息：`feat(config): 新增 [project] opener 段，默认 trae`

---

### Task 2: workspace 注册表与 Projects() 合并

**Files:**
- Create: `internal/workspace/registry.go`
- Create: `internal/workspace/registry_test.go`
- Modify: `internal/workspace/project.go`
- Modify: `internal/workspace/project_test.go`

- [ ] **Step 1: 写失败测试**（新建 `internal/workspace/registry_test.go`）

```go
package workspace

import (
	"os"
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
```

- [ ] **Step 2: 写失败测试**（追加到 `internal/workspace/project_test.go`）

```go
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
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./internal/workspace/ -v`
Expected: FAIL（`LoadRegistry` / `RegistryPath` / `Manual` 未定义）

- [ ] **Step 4: 实现 registry.go**（新建）

```go
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
```

- [ ] **Step 5: 改写 project.go**（整文件替换为）

```go
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
		seen[abs] = true
		name := filepath.Base(abs)
		// gs 自身：未注册则隐藏（manual=true 的注册表条目已在上层判定过）
		if name == "gs" && !manual && !w.gsRegistered() {
			return
		}
		// 自动发现仍要求项目标志；注册表条目无条件纳入
		if !manual && !isProjectDir(abs) {
			return
		}
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
```

- [ ] **Step 6: 跑测试确认通过**

Run: `go test ./internal/workspace/ -v`
Expected: 全部 PASS（含既有 TestProjects——无注册表文件时行为不变）

- [ ] **Step 7: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 全绿

- [ ] **Step 8: 提交点（开发者执行）**

建议信息：`feat(workspace): .gs/projects.toml 注册表；gs 自身注册后纳入项目列表`

---

### Task 3: internal/structure — 框架树扫描 scan.go

**Files:**
- Create: `internal/structure/scan.go`
- Create: `internal/structure/scan_test.go`

- [ ] **Step 1: 写失败测试**（新建 `internal/structure/scan_test.go`）

```go
package structure

import (
	"os"
	"path/filepath"
	"testing"
)

func treeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkDir := func(rel string) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mkFile := func(rel string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mkFile("main.go")
	mkFile("go.mod")
	mkFile("README.md")
	mkFile("logo.png") // 非关注类型，应过滤
	mkDir("internal/config")
	mkFile("internal/config/config.go")
	mkDir("web")
	mkDir(".git")       // 隐藏目录恒跳过
	mkDir("node_modules") // 在 exclude 中，应跳过
	return root
}

func TestScanTopLevel(t *testing.T) {
	root := treeFixture(t)
	entries := Scan(root, "", []string{"node_modules"})
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	for _, want := range []string{"main.go", "go.mod", "README.md", "internal", "web"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("缺节点 %q: %v", want, names)
		}
	}
	for _, banned := range []string{"logo.png", ".git", "node_modules"} {
		for _, n := range names {
			if n == banned {
				t.Fatalf("不应包含 %q: %v", banned, names)
			}
		}
	}
	// 目录在前；同类型按名排序
	if !entries[0].IsDir {
		t.Fatalf("首项应为目录: %+v", entries[0])
	}
}

func TestScanSubdirRel(t *testing.T) {
	root := treeFixture(t)
	entries := Scan(filepath.Join(root, "internal"), "internal", []string{"node_modules"})
	if len(entries) != 1 || entries[0].Name != "config" || !entries[0].IsDir || entries[0].Rel != "internal/config" {
		t.Fatalf("子目录扫描结果不符: %+v", entries)
	}
}

func TestScanMissingDir(t *testing.T) {
	if entries := Scan(filepath.Join(t.TempDir(), "nope"), "", nil); entries != nil {
		t.Fatalf("不存在的目录应返回 nil: %v", entries)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/structure/ -v`
Expected: FAIL（包不存在，编译错误）

- [ ] **Step 3: 实现 scan.go**（新建）

```go
// Package structure 项目结构纯逻辑：框架树扫描、介绍缓存、笔记关联、git 统计
package structure

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry 框架树节点
type Entry struct {
	Name  string // 文件或目录名
	Rel   string // 相对项目根的正斜杠路径
	IsDir bool
}

// 参与框架树展示的文件类型（其余忽略降噪）
var scanExts = map[string]bool{
	".go": true, ".md": true, ".mod": true, ".sum": true,
	".yaml": true, ".yml": true, ".json": true, ".toml": true,
	".html": true, ".css": true, ".js": true,
}

// Scan 列出 dir 下一层节点：目录在前、同类按名排序。
// prefix 为 dir 相对项目根的正斜杠路径（根传 ""），Rel 据此拼接。
// 跳过隐藏项、exclude 段名与无关文件类型。
func Scan(dir, prefix string, exclude []string) []Entry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	ex := map[string]bool{}
	for _, e := range exclude {
		ex[e] = true
	}
	var out []Entry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		rel := name
		if prefix != "" {
			rel = prefix + "/" + name
		}
		if e.IsDir() {
			if ex[name] {
				continue
			}
			out = append(out, Entry{Name: name, Rel: rel, IsDir: true})
			continue
		}
		if !scanExts[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		out = append(out, Entry{Name: name, Rel: rel, IsDir: false})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/structure/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(structure): 框架树逐层扫描纯逻辑包`

---

### Task 4: internal/structure — 介绍缓存 cache.go

**Files:**
- Create: `internal/structure/cache.go`
- Create: `internal/structure/cache_test.go`

- [ ] **Step 1: 写失败测试**（新建 `internal/structure/cache_test.go`）

```go
package structure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/workspace"
)

func TestStructureCacheRoundtrip(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	path := StructurePath(ws, "demo")
	c := &StructureCache{}
	c.Merge([]Node{{Path: "main.go", Desc: "入口"}, {Path: "internal", Desc: "核心"}})
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	c2, err := LoadStructureCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if c2.Find("main.go") != "入口" || c2.Find("internal") != "核心" || c2.Find("nope") != "" {
		t.Fatalf("Find 结果不符: %+v", c2.Nodes)
	}
	if !strings.HasPrefix(c2.Generated, "20") {
		t.Fatalf("generated 应有时间戳: %q", c2.Generated)
	}
}

func TestStructureCacheMergeOverwrite(t *testing.T) {
	c := &StructureCache{}
	c.Merge([]Node{{Path: "main.go", Desc: "旧"}, {Path: "a.go", Desc: "A"}})
	c.Merge([]Node{{Path: "main.go", Desc: "新"}})
	if c.Find("main.go") != "新" || c.Find("a.go") != "A" || len(c.Nodes) != 2 {
		t.Fatalf("合并覆盖结果不符: %+v", c.Nodes)
	}
	// 按 path 排序，便于 diff 稳定
	if c.Nodes[0].Path != "a.go" {
		t.Fatalf("节点应按 path 排序: %+v", c.Nodes)
	}
}

func TestStructureCacheLoadMissingAndBroken(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	c, err := LoadStructureCache(StructurePath(ws, "nope"))
	if err != nil || len(c.Nodes) != 0 {
		t.Fatalf("不存在的缓存应返回空: %+v %+v", c, err)
	}
	broken := StructurePath(ws, "broken")
	if err := os.MkdirAll(filepath.Dir(broken), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("[[[不是 toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadStructureCache(broken); err == nil {
		t.Fatal("损坏缓存应报错")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/structure/ -run TestStructureCache -v`
Expected: FAIL（`StructurePath` / `StructureCache` 未定义）

- [ ] **Step 3: 实现 cache.go**（新建）

```go
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
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/structure/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(structure): AI 介绍缓存读写与合并`

---

### Task 5: internal/structure — 笔记关联 notes.go

**Files:**
- Create: `internal/structure/notes.go`
- Create: `internal/structure/notes_test.go`

- [ ] **Step 1: 写失败测试**（新建 `internal/structure/notes_test.go`）

```go
package structure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchNotes(t *testing.T) {
	goFiles := []Entry{
		{Name: "main.go", Rel: "main.go"},
		{Name: "config.go", Rel: "internal/config/config.go"},
		{Name: "go.mod", Rel: "go.mod", IsDir: false}, // 非 .go 名不影响（文件名无 .go 不匹配）
	}
	notes := []Note{
		{Abs: "/x/main.go-context串讲.md", Display: "main.go-context串讲.md"}, // 完整名命中
		{Abs: "/x/main函数分析.md", Display: "main函数分析.md"},                   // 基名 main(4) 命中
		{Abs: "/x/配置config解析.md", Display: "配置config解析.md"},               // 基名 config 命中
		{Abs: "/x/蹲课实现.md", Display: "蹲课实现.md"},                             // 不含任何源文件名
		{Abs: "/x/go笔记.md", Display: "go笔记.md"},                             // 短基名（len<4）不命中 main
	}
	m := MatchNotes(goFiles, notes)
	if len(m["main.go"]) != 2 {
		t.Fatalf("main.go 应命中 2 篇: %+v", m["main.go"])
	}
	if len(m["internal/config/config.go"]) != 1 {
		t.Fatalf("config.go 应命中 1 篇: %+v", m["internal/config/config.go"])
	}
	if _, ok := m["go.mod"]; ok {
		t.Fatal("go.mod 不应命中")
	}
}

func TestMatchNotesCaseInsensitive(t *testing.T) {
	goFiles := []Entry{{Name: "Main.go", Rel: "Main.go"}}
	notes := []Note{{Abs: "/x/MAIN.GO 串讲.md", Display: "MAIN.GO 串讲.md"}}
	if len(MatchNotes(goFiles, notes)["Main.go"]) != 1 {
		t.Fatal("匹配应大小写不敏感")
	}
}

func TestReverseResolve(t *testing.T) {
	goFiles := []Entry{
		{Name: "main.go", Rel: "main.go"},
		{Name: "web.go", Rel: "web/web.go"},
	}
	if rel, ok := ReverseResolve("/x/main.go-context串讲.md", goFiles); !ok || rel != "main.go" {
		t.Fatalf("应唯一反解 main.go: %q %v", rel, ok)
	}
	if _, ok := ReverseResolve("/x/蹲课实现.md", goFiles); ok {
		t.Fatal("无 .go 名应反解失败")
	}
	// 歧义：两个同名文件都不给结果
	amb := append(goFiles, Entry{Name: "main.go", Rel: "cmd/main.go"})
	if _, ok := ReverseResolve("/x/main.go串讲.md", amb); ok {
		t.Fatal("多命中应反解失败")
	}
}

func TestCollectAndWalkGoFiles(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("docs/study/a.md", "A")
	mk("docs/study/sub/b.md", "B")
	mk("main.go", "package main")
	mk("internal/x.go", "package x")
	mk(".git/y.go", "package y") // 隐藏目录跳过
	goFiles := WalkGoFiles(root, nil)
	if len(goFiles) != 2 {
		t.Fatalf("应收集 2 个 go 文件: %+v", goFiles)
	}
	projNotes, seaNotes := CollectNotes(root, filepath.Join(root, "sea", "demo"))
	if len(projNotes) != 2 {
		t.Fatalf("项目内笔记应 2 篇: %+v", projNotes)
	}
	if len(seaNotes) != 0 {
		t.Fatalf("sea 目录不存在应返回空: %+v", seaNotes)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/structure/ -run "TestMatch|TestReverse|TestCollect" -v`
Expected: FAIL（`Note` / `MatchNotes` / `ReverseResolve` / `CollectNotes` / `WalkGoFiles` 未定义）

- [ ] **Step 3: 实现 notes.go**（新建）

```go
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
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/structure/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(structure): 学习笔记聚合与命名约定关联`

---

### Task 6: internal/structure — git 统计 gitinfo.go

**Files:**
- Create: `internal/structure/gitinfo.go`
- Create: `internal/structure/gitinfo_test.go`
- Modify: `internal/command/project.go`（仅删除 `goStats`，Task 7 一并重写；本任务先新增不删旧，避免编译断裂——`goStats` 在 Task 7 删除）

说明：为保持每任务可编译，本任务**只新增** `structure.GoStats`，旧 `command.goStats` 的删除放在 Task 7 重写时一并完成。

- [ ] **Step 1: 写失败测试**（新建 `internal/structure/gitinfo_test.go`）

```go
package structure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitStatsStubbed(t *testing.T) {
	orig := GitRun
	defer func() { GitRun = orig }()
	GitRun = func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "log":
			if len(args) >= 2 && args[1] == "--reverse" {
				return "2025-09-01\n2025-09-02\n", nil
			}
			return "2026-10-20\n", nil
		case "rev-list":
			if len(args) >= 2 && args[1] == "--count" {
				if len(args) >= 4 && args[2] == "--since=30 days ago" {
					return "5\n", nil
				}
				return "42\n", nil
			}
		}
		return "", os.ErrNotExist
	}
	info := GitStats("any")
	if !info.Available || info.Created != "2025-09-01" || info.LastCommit != "2026-10-20" ||
		info.Total != 42 || info.Recent30 != 5 {
		t.Fatalf("GitStats 结果不符: %+v", info)
	}
}

func TestGitStatsUnavailable(t *testing.T) {
	orig := GitRun
	defer func() { GitRun = orig }()
	GitRun = func(dir string, args ...string) (string, error) {
		return "", os.ErrPermission
	}
	if info := GitStats("any"); info.Available {
		t.Fatal("git 失败应 Available=false")
	}
}

func TestGoStatsAndLogDates(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("a.go", "package a\n\nfunc f() {}\n")
	mk("sub/b.go", "package b\n")
	mk("c.txt", "not go")
	files, lines := GoStats(root)
	if files != 2 || lines != 5 {
		t.Fatalf("GoStats 应为 2 文件 5 行: %d %d", files, lines)
	}
	mk("docs/log/260101.md", "x")
	mk("docs/log/251231.md", "x")
	mk("docs/log/随手记.md", "x") // 非日志名过滤
	dates := ProjectLogDates(root)
	if len(dates) != 2 || dates[0].Raw != "260101" || dates[0].Display != "2026-01-01" {
		t.Fatalf("ProjectLogDates 结果不符: %+v", dates)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/structure/ -run "TestGit|TestGoStats" -v`
Expected: FAIL（`GitRun` / `GitStats` / `GoStats` / `ProjectLogDates` 未定义）

- [ ] **Step 3: 实现 gitinfo.go**（新建）

```go
package structure

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

// GitRun git 执行器（函数变量，测试可打桩）；dir 为仓库目录，全部只读命令
var GitRun = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

// GitInfo 项目 git 活跃信息；Available=false 表示无 git 或非仓库
type GitInfo struct {
	Available  bool
	Created    string // YYYY-MM-DD 首次提交
	LastCommit string // YYYY-MM-DD 最近提交
	Total      int
	Recent30   int
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// GitStats 汇总项目 git 信息；任一命令失败整体降级为不可用
func GitStats(projectPath string) GitInfo {
	created, err := GitRun(projectPath, "log", "--reverse", "--format=%as")
	if err != nil {
		return GitInfo{}
	}
	last, err := GitRun(projectPath, "log", "-1", "--format=%as")
	if err != nil {
		return GitInfo{}
	}
	totalS, err := GitRun(projectPath, "rev-list", "--count", "HEAD")
	if err != nil {
		return GitInfo{}
	}
	recentS, err := GitRun(projectPath, "rev-list", "--count", "--since=30 days ago", "HEAD")
	if err != nil {
		return GitInfo{}
	}
	total, _ := strconv.Atoi(strings.TrimSpace(totalS))
	recent, _ := strconv.Atoi(strings.TrimSpace(recentS))
	return GitInfo{
		Available:  true,
		Created:    firstLine(created),
		LastCommit: firstLine(last),
		Total:      total,
		Recent30:   recent,
	}
}

// LogDate 项目学习日志条目
type LogDate struct {
	Raw     string // 260101，拼文件名用
	Display string // 2026-01-01，展示与比较用
}

// ProjectLogDates 项目 docs/log 日期，新在前；目录缺失返回空
func ProjectLogDates(projectPath string) []LogDate {
	entries, err := os.ReadDir(filepath.Join(projectPath, "docs", "log"))
	if err != nil {
		return nil
	}
	var out []LogDate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		if len(base) != 6 {
			continue
		}
		t, err := time.ParseInLocation("060102", base, workspace.CST)
		if err != nil {
			continue
		}
		out = append(out, LogDate{Raw: base, Display: t.Format("2006-01-02")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Display > out[j].Display })
	return out
}

// GoStats 统计 .go 文件数与总行数（自 command 包迁入）
func GoStats(projectPath string) (files, lines int) {
	filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".go") {
			files++
			if data, err := os.ReadFile(p); err == nil {
				lines += strings.Count(string(data), "\n")
			}
		}
		return nil
	})
	return files, lines
}
```

（`fs` 未直接使用则不加 import；`filepath.WalkDir` 回调的 `d` 类型为 `fs.DirEntry`，需 `io/fs` 时补 import。实际编译时按报错补齐即可。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/structure/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(structure): git 统计（可注入）、GoStats 迁入、项目日志日期`

---

### Task 7: command 壳重写 — 列表提示项 + 菜单骨架 + README 块

本任务起 `/project` 行为变化：带参/无参都进入菜单（先只有 README 一块 + 返回，后续任务逐块追加）。旧静态渲染 `renderProject` 删除；`listDocs`、`project edit` 子命令原样保留。

**Files:**
- Modify: `internal/command/project.go`（整文件重写）
- Modify: `internal/command/project_test.go`（改写 TestProjectDetail，新增纯函数测试）

- [ ] **Step 1: 改写测试**（`internal/command/project_test.go`：删除 `TestProjectDetail`，追加）

```go
func TestResolveProject(t *testing.T) {
	ps := []workspace.Project{
		{Name: "HDU-KillCourse", Path: "/x/k", Rel: "Study/Project/HDU-KillCourse"},
		{Name: "gs", Path: "/x/g", Rel: "Project/gs"},
	}
	if p := resolveProject(ps, "kill"); p == nil || p.Name != "HDU-KillCourse" {
		t.Fatalf("关键词应命中: %+v", p)
	}
	if p := resolveProject(ps, "GS"); p == nil || p.Name != "gs" {
		t.Fatal("匹配应大小写不敏感")
	}
	if resolveProject(ps, "不存在") != nil {
		t.Fatal("未命中应返回 nil")
	}
}

func TestProjectOptions(t *testing.T) {
	// 无 gs：尾部出现加入提示项
	opts, prompt := projectOptions([]workspace.Project{
		{Name: "demo", Path: "/x", Rel: "Study/Project/demo"},
	})
	if !prompt {
		t.Fatal("gs 未在列表时应显示提示项")
	}
	last := stripANSI(opts[len(opts)-1].Label)
	if !strings.Contains(last, "加入项目列表") {
		t.Fatalf("尾项应为提示项: %q", last)
	}
	// 有 gs：无提示项，标签带「本工具」
	opts2, prompt2 := projectOptions([]workspace.Project{
		{Name: "gs", Path: "/x", Rel: "Project/gs"},
	})
	if prompt2 {
		t.Fatal("gs 在列表时不应显示提示项")
	}
	if l := stripANSI(opts2[0].Label); !strings.Contains(l, "本工具") {
		t.Fatalf("gs 标签应含本工具: %q", l)
	}
	// 手动项目标签
	opts3, _ := projectOptions([]workspace.Project{
		{Name: "misc", Path: "/x", Rel: "Study/misc", Manual: true},
	})
	if l := stripANSI(opts3[0].Label); !strings.Contains(l, "手动") {
		t.Fatalf("手动项目标签应含「手动」: %q", l)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run "TestResolveProject|TestProjectOptions" -v`
Expected: FAIL（`resolveProject` / `projectOptions` 未定义）

- [ ] **Step 3: 重写 project.go**（整文件替换为）

```go
package command

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// listDocs 递归列出项目某子目录下全部 .md（正斜杠相对项目路径），目录不存在返回空
func listDocs(projectPath, sub string) []string {
	var out []string
	root := filepath.Join(projectPath, filepath.FromSlash(sub))
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(projectPath, p)
		if err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return out
}

// readFileString 读文件全文（CRLF 归一），失败返回占位文本
func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "（读取失败）"
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// resolveProject 关键词匹配项目名（大小写不敏感的包含匹配）
func resolveProject(projects []workspace.Project, kw string) *workspace.Project {
	lkw := strings.ToLower(kw)
	for i := range projects {
		if strings.Contains(strings.ToLower(projects[i].Name), lkw) {
			return &projects[i]
		}
	}
	return nil
}

// projectOptions 构建选择列表；gs 不在列表时尾部追加一键加入提示项
func projectOptions(projects []workspace.Project) ([]huhOption, bool) {
	hasGS := false
	opts := make([]huhOption, 0, len(projects)+1)
	for _, p := range projects {
		label := p.Name + "（" + p.Rel + "）"
		switch {
		case p.Name == "gs":
			label = "gs · 本工具（" + p.Rel + "）"
			hasGS = true
		case p.Manual:
			label += "（手动）"
		}
		opts = append(opts, option(label, p.Name))
	}
	prompt := false
	if !hasGS {
		opts = append(opts, option("＋ 将 gs（本工具）加入项目列表", "@add-gs"))
		prompt = true
	}
	return opts, prompt
}

// addGSSelf 将 gs 自身写入注册表
func addGSSelf(ctx *Ctx) error {
	reg, err := workspace.LoadRegistry(ctx.WS.RegistryPath())
	if err != nil {
		return err
	}
	if err := reg.Add(ctx.WS.RegistryPath(), "Project/gs"); err != nil {
		return err
	}
	fmt.Fprintln(ctx.Out, ui.Dim("已将 gs（本工具）加入项目列表，再次 /project 即可选择"))
	return nil
}

// runProjectMenu 项目交互菜单：逐块内容看完回菜单，选「返回」退出
func runProjectMenu(ctx *Ctx, p workspace.Project) error {
	for {
		opts := []huhOption{
			option("README", "README"),
			option("返回", "返回"),
		}
		var pick string
		if err := selectFrom(ctx, "项目 · "+p.Name+"　"+p.Rel, opts, &pick); err != nil {
			return err
		}
		switch pick {
		case "README":
			if err := showProjectReadme(ctx, p); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// showProjectReadme README 全文查阅（可编辑）
func showProjectReadme(ctx *Ctx, p workspace.Project) error {
	path := filepath.Join(p.Path, "README.md")
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintln(ctx.Out, ui.Dim("（无 README.md）"))
		return nil
	}
	return runViewPager(ctx,
		func() string { return ui.RenderMD(readFileString(path)) },
		func() (string, bool) { return path, true })
}

func newProject() Command {
	return Command{
		Name:  "project",
		Short: "学习项目聚合信息（交互菜单：README/框架树/笔记/日志/升级计划）",
		Subs:  []string{"edit"},
		Run: func(ctx *Ctx) error {
			if len(ctx.Args) >= 1 && ctx.Args[0] == "edit" {
				return projectEdit(ctx)
			}
			projects := ctx.WS.Projects()
			if len(ctx.Args) >= 1 {
				p := resolveProject(projects, strings.Join(ctx.Args, " "))
				if p == nil {
					return fmt.Errorf("没有匹配 %q 的项目，可用 /project 浏览全部", strings.Join(ctx.Args, " "))
				}
				return runProjectMenu(ctx, *p)
			}
			opts, prompt := projectOptions(projects)
			var name string
			if err := selectFrom(ctx, "选择项目", opts, &name); err != nil {
				return err
			}
			if prompt && name == "@add-gs" {
				return addGSSelf(ctx)
			}
			for i := range projects {
				if projects[i].Name == name {
					return runProjectMenu(ctx, projects[i])
				}
			}
			return fmt.Errorf("未找到项目 %q", name)
		},
	}
}

// projectEdit 原有 /project edit 子命令，逻辑不变
func projectEdit(ctx *Ctx) error {
	projects := ctx.WS.Projects()
	if len(projects) == 0 {
		return fmt.Errorf("未发现项目（需含 go.mod / README / .git 之一）")
	}
	kw := strings.Join(ctx.Args[1:], " ")
	p := resolveProject(projects, kw)
	if p == nil {
		return fmt.Errorf("没有匹配 %q 的项目", kw)
	}
	docs := []string{"README.md"}
	docs = append(docs, listDocs(p.Path, "docs")...)
	var pick string
	opts := make([]huhOption, 0, len(docs))
	for _, d := range docs {
		opts = append(opts, option(d, d))
	}
	if err := selectFrom(ctx, "选择要编辑的文档", opts, &pick); err != nil {
		return err
	}
	return editOpen(ctx, filepath.Join(p.Path, filepath.FromSlash(pick)))
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/command/ ./internal/workspace/ -v`
Expected: 全部 PASS（TestProjectNoMatch 不受影响）

- [ ] **Step 5: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 全绿

注意：`truncateLines` 现定义于 `internal/command/project.go:52` 且被 `ai.go` 的 `applyOne` 使用；重写后的 project.go 未包含它，**必须**将该函数（连同其依赖）迁入 `internal/command/ai.go`，否则编译失败。

- [ ] **Step 6: 提交点（开发者执行）**

建议信息：`refactor(project): /project 改为交互菜单骨架，gs 未注册时提示一键加入`

---

### Task 8: 菜单块一 — 项目日志

**Files:**
- Modify: `internal/command/project.go`（菜单加「项目日志」项 + `showProjectLog` + `buildProjectLogText`）
- Modify: `internal/command/project_test.go`（追加 `buildProjectLogText` 测试）

- [ ] **Step 1: 写失败测试**（追加）

```go
func TestBuildProjectLogText(t *testing.T) {
	ws := projectFixture(t) // 含 Study/Project/demo：go.mod、README、docs/log/260101.md
	p := workspace.Project{Name: "demo", Path: filepath.Join(ws.Root, "Study", "Project", "demo")}
	text := buildProjectLogText(p, structure.GitInfo{Available: true, Created: "2025-12-01", LastCommit: "2026-01-05", Total: 10, Recent30: 2})
	for _, want := range []string{"2025-12-01", "2026-01-05", "共 10 次", "近 30 天 2 次", "2026-01-01", "1 篇"} {
		if !strings.Contains(stripANSI(text), want) {
			t.Fatalf("日志汇总缺 %q: %s", want, stripANSI(text))
		}
	}
	// 无 git：目录时间降级
	text2 := buildProjectLogText(p, structure.GitInfo{})
	if !strings.Contains(stripANSI(text2), "目录时间") {
		t.Fatalf("无 git 应降级显示目录时间: %s", stripANSI(text2))
	}
}
```

（文件顶部 import 补 `"github.com/bugzi/gs/internal/structure"`。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestBuildProjectLogText -v`
Expected: FAIL（`buildProjectLogText` 未定义）

- [ ] **Step 3: 实现**（project.go 追加；菜单 opts 中「返回」前插入 `option("项目日志", "项目日志")`，switch 加分支）

```go
// buildProjectLogText 项目日志汇总文本（纯函数，便于测试）
func buildProjectLogText(p workspace.Project, info structure.GitInfo) string {
	files, lines := structure.GoStats(p.Path)
	dates := structure.ProjectLogDates(p.Path)
	var b strings.Builder
	b.WriteString("# 项目日志 · " + p.Name + "\n\n")
	if info.Available {
		fmt.Fprintf(&b, "- 创建时间：%s（git 首次提交）\n", info.Created)
		fmt.Fprintf(&b, "- 最近提交：%s\n", info.LastCommit)
		fmt.Fprintf(&b, "- 提交：共 %d 次，近 30 天 %d 次\n", info.Total, info.Recent30)
	} else if mt := modTimeOrZero(p.Path); !mt.IsZero() {
		fmt.Fprintf(&b, "- 创建时间：%s（目录时间，非 git 仓库）\n", mt.Format("2006-01-02"))
	}
	switch latest := latestLearnDate(dates, info); {
	case latest != "":
		fmt.Fprintf(&b, "- 最近学习：%s\n", latest)
	default:
		b.WriteString("- 最近学习：（无记录）\n")
	}
	fmt.Fprintf(&b, "- 学习日志：%d 篇\n", len(dates))
	fmt.Fprintf(&b, "- 代码规模：%d 个 .go 文件，共 %d 行\n", files, lines)
	return b.String()
}

func modTimeOrZero(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// latestLearnDate 项目日志最新日期与 git 最近提交取大（Display 均为 YYYY-MM-DD，可字典序比较）
func latestLearnDate(dates []structure.LogDate, info structure.GitInfo) string {
	var best string
	if len(dates) > 0 {
		best = dates[0].Display // 已按新在前排序
	}
	if info.Available && info.LastCommit > best {
		best = info.LastCommit
	}
	return best
}

// showProjectLog 项目日志汇总 + 项目内学习日志阅读
func showProjectLog(ctx *Ctx, p workspace.Project) error {
	info := structure.GitStats(p.Path)
	fmt.Fprint(ctx.Out, ui.RenderMD(buildProjectLogText(p, info)))
	dates := structure.ProjectLogDates(p.Path)
	if len(dates) == 0 {
		return nil
	}
	opts := []huhOption{option("返回", "")}
	for _, d := range dates {
		opts = append(opts, option(d.Display, d.Raw))
	}
	var pick string
	if err := selectFrom(ctx, "项目学习日志", opts, &pick); err != nil || pick == "" {
		return err
	}
	path := filepath.Join(p.Path, "docs", "log", pick+".md")
	return runViewPager(ctx,
		func() string { return readFileString(path) },
		func() (string, bool) { return path, true })
}
```

（import 补 `"time"` 与 `"github.com/bugzi/gs/internal/structure"`；`runProjectMenu` 的 opts 与 switch 更新：）

```go
		opts := []huhOption{
			option("README", "README"),
			option("项目日志", "项目日志"),
			option("返回", "返回"),
		}
		// switch 增加：
		case "项目日志":
			if err := showProjectLog(ctx, p); err != nil {
				return err
			}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(project): 项目日志块（git 统计 + 学习日志阅读）`

---

### Task 9: 菜单块二 — 学习笔记 + TRAE 打开

**Files:**
- Modify: `internal/command/project.go`（菜单加「学习笔记」项 + `showProjectNotes` + `openInIDE`）
- Modify: `internal/command/project_test.go`（追加 `openInIDE` 未安装报错测试）

- [ ] **Step 1: 写失败测试**（追加）

```go
func TestOpenInIDEMissingCommand(t *testing.T) {
	ws := projectFixture(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	cfg.Project.Opener = "gs-definitely-not-exist-cmd"
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := openInIDE(ctx, "x.go"); err == nil {
		t.Fatal("命令不存在应报错")
	} else if !strings.Contains(err.Error(), "opener") {
		t.Fatalf("报错应提示 opener 配置: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestOpenInIDE -v`
Expected: FAIL（`openInIDE` 未定义）

- [ ] **Step 3: 实现**（project.go 追加；菜单加「学习笔记」分支，方式同 Task 8）

```go
// openInIDE 用配置的 opener 打开文件（默认 trae），异步不等待 IDE 退出
func openInIDE(ctx *Ctx, path string) error {
	opener := ctx.Cfg.Project.Opener
	if opener == "" {
		opener = "trae"
	}
	if _, err := exec.LookPath(opener); err != nil {
		return fmt.Errorf("未找到 %s 命令，请安装 IDE 或在 ~/.gs/config.toml [project] opener 配置", opener)
	}
	return exec.Command(opener, path).Start()
}

// showProjectNotes 两处聚合笔记浏览：阅读 / 编辑 / 打开对应源文件
func showProjectNotes(ctx *Ctx, p workspace.Project) error {
	projNotes, seaNotes := structure.CollectNotes(p.Path, filepath.Join(ctx.WS.SeaDir(), p.Name))
	if len(projNotes)+len(seaNotes) == 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("（无学习笔记）"))
		return nil
	}
	var opts []huhOption
	if len(projNotes) > 0 {
		opts = append(opts, option("── 项目内 docs/study ──", "@h1"))
		for _, n := range projNotes {
			opts = append(opts, option(n.Display, "@p:"+n.Abs))
		}
	}
	if len(seaNotes) > 0 {
		opts = append(opts, option("── 笔记海 Study/sea ──", "@h2"))
		for _, n := range seaNotes {
			opts = append(opts, option(n.Display, "@p:"+n.Abs))
		}
	}
	opts = append(opts, option("返回", ""))
	var pick string
	if err := selectFrom(ctx, "学习笔记 · "+p.Name, opts, &pick); err != nil {
		return err
	}
	if pick == "" || pick == "@h1" || pick == "@h2" {
		return nil
	}
	notePath := strings.TrimPrefix(pick, "@p:")
	// 笔记动作：能唯一反解源文件时多一项「打开源文件」
	var actOpts []huhOption
	if src, ok := structure.ReverseResolve(notePath, structure.WalkGoFiles(p.Path, ctx.Cfg.Scan.Exclude)); ok {
		abs := filepath.Join(p.Path, filepath.FromSlash(src))
		actOpts = append(actOpts, option("打开对应源文件（"+src+"）", "@open:"+abs))
	}
	actOpts = append(actOpts,
		option("阅读", "@read"),
		option("编辑", "@edit"),
		option("返回", ""))
	var act string
	if err := selectFrom(ctx, filepath.Base(notePath), actOpts, &act); err != nil || act == "" {
		return err
	}
	switch {
	case act == "@read":
		return runViewPager(ctx,
			func() string { return ui.RenderMD(readFileString(notePath)) },
			func() (string, bool) { return notePath, true })
	case act == "@edit":
		return editOpen(ctx, notePath)
	case strings.HasPrefix(act, "@open:"):
		return openInIDE(ctx, strings.TrimPrefix(act, "@open:"))
	}
	return nil
}
```

（import 补 `"os/exec"`；`runProjectMenu` opts 插入 `option("学习笔记", "学习笔记")`，switch 加 `case "学习笔记": if err := showProjectNotes(ctx, p); err != nil { return err }`。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(project): 学习笔记块（两处聚合 + 命名约定反解 + TRAE 打开）`

---

### Task 10: 菜单块三 — 框架树 + AI 介绍生成

**Files:**
- Modify: `internal/command/project.go`（菜单加「框架树」项 + `showProjectTree` / `genTreeDescriptions` / `parseDescJSON` / `fileHead` / `aiConfigured`）
- Create: `internal/command/project_tree_test.go`（`parseDescJSON` / `fileHead` 纯函数测试）

- [ ] **Step 1: 写失败测试**（新建 `internal/command/project_tree_test.go`）

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/structure"
)

func TestParseDescJSON(t *testing.T) {
	nodes, err := parseDescJSON(`[{"path":"main.go","desc":"入口"},{"path":"web/","desc":"前端"}]`)
	if err != nil || len(nodes) != 2 || nodes[0].Path != "main.go" {
		t.Fatalf("裸数组解析失败: %+v %+v", nodes, err)
	}
	nodes, err = parseDescJSON("```json\n[{\"path\":\"a\",\"desc\":\"b\"}]\n```")
	if err != nil || len(nodes) != 1 {
		t.Fatalf("围栏包裹应可解析: %+v %+v", nodes, err)
	}
	if _, err := parseDescJSON("不是 JSON"); err == nil {
		t.Fatal("无数组应报错")
	}
	if _, err := parseDescJSON(`[{"path":"","desc":"b"}]`); err == nil {
		t.Fatal("空 path 应报错")
	}
}

func TestFileHeadBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 100)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	budget := 50
	head := fileHead(path, 40, &budget)
	if len(head) != 50 || budget != 0 {
		t.Fatalf("应按预算截断: len=%d budget=%d", len(head), budget)
	}
	if fileHead(filepath.Join(t.TempDir(), "nope"), 10, &budget) != "" {
		t.Fatal("读不到的文件应返回空")
	}
}

func TestRenderTreeLine(t *testing.T) {
	cache := &structure.StructureCache{}
	cache.Merge([]structure.Node{{Path: "main.go", Desc: "入口"}})
	entries := []structure.Entry{
		{Name: "main.go", Rel: "main.go"},
		{Name: "web", Rel: "web", IsDir: true},
	}
	matched := map[string][]structure.Note{
		"main.go": {{Abs: "/x/main.go串讲.md", Display: "main.go串讲.md"}},
	}
	out := stripANSI(renderTreeLines("demo", "", cache, entries, matched))
	for _, want := range []string{"框架树 · demo", "main.go", "入口", "·笔1", "web/"} {
		if !strings.Contains(out, want) {
			t.Fatalf("树渲染缺 %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run "TestParseDesc|TestFileHead|TestRenderTree" -v`
Expected: FAIL（函数未定义）

- [ ] **Step 3: 实现**（project.go 追加；菜单加「框架树」分支）

```go
// aiConfigured 是否已配置 AI（base_url 与 model 齐备）
func aiConfigured(ctx *Ctx) bool {
	return ctx.Cfg.AI.BaseURL != "" && ctx.Cfg.AI.Model != ""
}

// fileHead 读文件前 n 行，budget 为剩余字符预算（就地扣减），超预算截断
func fileHead(path string, n int, budget *int) string {
	data, err := os.ReadFile(path)
	if err != nil || *budget <= 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	head := strings.Join(lines, "\n")
	if len(head) > *budget {
		head = head[:*budget]
	}
	*budget -= len(head)
	return head
}

// parseDescJSON 从 AI 回复解析节点介绍 JSON 数组（容忍 ```json 围栏与前后杂质）
func parseDescJSON(reply string) ([]structure.Node, error) {
	start := strings.Index(reply, "[")
	end := strings.LastIndex(reply, "]")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("回复中未找到 JSON 数组")
	}
	var nodes []structure.Node
	if err := json.Unmarshal([]byte(reply[start:end+1]), &nodes); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	for _, n := range nodes {
		if n.Path == "" || n.Desc == "" {
			return nil, fmt.Errorf("AI 输出存在空 path/desc")
		}
	}
	return nodes, nil
}

// renderTreeLines 当前层树文本（纯函数）：名字对齐 + 介绍 + 笔记标记
func renderTreeLines(projectName, dir string, cache *structure.StructureCache,
	entries []structure.Entry, matched map[string][]structure.Note) string {
	var b strings.Builder
	header := "框架树 · " + projectName
	if dir != "" {
		header += "　" + dir + "/"
	}
	b.WriteString(header + "\n\n")
	for _, e := range entries {
		name := e.Name
		if e.IsDir {
			name += "/"
		}
		line := name
		if d := cache.Find(e.Rel); d != "" {
			line += "　" + d
		}
		if n := len(matched[e.Rel]); n > 0 {
			line += fmt.Sprintf("　·笔%d", n)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// showProjectTree 框架树浏览：逐层进入，文件可打开/看笔记；首层无缓存时 AI 生成
func showProjectTree(ctx *Ctx, p workspace.Project) error {
	cache, err := structure.LoadStructureCache(structure.StructurePath(ctx.WS, p.Name))
	if err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("介绍缓存解析失败，可重新生成覆盖"))
		cache = &structure.StructureCache{}
	}
	dir := ""
	for {
		entries := structure.Scan(filepath.Join(p.Path, filepath.FromSlash(dir)), dir, ctx.Cfg.Scan.Exclude)
		if len(cache.Nodes) == 0 && aiConfigured(ctx) {
			fmt.Fprintln(ctx.Out, ui.Dim("（AI 分析中…）"))
			if err := genTreeDescriptions(ctx, p, cache, dir); err != nil {
				fmt.Fprintln(ctx.Out, ui.Err("AI 生成介绍失败："+err.Error()))
			} else {
				fmt.Fprintln(ctx.Out, ui.Dim("已缓存至 "+ctx.WS.Rel(structure.StructurePath(ctx.WS, p.Name))))
			}
		}
		notes := allProjectNotes(ctx, p)
		matched := structure.MatchNotes(structure.WalkGoFiles(p.Path, ctx.Cfg.Scan.Exclude), notes)
		fmt.Fprint(ctx.Out, ui.RenderMD(renderTreeLines(p.Name, dir, cache, entries, matched)))

		var opts []huhOption
		for _, e := range entries {
			label := e.Name
			if e.IsDir {
				label += "/"
			}
			opts = append(opts, option(label, "@e:"+e.Rel))
		}
		if dir == "" && aiConfigured(ctx) {
			opts = append(opts, option("重新生成介绍（AI）", "@regen"))
		}
		back := "返回"
		if dir != "" {
			back = "返回上级"
		}
		opts = append(opts, option(back, ""))
		var pick string
		if err := selectFrom(ctx, "框架树 · "+p.Name, opts, &pick); err != nil || pick == "" {
			return err
		}
		switch {
		case pick == "@regen":
			if err := genTreeDescriptions(ctx, p, cache, dir); err != nil {
				fmt.Fprintln(ctx.Out, ui.Err("AI 生成介绍失败："+err.Error()))
			} else {
				fmt.Fprintln(ctx.Out, ui.Dim("介绍已更新"))
			}
		case strings.HasPrefix(pick, "@e:"):
			rel := strings.TrimPrefix(pick, "@e:")
			if isDirRel(entries, rel) {
				dir = rel
			} else if err := fileActions(ctx, p, cache, rel); err != nil {
				return err
			}
		}
		if pick == "" {
			if dir == "" {
				return nil
			}
			if i := strings.LastIndex(dir, "/"); i >= 0 {
				dir = dir[:i]
			} else {
				dir = ""
			}
		}
	}
}

func isDirRel(entries []structure.Entry, rel string) bool {
	for _, e := range entries {
		if e.Rel == rel {
			return e.IsDir
		}
	}
	return false
}

// allProjectNotes 聚合项目两处笔记（供树标记与跳转共用）
func allProjectNotes(ctx *Ctx, p workspace.Project) []structure.Note {
	a, b := structure.CollectNotes(p.Path, filepath.Join(ctx.WS.SeaDir(), p.Name))
	return append(append([]structure.Note{}, a...), b...)
}

// fileActions 文件节点操作：TRAE 打开 / 查看相关笔记 / 返回
func fileActions(ctx *Ctx, p workspace.Project, cache *structure.StructureCache, rel string) error {
	abs := filepath.Join(p.Path, filepath.FromSlash(rel))
	matched := structure.MatchNotes(structure.WalkGoFiles(p.Path, ctx.Cfg.Scan.Exclude), allProjectNotes(ctx, p))
	var opts []huhOption
	opts = append(opts, option("用 "+openerName(ctx)+" 打开", "@open"))
	if ns := matched[rel]; len(ns) > 0 {
		opts = append(opts, option(fmt.Sprintf("查看相关笔记（%d 篇）", len(ns)), "@notes"))
	}
	opts = append(opts, option("返回", ""))
	var act string
	if err := selectFrom(ctx, rel, opts, &act); err != nil || act == "" {
		return err
	}
	switch act {
	case "@open":
		return openInIDE(ctx, abs)
	case "@notes":
		var noteOpts []huhOption
		for _, n := range matched[rel] {
			noteOpts = append(noteOpts, option(n.Display, n.Abs))
		}
		noteOpts = append(noteOpts, option("返回", ""))
		var pick string
		if err := selectFrom(ctx, "相关笔记", noteOpts, &pick); err != nil || pick == "" {
			return err
		}
		return runViewPager(ctx,
			func() string { return ui.RenderMD(readFileString(pick)) },
			func() (string, bool) { return pick, true })
	}
	return nil
}

func openerName(ctx *Ctx) string {
	if ctx.Cfg.Project.Opener != "" {
		return ctx.Cfg.Project.Opener
	}
	return "trae"
}

// genTreeDescriptions AI 生成 dir 层节点介绍并并入缓存保存
func genTreeDescriptions(ctx *Ctx, p workspace.Project, cache *structure.StructureCache, dir string) error {
	st, err := ensureAIState(ctx)
	if err != nil {
		return err
	}
	entries := structure.Scan(filepath.Join(p.Path, filepath.FromSlash(dir)), dir, ctx.Cfg.Scan.Exclude)
	if len(entries) == 0 {
		return fmt.Errorf("目录为空")
	}
	budget := 12000
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "- %s\n", e.Rel)
		if !e.IsDir {
			if head := fileHead(filepath.Join(p.Path, filepath.FromSlash(e.Rel)), 40, &budget); head != "" {
				fmt.Fprintf(&b, "  文件开头:\n%s\n", indentBlock(head, "    "))
			}
		}
	}
	sys := "你是代码导览助手。根据项目文件清单与文件开头内容，为每个节点写一句功能介绍：中文、不超过 30 字。只输出 JSON 数组 [{\"path\":\"相对路径\",\"desc\":\"介绍\"}]，path 与清单一致，覆盖每个节点，不要输出其他内容。"
	label := dir
	if label == "" {
		label = "（根目录）"
	}
	reply, err := st.Client.Chat(context.Background(), []ai.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: "项目: " + p.Name + "\n当前目录: " + label + "\n文件清单:\n" + b.String()},
	})
	if err != nil {
		return err
	}
	nodes, err := parseDescJSON(reply)
	if err != nil {
		return err
	}
	cache.Merge(nodes)
	return cache.Save(structure.StructurePath(ctx.WS, p.Name))
}

// indentBlock 多行文本整体缩进
func indentBlock(s, pad string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}
```

（import 补 `"context"`、`"encoding/json"`、`"os/exec"` 已在 Task 9；`runProjectMenu` opts 插入 `option("框架树", "框架树")`，switch 加对应分支调 `showProjectTree`。）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（开发者执行）**

建议信息：`feat(project): 框架树块（逐层浏览 + AI 介绍缓存 + 笔记联动）`

---

### Task 11: 菜单块四 — 升级计划

**Files:**
- Modify: `internal/command/project.go`（菜单加「升级计划」项 + `showProjectPlan` / `genProjectPlan`）

- [ ] **Step 1: 实现**（project.go 追加；菜单加「升级计划」分支。本块交互链路依赖 huh/AI，难以无 TTY 单测，验证以构建 + 既有测试回归 + Task 12 人工冒烟为准）

```go
// projectPlanPath 升级计划正本路径
func projectPlanPath(p workspace.Project) string {
	return filepath.Join(p.Path, "docs", "project", "升级计划.md")
}

// showProjectPlan 升级计划：有正本直接查阅；无则提供生成/创建入口
func showProjectPlan(ctx *Ctx, p workspace.Project) error {
	path := projectPlanPath(p)
	if _, err := os.Stat(path); err == nil {
		return runViewPager(ctx,
			func() string { return ui.RenderMD(readFileString(path)) },
			func() (string, bool) { return path, true })
	}
	var opts []huhOption
	if aiConfigured(ctx) {
		opts = append(opts, option("用 AI 根据项目现状生成初稿", "@ai"))
	}
	opts = append(opts, option("手动创建空文件", "@empty"), option("返回", ""))
	var act string
	if err := selectFrom(ctx, "未找到 "+ctx.WS.Rel(path), opts, &act); err != nil || act == "" {
		return err
	}
	switch act {
	case "@empty":
		content := "# " + p.Name + " 升级计划\n"
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := workspace.WriteFileAtomic(path, []byte(content)); err != nil {
			return err
		}
		fmt.Fprintln(ctx.Out, ui.Dim("已创建 "+ctx.WS.Rel(path)))
		return nil
	case "@ai":
		return genProjectPlan(ctx, p, path)
	}
	return nil
}

// genProjectPlan AI 起草升级计划，确认后经 create 动作原子写入正本
func genProjectPlan(ctx *Ctx, p workspace.Project, path string) error {
	st, err := ensureAIState(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(ctx.Out, ui.Dim("（AI 分析项目现状…）"))
	var src strings.Builder
	if data, err := os.ReadFile(filepath.Join(p.Path, "README.md")); err == nil {
		fmt.Fprintf(&src, "## README\n%s\n", truncateLines(string(data), 60))
	}
	if cache, err := structure.LoadStructureCache(structure.StructurePath(ctx.WS, p.Name)); err == nil {
		for _, n := range cache.Nodes {
			fmt.Fprintf(&src, "- %s：%s\n", n.Path, n.Desc)
		}
	}
	for _, d := range listDocs(p.Path, "docs/project") {
		src.WriteString("- " + d + "\n")
	}
	sys := "你是软件项目规划助手。基于项目资料为该项目起草「升级计划」markdown 文档：含「现状小结 / 升级方向（按优先级列条目，每条一两句）/ 建议下一步」三节，中文、务实具体、不超过 60 行。只输出 markdown 正文。"
	reply, err := st.Client.Chat(context.Background(), []ai.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: src.String()},
	})
	if err != nil {
		return err
	}
	prose, _ := ai.SplitReply(reply)
	if prose == "" {
		prose = reply
	}
	ok, err := confirm(ctx, "将创建 "+ctx.WS.Rel(path)+"，确认写入？")
	if err != nil || !ok {
		return err
	}
	return applyOne(ctx, path, ai.Action{File: ctx.WS.Rel(path), Action: "create", Content: prose})
}
```

（import 补 `"github.com/bugzi/gs/internal/ai"`；`runProjectMenu` opts 插入 `option("升级计划", "升级计划")`，switch 加对应分支。最终菜单六项：README / 框架树 / 学习笔记 / 项目日志 / 升级计划 / 返回。）

- [ ] **Step 2: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 全绿

- [ ] **Step 3: 提交点（开发者执行）**

建议信息：`feat(project): 升级计划块（正本约定 + AI 初稿经确认写入）`

---

### Task 12: 收尾 — 回归、人工冒烟与文档核对

**Files:**
- 无新增；必要时微调

- [ ] **Step 1: 全量构建与测试**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全绿、vet 无告警

- [ ] **Step 2: 人工冒烟（真实工作区 `D:\Code\Go`，开发者配合）**

1. `/project`：列表尾部出现「＋ 将 gs（本工具）加入项目列表」；选中后提示成功；再次 `/project` 出现 `gs · 本工具（Project/gs）`，`.gs/projects.toml` 内容正确。
2. 选 HDU-KillCourse 进菜单：五块 + 返回齐全。
3. README：全文进 pager，`e` 可编辑，`q` 返回菜单。
4. 框架树：未配 AI 或首次 → 「AI 分析中…」生成介绍；二次进入直接渲染；选目录进入、选文件弹操作；「重新生成介绍（AI）」覆盖缓存。
5. 学习笔记：`docs/study` 与 `sea/<项目名>`（若建了）两处列出；阅读/编辑可用；笔记名含 `xxx.go` 时出现「打开对应源文件」，确认 TRAE 拉起。
6. 项目日志：git 统计与目录时间降级两种口径正确；日志篇目可阅读。
7. 升级计划：不存在时三入口；AI 初稿经确认写入 `docs/project/升级计划.md`；手动创建为空骨架。
8. 回归：`/project edit`、`/AI`、`/search` 正常。

- [ ] **Step 3: 日志与归档**

- 按 `docs/log/` 机制记当天开发日志（含创建时间行）；
- 本 spec/plan 按 `docs/project/README.md` 归档机制处理（实施完成后移入 `docs/project/project/archive/`）。

- [ ] **Step 4: 提交点（开发者执行）**

建议信息：`feat(project): /project 交互详情五块落地（1.2.0 功能集）`

---

## Self-Review 记录

- **Spec 覆盖**：注册表与 gs 纳管（Task 2/7）、菜单骨架（Task 7）、README（Task 7）、项目日志（Task 8）、学习笔记+反解+TRAE（Task 9/10）、框架树+AI 缓存+重生成（Task 10）、升级计划三入口（Task 11）、错误降级各任务内嵌、opener 配置（Task 1/9）——设计文档第 3–9 节均有对应任务。
- **占位符扫描**：无 TBD/TODO；所有代码步骤给全量代码。
- **类型一致性**：`structure.Entry/Note/Node/StructureCache/GitInfo/LogDate`、`workspace.Registry/RegistryEntry/Project.Manual`、`projectOptions/resolveProject/openInIDE/aiConfigured/buildProjectLogText` 各任务间签名一致；`GitRun` 打桩变量在 Task 6 定义、测试同包直接赋值。
