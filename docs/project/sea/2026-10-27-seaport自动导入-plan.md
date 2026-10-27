# seaport 自动导入 · 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> 创建于 2026-10-27 · 依据 [2026-10-27-seaport自动导入-design.md](./2026-10-27-seaport自动导入-design.md)

**Goal:** gs 启动时自动检测 `<工作区根>/seaport/` 下的 `.md`/`.txt` 文件,后台用 AI 按内容分类迁入 `Study/sea/` 主题目录,失败兜底进 `sea/未分类/`。

**Architecture:** 新纯逻辑包 `internal/seaport`(扫描/分类/迁移/结果渲染,`out` 注入可测试),`internal/repl` 启动时扫描并起后台 goroutine,经 channel 非阻塞收割结果打印。分类复用 `internal/ai.Client`(`~/.gs/config.toml` 的 `[ai]` 段)。

**Tech Stack:** Go 标准库 + 现有 `internal/ai`、`internal/workspace`、`internal/ui`;测试用标准 `testing` + `t.TempDir()`。

**纪律说明:** 本项目所有 git 操作由开发者本人执行(AI 禁止 git)。每个 Task 结束即为建议的提交点,提交由开发者完成。测试与构建命令统一在 `D:\Code\Go\Project\gs` 下执行;落地标准 = `go build ./...` 与 `go test ./...` 全绿。

---

### Task 1: 包骨架 — Dir / Ensure / Scan

**Files:**
- Create: `internal/seaport/seaport.go`
- Create: `internal/seaport/seaport_test.go`

- [ ] **Step 1: 写失败测试**

```go
package seaport

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/workspace"
)

// portFixture 最小工作区:Study/logs+plan(workspace 判定)+ sea 两个主题目录 + seaport
func portFixture(t *testing.T) *workspace.Workspace {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{
		"Study/logs", "Study/plan",
		"Study/sea/Go语言基础", "Study/sea/GPT",
		"seaport",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &workspace.Workspace{Root: root}
}

func TestDir(t *testing.T) {
	ws := portFixture(t)
	want := filepath.Join(ws.Root, "seaport")
	if got := Dir(ws); got != want {
		t.Fatalf("Dir = %s, want %s", got, want)
	}
}

func TestEnsureCreatesDir(t *testing.T) {
	ws := portFixture(t)
	if err := os.RemoveAll(Dir(ws)); err != nil {
		t.Fatal(err)
	}
	if err := Ensure(ws); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(Dir(ws)); err != nil || !info.IsDir() {
		t.Fatal("seaport 目录应被创建")
	}
}

func TestScanFiltersExtAndSubdirs(t *testing.T) {
	ws := portFixture(t)
	for _, name := range []string{"闭包.md", "笔记.TXT", "图.png", "小抄.txt"} {
		if err := os.WriteFile(filepath.Join(Dir(ws), name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(Dir(ws), "子目录"), 0o755); err != nil {
		t.Fatal(err)
	}
	files, skipped := Scan(ws)
	if len(files) != 3 {
		t.Fatalf("files = %v, want 3 个(md/txt 不分大小写)", files)
	}
	if len(skipped) != 1 || skipped[0] != "图.png" {
		t.Fatalf("skipped = %v, want [图.png]", skipped)
	}
}

func TestScanMissingDir(t *testing.T) {
	ws := portFixture(t)
	if err := os.RemoveAll(Dir(ws)); err != nil {
		t.Fatal(err)
	}
	files, skipped := Scan(ws)
	if files != nil || skipped != nil {
		t.Fatalf("目录缺失应返回空:%v %v", files, skipped)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/seaport/ -v`
Expected: 编译失败(`Dir`/`Ensure`/`Scan` 未定义)

- [ ] **Step 3: 最小实现**

```go
// Package seaport 海港:工作区根下 seaport/ 目录的自动导入——
// 扫描待导入文件,经 AI 按内容分类迁入 Study/sea/ 主题目录。
package seaport

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

// importableExts 可导入的文本扩展名(小写)
var importableExts = map[string]bool{".md": true, ".txt": true}

// Fallback 未分类兜底目录名(sea 下,始终是合法分类目标)
const Fallback = "未分类"

// Dir 海港目录 <root>/seaport
func Dir(ws *workspace.Workspace) string {
	return filepath.Join(ws.Root, "seaport")
}

// Ensure seaport 目录不存在则创建
func Ensure(ws *workspace.Workspace) error {
	return os.MkdirAll(Dir(ws), 0o755)
}

// Scan 列出 seaport 顶层可导入文件与非文本文件;目录缺失返回空
func Scan(ws *workspace.Workspace) (files, skipped []string) {
	entries, err := os.ReadDir(Dir(ws))
	if err != nil {
		return nil, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if importableExts[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, e.Name())
		} else {
			skipped = append(skipped, e.Name())
		}
	}
	return files, skipped
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/seaport/ -v`
Expected: 4 个测试全部 PASS

---

### Task 2: 候选目录与 LLM 响应解析

**Files:**
- Modify: `internal/seaport/seaport.go`(追加)
- Modify: `internal/seaport/seaport_test.go`(追加)

- [ ] **Step 1: 写失败测试**

```go
func TestCandidates(t *testing.T) {
	ws := portFixture(t)
	got := Candidates(ws)
	// sea 下两个主题目录 + 兜底"未分类"始终在列
	want := map[string]bool{"Go语言基础": true, "GPT": true, Fallback: true}
	if len(got) != len(want) {
		t.Fatalf("Candidates = %v, want %v", got, want)
	}
	for _, c := range got {
		if !want[c] {
			t.Fatalf("Candidates = %v, 多出 %q", got, c)
		}
	}
}

func TestParseClassify(t *testing.T) {
	m, err := parseClassify("```json\n{\"分类\":{\"闭包.md\":\"Go语言基础\"}}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if m["闭包.md"] != "Go语言基础" {
		t.Fatalf("m = %v", m)
	}
	if _, err := parseClassify("抱歉我无法回答"); err == nil {
		t.Fatal("无 JSON 应报错")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/seaport/ -v`
Expected: 编译失败(`Candidates`/`parseClassify` 未定义)

- [ ] **Step 3: 最小实现**

```go
// Candidates sea 一级子目录名作为分类候选;兜底"未分类"始终在列
func Candidates(ws *workspace.Workspace) []string {
	entries, err := os.ReadDir(ws.SeaDir())
	var dirs []string
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
	}
	for _, d := range dirs {
		if d == Fallback {
			return dirs
		}
	}
	return append(dirs, Fallback)
}

// parseClassify 从回复中提取 {"分类":{文件名:目录名}};容忍围栏与前后杂文本
func parseClassify(reply string) (map[string]string, error) {
	s := reply
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return nil, errors.New("回复中没有 JSON")
	}
	s = s[i : j+1]
	var wrapper struct {
		Fenlei map[string]string `json:"分类"`
	}
	if err := json.Unmarshal([]byte(s), &wrapper); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return wrapper.Fenlei, nil
}
```

import 区补充 `"encoding/json"`、`"errors"`、`"fmt"`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/seaport/ -v`
Expected: 全部 PASS

---

### Task 3: 分类与导入主流程 Import

**Files:**
- Modify: `internal/seaport/seaport.go`(追加)
- Modify: `internal/seaport/seaport_test.go`(追加)

- [ ] **Step 1: 写失败测试**

```go
type fakeChat struct {
	reply string
	err   error
}

func (f fakeChat) Chat(ctx context.Context, msgs []ai.Message) (string, error) {
	return f.reply, f.err
}

func putPort(t *testing.T, ws *workspace.Workspace, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(Dir(ws), name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportMovesByClass(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "闭包.md", "# 闭包\nGo 闭包捕获变量……")
	res := Import(context.Background(), ws, fakeChat{reply: `{"分类":{"闭包.md":"Go语言基础"}}`})
	if res.Note != "" {
		t.Fatalf("Note = %s", res.Note)
	}
	if len(res.Items) != 1 || res.Items[0].Err != "" {
		t.Fatalf("Items = %+v", res.Items)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包.md")); err != nil {
		t.Fatal("应迁移到 Go语言基础/", err)
	}
	if _, err := os.Stat(filepath.Join(Dir(ws), "闭包.md")); !os.IsNotExist(err) {
		t.Fatal("seaport 原件应消失")
	}
}

func TestImportUnknownDirFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, fakeChat{reply: `{"分类":{"a.md":"不存在的目录"}}`})
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal("目录不匹配应进未分类:", err)
	}
	if res.Items[0].Err != "" {
		t.Fatalf("未分类是正常迁移,不应有 Err: %+v", res.Items)
	}
}

func TestImportParseFailFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, fakeChat{reply: "我分不出来"})
	if res.Note == "" {
		t.Fatal("应有失败说明 Note")
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestImportChatErrFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, fakeChat{err: errors.New("网络断开")})
	if !strings.Contains(res.Note, "网络断开") {
		t.Fatalf("Note = %s", res.Note)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestImportNilClientFallsBack(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "内容")
	res := Import(context.Background(), ws, nil)
	if !strings.Contains(res.Note, "AI 未配置") {
		t.Fatalf("Note = %s", res.Note)
	}
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), Fallback, "a.md")); err != nil {
		t.Fatal(err)
	}
}

func TestImportNameConflictSuffix(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "闭包.md", "内容")
	if err := os.MkdirAll(filepath.Join(ws.SeaDir(), "Go语言基础"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包.md"), []byte("旧的"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Import(context.Background(), ws, fakeChat{reply: `{"分类":{"闭包.md":"Go语言基础"}}`})
	if _, err := os.Stat(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包-2.md")); err != nil {
		t.Fatal("重名应加后缀:", err)
	}
	old, _ := os.ReadFile(filepath.Join(ws.SeaDir(), "Go语言基础", "闭包.md"))
	if string(old) != "旧的" {
		t.Fatal("已存在文件不应被覆盖")
	}
	if res.Items[0].Err != "" {
		t.Fatalf("Items = %+v", res.Items)
	}
}

func TestImportCtxCancelledStops(t *testing.T) {
	ws := portFixture(t)
	putPort(t, ws, "a.md", "1")
	putPort(t, ws, "b.md", "2")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := Import(ctx, ws, fakeChat{reply: `{"分类":{"a.md":"GPT","b.md":"GPT"}}`})
	if _, err := os.Stat(filepath.Join(Dir(ws), "a.md")); err != nil {
		t.Fatal("取消后文件应留在 seaport:", err)
	}
	if res.Note == "" {
		t.Fatal("应有取消说明")
	}
}

func TestImportEmpty(t *testing.T) {
	ws := portFixture(t)
	res := Import(context.Background(), ws, nil)
	if len(res.Items) != 0 {
		t.Fatalf("Items = %+v", res.Items)
	}
}
```

import 区补充 `"context"`、`"strings"` 与 `"github.com/bugzi/gs/internal/ai"`。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/seaport/ -v`
Expected: 编译失败(`Import`/`Result`/`Item` 未定义)

- [ ] **Step 3: 实现 Import 与 classify**

```go
// ChatClient 分类所需的最小 AI 能力(*ai.Client 满足;测试注入 fake)
type ChatClient interface {
	Chat(ctx context.Context, msgs []ai.Message) (string, error)
}

// Item 单篇导入结果
type Item struct {
	File string // seaport 中的原文件名
	Dest string // 工作区相对目标路径(仅成功时非空)
	Err  string // 迁移失败原因(空 = 成功;分类兜底进未分类不算失败)
}

// Result 一次后台导入的完整结果
type Result struct {
	Items []Item
	Note  string // 整体说明:AI 失败原因/取消原因(空 = 分类正常)
}

const classifyPrompt = `你是笔记分类器。根据每篇文件的文件名与内容,从候选目录中为它选择最合适的一个目录名。
只能从候选目录中选择,不得创造新目录。只输出一个 JSON 对象,不要输出任何其他文字,格式:
{"分类":{"<文件名>":"<候选目录名>"}}`

// Import 完整导入:扫描 → 批量 AI 分类 → 逐篇迁移。
// cc 为 nil 或分类失败时全部进未分类;ctx 取消则停止,剩余文件留在 seaport。
func Import(ctx context.Context, ws *workspace.Workspace, cc ChatClient) Result {
	files, _ := Scan(ws)
	var res Result
	if len(files) == 0 {
		return res
	}
	dests, note := classify(ctx, ws, cc, files)
	if note != "" {
		res.Note = note
	}
	for _, name := range files {
		if ctx.Err() != nil {
			res.Note = strings.TrimSpace(res.Note + " 已取消,剩余文件留在 seaport")
			return res
		}
		dir := dests[name]
		if dir == "" {
			dir = Fallback
		}
		dst, err := moveFile(ws, name, dir)
		if err != nil {
			res.Items = append(res.Items, Item{File: name, Err: err.Error()})
			continue
		}
		res.Items = append(res.Items, Item{File: name, Dest: ws.Rel(dst)})
	}
	return res
}

// classify 批量一次调用;返回 文件名→目录名 映射与失败说明(成功时说明为空)
func classify(ctx context.Context, ws *workspace.Workspace, cc ChatClient, files []string) (map[string]string, string) {
	if cc == nil {
		return nil, "AI 未配置,全部放入" + Fallback
	}
	cands := Candidates(ws)
	var b strings.Builder
	fmt.Fprintf(&b, "候选目录:%s\n", strings.Join(cands, "、"))
	for _, name := range files {
		content := ""
		if data, err := os.ReadFile(filepath.Join(Dir(ws), name)); err == nil {
			r := []rune(string(data))
			if len(r) > 2000 {
				r = r[:2000]
			}
			content = string(r)
		}
		fmt.Fprintf(&b, "\n【%s】\n%s\n", name, content)
	}
	reply, err := cc.Chat(ctx, []ai.Message{
		{Role: "system", Content: classifyPrompt},
		{Role: "user", Content: b.String()},
	})
	if err != nil {
		return nil, "AI 分类失败(" + err.Error() + "),全部放入" + Fallback
	}
	m, err := parseClassify(reply)
	if err != nil {
		return nil, "AI 响应无法解析(" + err.Error() + "),全部放入" + Fallback
	}
	valid := map[string]bool{}
	for _, c := range cands {
		valid[strings.ToLower(c)] = true
	}
	out := make(map[string]string, len(files))
	for _, name := range files {
		d := strings.TrimSpace(m[name])
		if d == "" || !valid[strings.ToLower(d)] {
			d = Fallback
		}
		out[name] = d
	}
	return out, ""
}

// moveFile 迁移单篇到 sea/<dir>/;重名自动加 -2、-3 后缀,绝不覆盖
func moveFile(ws *workspace.Workspace, name, dir string) (string, error) {
	dstDir := filepath.Join(ws.SeaDir(), dir)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	dst := filepath.Join(dstDir, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(dst); err != nil {
			break // 不存在即可用;真正的错误由 rename 暴露
		}
		dst = filepath.Join(dstDir, fmt.Sprintf("%s-%d%s", base, i, ext))
	}
	return dst, os.Rename(filepath.Join(Dir(ws), name), dst)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/seaport/ -v`
Expected: 全部 PASS

---

### Task 4: 结果渲染 Print

**Files:**
- Modify: `internal/seaport/seaport.go`(追加)
- Modify: `internal/seaport/seaport_test.go`(追加)

- [ ] **Step 1: 写失败测试**

```go
func TestPrint(t *testing.T) {
	var out strings.Builder
	Print(&out, Result{
		Items: []Item{
			{File: "闭包.md", Dest: "Study/sea/Go语言基础/闭包.md"},
			{File: "随手.md", Dest: "Study/sea/未分类/随手.md"},
			{File: "锁.md", Err: "被占用"},
		},
		Note: "",
	})
	s := out.String()
	for _, want := range []string{"seaport 导入", "Go语言基础", "未分类", "锁.md 导入失败:被占用"} {
		if !strings.Contains(s, want) {
			t.Fatalf("输出缺 %q:\n%s", want, s)
		}
	}
}

func TestPrintNoteAndEmpty(t *testing.T) {
	var out strings.Builder
	Print(&out, Result{Note: "AI 未配置,全部放入未分类"})
	if !strings.Contains(out.String(), "AI 未配置") {
		t.Fatalf("输出 = %s", out.String())
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/seaport/ -v`
Expected: 编译失败(`Print` 未定义)

- [ ] **Step 3: 实现 Print**

```go
// Print 渲染导入结果(由 REPL 在提示符前调用)
func Print(out io.Writer, res Result) {
	fmt.Fprintln(out, "seaport 导入完成:")
	if res.Note != "" {
		fmt.Fprintf(out, "  %s\n", res.Note)
	}
	for _, it := range res.Items {
		switch {
		case it.Err != "":
			fmt.Fprintf(out, "  %s 导入失败:%s\n", it.File, it.Err)
		default:
			fmt.Fprintf(out, "  %s → %s\n", it.File, it.Dest)
		}
	}
}
```

import 区补充 `"io"`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/seaport/ -v`
Expected: 全部 PASS

---

### Task 5: REPL 集成 — 后台启动与结果收割

**Files:**
- Modify: `internal/repl/repl.go`(结构体字段、Run、新方法 portCheck/drainPort)
- Modify: `internal/repl/repl_test.go`(追加收割测试)

- [ ] **Step 1: 扩展 REPL 结构体与 import**

repl.go 顶部 import 追加 `"context"`、`"github.com/bugzi/gs/internal/ai"`、`"github.com/bugzi/gs/internal/seaport"`;结构体追加两个字段:

```go
type REPL struct {
	ws      *workspace.Workspace
	cfg     *config.Config
	cmds    []command.Command
	out     io.Writer
	mode    string             // "" 主模式;"ai" AI 子模式
	aiState *command.AIState   // AI 会话状态
	portRes chan seaport.Result // seaport 后台导入结果(有任务时非 nil)
	portCancel context.CancelFunc // 取消后台导入
}
```

- [ ] **Step 2: 写失败测试(repl_test.go 追加)**

```go
func TestDrainPort(t *testing.T) {
	var out strings.Builder
	r := &REPL{out: &out}
	r.drainPort() // 无任务:不应有任何输出
	if out.Len() != 0 {
		t.Fatalf("无任务不应输出: %s", out.String())
	}
	r.portRes = make(chan seaport.Result, 1)
	r.portRes <- seaport.Result{Items: []seaport.Item{
		{File: "闭包.md", Dest: "Study/sea/Go语言基础/闭包.md"},
	}}
	r.drainPort()
	if !strings.Contains(out.String(), "seaport 导入完成") {
		t.Fatalf("应打印导入结果: %s", out.String())
	}
	out.Reset()
	r.drainPort() // 已收割:不应重复打印
	if out.Len() != 0 {
		t.Fatalf("不应重复打印: %s", out.String())
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/repl/ -run TestDrainPort -v`
Expected: 编译失败(`portRes`/`drainPort` 未定义)

- [ ] **Step 4: Run 接线并实现 portCheck/drainPort**

`Run()` 开头(`r.banner()` 之后)接入检查与退出取消:

```go
func (r *REPL) Run() error {
	r.banner()
	defer func() {
		if r.portCancel != nil {
			r.portCancel()
		}
	}()
	r.portCheck()
	// ……以下原有代码不变
```

主循环 `reader.Read()` 成功返回后、`r.exec(line)` 之前插入收割:

```go
		r.drainPort()
		if r.exec(line) {
			return nil
		}
```

repl.go 末尾追加两个方法:

```go
// portCheck 启动时检查海港:有待导入文件则后台分类导入,不阻塞进 REPL
func (r *REPL) portCheck() {
	_ = seaport.Ensure(r.ws)
	files, skipped := seaport.Scan(r.ws)
	if len(skipped) > 0 {
		fmt.Fprintf(r.out, ui.Dim("seaport 有 %d 个非文本文件未处理(仅支持 .md/.txt)\n"), len(skipped))
	}
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(r.out, ui.Dim("seaport 有 %d 个文件,后台导入中…\n"), len(files))
	var cc seaport.ChatClient
	if c, err := ai.NewClient(r.cfg); err == nil {
		c.Temperature = 0 // 分类求稳定,不跟随会话温度
		cc = c
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.portCancel = cancel
	r.portRes = make(chan seaport.Result, 1)
	go func() {
		r.portRes <- seaport.Import(ctx, r.ws, cc)
	}()
}

// drainPort 非阻塞收割后台导入结果;有结果则在提示符前打印
func (r *REPL) drainPort() {
	if r.portRes == nil {
		return
	}
	select {
	case res := <-r.portRes:
		seaport.Print(r.out, res)
	default:
	}
}
```

- [ ] **Step 5: 构建与测试验证**

Run: `go build ./... && go test ./internal/repl/ -v`
Expected: 构建成功,含 TestDrainPort 在内的 repl 测试全部 PASS

- [ ] **Step 5: 手动冒烟(需真实 AI 配置)**

1. 向 `D:\Code\Go\seaport\` 放一篇含明确主题的 `.md`(如 Go channel 笔记)
2. 启动 gs → 应见「seaport 有 1 个文件,后台导入中…」
3. 随意执行一条命令(如 `/help`)→ 提示符前应见导入明细行
4. 确认文件已迁入 `Study/sea/` 对应主题目录,seaport 已清空
5. 再放一个 `.png` 启动 → 应见「非文本文件未处理」提示且 png 留存

---

### Task 6: 落地验证与归档

**Files:**
- 移动: `docs/project/sea/2026-10-27-seaport自动导入-design.md`、`docs/project/sea/2026-10-27-seaport自动导入-plan.md` → `docs/project/sea/archive/`

- [ ] **Step 1: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 全部通过(落地标准)

- [ ] **Step 2: 归档(仅 mv,不删文件)**

将 design 与 plan 两个文档移入 `docs/project/sea/archive/`(目录不存在则创建)。

- [ ] **Step 3: 提醒开发者 git 提交**

git 操作由开发者执行;建议提交点:Task 1-5 各一步,或本次功能整体一笔。
