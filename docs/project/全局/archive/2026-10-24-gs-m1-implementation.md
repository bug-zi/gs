# gs M1（日常维护）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 gs M1：交互式 REPL 学习工作台，支持 /help /logs /log add /logs edit /notes(new/edit) /plan(edit/next) /search /exit，含配置加载与编辑器调起。

**Architecture:** 单二进制 Go 程序。`internal/` 下按职责分包：workspace（工作区定位与日志日期）、config、edit（编辑器调起）、search（扫描引擎）、ui（渲染）、command（各命令）、repl（循环与路由）。命令层是纯逻辑函数 + 薄交互壳，纯逻辑全部表驱动测试。

**Tech Stack:** Go 标准库 + ergochat/readline（行编辑）、charmbracelet/huh（选择/确认/输入）、charmbracelet/glamour（Markdown 渲染）、charmbracelet/lipgloss（颜色）、BurntSushi/toml。

**设计文档:** `Project/gs/docs/design.md`（已确认）

---

## 全局约定（所有任务必读）

- **git 由用户执行**：每个任务末尾的「提交检查点」只给出建议命令，执行代理必须暂停并请用户执行；代理不得运行任何 git 命令（用户全局硬性规则）。
- 所有 `go` 命令的工作目录是 `D:/Code/Go/Project/gs`；git 建议命令的工作目录是 `D:/Code/Go`（外层总仓库）。
- 所有源码与测试文件使用 UTF-8；代码中不写英文注释，仅在非直观处写一行中文注释。
- 测试命令统一为 `go test ./...`（跑全部）或 `go test ./internal/xxx/ -v`（跑单个包）。
- 模块名：`github.com/bugzi/gs`。

## 文件结构总览

```
Project/gs/
├── go.mod
├── cmd/gs/main.go              # 入口
├── cmd/spike/main.go           # Task 1 的中文输入验证程序（验证后删除）
└── internal/
    ├── workspace/workspace.go  # 工作区定位、日志日期、原子写
    ├── config/config.go        # 配置加载合并
    ├── edit/editor.go          # 编辑器解析与调起
    ├── search/search.go        # 扫描与关键词检索
    ├── ui/render.go            # glamour/lipgloss 封装
    ├── command/command.go      # Ctx/Command/Build
    ├── command/help.go
    ├── command/logs.go
    ├── command/notes.go
    ├── command/plan.go
    ├── command/search.go
    └── repl/
        ├── repl.go             # 循环、路由、横幅
        ├── suggest.go          # 编辑距离建议
        └── input.go            # readline / plain 双实现
```

各测试文件与被测文件同目录同名 `_test.go`。

---

### Task 0: 工程骨架与依赖

**Files:**
- Create: `Project/gs/go.mod`
- Create: `Project/gs/cmd/gs/main.go`

- [ ] **Step 1: 初始化模块**

在 `D:/Code/Go/Project/gs` 下运行：

```
go mod init github.com/bugzi/gs
go get github.com/ergochat/readline@latest
go get github.com/charmbracelet/huh@latest
go get github.com/charmbracelet/glamour@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/BurntSushi/toml@latest
```

Expected: 每个 go get 成功，go.mod 中出现 5 个 require。

- [ ] **Step 2: 写入口占位**

`cmd/gs/main.go`：

```go
package main

import "fmt"

func main() {
	fmt.Println("gs dev build")
}
```

- [ ] **Step 3: 验证编译运行**

Run: `go run ./cmd/gs`
Expected: 输出 `gs dev build`

- [ ] **Step 4: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: M1 工程骨架与依赖"
```

---

### Task 1: 行输入 spike（中文/历史验证）

**Files:**
- Create: `Project/gs/cmd/spike/main.go`（验证后删除）

- [ ] **Step 1: 写 spike 程序**

`cmd/spike/main.go`：

```go
package main

import (
	"fmt"
	"path/filepath"

	"github.com/ergochat/readline"
)

func main() {
	home, _ := os.UserHomeDir()
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "spike> ",
		HistoryFile:  filepath.Join(home, ".gs", "spike-history"),
		UniqueEditLine: true,
	})
	if err != nil {
		fmt.Println("readline 初始化失败:", err)
		return
	}
	defer rl.Close()
	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		fmt.Println("echo:", line)
	}
}
```

注意：`os` 未导入则补 `"os"`。若 `NewEx`/`UniqueEditLine` 等符号名与所装版本不符，先运行 `go doc github.com/ergochat/readline | head -50` 确认构造函数与 Config 字段名，再按实际 API 微调（保持逻辑不变）。

- [ ] **Step 2: 请用户交互验证（必须暂停等待用户）**

Run: `go run ./cmd/spike`

请用户依次验证并反馈：① 输入中文「学习 context 并发」光标位置是否正确；② 按 ↑ 能否调出历史；③ Ctrl+C / Ctrl+D 行为是否正常退出。

- [ ] **Step 3: 记录结论**

- 中文正常 → 后续 Task 12 默认 readline 模式。
- 中文光标错位 → 后续 Task 12 默认 plain 模式（`GS_PLAIN_INPUT` 语义反转：默认即 plain，设为 `0` 强制 readline）。

- [ ] **Step 4: 删除 spike**

删除 `cmd/spike/` 整个目录，运行 `go build ./...` 确认通过。

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: 行输入 spike 验证"
```

---

### Task 2: workspace 包

**Files:**
- Create: `Project/gs/internal/workspace/workspace.go`
- Test: `Project/gs/internal/workspace/workspace_test.go`

- [ ] **Step 1: 写失败测试**

`internal/workspace/workspace_test.go`：

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newWS(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Study/logs", "Study/plan"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &Workspace{Root: root}
}

func TestDetect(t *testing.T) {
	ws := newWS(t)
	deep := filepath.Join(ws.Root, "Study", "Project", "demo")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Detect(deep)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Root != ws.Root {
		t.Fatalf("got %s want %s", got.Root, ws.Root)
	}
	if _, err := Detect(t.TempDir()); err == nil {
		t.Fatal("非工作区目录应报错")
	}
}

func TestLogFileName(t *testing.T) {
	tm := time.Date(2026, 10, 24, 20, 0, 0, 0, time.UTC) // 东八区为 10-25 凌晨
	if got := LogFileName(tm); got != "261025.md" {
		t.Fatalf("got %s", got)
	}
}

func TestParseLogDate(t *testing.T) {
	if _, ok := ParseLogDate("261024.md"); !ok {
		t.Fatal("261024.md 应可解析")
	}
	if _, ok := ParseLogDate("notes.md"); ok {
		t.Fatal("notes.md 不应可解析")
	}
}

func TestListLogsOrder(t *testing.T) {
	ws := newWS(t)
	names := []string{"261015.md", "261024.md", "261021.md", "readme.md"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(ws.LogsDir(), n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dates, err := ws.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"261024", "261021", "261015"}
	for i, w := range want {
		if got := dates[i].Format("060102"); got != w {
			t.Fatalf("第 %d 个日志 got %s want %s", i, got, w)
		}
	}
}

func TestStreak(t *testing.T) {
	ws := newWS(t)
	now := time.Now().In(CST)
	for _, d := range []time.Time{now, now.AddDate(0, 0, -1), now.AddDate(0, 0, -3)} {
		p := filepath.Join(ws.LogsDir(), LogFileName(d))
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := ws.Streak(); got != 2 {
		t.Fatalf("streak got %d want 2", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/workspace/ -v`
Expected: 编译错误 `undefined: Workspace`（或同类 undefined）

- [ ] **Step 3: 实现**

`internal/workspace/workspace.go`：

```go
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CST 东八区，日志文件名与“今天”的判定一律用它
var CST = time.FixedZone("CST", 8*3600)

type Workspace struct{ Root string }

func Detect(start string) (*Workspace, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		if isWorkspace(dir) {
			return &Workspace{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("在 %s 及其上级目录中未找到学习工作区（需含 Study/logs 与 Study/plan），可用 gs --root <路径> 指定", start)
		}
		dir = parent
	}
}

func isWorkspace(dir string) bool {
	return dirExists(filepath.Join(dir, "Study", "logs")) && dirExists(filepath.Join(dir, "Study", "plan"))
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func (w *Workspace) LogsDir() string  { return filepath.Join(w.Root, "Study", "logs") }
func (w *Workspace) NotesDir() string { return filepath.Join(w.Root, "Study", "notes") }
func (w *Workspace) PlanDir() string  { return filepath.Join(w.Root, "Study", "plan") }
func (w *Workspace) StateFile() string {
	return filepath.Join(w.PlanDir(), "学习状态.md")
}

// Rel 返回相对 Root 的正斜杠路径，用于展示与索引
func (w *Workspace) Rel(path string) string {
	rel, err := filepath.Rel(w.Root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func LogFileName(t time.Time) string { return t.In(CST).Format("060102") + ".md" }

func ParseLogDate(name string) (time.Time, bool) {
	base := strings.TrimSuffix(name, ".md")
	if len(base) != 6 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("060102", base, CST)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ListLogs 全部日志日期，新在前
func (w *Workspace) ListLogs() ([]time.Time, error) {
	entries, err := os.ReadDir(w.LogsDir())
	if err != nil {
		return nil, err
	}
	var dates []time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if t, ok := ParseLogDate(e.Name()); ok {
			dates = append(dates, t)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].After(dates[j]) })
	return dates, nil
}

// Streak 以东八区今天为起点向回数连续日志天数；今天无日志则从昨天起算
func (w *Workspace) Streak() int {
	dates, err := w.ListLogs()
	if err != nil || len(dates) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, d := range dates {
		seen[d.Format("20060102")] = true
	}
	day := time.Now().In(CST)
	if !seen[day.Format("20060102")] {
		day = day.AddDate(0, 0, -1)
	}
	n := 0
	for ; seen[day.Format("20060102")]; day = day.AddDate(0, 0, -1) {
		n++
	}
	return n
}

// WriteFileAtomic 先写临时文件再 rename，避免半截文件
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".gs-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/workspace/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: workspace 包——工作区定位/日志日期/连续天数/原子写"
```

---

### Task 3: config 包

**Files:**
- Create: `Project/gs/internal/config/config.go`
- Test: `Project/gs/internal/config/config_test.go`

- [ ] **Step 1: 写失败测试**

`internal/config/config_test.go`：

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/config/ -v`
Expected: 编译错误 `undefined: Load`

- [ ] **Step 3: 实现**

`internal/config/config.go`：

```go
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

type Config struct {
	AI   AI   `toml:"ai"`
	Scan Scan `toml:"scan"`
	UI   UI   `toml:"ui"`
}

func defaultConfig() Config {
	return Config{
		Scan: Scan{
			Exclude:   []string{".git", ".codegraph", ".agents", "Resource", "node_modules", "_docx_render"},
			MaxFileKB: 512,
		},
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
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/config/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: config 包——全局/工作区合并与 GS_API_KEY"
```

---

### Task 4: edit 包（编辑器调起）

**Files:**
- Create: `Project/gs/internal/edit/editor.go`
- Test: `Project/gs/internal/edit/editor_test.go`

- [ ] **Step 1: 写失败测试**

`internal/edit/editor_test.go`：

```go
package edit

import (
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestResolvePriority(t *testing.T) {
	t.Setenv("EDITOR", "vi")
	t.Setenv("VISUAL", "emacs")
	cfg := &config.Config{UI: config.UI{Editor: "myed"}}
	if got := Resolve(cfg); got != "myed" {
		t.Fatalf("配置编辑器应最优先, got %q", got)
	}
	if got := Resolve(&config.Config{}); got != "vi" {
		t.Fatalf("应回退 $EDITOR, got %q", got)
	}
	t.Setenv("EDITOR", "")
	if got := Resolve(&config.Config{}); got != "emacs" {
		t.Fatalf("应回退 $VISUAL, got %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/edit/ -v`
Expected: 编译错误 `undefined: Resolve`

- [ ] **Step 3: 实现**

`internal/edit/editor.go`：

```go
package edit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/config"
)

// Resolve 编辑器解析顺序：config UI.Editor > $EDITOR > $VISUAL > code > notepad。
// 都找不到时返回空串。
func Resolve(cfg *config.Config) string {
	if cfg != nil && cfg.UI.Editor != "" {
		return cfg.UI.Editor
	}
	for _, k := range []string{"EDITOR", "VISUAL"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	for _, c := range []string{"code", "notepad"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// Open 调起编辑器打开 path，阻塞直到编辑器退出（code 不带 -w 时立即返回，属正常）。
func Open(path string, cfg *config.Config) error {
	ed := Resolve(cfg)
	if ed == "" {
		return errors.New("未找到可用编辑器，请在 .gs/config.toml 的 [ui] 段配置 editor = \"...\"")
	}
	parts := strings.Fields(ed)
	lp, err := exec.LookPath(parts[0])
	if err != nil {
		return errors.New("编辑器不可用: " + parts[0])
	}
	args := append(parts[1:], path)
	// Windows 下 .cmd/.bat（如 code.cmd）必须经 cmd /c 启动
	if ext := strings.ToLower(filepath.Ext(lp)); ext == ".cmd" || ext == ".bat" {
		return exec.Command("cmd", "/c", append([]string{lp}, args...)...).Run()
	}
	cmd := exec.Command(lp, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/edit/ -v`
Expected: PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: edit 包——编辑器解析链与 Windows .cmd 适配"
```

---

### Task 5: search 包

**Files:**
- Create: `Project/gs/internal/search/search.go`
- Test: `Project/gs/internal/search/search_test.go`

- [ ] **Step 1: 写失败测试**

`internal/search/search_test.go`：

```go
package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func fixture(t *testing.T) (root string, cfg *config.Config) {
	t.Helper()
	root = t.TempDir()
	cfg, _ = config.Load(root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/notes/GPT/a.md", "# A标题\n\ncontext 用于跨协程传值。\n\n## 小节二\ncontext 超时控制。\n")
	w("Study/code.go", "package main\n\nfunc Foo() {\n\tctx := context.Background()\n}\n")
	w("Resource/hidden.md", "context 不应被扫到\n")
	return root, cfg
}

func TestSearchANDAndExclusion(t *testing.T) {
	root, cfg := fixture(t)
	// 双词 AND：两词都在才收录
	results, words, err := Search(root, cfg, "context 超时")
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 {
		t.Fatalf("words = %v", words)
	}
	var paths []string
	for _, r := range results {
		paths = append(paths, r.Path)
	}
	want := "Study/notes/GPT/a.md"
	if len(paths) != 1 || paths[0] != want {
		t.Fatalf("paths = %v, want [%s]", paths, want)
	}
	// Resource 已被排除；a.md 中两个命中单元分别为标题与小节二
	if results[0].Hits[0].Unit != "A标题" {
		t.Fatalf("Unit = %q", results[0].Hits[0].Unit)
	}
	last := results[0].Hits[len(results[0].Hits)-1]
	if last.Unit != "小节二" {
		t.Fatalf("末命中 Unit = %q", last.Unit)
	}
}

func TestSearchGoFuncUnit(t *testing.T) {
	root, cfg := fixture(t)
	results, _, err := Search(root, cfg, "Background")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != "Study/code.go" {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Hits[0].Unit != "Foo" {
		t.Fatalf("go 命中 Unit = %q", results[0].Hits[0].Unit)
	}
	if results[0].Hits[0].Line != 4 {
		t.Fatalf("Line = %d", results[0].Hits[0].Line)
	}
	if len(results[0].Hits[0].Context) == 0 {
		t.Fatal("Context 不应为空")
	}
}

func TestSearchNoHit(t *testing.T) {
	root, cfg := fixture(t)
	results, _, err := Search(root, cfg, "不存在的词")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("results = %+v", results)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/search/ -v`
Expected: 编译错误 `undefined: Search`

- [ ] **Step 3: 实现**

`internal/search/search.go`：

```go
package search

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bugzi/gs/internal/config"
)

type Hit struct {
	Unit    string   // 命中所在小节标题或函数名
	Line    int      // 1 起始行号
	Context []string // 命中行及前后各 1 行
}

type FileResult struct {
	Path string // 相对 root 的正斜杠路径
	Hits []Hit
}

var funcRe = regexp.MustCompile(`func\s+(?:\([^)]*\)\s*)?(\w+)\s*\(`)

// Search 多词 AND：文件内全部词命中才收录；行级任一词命中即为 Hit
func Search(root string, cfg *config.Config, query string) ([]FileResult, []string, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, nil, nil
	}
	var results []FileResult
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excluded(cfg, root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".md" && ext != ".go" {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > cfg.Scan.MaxFileKB*1024 {
			return nil
		}
		if fr := searchFile(path, root, words); fr != nil {
			results = append(results, *fr)
		}
		return nil
	})
	return results, words, err
}

func excluded(cfg *config.Config, root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		for _, ex := range cfg.Scan.Exclude {
			if seg == ex {
				return true
			}
		}
	}
	return false
}

func searchFile(path, root string, words []string) *FileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lower := make([]string, len(lines))
	for i, l := range lines {
		lower[i] = strings.ToLower(l)
	}
	for _, w := range words {
		found := false
		for _, l := range lower {
			if strings.Contains(l, w) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	isMd := strings.EqualFold(filepath.Ext(path), ".md")
	unit := ""
	var hits []Hit
	for i := range lines {
		trimmed := strings.TrimSpace(lines[i])
		if isMd && strings.HasPrefix(trimmed, "#") {
			unit = strings.TrimLeft(trimmed, "# ")
		} else if !isMd && strings.HasPrefix(trimmed, "func ") {
			if m := funcRe.FindStringSubmatch(trimmed); m != nil {
				unit = m[1]
			}
		}
		for _, w := range words {
			if strings.Contains(lower[i], w) {
				hits = append(hits, Hit{Unit: unit, Line: i + 1, Context: contextOf(lines, i)})
				break
			}
		}
	}
	if len(hits) == 0 {
		return nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return &FileResult{Path: filepath.ToSlash(rel), Hits: hits}
}

func contextOf(lines []string, i int) []string {
	lo := i - 1
	if lo < 0 {
		lo = 0
	}
	hi := i + 2
	if hi > len(lines) {
		hi = len(lines)
	}
	return lines[lo:hi]
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/search/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: search 包——排除规则/md 小节切分/go 函数切分/多词 AND"
```

---

### Task 6: ui 包

**Files:**
- Create: `Project/gs/internal/ui/render.go`
- Test: `Project/gs/internal/ui/render_test.go`

- [ ] **Step 1: 写失败测试**

`internal/ui/render_test.go`：

```go
package ui

import (
	"strings"
	"testing"
)

func TestRenderMD(t *testing.T) {
	out := RenderMD("# 标题\n\n- 条目一\n")
	if strings.TrimSpace(out) == "" {
		t.Fatal("渲染结果不应为空")
	}
	if !strings.Contains(out, "条目一") {
		t.Fatalf("内容丢失: %q", out)
	}
}

func TestHighlightWords(t *testing.T) {
	got := HighlightWords("context 超时与 context 取消", []string{"context"})
	// 全部命中且上色（ANSI 包裹不影响子串计数）
	if strings.Count(got, "context") != 2 {
		t.Fatalf("got %q", got)
	}
	if HighlightWords("无命中", []string{"zzz"}) != "无命中" {
		t.Fatal("无命中应原样返回")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/ui/ -v`
Expected: 编译错误 `undefined: RenderMD`

- [ ] **Step 3: 实现**

`internal/ui/render.go`：

```go
package ui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	matchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // 黄：命中词
	pathStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // 红：错误
)

var (
	rendererOnce sync.Once
	renderer     *glamour.TermRenderer
)

func initRenderer() {
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(90))
	if err == nil {
		renderer = r
	}
}

// RenderMD 渲染 Markdown；渲染器不可用时原样返回
func RenderMD(src string) string {
	rendererOnce.Do(initRenderer)
	if renderer == nil {
		return src
	}
	out, err := renderer.Render(src)
	if err != nil {
		return src
	}
	return out
}

// HighlightWords 大小写不敏感地把 words 命中处上色，保留原大小写
func HighlightWords(line string, words []string) string {
	type span struct{ lo, hi int }
	var spans []span
	lower := strings.ToLower(line)
	for _, w := range words {
		w = strings.ToLower(w)
		if w == "" {
			continue
		}
		for i := 0; ; {
			j := strings.Index(lower[i:], w)
			if j < 0 {
				break
			}
			spans = append(spans, span{i + j, i + j + len(w)})
			i += j + len(w)
		}
	}
	if len(spans) == 0 {
		return line
	}
	// 合并重叠区间后从后往前替换，避免索引位移
	sort.Slice(spans, func(a, b int) bool { return spans[a].lo < spans[b].lo })
	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		if s.lo <= merged[len(merged)-1].hi {
			if s.hi > merged[len(merged)-1].hi {
				merged[len(merged)-1].hi = s.hi
			}
		} else {
			merged = append(merged, s)
		}
	}
	var b strings.Builder
	last := len(line)
	for i := len(merged) - 1; i >= 0; i-- {
		s := merged[i]
		if s.hi > last {
			continue
		}
		b.WriteString(line[s.hi:last])
		b.WriteString(matchStyle.Render(line[s.lo:s.hi]))
		last = s.lo
	}
	b.WriteString(line[:last])
	return b.String()
}

func Path(s string) string { return pathStyle.Render(s) }
func Dim(s string) string  { return dimStyle.Render(s) }
func Err(s string) string  { return errStyle.Render(s) }
```

注意缺 `"sort"` 导入，实现时补上。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/ui/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: ui 包——glamour 渲染与命中高亮"
```

---

### Task 7: command 框架 + /help + /exit

**Files:**
- Create: `Project/gs/internal/command/command.go`
- Create: `Project/gs/internal/command/help.go`
- Test: `Project/gs/internal/command/command_test.go`

- [ ] **Step 1: 写框架与失败测试**

`internal/command/command.go`：

```go
package command

import (
	"io"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

// Ctx 每个命令执行时拿到的一切
type Ctx struct {
	WS   *workspace.Workspace
	Cfg  *config.Config
	Out  io.Writer
	Args []string // 命令名之后的参数
	Cmds []Command // 由 repl 注入，供 /help 列表用
}

type Command struct {
	Name  string
	Alias []string
	Short string
	Run   func(ctx *Ctx) error
}

func hasAlias(c Command, name string) bool {
	for _, a := range c.Alias {
		if a == name {
			return true
		}
	}
	return false
}

// Find 按名称或别名查找命令
func Find(cmds []Command, name string) *Command {
	for i := range cmds {
		if cmds[i].Name == name || hasAlias(cmds[i], name) {
			return &cmds[i]
		}
	}
	return nil
}
```

`internal/command/help.go`：

```go
package command

import (
	"fmt"
	"strings"
)

const helpText = `gs · Go 学习工作台

命令：
  /logs [关键词|MMDD]     查看日志：无参=今天(无则最近一天)，参数为4位数字时按月日，否则按关键词过滤
  /logs edit [MMDD]       调起编辑器编辑日志（默认今天，不存在则创建）
  /log add "<内容>"       向今日日志追加一条（超过40字会确认）
  /notes [关键词]         查看笔记：无参=列表选择，有关词=模糊匹配
  /notes new <主题>       新建笔记
  /notes edit [关键词]    调起编辑器编辑笔记
  /plan                   查看当前断点与阶段清单
  /plan edit              调起编辑器编辑计划文件
  /plan next "<断点>"     更新学习状态.md 的“下一步”
  /search <关键词>        全局搜索 md 与 go 源码（支持空格分隔多词 AND）
  /AI                     （M3 提供）
  /help                   显示本说明
  /exit                   退出 gs

示例：
  /logs context        → 在全部日志中搜含 context 的记录
  /notes GC            → 打开笔记查看 GC 相关主题
  /plan next 登录模块源码走读
  /search context 超时  → 同时含两词的文件`

func newHelp() Command {
	return Command{
		Name:  "help",
		Alias: []string{"h", "?"},
		Short: "显示使用说明",
		Run: func(ctx *Ctx) error {
			var b strings.Builder
			b.WriteString(helpText)
			b.WriteString("\n\n已启用命令：")
			for _, c := range ctx.Cmds {
				fmt.Fprintf(&b, "\n  /%s  %s", c.Name, c.Short)
			}
			fmt.Fprintln(ctx.Out, b.String())
			return nil
		},
	}
}

func newExit() Command {
	return Command{Name: "exit", Alias: []string{"q", "quit"}, Short: "退出 gs", Run: func(ctx *Ctx) error { return nil }}
}
```

`internal/command/command_test.go`：

```go
package command

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpListsCommands(t *testing.T) {
	var out bytes.Buffer
	help := newHelp()
	ctx := &Ctx{Out: &out, Cmds: []Command{help, newExit()}}
	if err := help.Run(ctx); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/logs", "/notes new", "/plan next", "/search", "/exit"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("帮助缺 %s", want)
		}
	}
}

func TestFindWithAlias(t *testing.T) {
	cmds := []Command{newExit()}
	if Find(cmds, "q") == nil {
		t.Fatal("应通过别名 q 找到 exit")
	}
	if Find(cmds, "nope") != nil {
		t.Fatal("未知命令应返回 nil")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ -v`
Expected: 编译错误（缺 Build 等）——本任务尚未写 Build，先只跑现有测试：`go test ./internal/command/ -v` Expected: PASS（框架任务以现有两测试为准）

- [ ] **Step 3: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: command 框架与 /help /exit"
```

---

### Task 8: /logs 命令（读 + add + edit）

**Files:**
- Create: `Project/gs/internal/command/logs.go`
- Test: `Project/gs/internal/command/logs_test.go`
- Modify: `Project/gs/internal/command/help.go`（注册进 Build）

- [ ] **Step 1: 写失败测试**

`internal/command/logs_test.go`：

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

func wsForTest(t *testing.T) *workspace.Workspace {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Study/logs", "Study/plan"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &workspace.Workspace{Root: root}
}

func TestAppendEntry(t *testing.T) {
	ws := wsForTest(t)
	today := time.Now().In(workspace.CST)
	if err := AppendEntry(ws, today, "跑通了CAS登录"); err != nil {
		t.Fatal(err)
	}
	if err := AppendEntry(ws, today, "看了GC三色标记"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(ws.LogsDir(), workspace.LogFileName(today)))
	if err != nil {
		t.Fatal(err)
	}
	want := "- 跑通了CAS登录\n- 看了GC三色标记\n"
	if string(data) != want {
		t.Fatalf("got %q want %q", data, want)
	}
}

func TestEntryTooLong(t *testing.T) {
	s40 := strings.Repeat("学", 40)
	s41 := s40 + "习"
	if EntryTooLong(s40) {
		t.Fatal("40字不应超限")
	}
	if !EntryTooLong(s41) {
		t.Fatal("41字应超限")
	}
}

func TestFilterLines(t *testing.T) {
	content := "- 确认空版本提示\n- 继续走读登录模块\n- 建立学习总览\n"
	got := FilterLines(content, "登录")
	if len(got) != 1 || !strings.Contains(got[0], "登录") {
		t.Fatalf("got %v", got)
	}
}

func TestViewFormat(t *testing.T) {
	ws := wsForTest(t)
	yesterday := time.Now().In(workspace.CST).AddDate(0, 0, -1)
	if err := AppendEntry(ws, yesterday, "复习了map底层"); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	n, err := viewLog(&out, ws, yesterday, false)
	if err != nil || n != 1 {
		t.Fatalf("viewLog n=%d err=%v", n, err)
	}
	if !strings.Contains(out.String(), "复习了map底层") {
		t.Fatalf("输出缺内容: %q", out.String())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ -v`
Expected: 编译错误 `undefined: AppendEntry`

- [ ] **Step 3: 实现 logs.go 并注册**

`internal/command/logs.go`：

```go
package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// AppendEntry 向指定日日志追加一条；文件不存在则创建
func AppendEntry(ws *workspace.Workspace, t time.Time, entry string) error {
	path := filepath.Join(ws.LogsDir(), workspace.LogFileName(t))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "- %s\n", strings.TrimSpace(entry))
	return err
}

func EntryTooLong(entry string) bool { return utf8.RuneCountInString(entry) > 40 }

// FilterLines 返回包含关键词的行
func FilterLines(content, kw string) []string {
	var out []string
	for _, l := range strings.Split(content, "\n") {
		if strings.Contains(l, kw) {
			out = append(out, l)
		}
	}
	return out
}

// viewLog 渲染单日日志；create=true 时对不存在的今日文件先建空文件。
// 返回是否显示了内容。
func viewLog(out io.Writer, ws *workspace.Workspace, t time.Time, create bool) (bool, error) {
	path := filepath.Join(ws.LogsDir(), workspace.LogFileName(t))
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		if !create {
			return false, nil
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return false, err
		}
		fmt.Fprintf(out, "%s（新文件）已用编辑器打开\n", ui.Path(ws.Rel(path)))
		return true, nil
	}
	fmt.Fprintf(out, "%s\n%s", ui.Path(ws.Rel(path)), ui.RenderMD(string(data)))
	return true, nil
}

// viewByKeyword 全部日志按关键词过滤，新在前
func viewByKeyword(out io.Writer, ws *workspace.Workspace, kw string) error {
	dates, err := ws.ListLogs()
	if err != nil {
		return err
	}
	found := false
	for _, d := range dates {
		data, err := os.ReadFile(filepath.Join(ws.LogsDir(), workspace.LogFileName(d)))
		if err != nil {
			continue
		}
		lines := FilterLines(string(data), kw)
		if len(lines) == 0 {
			continue
		}
		found = true
		fmt.Fprintf(out, "\n%s\n", ui.Path(workspace.LogFileName(d)))
		for _, l := range lines {
			fmt.Fprintln(out, ui.HighlightWords(l, strings.Fields(kw)))
		}
	}
	if !found {
		fmt.Fprintf(out, "全部日志中未找到含 %q 的记录\n", kw)
	}
	return nil
}

// viewByMMDD 按四位月日定位日志
func viewByMMDD(out io.Writer, ws *workspace.Workspace, mmdd string) (bool, error) {
	dates, err := ws.ListLogs()
	if err != nil {
		return false, err
	}
	for _, d := range dates {
		if d.Format("0102") == mmdd {
			return viewLog(out, ws, d, false)
		}
	}
	fmt.Fprintf(out, "没有 %s 的日志（可用 /logs 直接看最近）\n", mmdd)
	return true, nil
}

func newLogs() Command {
	return Command{
		Name:  "logs",
		Alias: []string{"log"},
		Short: "查看/追加/编辑学习日志",
		Run: func(ctx *Ctx) error {
			today := time.Now().In(workspace.CST)
			if len(ctx.Args) > 0 {
				switch ctx.Args[0] {
				case "add":
					entry := strings.Join(ctx.Args[1:], " ")
					if entry == "" {
						return fmt.Errorf("用法: /log add \"<内容>\"")
					}
					if EntryTooLong(entry) {
						fmt.Fprintln(ctx.Out, ui.Err("超过 40 字（学习日志应一句话概括）"))
					}
					if err := AppendEntry(ctx.WS, today, entry); err != nil {
						return err
					}
					fmt.Fprintf(ctx.Out, "已记入 %s\n", ui.Path(wsRel(ctx.WS, today)))
					return nil
				case "edit":
					t := today
					if len(ctx.Args) > 1 {
						if mmddOK(ctx.Args[1]) {
							if d, ok := findDate(ctx.WS, ctx.Args[1]); ok {
								t = d
							} else {
								return fmt.Errorf("没有 %s 的日志", ctx.Args[1])
							}
						} else {
							return fmt.Errorf("日期格式应为 MMDD，如 1024")
						}
					}
					return editLogFile(ctx, t)
				}
				if mmddOK(ctx.Args[0]) {
					_, err := viewByMMDD(ctx.Out, ctx.WS, ctx.Args[0])
					return err
				}
				return viewByKeyword(ctx.Out, ctx.WS, strings.Join(ctx.Args, " "))
			}
			// 无参：今天，无则最近一天
			shown, err := viewLog(ctx.Out, ctx.WS, today, false)
			if err != nil {
				return err
			}
			if shown {
				return nil
			}
			dates, err := ctx.WS.ListLogs()
			if err != nil || len(dates) == 0 {
				fmt.Fprintln(ctx.Out, "还没有任何日志，用 /log add \"...\" 记录第一条吧")
				return err
			}
			fmt.Fprintln(ctx.Out, ui.Dim("今天还没有日志，显示最近一次："))
			_, err = viewLog(ctx.Out, ctx.WS, dates[0], false)
			return err
		},
	}
}

func wsRel(ws *workspace.Workspace, t time.Time) string {
	return ws.Rel(filepath.Join(ws.LogsDir(), workspace.LogFileName(t)))
}

func mmddOK(s string) bool {
	if len(s) != 4 {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func findDate(ws *workspace.Workspace, mmdd string) (time.Time, bool) {
	dates, err := ws.ListLogs()
	if err != nil {
		return time.Time{}, false
	}
	for _, d := range dates {
		if d.Format("0102") == mmdd {
			return d, true
		}
	}
	return time.Time{}, false
}

func editLogFile(ctx *Ctx, t time.Time) error {
	path := filepath.Join(ctx.WS.LogsDir(), workspace.LogFileName(t))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(ctx.Out, "正在打开 %s ...\n", ui.Path(ctx.WS.Rel(path)))
	return editOpen(ctx, path)
}
```

其中 `editOpen` 在 Task 9 之后统一放 `command/util.go`（本任务先内联为 `edit.Open(path, ctx.Cfg)` 调用，直接 `import "github.com/bugzi/gs/internal/edit"` 并写 `return edit.Open(path, ctx.Cfg)`）：

把上面 `editLogFile` 最后一行改为：

```go
	return editOpen(ctx, path)
```

并新建 `internal/command/util.go`：

```go
package command

import (
	"github.com/bugzi/gs/internal/edit"
)

func editOpen(ctx *Ctx, path string) error {
	return edit.Open(path, ctx.Cfg)
}
```

再把 `newLogs()` 注册进 Build：本任务在 `help.go` 末尾追加：

```go
// Build 全部 M1 命令；后续任务把新命令 append 进来
func Build() []Command {
	return []Command{newLogs(), newHelp(), newExit()}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS（含 Task 7 的测试）

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /logs 查看/关键词过滤/add/edit"
```

---

### Task 9: /notes 命令

**Files:**
- Create: `Project/gs/internal/command/notes.go`
- Test: `Project/gs/internal/command/notes_test.go`
- Modify: `Project/gs/internal/command/help.go`（Build 中追加 `newNotes()`）

- [ ] **Step 1: 写失败测试**

`internal/command/notes_test.go`：

```go
package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListNotes(t *testing.T) {
	ws := wsForTest(t)
	gpt := filepath.Join(ws.NotesDir(), "GPT")
	if err := os.MkdirAll(gpt, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"GPT/Go并发与互斥锁.md", "GPT/Go序列化与反序列化.md", "顶层.md"} {
		if err := os.WriteFile(filepath.Join(ws.NotesDir(), filepath.FromSlash(n)), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	names, err := ListNotes(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("names = %v", names)
	}
	if names[0] != "GPT/Go并发与互斥锁.md" && names[0] != "GPT/Go序列化与反序列化.md" && names[0] != "顶层.md" {
		t.Fatalf("路径应为正斜杠相对路径: %v", names)
	}
}

func TestMatchNotes(t *testing.T) {
	names := []string{"GPT/Go并发与互斥锁.md", "GPT/Go序列化与反序列化.md", "gc笔记.md"}
	if got := MatchNotes(names, "GC"); len(got) != 1 || got[0] != "gc笔记.md" {
		t.Fatalf("got %v", got)
	}
	if got := MatchNotes(names, "go"); len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestNewNote(t *testing.T) {
	ws := wsForTest(t)
	p, err := NewNote(ws, "Go垃圾回收")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Go垃圾回收\n" {
		t.Fatalf("内容 = %q", data)
	}
	if _, err := NewNote(ws, "Go垃圾回收"); err == nil {
		t.Fatal("重复创建应报错")
	}
	if _, err := NewNote(ws, "a/b"); err == nil {
		t.Fatal("含路径分隔符应报错")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ -v`
Expected: 编译错误 `undefined: ListNotes`

- [ ] **Step 3: 实现 notes.go 并注册**

`internal/command/notes.go`：

```go
package command

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// ListNotes 返回 notes 目录下全部 .md 的正斜杠相对路径（相对 notes 目录），排序稳定
func ListNotes(ws *workspace.Workspace) ([]string, error) {
	var names []string
	root := ws.NotesDir()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	return names, err
}

// MatchNotes 大小写不敏感的子串匹配
func MatchNotes(names []string, kw string) []string {
	kw = strings.ToLower(kw)
	var out []string
	for _, n := range names {
		if strings.Contains(strings.ToLower(n), kw) {
			out = append(out, n)
		}
	}
	return out
}

// NewNote 新建笔记，主题即文件名；已存在或含非法字符时报错
func NewNote(ws *workspace.Workspace, topic string) (string, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return "", errors.New("主题不能为空")
	}
	if strings.ContainsAny(topic, `/\:*?"<>|`) || strings.Contains(topic, "..") {
		return "", errors.New("主题不能包含路径分隔符或特殊字符")
	}
	path := filepath.Join(ws.NotesDir(), topic+".md")
	if _, err := os.Stat(path); err == nil {
		return "", errors.New("笔记已存在: " + path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte("# "+topic+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func showNote(out io.Writer, ws *workspace.Workspace, name string) error {
	data, err := os.ReadFile(filepath.Join(ws.NotesDir(), filepath.FromSlash(name)))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n%s", ui.Path("Study/notes/"+name), ui.RenderMD(string(data)))
	return nil
}

func pickNote(ctx *Ctx, names []string, label string) (string, error) {
	if len(names) == 1 {
		return names[0], nil
	}
	var choice string
	opts := make([]huhOption, 0, len(names))
	for _, n := range names {
		opts = append(opts, option(n, n))
	}
	if err := selectFrom(ctx, label, opts, &choice); err != nil {
		return "", err
	}
	return choice, nil
}

func newNotes() Command {
	return Command{
		Name:  "notes",
		Short: "查看/新建/编辑复习笔记",
		Run: func(ctx *Ctx) error {
			names, err := ListNotes(ctx.WS)
			if err != nil {
				return err
			}
			switch {
			case len(ctx.Args) >= 2 && ctx.Args[0] == "new":
				p, err := NewNote(ctx.WS, strings.Join(ctx.Args[1:], " "))
				if err != nil {
					return err
				}
				fmt.Fprintf(ctx.Out, "已创建 %s，继续编辑可执行 /notes edit %s\n",
					ui.Path(ctx.WS.Rel(p)), strings.TrimSuffix(filepath.Base(p), ".md"))
				return nil
			case len(ctx.Args) >= 1 && ctx.Args[0] == "new":
				return fmt.Errorf("用法: /notes new <主题>")
			case len(ctx.Args) >= 1 && ctx.Args[0] == "edit":
				kw := strings.Join(ctx.Args[1:], " ")
				targets := names
				if kw != "" {
					targets = MatchNotes(names, kw)
				}
				if len(targets) == 0 {
					return fmt.Errorf("没有匹配 %q 的笔记", kw)
				}
				pick, err := pickNote(ctx, targets, "选择要编辑的笔记")
				if err != nil {
					return err
				}
				path := filepath.Join(ctx.WS.NotesDir(), filepath.FromSlash(pick))
				return editOpen(ctx, path)
			case len(ctx.Args) == 0:
				if len(names) == 0 {
					fmt.Fprintln(ctx.Out, "notes 目录还没有笔记，用 /notes new <主题> 创建")
					return nil
				}
				pick, err := pickNote(ctx, names, "选择笔记")
				if err != nil {
					return err
				}
				return showNote(ctx.Out, ctx.WS, pick)
			default:
				kw := strings.Join(ctx.Args, " ")
				targets := MatchNotes(names, kw)
				switch len(targets) {
				case 0:
					return fmt.Errorf("没有匹配 %q 的笔记，可用 /notes 浏览全部", kw)
				case 1:
					return showNote(&out, ctx.WS, targets[0])
				default:
					pick, err := pickNote(ctx, targets, "多个匹配，选择一个")
					if err != nil {
						return err
					}
					return showNote(ctx.Out, ctx.WS, pick)
				}
			}
		},
	}
}
```

`pickNote` 依赖的 huh 封装放在 `internal/command/prompt.go`（huh 在非交互测试中不应被触发——纯逻辑测试都不经过 pickNote）：

```go
package command

import (
	"github.com/charmbracelet/huh"
)

type huhOption = huh.Option[string]

func option(label, value string) huh.Option[string] {
	return huh.NewOption(label, value)
}

// selectFrom 单选；运行在 REPL 主线程，终端由 huh 接管
func selectFrom(ctx *Ctx, title string, opts []huh.Option[string], out *string) error {
	return huh.NewSelect[string]().
		Title(title).
		Options(opts...).
		Value(out).
		Run()
}

// confirm 是/否确认
func confirm(ctx *Ctx, title string) (bool, error) {
	var ok bool
	err := huh.NewConfirm().Title(title).Value(&ok).Run()
	return ok, err
}

// inputLine 单行输入
func inputLine(ctx *Ctx, title string) (string, error) {
	var s string
	err := huh.NewInput().Title(title).Value(&s).Run()
	return s, err
}
```

同时把 Task 8 里 /logs add 的超长分支改用 confirm（`editLogFile` 保持不变）：

```go
				if EntryTooLong(entry) {
					ok, err := confirm(ctx, "超过 40 字（学习日志应一句话概括），仍要记录？")
					if err != nil {
						return err
					}
					if !ok {
						fmt.Fprintln(ctx.Out, "已取消")
						return nil
					}
				}
```

Build 更新（help.go）：

```go
func Build() []Command {
	return []Command{newLogs(), newNotes(), newHelp(), newExit()}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /notes 列表/匹配/新建/编辑与 huh 交互封装"
```

---

### Task 10: /plan 命令

**Files:**
- Create: `Project/gs/internal/command/plan.go`
- Test: `Project/gs/internal/command/plan_test.go`
- Modify: `Project/gs/internal/command/help.go`（Build 追加 `newPlan()`）

- [ ] **Step 1: 写失败测试**

`internal/command/plan_test.go`：

```go
package command

import (
	"strings"
	"testing"
)

const stateFixture = `# 学习状态

更新时间：2026-10-20（东八区）。本文件记录执行断点与已确认变更。

## 当前断点

- 当前优先模块：KillCourse 学习总览核对。
- 下一步：用户核对总览后设置入口。
- 当前疑点：版本提示误报。

## 已确认的顺序与节点

1. KillCourse 速成
2. Go 核心补缺

## 其他

无关内容。
`

func TestParseState(t *testing.T) {
	st := ParseState(stateFixture)
	if len(st.Breakpoints) != 3 {
		t.Fatalf("Breakpoints = %+v", st.Breakpoints)
	}
	if st.Breakpoints[0].Key != "当前优先模块" || st.Breakpoints[0].Value != "KillCourse 学习总览核对。" {
		t.Fatalf("首条 = %+v", st.Breakpoints[0])
	}
	if len(st.Phases) != 2 || st.Phases[0] != "KillCourse 速成" {
		t.Fatalf("Phases = %+v", st.Phases)
	}
}

func TestUpdateNextReplace(t *testing.T) {
	out, err := UpdateNext(stateFixture, "开始登录模块走读", "2026-10-24")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- 下一步：开始登录模块走读") {
		t.Fatalf("未替换: %s", out)
	}
	if strings.Contains(out, "用户核对总览后设置入口") {
		t.Fatal("旧内容应被替换")
	}
	if !strings.Contains(out, "更新时间：2026-10-24") {
		t.Fatal("更新时间应刷新")
	}
	if strings.Count(out, "## 当前断点") != 1 {
		t.Fatal("结构不应被破坏")
	}
}

func TestUpdateNextAppend(t *testing.T) {
	in := strings.ReplaceAll(stateFixture, "- 下一步：用户核对总览后设置入口。\n", "")
	out, err := UpdateNext(in, "新建断点", "2026-10-24")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- 下一步：新建断点") {
		t.Fatalf("未追加: %s", out)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ -v`
Expected: 编译错误 `undefined: ParseState`

- [ ] **Step 3: 实现 plan.go 并注册**

`internal/command/plan.go`：

```go
package command

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

type KV struct{ Key, Value string }

type State struct {
	UpdateLine  string // 更新时间：开头那一行
	Breakpoints []KV   // 「## 当前断点」小节内的 - 键：值 行
	Phases      []string // 「## 已确认的顺序与节点」小节内的编号项
}

var (
	updateTimeRe = regexp.MustCompile(`^更新时间：(\d{4}-\d{2}-\d{2})`)
	phaseNumRe   = regexp.MustCompile(`^\d+[.、]\s*(.+)$`)
)

// ParseState 按 design.md 7.3 的约定解析学习状态文件
func ParseState(content string) State {
	lines := strings.Split(content, "\n")
	var st State
	section := ""
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") {
			section = strings.TrimSpace(strings.TrimPrefix(t, "## "))
			continue
		}
		switch section {
		case "当前断点":
			if strings.HasPrefix(t, "- ") && strings.Contains(t, "：") {
				body := strings.TrimPrefix(t, "- ")
				i := strings.Index(body, "：")
				st.Breakpoints = append(st.Breakpoints, KV{Key: body[:i], Value: body[i+len("："):]})
			}
		case "已确认的顺序与节点":
			if m := phaseNumRe.FindStringSubmatch(t); m != nil {
				st.Phases = append(st.Phases, strings.TrimSpace(m[1]))
			}
		}
		if strings.HasPrefix(t, "更新时间：") {
			st.UpdateLine = t
		}
	}
	return st
}

// UpdateNext 更新「下一步」键（无则追加到「## 当前断点」末尾），并把更新时间刷新为 today（YYYY-MM-DD）
func UpdateNext(content, next, today string) (string, error) {
	if next == "" {
		return "", errors.New("断点描述不能为空")
	}
	lines := strings.Split(content, "\n")
	inSection := false
	lastKV := -1
	nextLine := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") {
			if inSection {
				break // 离开当前断点小节
			}
			inSection = t == "## 当前断点"
			continue
		}
		if !inSection || t == "" {
			continue
		}
		if strings.HasPrefix(t, "- 下一步") {
			nextLine = i
		}
		if strings.HasPrefix(t, "- ") {
			lastKV = i
		}
	}
	newLine := "- 下一步：" + next
	if nextLine >= 0 {
		lines[nextLine] = newLine
	} else if lastKV >= 0 {
		lines = append(lines[:lastKV+1], append([]string{newLine}, lines[lastKV+1:]...)...)
	} else {
		return "", errors.New("未找到「## 当前断点」小节")
	}
	for i, l := range lines {
		if updateTimeRe.MatchString(strings.TrimSpace(l)) {
			lines[i] = updateTimeRe.ReplaceAllString(l, "更新时间："+today)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func renderPlan(out *strings.Builder, st State) {
	out.WriteString("# 学习状态速览\n\n")
	for _, kv := range st.Breakpoints {
		fmt.Fprintf(out, "- **%s**：%s\n", kv.Key, kv.Value)
	}
	if len(st.Phases) > 0 {
		out.WriteString("\n## 阶段清单\n\n")
		for i, p := range st.Phases {
			fmt.Fprintf(out, "%d. %s\n", i+1, p)
		}
	}
}

func newPlan() Command {
	return Command{
		Name:  "plan",
		Short: "查看/更新学习计划与断点",
		Run: func(ctx *Ctx) error {
			switch {
			case len(ctx.Args) >= 1 && ctx.Args[0] == "next":
				next := strings.Join(ctx.Args[1:], " ")
				if next == "" {
					var err error
					next, err = inputLine(ctx, "新的下一步断点")
					if err != nil {
						return err
					}
					next = strings.TrimSpace(next)
					if next == "" {
						return errors.New("已取消：断点描述为空")
					}
				}
				data, err := os.ReadFile(ctx.WS.StateFile())
				if err != nil {
					return err
				}
				today := timeNowCST().Format("2006-01-02")
				updated, err := UpdateNext(string(data), next, today)
				if err != nil {
					return err
				}
				if err := workspace.WriteFileAtomic(ctx.WS.StateFile(), []byte(updated)); err != nil {
					return err
				}
				fmt.Fprintf(ctx.Out, "已更新 %s\n", ui.Path("Study/plan/学习状态.md"))
				return nil
			case len(ctx.Args) >= 1 && ctx.Args[0] == "edit":
				entries, err := os.ReadDir(ctx.WS.PlanDir())
				if err != nil {
					return err
				}
				var mds []string
				for _, e := range entries {
					if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".md") {
						mds = append(mds, e.Name())
					}
				}
				if len(mds) == 0 {
					return errors.New("plan 目录下没有 .md 文件")
				}
				var choice string
				opts := make([]huhOption, 0, len(mds))
				for _, m := range mds {
					opts = append(opts, option(m, m))
				}
				if err := selectFrom(ctx, "选择要编辑的计划文件（DOCX 请用 Office 打开）", opts, &choice); err != nil {
					return err
				}
				return editOpen(ctx, filepath.Join(ctx.WS.PlanDir(), choice))
			default:
				data, err := os.ReadFile(ctx.WS.StateFile())
				if err != nil {
					return err
				}
				var out strings.Builder
				renderPlan(&out, ParseState(string(data)))
				fmt.Fprint(ctx.Out, ui.RenderMD(out.String()))
				return nil
			}
		},
	}
}

func timeNowCST() (t time.Time) { return time.Now().In(workspace.CST) }
```

注意补 `"time"` 导入。Build 更新：

```go
func Build() []Command {
	return []Command{newLogs(), newNotes(), newPlan(), newHelp(), newExit()}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /plan 速览/断点更新/编辑"
```

---

### Task 11: /search 命令 + 未知命令建议

**Files:**
- Create: `Project/gs/internal/command/search.go`
- Test: `Project/gs/internal/command/search_cmd_test.go`
- Create: `Project/gs/internal/repl/suggest.go`
- Test: `Project/gs/internal/repl/suggest_test.go`
- Modify: `Project/gs/internal/command/help.go`（Build 追加 `newSearch()`）

- [ ] **Step 1: 写失败测试**

`internal/command/search_cmd_test.go`：

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestRunSearchCmd(t *testing.T) {
	ws := wsForTest(t)
	if err := os.WriteFile(filepath.Join(ws.LogsDir(), "261024.md"), []byte("- 学习了 context 超时\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, Args: []string{"context"}}
	if err := newSearch().Run(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Study/logs/261024.md") {
		t.Fatalf("缺文件路径: %q", out.String())
	}
}

func TestRunSearchNoArgs(t *testing.T) {
	ws := wsForTest(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out}
	if err := newSearch().Run(ctx); err == nil {
		t.Fatal("无参数应报用法错误")
	}
}
```

`internal/repl/suggest_test.go`：

```go
package repl

import "testing"

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"logs", "logs", 0},
		{"logs", "log", 1},
		{"notes", "note", 1},
		{"plan", "plans", 1},
		{"abc", "xyz", 3},
		{"", "ab", 2},
	}
	for _, c := range cases {
		if got := Levenshtein(c.a, c.b); got != c.want {
			t.Fatalf("Levenshtein(%q,%q) = %d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	names := []string{"logs", "log", "notes", "plan", "search", "help", "exit"}
	if got := suggest(names, "logz"); got != "logs" {
		t.Fatalf("got %q", got)
	}
	if got := suggest(names, " совершенно无关的长命令"); got != "" {
		t.Fatalf("远距命令不应建议, got %q", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ ./internal/repl/ -v`
Expected: 编译错误 `undefined: newSearch` 与 `undefined: Levenshtein`

- [ ] **Step 3: 实现**

`internal/command/search.go`：

```go
package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bugzi/gs/internal/search"
	"github.com/bugzi/gs/internal/ui"
)

func newSearch() Command {
	return Command{
		Name:  "search",
		Alias: []string{"s"},
		Short: "全局搜索 md 与 go 源码",
		Run: func(ctx *Ctx) error {
			query := strings.Join(ctx.Args, " ")
			if query == "" {
				return errors.New("用法: /search <关键词> [更多词…]（空格分隔为 AND）")
			}
			results, words, err := search.Search(ctx.WS.Root, ctx.Cfg, query)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintf(ctx.Out, "未找到含 %q 的内容\n", query)
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(ctx.Out, "\n%s\n", ui.Path(r.Path))
				for _, h := range r.Hits {
					loc := ""
					if h.Unit != "" {
						loc = h.Unit + " "
					}
					fmt.Fprintf(ctx.Out, "%s\n", ui.Dim(fmt.Sprintf("  %s:%d", loc, h.Line)))
					for _, l := range h.Context {
						fmt.Fprintf(ctx.Out, "  | %s\n", ui.HighlightWords(strings.TrimRight(l, "\r"), words))
					}
				}
			}
			fmt.Fprintf(ctx.Out, "\n共 %d 个文件命中\n", len(results))
			return nil
		},
	}
}
```

`internal/repl/suggest.go`：

```go
package repl

// Levenshtein 按 rune 计算编辑距离
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// suggest 返回编辑距离 ≤2 的最近命令名；没有则返回空串
func suggest(names []string, input string) string {
	best := ""
	bestD := 3
	for _, n := range names {
		d := Levenshtein(n, input)
		if d < bestD {
			bestD = d
			best = n
		}
	}
	if bestD <= 2 {
		return best
	}
	return ""
}
```

Build 更新：

```go
func Build() []Command {
	return []Command{newLogs(), newNotes(), newPlan(), newSearch(), newHelp(), newExit()}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/command/ ./internal/repl/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /search 命令与未知命令编辑距离建议"
```

---

### Task 12: REPL 装配 + main + 真机验收

**Files:**
- Create: `Project/gs/internal/repl/input.go`
- Create: `Project/gs/internal/repl/repl.go`
- Modify: `Project/gs/cmd/gs/main.go`

- [ ] **Step 1: 实现 input.go**

`internal/repl/input.go`：

```go
package repl

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/ergochat/readline"
)

// Reader 是 REPL 的行输入抽象；spike 结论决定默认实现
type Reader interface {
	Read() (string, error) // Ctrl-D 返回 io.EOF
	Close() error
}

// NewReader GS_PLAIN_INPUT=1 时用 plain（spike 发现中文光标错位时把默认改为 plain）
func NewReader(prompt, historyFile string, completer readline.PrefixCompleterIface) (Reader, error) {
	if os.Getenv("GS_PLAIN_INPUT") == "1" {
		return newPlainReader(prompt), nil
	}
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, err
	}
	return &readlineReader{rl: rl}, nil
}

type readlineReader struct{ rl *readline.Instance }

func (r *readlineReader) Read() (string, error) {
	s, err := r.rl.Readline()
	return s, err
}

func (r *readlineReader) Close() error { return r.rl.Close() }

// plainReader 降级实现：无历史无补全，但中文输入绝对正确
type plainReader struct {
	br     *bufio.Reader
	prompt string
}

func newPlainReader(prompt string) *plainReader {
	return &plainReader{br: bufio.NewReader(os.Stdin), prompt: prompt}
}

func (r *plainReader) Read() (string, error) {
	os.Stdout.WriteString(r.prompt)
	line, err := r.br.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (r *plainReader) Close() error { return nil }
```

若构造时 `NewEx` 等符号与本版本不符，按 `go doc github.com/ergochat/readline` 实际 API 微调字段名，逻辑不变。

- [ ] **Step 2: 实现 repl.go**

`internal/repl/repl.go`：

```go
package repl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ergochat/readline"

	"github.com/bugzi/gs/internal/command"
	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

type REPL struct {
	ws   *workspace.Workspace
	cfg  *config.Config
	cmds []command.Command
	out  io.Writer
}

func New(ws *workspace.Workspace, cfg *config.Config, out io.Writer) *REPL {
	return &REPL{ws: ws, cfg: cfg, cmds: command.Build(), out: out}
}

func (r *REPL) completer() readline.PrefixCompleterIface {
	items := make([]readline.PrefixCompleterInterface, 0, len(r.cmds))
	for _, c := range r.cmds {
		items = append(items, readline.PcItem("/"+c.Name))
	}
	return readline.NewPrefixCompleter(items...)
}

func (r *REPL) banner() {
	fmt.Fprintf(r.out, "gs · Go 学习工作台 · %s\n", r.ws.Root)
	data, _ := os.ReadFile(r.ws.StateFile())
	st := command.ParseState(string(data))
	for _, kv := range st.Breakpoints {
		switch kv.Key {
		case "当前优先模块":
			fmt.Fprintf(r.out, "当前断点: %s\n", kv.Value)
		case "目标":
			fmt.Fprintf(r.out, "目标: %s\n", kv.Value)
		}
	}
	if dates, err := r.ws.ListLogs(); err == nil && len(dates) > 0 {
		last := dates[0]
		today := time.Now().In(workspace.CST)
		tag := ""
		if last.Format("20060102") == today.Format("20060102") {
			tag = " (今天)"
		}
		fmt.Fprintf(r.out, "上次学习: %s%s · 连续 %d 天\n", last.Format("01-02"), tag, r.ws.Streak())
	}
	fmt.Fprintln(r.out, ui.Dim("输入 /help 查看命令，/exit 退出"))
}

// Run 阻塞运行 REPL；返回时用户已退出
func (r *REPL) Run() error {
	r.banner()
	home, _ := os.UserHomeDir()
	_ = os.MkdirAll(filepath.Join(home, ".gs"), 0o755)
	reader, err := NewReader("gs> ", filepath.Join(home, ".gs", "history"), r.completer())
	if err != nil {
		return err
	}
	defer reader.Close()
	for {
		line, err := reader.Read()
		if err != nil {
			fmt.Fprintln(r.out) // Ctrl-D 换行
			return nil
		}
		if r.exec(line) {
			return nil
		}
	}
}

// exec 返回 true 表示要退出
func (r *REPL) exec(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if !strings.HasPrefix(line, "/") {
		fmt.Fprintln(r.out, ui.Dim("命令以 / 开头，输入 /help 查看命令列表"))
		return false
	}
	fields := strings.Fields(strings.TrimPrefix(line, "/"))
	name := strings.ToLower(fields[0])
	args := fields[1:]
	cmd := command.Find(r.cmds, name)
	if cmd == nil {
		var names []string
		for _, c := range r.cmds {
			names = append(names, c.Name)
			names = append(names, c.Alias...)
		}
		msg := fmt.Sprintf("未知命令 /%s", name)
		if s := suggest(names, name); s != "" {
			msg += fmt.Sprintf("，是否想输入 /%s？", s)
		}
		fmt.Fprintln(r.out, ui.Err(msg))
		return false
	}
	if cmd.Name == "exit" {
		fmt.Fprintln(r.out, "再见，学习顺利。")
		return true
	}
	ctx := &command.Ctx{WS: r.ws, Cfg: r.cfg, Out: r.out, Args: args, Cmds: r.cmds}
	if err := cmd.Run(ctx); err != nil {
		fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
	}
	return false
}
```

- [ ] **Step 3: 写 main.go**

`cmd/gs/main.go` 整体替换：

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/repl"
	"github.com/bugzi/gs/internal/workspace"
)

func main() {
	root := flag.String("root", ".", "学习工作区根目录（默认当前目录及其上级）")
	flag.Parse()
	ws, err := workspace.Detect(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg, err := config.Load(ws.Root, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败:", err)
		os.Exit(1)
	}
	if err := repl.New(ws, cfg, os.Stdout).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: 全量测试与静态检查**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全部通过，无 vet 告警

- [ ] **Step 5: 真机验收（必须暂停，请用户逐项操作）**

Run: `go run ./cmd/gs`（在 D:/Code/Go 下）

用户逐项验证：
1. 横幅显示当前断点/上次学习/连续天数
2. `/help` 显示说明；`/h` 别名生效
3. `/logs` 显示最近日志；`/logs 登录` 关键词过滤；`/logs 1023` 按日期
4. `/log add "gs 冒烟测试"` 后 `/logs` 可见新记录；超 40 字出现确认
5. `/notes` 列表选择并渲染；`/notes new 测试主题`；`/notes edit 测试主题` 打开编辑器；验证后手动删除该测试笔记
6. `/plan` 显示速览；`/plan next "gs M1 验收"` 后 `/plan` 反映新断点（验证后请用户自行改回）
7. `/search context 超时` 有结果且命中词高亮
8. 输入 `/logz` 提示"是否想输入 /logs？"
9. `/exit` 退出；直接 Ctrl-D 也退出
10. 中文输入光标正常（若异常：设置 `GS_PLAIN_INPUT=1` 后复测，并在后续任务中把默认改为 plain）

- [ ] **Step 6: 修复验收中发现的问题**

每个问题：先写复现测试（能自动化的部分），再修，再 `go test ./...` 通过。

- [ ] **Step 7: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: REPL 装配与 M1 完成"
```

---

## 收尾清单

- [ ] `go build ./... && go vet ./... && go test ./...` 全绿
- [ ] `go install ./cmd/gs` 后任意目录可运行 `gs`（GOBIN 在 PATH 中）
- [ ] M1 遗留明确化：/show /project /AI /review 属 M2/M3，/help 中已标注
- [ ] 用户完成最终 git 提交

## 自查记录（写计划时已核对）

- 覆盖 design.md M1 范围：骨架✓ spike✓ /help✓ /logs(读/add/edit)✓ /notes(查/new/edit)✓ /plan(查/edit/next)✓ /search✓ 配置✓ 编辑器调起✓ 未知命令建议✓ 原子写✓
- M2/M3（index/show/project/ai/diffx）明确不在本计划内
- 签名一致性：`edit.Open(path, *config.Config)`、`search.Search(root,*Config,query)([]FileResult,[]string,error)`、`command.Build() []Command`、`workspace.WriteFileAtomic(path, []byte)`、`input.NewReader(prompt, historyFile, completer)` 在各任务间一致
