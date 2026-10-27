# gs M2+M3（验收演示 + AI 增强）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 gs M2（/show 五节演示视图＋沉淀索引、/project 项目聚合）与 M3（diffx、OpenAI 兼容 AI 客户端、/AI 子模式含 diff 确认写入、/review 体检）。

**Architecture:** 沿用 M1 架构：`command.Build()` 注册命令、`Ctx` 注入依赖、huh 交互封装在 `command/prompt.go`、`workspace.WriteFileAtomic` 原子写、`ui` 渲染。新增 `index`（topics.toml）、`diffx`（行级 LCS）、`ai`（纯逻辑：客户端/动作提取/路径校验）三个无 UI 包；`Ctx` 增加 `AI *AIState` 与 `NextMode` 两个字段支撑 REPL 子模式。

**Tech Stack:** 沿用 M1 依赖，无新增第三方库。AI 调用用 `net/http` + `encoding/json`。

**设计文档:** `Project/gs/docs/design.md`（5.2 / 7.1 / 8 / 9 / 10 节）

---

## 全局约定（与 M1 相同）

- git 提交检查点由用户执行，代理不得运行 git 命令。
- `go` 命令工作目录 `D:/Code/Go/Project/gs`；测试统一 `go test ./...`。
- M1 已有符号速查：`workspace.Workspace{Root}`、`ws.Rel(path)`、`workspace.WriteFileAtomic(path, []byte)`、`ws.ListNotes()`/`command.ListNotes(ws)`、`command.MatchNotes`、`command.ParseState(string) State`（`State.Breakpoints []KV{Key,Value}`、`State.Phases`）、`search.Search(root,cfg,query) ([]FileResult, []string, error)`、`FileResult{Path string; Hits []Hit{Unit,Line,Context}}`、`ui.RenderMD/Path/Dim/Err/HighlightWords`、`command.Ctx{WS,Cfg,Out,Args,Cmds}`、`command.Command{Name,Alias,Short,Run}`、`huhOption`/`option(label,value)`/`selectFrom(ctx,title,opts,*string)`/`confirm(ctx,title)`/`inputLine(ctx,title)`、`editOpen(ctx,path)`、`wsForTest(t)`（command 测试夹具）。

## 文件结构总览

```
Project/gs/
├── internal/
│   ├── index/index.go            # Task 1: topics.toml 读写
│   ├── search/hot.go             # Task 2: 热门关键词
│   ├── workspace/project.go      # Task 2: 项目发现
│   ├── command/project.go        # Task 3: /project
│   ├── command/show.go           # Task 4+5: /show（视图构建 + pager）
│   ├── diffx/diffx.go            # Task 6: 行级 diff
│   ├── ai/client.go              # Task 7: OpenAI 兼容客户端
│   ├── ai/actions.go             # Task 7: 动作提取 + 路径校验
│   ├── command/ai.go             # Task 8: /AI 与 aiTurn/applyActions
│   ├── command/review.go         # Task 9: /review
│   ├── command/command.go        # Task 8: Ctx 增 AI/NextMode 字段
│   ├── repl/repl.go              # Task 8: ai 子模式路由
│   ├── repl/input.go             # Task 8: Reader 增 SetPrompt
│   └── command/help.go           # Task 3/10: Build 注册 + 帮助文本
└── docs/plans/2026-10-25-gs-m2-m3-implementation.md
```

---

### Task 1: index 包（topics.toml 沉淀索引）

**Files:**
- Create: `internal/index/index.go`
- Test: `internal/index/index_test.go`
- Modify: `internal/workspace/workspace.go`（加 IndexPath）

- [ ] **Step 1: 写失败测试**

`internal/index/index_test.go`：

```go
package index

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topics.toml")
	ix := &Index{Topics: []Topic{{
		Name:    "context",
		Created: "2026-10-25",
		Files:   []FileRef{{Path: "Study/notes/GPT/a.md", Hint: "重点讲超时"}},
		Logs:    []string{"261021"},
		Interview: "Study/Project/x/doc/interview-questions.md",
	}}}
	if err := ix.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tp := got.Find("Context") // 大小写不敏感
	if tp == nil || tp.Name != "context" || len(tp.Files) != 1 || tp.Files[0].Hint != "重点讲超时" {
		t.Fatalf("Find 结果 = %+v", tp)
	}
	if got.Find("nope") != nil {
		t.Fatal("未登记主题应返回 nil")
	}
}

func TestLoadMissing(t *testing.T) {
	ix, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil {
		t.Fatalf("缺文件应返回空索引而非错误: %v", err)
	}
	if len(ix.Topics) != 0 {
		t.Fatal("应为空索引")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/index/ -v`
Expected: 编译错误 `undefined: Index`

- [ ] **Step 3: 实现**

`internal/index/index.go`：

```go
package index

import (
	"os"
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

// Load 文件不存在时返回空索引
func Load(path string) (*Index, error) {
	ix := &Index{}
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
```

注意补 `"path/filepath"` 导入。

`internal/workspace/workspace.go` 末尾追加：

```go
// IndexPath 沉淀索引文件路径 <root>/.gs/topics.toml
func (w *Workspace) IndexPath() string {
	return filepath.Join(w.Root, ".gs", "topics.toml")
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/index/ ./internal/workspace/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: index 包——topics.toml 沉淀索引"
```

---

### Task 2: search 热词 + workspace 项目发现

**Files:**
- Create: `internal/search/hot.go`
- Test: `internal/search/hot_test.go`
- Create: `internal/workspace/project.go`
- Test: `internal/workspace/project_test.go`

- [ ] **Step 1: 写失败测试**

`internal/search/hot_test.go`：

```go
package search

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestHotKeywords(t *testing.T) {
	root := t.TempDir()
	cfg, _ := config.Load(root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/logs/261024.md", "- 学习了 context 超时\n- 继续走读 context 源码\n")
	w("Study/logs/261023.md", "- 复习 slice 与 map\n")
	w("Study/notes/GPT/Go并发与互斥锁.md", "# x\n")
	w("Study/notes/GPT/Go序列化与反序列化.md", "# x\n")

	kw := HotKeywords(root, cfg, 8)
	joined := ""
	for _, k := range kw {
		joined += k + " "
	}
	for _, want := range []string{"context", "并发", "互斥锁", "序列化"} {
		if !contains(joined, want) {
			t.Fatalf("热词缺 %q: %v", want, kw)
		}
	}
	if contains(joined, "Go") {
		t.Fatalf("Go 前缀不应作为热词: %v", kw)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && strings.Contains(s, sub)
}
```

注意补 `"strings"` 导入。

`internal/workspace/project_test.go`：

```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

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
	mk("Study/logs")                 // 无标志文件，不应算项目
	mk("Project/gs", "go.mod")       // gs 自身排除

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

func sliceContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/search/ ./internal/workspace/ -v`
Expected: `undefined: HotKeywords`、`ws.Projects undefined`

- [ ] **Step 3: 实现**

`internal/search/hot.go`：

```go
package search

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

var engWordRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]+`)

var hotStop = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "err": true, "nil": true,
	"func": true, "return": true, "package": true, "import": true, "test": true,
	"testing": true, "md": true, "go": true,
}

// HotKeywords 英文词取自日志正文与笔记名，中文主题取自笔记文件名按「与/和/的」切分
func HotKeywords(root string, cfg *config.Config, limit int) []string {
	count := map[string]int{}
	add := func(w string, n int) {
		w = strings.TrimSpace(w)
		if len([]rune(w)) < 2 {
			return
		}
		count[w] += n
	}
	// 日志正文：英文词
	entries, _ := os.ReadDir(filepath.Join(root, "Study", "logs"))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "Study", "logs", e.Name()))
		if err != nil {
			continue
		}
		for _, w := range engWordRe.FindAllString(strings.ToLower(string(data)), -1) {
			if !hotStop[w] {
				add(w, 1)
			}
		}
	}
	// 笔记文件名：英文词 + 中文切分
	var names []string
	_ = filepath.WalkDir(filepath.Join(root, "Study", "notes"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			names = append(names, strings.TrimSuffix(d.Name(), ".md"))
		}
		return nil
	})
	for _, n := range names {
		for _, w := range engWordRe.FindAllString(strings.ToLower(n), -1) {
			if !hotStop[w] {
				add(w, 2)
			}
		}
		cn := strings.TrimPrefix(n, "Go")
		for _, seg := range splitCJK(cn) {
			add(seg, 2)
		}
	}
	type kv struct {
		k string
		v int
	}
	var list []kv
	for k, v := range count {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]string, 0, len(list))
	for _, it := range list {
		out = append(out, it.k)
	}
	return out
}

// splitCJK 按「与/和/的」切中文词组，丢弃纯 ASCII 段
func splitCJK(s string) []string {
	var out []string
	for _, seg := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '与' || r == '和' || r == '的' || r == '/' || r == '_' || r == '-'
	}) {
		hasCJK := false
		for _, r := range seg {
			if r >= 0x4e00 && r <= 0x9fff {
				hasCJK = true
				break
			}
		}
		if hasCJK {
			out = append(out, seg)
		}
	}
	return out
}
```

注意：`workspace` 导入若未使用则不要导入（此实现未用到，删除该 import）。

`internal/workspace/project.go`：

```go
package workspace

import (
	"os"
	"path/filepath"
)

type Project struct {
	Name string // 目录名
	Path string // 绝对路径
	Rel  string // 相对 Root 的正斜杠路径
}

// Projects 发现学习/实践项目：Study/Project/*、Project/*、Study/* 一层子目录，
// 含 go.mod、README.md、README 或 .git 之一即认定为项目；gs 自身排除。
func (w *Workspace) Projects() []Project {
	patterns := []string{"Study/Project/*", "Project/*", "Study/*"}
	seen := map[string]bool{}
	var out []Project
	for _, pat := range patterns {
		matches, _ := filepath.Glob(filepath.Join(w.Root, filepath.FromSlash(pat)))
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || !info.IsDir() || seen[m] {
				continue
			}
			seen[m] = true
			name := filepath.Base(m)
			if name == "gs" || name == "gs.exe" {
				continue
			}
			if !isProjectDir(m) {
				continue
			}
			out = append(out, Project{Name: name, Path: m, Rel: w.Rel(m)})
		}
	}
	return out
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

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/search/ ./internal/workspace/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: 热词提取与项目自动发现"
```

---

### Task 3: /project 命令

**Files:**
- Create: `internal/command/project.go`
- Test: `internal/command/project_test.go`
- Modify: `internal/command/help.go`（Build 注册 + 帮助文本加一行）

- [ ] **Step 1: 写失败测试**

`internal/command/project_test.go`：

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectFixture(t *testing.T) *wsType {
	t.Helper()
	ws := wsForTest(t)
	p := filepath.Join(ws.Root, "Study", "Project", "demo")
	if err := os.MkdirAll(filepath.Join(p, "docs", "log"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "README.md"), []byte("# demo 项目\n\n一个演示项目\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "docs", "log", "260101.md"), []byte("- 跑通 main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}
```

注意：`wsForTest` 返回 `*workspace.Workspace`。上面 `wsType` 写法改为直接用返回值——测试函数签名写成 `func projectFixture(t *testing.T) *workspace.Workspace` 并导入 `github.com/bugzi/gs/internal/workspace`。

`internal/command/project_test.go`（完整版）：

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/workspace"
)

func projectFixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	ws := wsForTest(t)
	p := filepath.Join(ws.Root, "Study", "Project", "demo")
	if err := os.MkdirAll(filepath.Join(p, "docs", "log"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "go.mod"), []byte("module demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "README.md"), []byte("# demo 项目\n\n一个演示项目\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "docs", "log", "260101.md"), []byte("- 跑通 main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestProjectDetail(t *testing.T) {
	ws := projectFixture(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, Args: []string{"demo"}}
	if err := newProject().Run(ctx); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"demo 项目", "一个演示项目", "docs/log/260101.md"} {
		if !strings.Contains(s, want) {
			t.Fatalf("输出缺 %q: %s", want, s)
		}
	}
}

func TestProjectNoMatch(t *testing.T) {
	ws := projectFixture(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	var out strings.Builder
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, Args: []string{"不存在"}}
	if err := newProject().Run(ctx); err == nil {
		t.Fatal("无匹配项目应报错")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ -run TestProject -v`
Expected: 编译错误 `undefined: newProject`

- [ ] **Step 3: 实现**

`internal/command/project.go`：

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

// goStats 统计 .go 文件数与总行数
func goStats(projectPath string) (files, lines int) {
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

func renderProject(out *strings.Builder, ws *workspace.Workspace, p workspace.Project) {
	fmt.Fprintf(out, "# 项目 · %s\n\n`%s`\n\n", p.Name, p.Rel)
	if data, err := os.ReadFile(filepath.Join(p.Path, "README.md")); err == nil {
		out.WriteString("## 介绍\n\n" + truncateLines(string(data), 40) + "\n")
	}
	sections := [][2]string{
		{"学习日志", "docs/log"},
		{"走读笔记", "docs/study"},
		{"设计文档", "docs/project"},
	}
	for _, sec := range sections {
		docs := listDocs(p.Path, sec[1])
		fmt.Fprintf(out, "\n## %s\n\n", sec[0])
		if len(docs) == 0 {
			out.WriteString("（无）\n")
			continue
		}
		for _, d := range docs {
			fmt.Fprintf(out, "- %s\n", d)
		}
	}
	files, lines := goStats(p.Path)
	fmt.Fprintf(out, "\n## 代码概况\n\n%d 个 .go 文件，共 %d 行\n", files, lines)
	_ = ws
}

// truncateLines 超过 n 行时截断并提示
func truncateLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n") + "\n…（截断）"
}

func newProject() Command {
	return Command{
		Name:  "project",
		Short: "查看学习项目聚合信息",
		Run: func(ctx *Ctx) error {
			projects := ctx.WS.Projects()
			if len(projects) == 0 {
				return fmt.Errorf("未发现项目（需含 go.mod / README / .git 之一）")
			}
			if len(ctx.Args) >= 1 && ctx.Args[0] == "edit" {
				kw := strings.Join(ctx.Args[1:], " ")
				var p *workspace.Project
				for i := range projects {
					if strings.Contains(strings.ToLower(projects[i].Name), strings.ToLower(kw)) {
						p = &projects[i]
						break
					}
				}
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
			var p *workspace.Project
			if len(ctx.Args) >= 1 {
				kw := strings.ToLower(strings.Join(ctx.Args, " "))
				for i := range projects {
					if strings.Contains(strings.ToLower(projects[i].Name), kw) {
						p = &projects[i]
						break
					}
				}
				if p == nil {
					return fmt.Errorf("没有匹配 %q 的项目，可用 /project 浏览全部", strings.Join(ctx.Args, " "))
				}
			} else {
				var name string
				opts := make([]huhOption, 0, len(projects))
				for _, pr := range projects {
					opts = append(opts, option(pr.Name+"（"+pr.Rel+"）", pr.Name))
				}
				if err := selectFrom(ctx, "选择项目", opts, &name); err != nil {
					return err
				}
				for i := range projects {
					if projects[i].Name == name {
						p = &projects[i]
					}
				}
			}
			var out strings.Builder
			renderProject(&out, ctx.WS, *p)
			fmt.Fprint(ctx.Out, ui.RenderMD(out.String()))
			return nil
		},
	}
}
```

help.go：Build 改为 `return []Command{newLogs(), newNotes(), newPlan(), newSearch(), newProject(), newHelp(), newExit()}`，帮助文本在 `/search` 行后加：

```
  /project [名称]        查看项目聚合信息（介绍/学习日志/走读/设计/代码概况）
  /project edit <名称>   编辑项目文档
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/command/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /project 项目发现与聚合展示"
```

---

### Task 4: /show 视图构建（纯逻辑）

**Files:**
- Create: `internal/command/show.go`（本任务只写视图构建部分）
- Test: `internal/command/show_test.go`

- [ ] **Step 1: 写失败测试**

`internal/command/show_test.go`：

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/index"
)

func showFixture(t *testing.T) (*wsInner, *config.Config) {
	t.Helper()
	ws := wsForTest(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/notes/GPT/Go并发与互斥锁.md", "# 并发\n\ncontext 传递取消信号。\n")
	w("Study/Project/demo/use.go", "package main\n\nfunc Use() {\n\t_ = context.Background()\n}\n")
	w("Study/logs/261024.md", "- 学习了 context 超时\n")
	w("Study/Project/demo/doc/interview-questions.md", "# 面试题\n\n1. context 的取消机制？\n")
	return ws, cfg
}
```

注意：Go 里类型写作 `*workspace.Workspace`，测试签名直接用它（导入 workspace 包），上文 `wsInner` 只是占位说明。

`internal/command/show_test.go`（完整版）：

```go
package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/workspace"
)

func showFixture(t *testing.T) (*workspace.Workspace, *config.Config) {
	t.Helper()
	ws := wsForTest(t)
	cfg, _ := config.Load(ws.Root, t.TempDir())
	w := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	w("Study/notes/GPT/Go并发与互斥锁.md", "# 并发\n\ncontext 传递取消信号。\n")
	w("Study/Project/demo/use.go", "package main\n\nfunc Use() {\n\t_ = context.Background()\n}\n")
	w("Study/logs/261024.md", "- 学习了 context 超时\n")
	w("Study/Project/demo/doc/interview-questions.md", "# 面试题\n\n1. context 的取消机制？\n")
	return ws, cfg
}

func TestBuildShowViewScan(t *testing.T) {
	ws, cfg := showFixture(t)
	v := buildShowView(ws, cfg, index.New(), "context")
	if v.Topic != "context" || v.Indexed {
		t.Fatalf("v = %+v", v)
	}
	if len(v.Sections) != 5 {
		t.Fatalf("应有五节, got %d", len(v.Sections))
	}
	// 笔记节包含 notes 下的 md
	if !strings.Contains(v.Sections[1].Body, "Go并发与互斥锁.md") {
		t.Fatalf("笔记节: %s", v.Sections[1].Body)
	}
	// 代码节包含 go 文件
	if !strings.Contains(v.Sections[2].Body, "Study/Project/demo/use.go") {
		t.Fatalf("代码节: %s", v.Sections[2].Body)
	}
	// 日志节包含命中行
	if !strings.Contains(v.Sections[3].Body, "学习了 context 超时") {
		t.Fatalf("日志节: %s", v.Sections[3].Body)
	}
	// 面试节包含面试题文件
	if !strings.Contains(v.Sections[4].Body, "interview-questions.md") {
		t.Fatalf("面试节: %s", v.Sections[4].Body)
	}
}

func TestBuildShowViewIndexed(t *testing.T) {
	ws, cfg := showFixture(t)
	ix := &index.Index{Topics: []index.Topic{{
		Name:    "context",
		Created: "2026-10-25",
		Files: []index.FileRef{
			{Path: "Study/notes/GPT/Go并发与互斥锁.md", Hint: "重点讲取消"},
			{Path: "Study/Project/demo/use.go", Hint: ""},
		},
		Logs:      []string{"261024"},
		Interview: "Study/Project/demo/doc/interview-questions.md",
	}}}
	v := buildShowView(ws, cfg, ix, "context")
	if !v.Indexed {
		t.Fatal("应识别为索引视图")
	}
	if !strings.Contains(v.Sections[1].Body, "重点讲取消") {
		t.Fatalf("笔记节应含演示提示: %s", v.Sections[1].Body)
	}
}

func TestShowSave(t *testing.T) {
	ws, cfg := showFixture(t)
	ix := index.New()
	v := buildShowView(ws, cfg, ix, "context")
	if err := showSave(ws, v); err != nil {
		t.Fatal(err)
	}
	got, err := index.Load(ws.IndexPath())
	if err != nil {
		t.Fatal(err)
	}
	if got.Find("context") == nil {
		t.Fatal("沉淀后索引应有该主题")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/command/ -run TestShow -v`
Expected: 编译错误 `undefined: buildShowView`

- [ ] **Step 3: 实现视图构建**

`internal/command/show.go`：

```go
package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/config"
	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/search"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

type showFile struct {
	Path string // 工作区相对路径（正斜杠）
	Hint string
}

type showSection struct {
	Title string
	Body  string
	Files []showFile
}

type showView struct {
	Topic    string
	Indexed  bool
	Sections []*showSection
}

func indexNewAlias() *index.Index { return index.New() }

func secFiles(results []search.FileResult, keep func(string) bool) ([]showFile, string) {
	var files []showFile
	var b strings.Builder
	for _, r := range results {
		if !keep(r.Path) {
			continue
		}
		files = append(files, showFile{Path: r.Path})
		fmt.Fprintf(&b, "- %s（%d 处命中）\n", r.Path, len(r.Hits))
	}
	return files, b.String()
}

// buildShowView 索引优先，否则实时扫描
func buildShowView(ws *workspace.Workspace, cfg *config.Config, ix *index.Index, topic string) *showView {
	if tp := ix.Find(topic); tp != nil {
		return buildIndexedView(ws, tp)
	}
	return buildScanView(ws, cfg, topic)
}

func newView(topic string, indexed bool) *showView {
	v := &showView{Topic: topic, Indexed: indexed}
	for _, t := range []string{"概览", "笔记", "代码", "日志", "面试题"} {
		v.Sections = append(v.Sections, &showSection{Title: t})
	}
	return v
}

func buildScanView(ws *workspace.Workspace, cfg *config.Config, topic string) *showView {
	v := newView(topic, false)
	results, _, err := search.Search(ws.Root, cfg, topic)
	if err != nil {
		v.Sections[0].Body = "扫描失败: " + err.Error()
		return v
	}
	noteFiles, _ := secFiles(results, func(p string) bool {
		return strings.HasPrefix(p, "Study/notes/") && strings.HasSuffix(p, ".md")
	})
	codeFiles, _ := secFiles(results, func(p string) bool { return strings.HasSuffix(p, ".go") })
	logFiles, _ := secFiles(results, func(p string) bool {
		return strings.HasPrefix(p, "Study/logs/") && strings.HasSuffix(p, ".md")
	})
	var interviewFiles []showFile
	var interviewBody strings.Builder
	for _, r := range results {
		lp := strings.ToLower(r.Path)
		if !strings.HasSuffix(lp, ".md") ||
			(!strings.Contains(lp, "interview") && !strings.Contains(lp, "面试")) {
			continue
		}
		interviewFiles = append(interviewFiles, showFile{Path: r.Path})
		fmt.Fprintf(&interviewBody, "- %s（%d 处命中）\n", r.Path, len(r.Hits))
	}
	var noteBody strings.Builder
	for _, f := range noteFiles {
		data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(f.Path)))
		if err != nil {
			continue
		}
		fmt.Fprintf(&noteBody, "【%s】\n%s\n", f.Path, truncateLines(string(data), 60))
	}
	var logBody strings.Builder
	for _, f := range logFiles {
		data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(f.Path)))
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(l), strings.ToLower(topic)) {
				fmt.Fprintln(&logBody, l)
			}
		}
	}
	v.Sections[0].Body = fmt.Sprintf("主题 %q（实时扫描）：笔记 %d · 代码 %d · 日志 %d · 面试题 %d\n按 s 可沉淀为索引，下次直达。",
		topic, len(noteFiles), len(codeFiles), len(logFiles), len(interviewFiles))
	v.Sections[1].Body, v.Sections[1].Files = noteBody.String(), noteFiles
	v.Sections[2].Body, v.Sections[2].Files = codeBody(codeFiles), codeFiles
	v.Sections[3].Body, v.Sections[3].Files = logBody.String(), logFiles
	v.Sections[4].Body, v.Sections[4].Files = interviewBody.String(), interviewFiles
	return v
}

func codeBody(files []showFile) string {
	var b strings.Builder
	if len(files) == 0 {
		return ""
	}
	for _, f := range files {
		fmt.Fprintf(&b, "- %s\n", f.Path)
	}
	b.WriteString("\n（演示时用 o 打开对应文件，结合行号讲解）\n")
	return b.String()
}

func buildIndexedView(ws *workspace.Workspace, tp *index.Topic) *showView {
	v := newView(tp.Name, true)
	var overview strings.Builder
	fmt.Fprintf(&overview, "沉淀主题 %q（建立于 %s）\n\n演示提示：\n", tp.Name, tp.Created)
	var noteFiles, codeFiles []showFile
	var noteBody, codeBodyB strings.Builder
	for _, f := range tp.Files {
		item := showFile{Path: f.Path, Hint: f.Hint}
		if f.Hint != "" {
			fmt.Fprintf(&overview, "- %s：%s\n", f.Path, f.Hint)
		}
		if strings.HasSuffix(f.Path, ".go") {
			codeFiles = append(codeFiles, item)
			fmt.Fprintf(&codeBodyB, "- %s", f.Path)
			if f.Hint != "" {
				fmt.Fprintf(&codeBodyB, "（%s）", f.Hint)
			}
			codeBodyB.WriteString("\n")
		} else {
			noteFiles = append(noteFiles, item)
			data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(f.Path)))
			if err != nil {
				fmt.Fprintf(&noteBody, "【%s】（读取失败）\n", f.Path)
				continue
			}
			fmt.Fprintf(&noteBody, "【%s】%s\n%s\n", f.Path, hintSuffix(f.Hint), truncateLines(string(data), 60))
		}
	}
	var logBody strings.Builder
	for _, d := range tp.Logs {
		data, err := os.ReadFile(filepath.Join(ws.LogsDir(), d+".md"))
		if err != nil {
			continue
		}
		fmt.Fprintf(&logBody, "【%s】\n%s\n", d, string(data))
	}
	var interviewBody strings.Builder
	interviewFiles := []showFile{}
	if tp.Interview != "" {
		interviewFiles = append(interviewFiles, showFile{Path: tp.Interview})
		data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(tp.Interview)))
		if err == nil {
			matched := 0
			for _, l := range strings.Split(string(data), "\n") {
				if strings.Contains(strings.ToLower(l), strings.ToLower(tp.Name)) {
					fmt.Fprintln(&interviewBody, l)
					matched++
				}
			}
			if matched == 0 {
				interviewBody.WriteString(truncateLines(string(data), 30))
			}
		}
	}
	v.Sections[0].Body = overview.String()
	v.Sections[1].Body, v.Sections[1].Files = noteBody.String(), noteFiles
	v.Sections[2].Body, v.Sections[2].Files = codeBodyB.String(), codeFiles
	v.Sections[3].Body, v.Sections[3].Files = logBody.String(), []showFile{}
	v.Sections[4].Body, v.Sections[4].Files = interviewBody.String(), interviewFiles
	return v
}

func hintSuffix(hint string) string {
	if hint == "" {
		return ""
	}
	return "〔" + hint + "〕"
}

// showSave 把扫描视图沉淀为索引条目
func showSave(ws *workspace.Workspace, v *showView) error {
	ix, err := index.Load(ws.IndexPath())
	if err != nil {
		return err
	}
	if ix.Find(v.Topic) != nil {
		return fmt.Errorf("主题 %q 已在索引中（/show edit %s 可维护）", v.Topic, v.Topic)
	}
	tp := index.Topic{Name: v.Topic, Created: time.Now().In(workspace.CST).Format("2006-01-02")}
	seen := map[string]bool{}
	for _, sec := range v.Sections {
		for _, f := range sec.Files {
			if !seen[f.Path] {
				seen[f.Path] = true
				tp.Files = append(tp.Files, index.FileRef{Path: f.Path})
			}
		}
	}
	sort.Strings(tp.Logs)
	return ix.Save(ws.IndexPath())
}
```

注意：`index.New()` 尚不存在——Task 4 在 `internal/index/index.go` 追加：

```go
// New 返回空索引
func New() *Index { return &Index{} }
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/command/ -run TestShow -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /show 视图构建（扫描+索引）"
```

---

### Task 5: /show 交互 pager + 沉淀 + 主题选择

**Files:**
- Modify: `internal/command/show.go`（追加 pager 与命令壳）
- Modify: `internal/command/help.go`（Build 注册 newShow）

- [ ] **Step 1: 实现 pager 与命令壳（追加到 show.go 末尾）**

```go
// runShowPager 逐节浏览；o 打开当前节文件，s 沉淀，q 退出
func runShowPager(ctx *Ctx, v *showView) error {
	cur := 0
	reader := newPagerReader()
	for {
		sec := v.Sections[cur]
		body := sec.Body
		if strings.TrimSpace(body) == "" {
			body = "（无相关内容）"
		}
		fmt.Fprintf(ctx.Out, "\n═══ [%d/5] %s ═══\n%s\n", cur+1, sec.Title, body)
		fmt.Fprint(ctx.Out, ui.Dim("[n]下一节 [p]上一节 [o]打开文件 [s]沉淀为索引 [q]退出 > "))
		key, err := reader()
		if err != nil {
			return nil
		}
		switch key {
		case "n", "":
			if cur < 4 {
				cur++
			}
		case "p":
			if cur > 0 {
				cur--
			}
		case "o":
			if len(sec.Files) == 0 {
				fmt.Fprintln(ctx.Out, ui.Dim("本节没有可打开的文件"))
				continue
			}
			opts := make([]huhOption, 0, len(sec.Files))
			for _, f := range sec.Files {
				label := f.Path
				if f.Hint != "" {
					label += "〔" + f.Hint + "〕"
				}
				opts = append(opts, option(label, f.Path))
			}
			var pick string
			if err := selectFrom(ctx, "打开哪个文件", opts, &pick); err != nil {
				return err
			}
			if err := editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(pick))); err != nil {
				return err
			}
		case "s":
			if v.Indexed {
				fmt.Fprintln(ctx.Out, ui.Dim("该主题已在索引中，/show edit 可维护"))
				continue
			}
			if err := showSave(ctx.WS, v); err != nil {
				return err
			}
			fmt.Fprintf(ctx.Out, "已沉淀 %q 到 %s\n", v.Topic, ui.Path("Project/.gs/topics.toml 相对工作区 .gs/topics.toml"))
		case "q":
			return nil
		}
	}
}

// newPagerReader 返回逐行读取 os.Stdin 的函数（REPL 主循环此时不持有 stdin）
func newPagerReader() func() (string, error) {
	br := bufio.NewReader(os.Stdin)
	return func() (string, error) {
		line, err := br.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
}

func newShow() Command {
	return Command{
		Name:  "show",
		Short: "按主题生成验收演示视图",
		Run: func(ctx *Ctx) error {
			ix, err := index.Load(ctx.WS.IndexPath())
			if err != nil {
				return err
			}
			topic := strings.Join(ctx.Args, " ")
			if topic == "" {
				topic, err = pickShowTopic(ctx, ix)
				if err != nil {
					return err
				}
				if topic == "" {
					return nil
				}
			}
			if len(ctx.Args) >= 2 && ctx.Args[0] == "edit" {
				return editOpen(ctx, ctx.WS.IndexPath())
			}
			v := buildShowView(ctx.WS, ctx.Cfg, ix, topic)
			return runShowPager(ctx, v)
		},
	}
}

// pickShowTopic 无参进入：沉淀主题 + 扫描热词
func pickShowTopic(ctx *Ctx, ix *index.Index) (string, error) {
	var names []string
	for _, t := range ix.Topics {
		names = append(names, t.Name+"〔沉淀〕")
	}
	for _, k := range search.HotKeywords(ctx.WS.Root, ctx.Cfg, 10) {
		names = append(names, k)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("没有沉淀主题也没有热词，直接 /show <主题> 试试")
	}
	var pick string
	opts := make([]huhOption, 0, len(names))
	for _, n := range names {
		opts = append(opts, option(n, n))
	}
	if err := selectFrom(ctx, "选择演示主题", opts, &pick); err != nil {
		return "", err
	}
	// 去掉〔沉淀〕装饰
	return strings.TrimSuffix(pick, "〔沉淀〕"), nil
}
```

注意两点修正：
1. `showSave` 成功提示里的路径文案直接写 `.gs/topics.toml`：把那行改为
   `fmt.Fprintf(ctx.Out, "已沉淀 %q 到 %s\n", v.Topic, ui.Path(".gs/topics.toml"))`
2. `/show edit` 参数解析要在 topic 拼接之前（否则 "edit x" 会被当主题）。把 newShow 的 Run 开头改为：

```go
		Run: func(ctx *Ctx) error {
			ix, err := index.Load(ctx.WS.IndexPath())
			if err != nil {
				return err
			}
			if len(ctx.Args) >= 1 && ctx.Args[0] == "edit" {
				return editOpen(ctx, ctx.WS.IndexPath())
			}
			topic := strings.Join(ctx.Args, " ")
			if topic == "" {
				topic, err = pickShowTopic(ctx, ix)
				if err != nil {
					return err
				}
				if topic == "" {
					return nil
				}
			}
			v := buildShowView(ctx.WS, ctx.Cfg, ix, topic)
			return runShowPager(ctx, v)
		},
```

并补导入：`"bufio"`、`"os"`（show.go 已有 os 则跳过）。

help.go：Build 加入 `newShow()`（放在 newProject 前面），帮助文本加：

```
  /show [主题]           验收演示：概览/笔记/代码/日志/面试题 五节视图
  /show edit             维护沉淀索引 topics.toml
```

- [ ] **Step 2: 运行全量测试**

Run: `go build ./... && go test ./...`
Expected: 全部通过

- [ ] **Step 3: 手工冒烟（非交互部分）**

Run: `printf '/show edit\n' | go run ./cmd/gs --root D:/Code/Go`（会拉起编辑器，Ctrl+C 关闭即可）
Expected: 无 panic；编辑器打开 topics.toml（不存在则为空文件）

- [ ] **Step 4: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /show 交互 pager 与沉淀"
```

---

### Task 6: diffx 包

**Files:**
- Create: `internal/diffx/diffx.go`
- Test: `internal/diffx/diffx_test.go`

- [ ] **Step 1: 写失败测试**

`internal/diffx/diffx_test.go`：

```go
package diffx

import (
	"strings"
	"testing"
)

func TestOps(t *testing.T) {
	ops := Ops([]string{"a", "b", "c"}, []string{"a", "B", "c"})
	var got strings.Builder
	for _, op := range ops {
		switch op.Type {
		case OpEqual:
			got.WriteRune('=')
		case OpAdd:
			got.WriteRune('+')
		case OpDel:
			got.WriteRune('-')
		}
	}
	if got.String() != "=-+=" {
		t.Fatalf("ops 序列错误: %s", got.String())
	}
}

func TestOpsEmpty(t *testing.T) {
	ops := Ops(nil, []string{"x", "y"})
	if len(ops) != 2 || ops[0].Type != OpAdd {
		t.Fatalf("ops = %+v", ops)
	}
}

func TestRender(t *testing.T) {
	out := Render("a\nb\nc\nd\ne\nf\ng\nh\n", "a\nb\nX\nd\ne\nf\ng\nh\n")
	if !strings.Contains(out, "- b") && !strings.Contains(out, "-b") {
		t.Fatalf("应包含删除行: %s", out)
	}
	if !strings.Contains(out, "X") {
		t.Fatalf("应包含新增行: %s", out)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/diffx/ -v`
Expected: 编译错误 `undefined: Ops`

- [ ] **Step 3: 实现**

`internal/diffx/diffx.go`：

```go
package diffx

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type OpType int

const (
	OpEqual OpType = iota
	OpAdd
	OpDel
)

type Op struct {
	Type OpType
	Text string
}

var (
	addStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // 绿
	delStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // 红
	dimStyle = lipgloss.NewStyle().Faint(true)
)

// Ops 行级 LCS diff；规模过大时退化为“前缀+后缀+整体替换”
func Ops(a, b []string) []Op {
	if len(a)*len(b) > 1_000_000 {
		return naiveOps(a, b)
	}
	// LCS 长度表
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max2(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []Op
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			ops = append(ops, Op{OpEqual, a[i]})
			i++
			j++
		} else if lcs[i+1][j] >= lcs[i][j+1] {
			ops = append(ops, Op{OpDel, a[i]})
			i++
		} else {
			ops = append(ops, Op{OpAdd, b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, Op{OpDel, a[i]})
	}
	for ; j < len(b); j++ {
		ops = append(ops, Op{OpAdd, b[j]})
	}
	return ops
}

func naiveOps(a, b []string) []Op {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	t := 0
	for t < len(a)-n && t < len(b)-n && a[len(a)-1-t] == b[len(b)-1-t] {
		t++
	}
	var ops []Op
	for _, l := range a[:n] {
		ops = append(ops, Op{OpEqual, l})
	}
	for _, l := range a[n : len(a)-t] {
		ops = append(ops, Op{OpDel, l})
	}
	for _, l := range b[n : len(b)-t] {
		ops = append(ops, Op{OpAdd, l})
	}
	for _, l := range a[len(a)-t:] {
		ops = append(ops, Op{OpEqual, l})
	}
	return ops
}

func max2(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Render 渲染为彩色 diff 文本；连续相同行超过 4 行压缩显示
func Render(oldContent, newContent string) string {
	a := splitLines(oldContent)
	b := splitLines(newContent)
	ops := Ops(a, b)
	var out strings.Builder
	eqRun := 0
	flushEq := func() {
		if eqRun > 4 {
			fmt.Fprintf(&out, "%s\n", dimStyle.Sprintf("  …（%d 行相同）", eqRun))
		} else {
			for k := 0; k < eqRun; k++ {
				fmt.Fprintf(&out, "%s\n", dimStyle.Sprintf("  %s", opsAt(ops, eqStart+)))
			}
		}
	}
	_ = flushEq
	// 简化实现：线性扫描输出，等值块压缩
	eqStart := 0
	for idx := 0; idx <= len(ops); idx++ {
		runEnded := idx == len(ops) || ops[idx].Type != OpEqual
		if runEnded {
			run := idx - eqStart
			if run > 4 {
				fmt.Fprintf(&out, "%s\n", dimStyle.Sprintf("  …（%d 行相同）", run))
			} else {
				for k := eqStart; k < idx; k++ {
					fmt.Fprintf(&out, "%s\n", dimStyle.Sprintf("  %s", ops[k].Text))
				}
			}
			if idx < len(ops) {
				switch ops[idx].Type {
				case OpDel:
					fmt.Fprintf(&out, "%s\n", delStyle.Sprintf("- %s", ops[idx].Text))
				case OpAdd:
					fmt.Fprintf(&out, "%s\n", addStyle.Sprintf("+ %s", ops[idx].Text))
				}
			}
			eqStart = idx + 1
		}
	}
	return out.String()
}

func opsAt(ops []Op, i int) Op { return ops[i] }

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
```

注意：上面 Render 初稿里残留了未用的 `flushEq`/`eqRun`/`opsAt`——实现时直接删掉这三者，只保留「线性扫描 + 等值块压缩」的最终逻辑（`eqStart` 变量与循环体）。清理后 Render 为：

```go
func Render(oldContent, newContent string) string {
	ops := Ops(splitLines(oldContent), splitLines(newContent))
	var out strings.Builder
	eqStart := 0
	for idx := 0; idx <= len(ops); idx++ {
		runEnded := idx == len(ops) || ops[idx].Type != OpEqual
		if !runEnded {
			continue
		}
		run := idx - eqStart
		if run > 4 {
			fmt.Fprintf(&out, "%s\n", dimStyle.Sprintf("  …（%d 行相同）", run))
		} else {
			for k := eqStart; k < idx; k++ {
				fmt.Fprintf(&out, "%s\n", dimStyle.Sprintf("  %s", ops[k].Text))
			}
		}
		if idx < len(ops) {
			switch ops[idx].Type {
			case OpDel:
				fmt.Fprintf(&out, "%s\n", delStyle.Sprintf("- %s", ops[idx].Text))
			case OpAdd:
				fmt.Fprintf(&out, "%s\n", addStyle.Sprintf("+ %s", ops[idx].Text))
			}
		}
		eqStart = idx + 1
	}
	return out.String()
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/diffx/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: diffx 行级 diff"
```

---

### Task 7: ai 包（客户端 + 动作提取 + 路径校验）

**Files:**
- Create: `internal/ai/client.go`
- Create: `internal/ai/actions.go`
- Test: `internal/ai/ai_test.go`

- [ ] **Step 1: 写失败测试**

`internal/ai/ai_test.go`：

```go
package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/config"
)

func TestChat(t *testing.T) {
	var gotAuth, gotPath, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotModel = r.PostFormValue("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你好"}}]}`))
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL + "/v1"
	cfg.AI.Model = "test-model"
	cfg.AI.APIKey = "sk-x"
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if out != "你好" {
		t.Fatalf("out = %q", out)
	}
	if gotAuth != "Bearer sk-x" || gotPath != "/v1/chat/completions" {
		t.Fatalf("auth=%q path=%q", gotAuth, gotPath)
	}
}

func TestChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	cfg := &config.Config{}
	cfg.AI.BaseURL = srv.URL
	cfg.AI.Model = "m"
	c, _ := NewClient(cfg)
	if _, err := c.Chat(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("应返回含状态码的错误: %v", err)
	}
}

func TestNewClientRequiresConfig(t *testing.T) {
	if _, err := NewClient(&config.Config{}); err == nil {
		t.Fatal("未配置应报错")
	}
}

func TestExtractActions(t *testing.T) {
	reply := "这是说明文字。\n\n```json\n{\"actions\":[{\"file\":\"Study/notes/a.md\",\"action\":\"edit\",\"content\":\"# 新\"}]}\n```\n完毕"
	prose, actions := SplitReply(reply)
	if len(actions) != 1 || actions[0].File != "Study/notes/a.md" || actions[0].Action != "edit" {
		t.Fatalf("actions = %+v", actions)
	}
	if strings.Contains(prose, "gs-actions") || strings.Contains(prose, "\"actions\"") {
		t.Fatalf("prose 不应含动作块: %s", prose)
	}
	if _, actions2 := SplitReply("普通回答，无动作"); len(actions2) != 0 {
		t.Fatal("无动作块应返回空")
	}
}

func TestSafePath(t *testing.T) {
	root := t.TempDir()
	p, err := SafePath(root, "Study/notes/a.md")
	if err != nil || p != filepath.Join(root, "Study", "notes", "a.md") {
		t.Fatalf("p=%s err=%v", p, err)
	}
	for _, bad := range []string{"../evil.md", "/etc/passwd", "a.exe", "Study/x.go"} {
		if bad == "Study/x.go" {
			if _, err := SafePath(root, bad); err != nil {
				t.Fatalf(".go 应放行: %v", err)
			}
			continue
		}
		if _, err := SafePath(root, bad); err == nil {
			t.Fatalf("%q 应被拒绝", bad)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/ai/ -v`
Expected: 编译错误 `undefined: NewClient`

- [ ] **Step 3: 实现 client.go**

`internal/ai/client.go`：

```go
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	HTTP        *http.Client
}

// NewClient base_url 与 model 必填；api_key 可空（部分本地端点不需要）
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.AI.BaseURL == "" || cfg.AI.Model == "" {
		return nil, fmt.Errorf("AI 未配置：在 ~/.gs/config.toml 写入 [ai] 段的 base_url 与 model（api_key 可用环境变量 GS_API_KEY）")
	}
	return &Client{
		BaseURL:     strings.TrimRight(cfg.AI.BaseURL, "/"),
		APIKey:      cfg.AI.APIKey,
		Model:       cfg.AI.Model,
		Temperature: cfg.AI.Temperature,
		HTTP:        &http.Client{Timeout: 120 * time.Second},
	}, nil
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`

	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用 OpenAI 兼容 chat/completions（非流式）
func (c *Client) Chat(ctx context.Context, msgs []Message) (string, error) {
	body, err := json.Marshal(chatReq{Model: c.Model, Messages: msgs, Temperature: c.Temperature})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var cr chatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("响应解析失败(HTTP %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if cr.Error != nil && cr.Error.Message != "" {
			msg = cr.Error.Message
		}
		return "", fmt.Errorf("AI 请求失败(HTTP %d): %s", resp.StatusCode, msg)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("AI 响应缺少 choices")
	}
	return cr.Choices[0].Message.Content, nil
}
```

`internal/ai/actions.go`：

```go
package ai

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
)

type Action struct {
	File    string `json:"file"`
	Action  string `json:"action"`
	Content string `json:"content"`
}

var fenceRe = regexp.MustCompile("(?s)```[a-zA-Z-]*\\s*\n(.*?)```")

var allowedExt = map[string]bool{".md": true, ".go": true, ".toml": true}

// SplitReply 从回复中提取 gs-actions 动作块，返回去除动作块后的正文与动作列表
func SplitReply(reply string) (string, []Action) {
	prose := reply
	var actions []Action
	for _, m := range fenceRe.FindAllStringSubmatch(reply, -1) {
		var holder struct {
			Actions []Action `json:"actions"`
		}
		if json.Unmarshal([]byte(m[1]), &holder) != nil || len(holder.Actions) == 0 {
			continue
		}
		valid := true
		for _, a := range holder.Actions {
			if (a.Action != "edit" && a.Action != "create") || a.File == "" {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		actions = append(actions, holder.Actions...)
		prose = strings.ReplaceAll(prose, m[0], "")
	}
	return strings.TrimSpace(prose), actions
}

// SafePath 校验动作目标必须位于 root 内且扩展名白名单
func SafePath(root, file string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(file)))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", filepath.ErrBadP
	}
	if !allowedExt[strings.ToLower(filepath.Ext(clean))] {
		return "", filepath.ErrBadExtension
	}
	return filepath.Join(root, clean), nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/ai/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: ai 包——OpenAI 兼容客户端与动作协议"
```

---

### Task 8: /AI 命令 + REPL 子模式

**Files:**
- Create: `internal/command/ai.go`
- Modify: `internal/command/command.go`（Ctx 增字段）
- Modify: `internal/repl/repl.go`（子模式路由）
- Modify: `internal/repl/input.go`（Reader 增 SetPrompt）
- Modify: `internal/command/help.go`（Build 注册 + 帮助文本）

- [ ] **Step 1: 扩展 Ctx**

`internal/command/command.go` 的 Ctx 结构体改为：

```go
// Ctx 每个命令执行时拿到的一切
type Ctx struct {
	WS   *workspace.Workspace
	Cfg  *config.Config
	Out  io.Writer
	Args []string  // 命令名之后的参数
	Cmds []Command // 由 repl 注入，供 /help 列表用

	AI       *AIState // AI 对话状态，repl 注入并在命令间共享
	NextMode string   // 命令可请求切换 REPL 模式（"ai" 进入子模式，"main" 返回）
}
```

- [ ] **Step 2: 实现 ai.go**

`internal/command/ai.go`：

```go
package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/diffx"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// AIState 一个 REPL 会话内共享的 AI 上下文
type AIState struct {
	Client *ai.Client
	Hist   []ai.Message
	System string
}

var atFileRe = regexp.MustCompile(`@([^\s]+)`)

const aiSystemTpl = `你是 gs 学习工作台的 AI 助手，帮助用户整理 Go 学习笔记、维护学习计划、解答 Go 问题。

工作区根目录：%s
当前断点：
%s
阶段清单：
%s

规则：
1. 需要修改或新建文件时，在回答末尾输出一个 json 代码块，格式：
{"actions":[{"file":"工作区相对路径(正斜杠)","action":"edit或create","content":"完整新内容"}]}
2. edit 的 content 必须是修改后的完整文件内容；只在用户明确要求改动文件时输出动作块。
3. 回答使用中文，简洁教学式。`

// ensureAIState 惰性创建并注入 ctx.AI；未配置时返回友好错误
func ensureAIState(ctx *Ctx) (*AIState, error) {
	if ctx.AI != nil {
		return ctx.AI, nil
	}
	c, err := ai.NewClient(ctx.Cfg)
	if err != nil {
		return nil, err
	}
	st := &AIState{Client: c}
	ctx.AI = st
	return st, nil
}

// ensureSystem 补全系统提示（需要 ws 信息，首次对话时构建）
func (st *AIState) ensureSystem(ws *workspace.Workspace) {
	if st.System != "" {
		return
	}
	data, _ := os.ReadFile(ws.StateFile())
	state := ParseState(string(data))
	var bp, ph strings.Builder
	for _, kv := range state.Breakpoints {
		fmt.Fprintf(&bp, "- %s：%s\n", kv.Key, kv.Value)
	}
	for i, p := range state.Phases {
		fmt.Fprintf(&ph, "%d. %s\n", i+1, p)
	}
	st.System = fmt.Sprintf(aiSystemTpl, ws.Root, bp.String(), ph.String())
}

// expandAtFile 把 @路径 替换为文件内容引用
func expandAtFile(ws *workspace.Workspace, input string) string {
	return atFileRe.ReplaceAllStringFunc(input, func(m string) string {
		rel := strings.TrimPrefix(m, "@")
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		data, err := os.ReadFile(p)
		if err != nil {
			return m + "（文件读取失败）"
		}
		content := string(data)
		if len(content) > 8000 {
			content = content[:8000] + "\n…（截断）"
		}
		return fmt.Sprintf("\n--- 文件 %s ---\n%s\n--- 引用结束 ---\n", rel, content)
	})
}

// aiTurn 一轮对话：@文件 展开 → 调用 → 展示 → 动作确认
func aiTurn(ctx *Ctx, st *AIState, input string) error {
	st.ensureSystem(ctx.WS)
	user := expandAtFile(ctx.WS, input)
	msgs := append([]ai.Message{{Role: "system", Content: st.System}}, st.Hist...)
	msgs = append(msgs, ai.Message{Role: "user", Content: user})
	fmt.Fprintln(ctx.Out, ui.Dim("（AI 思考中…）"))
	reply, err := st.Client.Chat(context.Background(), msgs)
	if err != nil {
		return err
	}
	st.Hist = append(st.Hist,
		ai.Message{Role: "user", Content: user},
		ai.Message{Role: "assistant", Content: reply})
	if len(st.Hist) > 16 {
		st.Hist = st.Hist[len(st.Hist)-16:]
	}
	prose, actions := ai.SplitReply(reply)
	if prose != "" {
		fmt.Fprintln(ctx.Out, ui.RenderMD(prose))
	}
	if len(actions) > 0 {
		return applyActions(ctx, actions)
	}
	return nil
}

// applyActions 逐条 diff 预览并确认写入
func applyActions(ctx *Ctx, actions []ai.Action) error {
	fmt.Fprintf(ctx.Out, ui.Dim("AI 提出了 %d 个文件修改\n"), len(actions))
	applyAll := false
	for i, a := range actions {
		path, err := ai.SafePath(ctx.WS.Root, a.File)
		if err != nil {
			fmt.Fprintf(ctx.Out, ui.Err("跳过越界文件 %s"), a.File)
			continue
		}
		var choice string
		if applyAll {
			choice = "应用"
		} else {
			opts := []huhOption{
				option("应用", "应用"),
				option("跳过", "跳过"),
				option("应用剩余全部", "全部"),
				option("放弃剩余", "放弃"),
			}
			label := fmt.Sprintf("[%d/%d] %s（%s）", i+1, len(actions), a.File, a.Action)
			if err := selectFrom(ctx, label, opts, &choice); err != nil {
				return err
			}
		}
		switch choice {
		case "跳过":
			continue
		case "放弃":
			fmt.Fprintln(ctx.Out, "已放弃剩余修改")
			return nil
		case "全部":
			applyAll = true
		}
		if err := applyOne(ctx, path, a); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx *Ctx, path string, a ai.Action) error {
	current, err := os.ReadFile(path)
	switch {
	case a.Action == "create":
		if err == nil {
			return fmt.Errorf("%s 已存在，不能 create", ctx.WS.Rel(path))
		}
		fmt.Fprintf(ctx.Out, "将创建 %s：\n%s\n", ui.Path(ctx.WS.Rel(path)),
			truncateLines(strings.ReplaceAll(a.Content, "\r\n", "\n"), 15))
	case err == nil:
		fmt.Fprintf(ctx.Out, "\n%s\n%s", ui.Path(ctx.WS.Rel(path)), diffx.Render(string(current), a.Content))
	case os.IsNotExist(err):
		return fmt.Errorf("%s 不存在，不能 edit", ctx.WS.Rel(path))
	default:
		return err
	}
	if err := workspace.WriteFileAtomic(path, []byte(a.Content)); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, ui.Dim("已写入 %s\n"), ctx.WS.Rel(path))
	return nil
}

func newAI() Command {
	return Command{
		Name:  "ai",
		Short: "进入 AI 对话（/AI 问题 单发；/AI 进入子模式；@文件 引用）",
		Run: func(ctx *Ctx) error {
			st, err := ensureAIState(ctx)
			if err != nil {
				return err
			}
			if len(ctx.Args) == 0 {
				ctx.NextMode = "ai"
				fmt.Fprintln(ctx.Out, ui.Dim("已进入 AI 模式，/back 返回，@路径 可引用文件"))
				return nil
			}
			return aiTurn(ctx, st, strings.Join(ctx.Args, " "))
		},
	}
}
```

- [ ] **Step 3: 修改 repl（Reader.SetPrompt + 子模式路由）**

`internal/repl/input.go`：Reader 接口与两个实现加 SetPrompt：

```go
type Reader interface {
	Read() (string, error) // Ctrl-D 返回 io.EOF
	Close() error
	SetPrompt(p string)
}
```

readlineReader 增加：

```go
func (r *readlineReader) SetPrompt(p string) { r.rl.SetPrompt(p) }
```

plainReader 增加（并把 prompt 字段改回可变使用）：

```go
func (r *plainReader) SetPrompt(p string) { r.prompt = p }
```

`internal/repl/repl.go` 修改：REPL 结构体加两个字段：

```go
type REPL struct {
	ws      *workspace.Workspace
	cfg     *config.Config
	cmds    []command.Command
	out     io.Writer
	mode    string               // "" 主模式；"ai" AI 子模式
	aiState *command.AIState     // AI 会话状态
}
```

Run 循环改为（只列改动部分，prompt 随模式变化；exec 前置 ai 分支）：

```go
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
		reader.SetPrompt(map[bool]string{true: "ai> ", false: "gs> "}[r.mode == "ai"])
		line, err := reader.Read()
		if err != nil {
			fmt.Fprintln(r.out)
			return nil
		}
		if r.exec(line) {
			return nil
		}
	}
}
```

exec 开头（trim 之后、主路由之前）插入：

```go
	if r.mode == "ai" {
		return r.execAI(line, name, args)
	}
	if ctxNeedsExit...（原有逻辑不变）
```

实际完整做法——把 exec 改为：

```go
// exec 返回 true 表示要退出
func (r *REPL) exec(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if !strings.HasPrefix(line, "/") {
		if r.mode == "ai" {
			r.aiChat(line)
		} else {
			fmt.Fprintln(r.out, ui.Dim("命令以 / 开头，输入 /help 查看命令列表"))
		}
		return false
	}
	fields := strings.Fields(strings.TrimPrefix(line, "/"))
	name := strings.ToLower(fields[0])
	args := fields[1:]
	if r.mode == "ai" {
		switch name {
		case "back":
			r.mode = ""
			fmt.Fprintln(r.out, ui.Dim("已返回主模式"))
			return false
		case "exit", "q", "quit":
			fmt.Fprintln(r.out, "再见，学习顺利。")
			return true
		default:
			fmt.Fprintln(r.out, ui.Dim("AI 模式：直接输入问题，/back 返回主模式"))
			return false
		}
	}
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
	ctx := r.newCtx(args)
	if err := cmd.Run(ctx); err != nil {
		fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
	}
	if ctx.AI != nil {
		r.aiState = ctx.AI
	}
	if ctx.NextMode != "" {
		r.mode = ctx.NextMode
	}
	return false
}

func (r *REPL) newCtx(args []string) *command.Ctx {
	return &command.Ctx{WS: r.ws, Cfg: r.cfg, Out: r.out, Args: args, Cmds: r.cmds, AI: r.aiState}
}

func (r *REPL) aiChat(input string) {
	if r.aiState == nil {
		fmt.Fprintln(r.out, ui.Err("AI 未初始化，请先执行 /AI"))
		return
	}
	ctx := r.newCtx(nil)
	if err := command.AITurn(ctx, r.aiState, input); err != nil {
		fmt.Fprintln(r.out, ui.Err("错误: "+err.Error()))
	}
}
```

因此 `command` 包导出两处改名：`aiTurn` → `AITurn`（ai.go 中函数名与调用处同步）、`AIState` 已是导出名。ai.go 里 `newAI` 的 Run 中 `aiTurn` 调用同步改为 `AITurn`。

help.go：Build 加入 `newAI()`；帮助文本把 `/AI （M3 提供）` 行替换为：

```
  /AI [问题]             AI 对话：带参单发提问，无参进入 AI 模式（/back 返回，@路径 引用文件）
```

- [ ] **Step 4: 全量测试**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全部通过

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /AI 子模式与 diff 确认写入"
```

---

### Task 9: /review 体检

**Files:**
- Create: `internal/command/review.go`
- Modify: `internal/command/help.go`（Build 注册 + 帮助文本）

- [ ] **Step 1: 实现 review.go**

`internal/command/review.go`：

```go
package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

const reviewPromptTpl = `请基于以下学习材料，给出三类建议，用 Markdown 分节输出：

## 笔记整理建议
（结构优化、过期内容、缺失主题）

## 合并建议
（主题重叠、应互相引用的笔记）

## 学习规划建议
（结合当前断点与实习面试目标，给出接下来 1-2 周的重点）

只对确有必要修改的文件输出 gs-actions 动作块（如合并笔记）；建议本身用文字表达即可。

=== 学习状态 ===
%s

=== 阶段清单 ===
%s

=== 笔记 ===
%s

=== 近 30 天日志 ===
%s`

// gatherReviewMaterials 收集体检材料，各部分限量防止超长
func gatherReviewMaterials(ws *workspace.Workspace) (state, phases, notes, logs string) {
	data, err := os.ReadFile(ws.StateFile())
	if err == nil {
		st := ParseState(string(data))
		var b1, b2 strings.Builder
		for _, kv := range st.Breakpoints {
			fmt.Fprintf(&b1, "- %s：%s\n", kv.Key, kv.Value)
		}
		for i, p := range st.Phases {
			fmt.Fprintf(&b2, "%d. %s\n", i+1, p)
		}
		state, phases = b1.String(), b2.String()
	}
	var nb strings.Builder
	if names, err := ListNotes(ws); err == nil {
		for _, n := range names {
			raw, err := os.ReadFile(filepath.Join(ws.NotesDir(), filepath.FromSlash(n)))
			if err != nil {
				continue
			}
			fmt.Fprintf(&nb, "【%s】\n%s\n", n, truncateLines(string(raw), 60))
		}
	}
	notes = nb.String()
	var lb strings.Builder
	cutoff := time.Now().In(workspace.CST).AddDate(0, 0, -30)
	if dates, err := ws.ListLogs(); err == nil {
		for _, d := range dates {
			if d.Before(cutoff) {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(ws.LogsDir(), workspace.LogFileName(d)))
			if err != nil {
				continue
			}
			fmt.Fprintf(&lb, "【%s】%s\n", d.Format("01-02"), strings.TrimSpace(string(raw)))
		}
	}
	logs = lb.String()
	return state, phases, notes, logs
}

func newReview() Command {
	return Command{
		Name:  "review",
		Short: "AI 体检：笔记整理/合并/学习规划建议",
		Run: func(ctx *Ctx) error {
			st, err := ensureAIState(ctx)
			if err != nil {
				return err
			}
			state, phases, notes, logs := gatherReviewMaterials(ctx.WS)
			prompt := fmt.Sprintf(reviewPromptTpl, state, phases, notes, logs)
			fmt.Fprintln(ctx.Out, ui.Dim("（AI 正在体检学习资料…）"))
			reply, err := st.Client.Chat(ctx, []ai.Message{
				{Role: "system", Content: "你是严格的 Go 学习教练，只给可执行的建议。"},
				{Role: "user", Content: prompt},
			})
			if err != nil {
				return err
			}
			prose, actions := ai.SplitReply(reply)
			if prose != "" {
				fmt.Fprintln(ctx.Out, ui.RenderMD(prose))
			}
			if len(actions) > 0 {
				return applyActions(ctx, actions)
			}
			return nil
		},
	}
}
```

注意：`st.Client.Chat(ctx, ...)` 的 ctx 是 `*command.Ctx` 不是 context.Context——改为 `context.Background()` 并补 `"context"` 与 `"github.com/bugzi/gs/internal/ai"` 导入。

help.go：Build 加入 `newReview()`；帮助文本 `/search` 行前加：

```
  /review                AI 体检：给出笔记整理/合并/学习规划建议（需配置 [ai]）
```

- [ ] **Step 2: 全量测试**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 全部通过

- [ ] **Step 3: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: /review AI 体检"
```

---

### Task 10: 收尾——帮助文本核对 + 真机验收

**Files:**
- Modify: `internal/command/help.go`（核对全部命令行齐全）

- [ ] **Step 1: 核对 Build 与帮助文本**

Build 最终顺序：

```go
func Build() []Command {
	return []Command{newLogs(), newNotes(), newPlan(), newSearch(), newShow(), newProject(), newReview(), newAI(), newHelp(), newExit()}
}
```

帮助文本命令区最终包含：/logs、/logs edit、/log add、/notes、/notes new、/notes edit、/plan、/plan edit、/plan next、/search、/show、/show edit、/project、/project edit、/review、/AI、/help、/exit。

- [ ] **Step 2: 全量测试**

Run: `go build ./... && go vet ./... && go test ./... && go install ./cmd/gs`
Expected: 全部通过，gs.exe 更新到 go/bin

- [ ] **Step 3: 真机验收（必须暂停，请用户逐项操作）**

在 `D:\Code\Go` 下运行 `gs`：

1. `/show context` → 五节视图出现（实时扫描）；按 n/p 翻节；按 s 沉淀；再 `/show context` 显示"沉淀主题"且秒出
2. `/show` 无参 → 选择列表出现（含刚沉淀的 context〔沉淀〕与热词）
3. `/show edit` → 编辑器打开 topics.toml，给某个文件加一行 hint 后保存，`/show context` 笔记节出现〔hint〕
4. `/project` → 项目列表（HDU-KillCourse、feedsystem_video_go、fileforge、7days-golang 等，无 gs）
5. `/project HDU-KillCourse` → README 介绍 + 学习日志列表 + 代码概况
6. 配置 `~/.gs/config.toml`（[ai] base_url/model/api_key 或 GS_API_KEY）后：`/AI 用一句话解释 context` → 回答出现
7. `/AI` 进入子模式 → `@Study/notes/GPT/Go并发与互斥锁.md 帮我总结这篇笔记` → 回答引用了文件内容 → `/back` 返回
8. 让 AI 改一个文件（如 `/AI 在 Study/notes 下新建一篇关于 context 的复习笔记`）→ 出现 diff/预览 → 选择「应用」→ 文件确实生成；选择「放弃」→ 无文件变化
9. `/review` → 三类建议输出；如有动作块走确认流程
10. 未配置 [ai] 时 `/AI` 与 `/review` → 友好提示配置方法，其余命令不受影响

- [ ] **Step 4: 修复验收问题**

每个问题：先写复现测试（能自动化的部分），修复后 `go test ./...` 通过。

- [ ] **Step 5: 提交检查点（用户执行）**

```
git add Project/gs
git commit -m "gs: M2+M3 完成——show/project/AI/review"
```

---

## 自查记录（写计划时已核对）

- 覆盖 design.md：7.1 topics.toml（Task 1）✓ 8 热词（Task 2）✓ 10 /project（Task 3）✓ 5.2 /show 五节+沉淀+o/s/q 键（Task 4/5）✓ 9.1 OpenAI 兼容（Task 7）✓ 9.2 动作协议+diff 确认+越界拒绝（Task 6/7/8）✓ /AI 子模式+@文件（Task 8）✓ /review 三类建议（Task 9）✓
- M1 已有符号签名全部沿用，新增导出：`index.New/Load/Find/Save`、`index.Topic/FileRef/Index`、`search.HotKeywords`、`workspace.Projects/Project/IndexPath`、`diffx.Ops/Render/Op/OpType`、`ai.NewClient/Chat/Message/SplitReply/SafePath/Action`、`command.AIState/AITurn/ensureAIState`（小写不导出）、`Ctx.AI/Ctx.NextMode`、`repl.Reader.SetPrompt`
- 已知风险：Task 5 pager 与 plain 模式共用 stdin 的缓冲竞争（用户逐键操作，实际影响可忽略）；Task 8 huh 与 readline 的终端接管交接（M1 /notes 已验证可行）
