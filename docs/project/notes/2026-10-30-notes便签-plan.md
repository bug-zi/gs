# /notes 便签：临时记录缓存与 AI 整理归位 · 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **本仓库纪律（覆盖模板默认）**：AI 禁止执行任何 git 命令。计划中的「提交点」只给出建议命令，由开发者自行执行；所有文件写入走原子写（`workspace.WriteFileAtomic` 或 `os.WriteFile` 临时文件 + rename）。

**Goal:** 为 gs 增加 `/notes` 便签命令：一行文本随手缓存进 `Study/notes.md`，支持手动归位（move/del/edit）与 AI 整批整理归位（tidy，只读检索 + 整批方案 + 一次确认）。

**Architecture:** 纯逻辑包 `internal/notes` 负责文件解析/保存/方案校验与执行（不 import ui，可独立测试）；`internal/command/notes.go` 为交互壳（路由、选择器、confirm、agent 只读循环、diffx 预览）。tidy 复用 `agent.Agent` 只读工具（search/read_file/list_dir），最终回答约定为 JSON 方案。

**Tech Stack:** Go（module `github.com/bugzi/gs`）、现有 `agent`/`ai`/`diffx`/`ui`/`workspace` 包、测试用 httptest SSE 回放（模式见 `internal/command/ai_agent_test.go`）。

**设计文档:** [2026-10-30-notes便签-design.md](./2026-10-30-notes便签-design.md)
**落地判据:** `go build ./...` 与 `go test ./...` 全部通过

**对设计的两处收敛（已同步回设计文档）:**
1. 路径知识收敛在 `notes.Path(root)`，不加 `workspace.NotesFile()`，避免路径常量双源。
2. `ParsePlanJSON` 需要便签清单做序号越界校验，签名为 `ParsePlanJSON(s string, notes []Note) (Plan, error)`。

---

### Task 1: notes 包基础——Note/Path/Format/Load/Save/Add

**Files:**
- Create: `internal/notes/notes.go`
- Test: `internal/notes/notes_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/notes/notes_test.go
package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

func ts(min int) time.Time { return time.Date(2026, 10, 29, 14, min, 0, 0, workspace.CST) }

func TestLoadMissing(t *testing.T) {
	root := t.TempDir()
	got, err := Load(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("无文件应返回空切片: %v %v", got, err)
	}
}

func TestAddAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := Add(root, "slice、map、channel 可以看作引用类型", ts(30)); err != nil {
		t.Fatal(err)
	}
	if err := Add(root, "defer 在 return 之后执行", ts(5)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "# 便签\n\n> 临时记录，待整理进 river/spring\n") {
		t.Fatalf("骨架头部缺失:\n%s", s)
	}
	if !strings.Contains(s, "- [1] 261029 14:30 slice、map、channel 可以看作引用类型") ||
		!strings.Contains(s, "- [2] 261029 14:05 defer 在 return 之后执行") {
		t.Fatalf("便签行缺失或编号/时间不对:\n%s", s)
	}
	got, err := Load(root)
	if err != nil || len(got) != 2 || got[0].Text != "slice、map、channel 可以看作引用类型" {
		t.Fatalf("往返不一致: %+v %v", got, err)
	}
}

func TestLoadSkipsUnknownLines(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "Study", "notes.md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("# 便签\n\n随手写的不是便签的行\n- [1] 261029 14:30 真便签\n- [x] 坏行\n- [2] 26102925:00 坏时间\n"), 0o644)
	got, err := Load(root)
	if err != nil || len(got) != 1 || got[0].Text != "真便签" {
		t.Fatalf("应只解析出 1 条合法便签: %+v %v", got, err)
	}
}

func TestSaveRenumbers(t *testing.T) {
	root := t.TempDir()
	// 手工造乱序编号文件，Save 后应重排
	os.MkdirAll(filepath.Dir(Path(root)), 0o755)
	os.WriteFile(Path(root), []byte("# 便签\n\n> 临时记录，待整理进 river/spring\n\n- [7] 261029 14:30 甲\n- [3] 261029 15:00 乙\n"), 0o644)
	ns, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(root, ns); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(Path(root))
	s := string(data)
	if strings.Contains(s, "[7]") || !strings.Contains(s, "- [1] 261029 14:30 甲") || !strings.Contains(s, "- [2] 261029 15:00 乙") {
		t.Fatalf("应重编号为 1/2:\n%s", s)
	}
}

func TestSaveEmptyKeepsSkeleton(t *testing.T) {
	root := t.TempDir()
	if err := Save(root, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(Path(root))
	if !strings.HasPrefix(string(data), "# 便签\n") {
		t.Fatalf("空保存应保留骨架:\n%s", data)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/notes/ -v`
Expected: 编译失败（包不存在 / Load 未定义）

- [ ] **Step 3: 实现 notes.go**

```go
// internal/notes/notes.go
// Package notes 便签缓存：Study/notes.md 的解析、保存与 tidy 方案执行。纯逻辑，不 import ui。
package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

// Note 单条便签：录入时刻（东八区）+ 文本
type Note struct {
	Time time.Time
	Text string
}

const (
	fileRel        = "Study/notes.md"
	noteTimeLayout = "060102 15:04"
	headerLines    = "# 便签\n\n> 临时记录，待整理进 river/spring\n"
)

// noteLineRe 锚定完整便签行格式，避免正文里 - [ 开头的行误判
var noteLineRe = regexp.MustCompile(`^- \[\d+\] (\d{6} \d{2}:\d{2}) (.*)$`)

// Path notes.md 绝对路径（路径知识唯一来源）
func Path(root string) string { return filepath.Join(root, filepath.FromSlash(fileRel)) }

// Load 解析 notes.md；文件不存在返回空切片；不识别的行跳过
func Load(root string) ([]Note, error) {
	data, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Note
	for _, line := range strings.Split(string(data), "\n") {
		m := noteLineRe.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		tm, perr := time.ParseInLocation(noteTimeLayout, m[1], workspace.CST)
		if perr != nil {
			continue
		}
		out = append(out, Note{Time: tm, Text: m[2]})
	}
	return out, nil
}

// Format 渲染整份文件：骨架 + 重编号便签
func Format(ns []Note) string {
	lines := []string{"# 便签", "", "> 临时记录，待整理进 river/spring", ""}
	for i, n := range ns {
		lines = append(lines, fmt.Sprintf("- [%d] %s %s", i+1, n.Time.In(workspace.CST).Format(noteTimeLayout), n.Text))
	}
	return strings.Join(lines, "\n") + "\n"
}

// Save 重编号并原子写；目录不存在自动创建
func Save(root string, ns []Note) error {
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(Path(root), []byte(Format(ns)))
}

// Add 追加一条便签
func Add(root, text string, now time.Time) error {
	ns, err := Load(root)
	if err != nil {
		return err
	}
	ns = append(ns, Note{Time: now.In(workspace.CST), Text: text})
	return Save(root, ns)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/notes/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（建议命令，开发者执行）**

```bash
git add internal/notes/
git commit -m "feat(notes): 便签缓存纯逻辑包（解析/保存/追加）"
```

---

### Task 2: AppendNote——move 的原文追加核心

**Files:**
- Modify: `internal/notes/notes.go`
- Test: `internal/notes/notes_test.go`

- [ ] **Step 1: 写失败测试（追加到 notes_test.go）**

```go
func TestAppendNote(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "Study", "river", "01-基础", "03-切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片"), 0o644)

	if err := AppendNote(root, "Study/river/01-基础/03-切片.md", "扩容翻倍"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if got := string(data); got != "# 切片\n\n扩容翻倍\n" {
		t.Fatalf("追加格式不对: %q", got)
	}

	// 无尾换行的文件补换行后再追加；二次追加空行分隔
	if err := AppendNote(root, "Study/river/01-基础/03-切片.md", "三色标记"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(target)
	if got := string(data); got != "# 切片\n\n扩容翻倍\n\n三色标记\n" {
		t.Fatalf("二次追加格式不对: %q", got)
	}
}

func TestAppendNoteRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := AppendNote(root, "../outside.md", "x"); err == nil {
		t.Fatal("应拒绝 .. 穿越")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/notes/ -run TestAppendNote -v`
Expected: FAIL（AppendNote 未定义）

- [ ] **Step 3: 实现（追加到 notes.go）**

```go
// checkRelPath 校验 target 为工作区相对路径形态（拒绝绝对路径、rooted 路径与 .. 穿越）
func checkRelPath(target string) error {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(target)))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") || strings.HasPrefix(clean, "..") || clean == "." {
		return fmt.Errorf("必须是工作区相对路径: %s", target)
	}
	return nil
}

// AppendNote 把便签文本原文追加到目标文件末尾（补尾换行 + 空行分隔），原子写
func AppendNote(root, targetRel, text string) error {
	if err := checkRelPath(targetRel); err != nil {
		return err
	}
	abs := filepath.Join(root, filepath.FromSlash(strings.TrimSpace(targetRel)))
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += "\n" + strings.TrimRight(text, "\n") + "\n"
	return workspace.WriteFileAtomic(abs, []byte(s))
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/notes/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（建议命令，开发者执行）**

```bash
git add internal/notes/
git commit -m "feat(notes): AppendNote 原文追加核心与路径校验"
```

---

### Task 3: tidy 方案——Placement/Plan/ParsePlanJSON/Apply

**Files:**
- Create: `internal/notes/plan.go`
- Test: `internal/notes/plan_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/notes/plan_test.go
package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planFixture(t *testing.T) (string, []Note) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "Study", "river", "01-基础", "03-切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片\n\n原文\n"), 0o644)
	ns := []Note{{Time: ts(30), Text: "甲"}, {Time: ts(40), Text: "乙"}}
	if err := Save(root, ns); err != nil {
		t.Fatal(err)
	}
	return root, ns
}

func TestParsePlanJSONOK(t *testing.T) {
	_, ns := planFixture(t)
	plan, err := ParsePlanJSON(`{"placements":[{"note":2,"target":"Study/river/01-基础/03-切片.md","content":"# 切片\n\n原文\n\n乙\n"}]}`, ns)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Placements) != 1 || plan.Placements[0].NoteIdx != 1 || plan.Placements[0].Target != "Study/river/01-基础/03-切片.md" {
		t.Fatalf("解析结果不对: %+v", plan.Placements)
	}
	if strings.Contains(plan.Placements[0].NewContent, "\\n") {
		t.Fatalf("\\n 应已解码为真实换行: %q", plan.Placements[0].NewContent)
	}
}

func TestParsePlanJSONInvalid(t *testing.T) {
	_, ns := planFixture(t)
	cases := []struct {
		name, json string
	}{
		{"坏JSON", `{"placements":`},
		{"序号越界", `{"placements":[{"note":3,"target":"a.md","content":"x"}]}`},
		{"序号为零", `{"placements":[{"note":0,"target":"a.md","content":"x"}]}`},
		{"空content", `{"placements":[{"note":1,"target":"a.md","content":"  "}]}`},
		{"空target", `{"placements":[{"note":1,"target":"  ","content":"x"}]}`},
		{"路径穿越", `{"placements":[{"note":1,"target":"../x.md","content":"x"}]}`},
		{"绝对路径", `{"placements":[{"note":1,"target":"C:/x.md","content":"x"}]}`},
	}
	for _, c := range cases {
		if _, err := ParsePlanJSON(c.json, ns); err == nil {
			t.Fatalf("%s 应报错", c.name)
		}
	}
}

func TestApply(t *testing.T) {
	root, ns := planFixture(t)
	plan := Plan{Placements: []Placement{{
		NoteIdx:    0,
		Target:     "Study/river/01-基础/03-切片.md",
		NewContent: "# 切片\n\n原文\n\n融入甲\n",
	}}}
	if err := Apply(root, plan, ns); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "river", "01-基础", "03-切片.md"))
	if string(data) != "# 切片\n\n原文\n\n融入甲\n" {
		t.Fatalf("目标未更新: %q", data)
	}
	rest, err := Load(root)
	if err != nil || len(rest) != 1 || rest[0].Text != "乙" {
		t.Fatalf("已归位便签未移除: %+v %v", rest, err)
	}
}

func TestApplyTargetFailureKeepsNotes(t *testing.T) {
	root, ns := planFixture(t)
	// 目标目录不存在 → 写失败 → notes.md 必须原样
	plan := Plan{Placements: []Placement{{
		NoteIdx:    0,
		Target:     "Study/river/不存在/xx.md",
		NewContent: "x",
	}}}
	if err := Apply(root, plan, ns); err == nil {
		t.Fatal("目标写失败应报错")
	}
	rest, err := Load(root)
	if err != nil || len(rest) != 2 {
		t.Fatalf("失败时不得动 notes.md: %+v %v", rest, err)
	}
}

func TestApplySameTargetInOrder(t *testing.T) {
	root, ns := planFixture(t)
	// 同目标两个 placement，content 累计，按序应用后者覆盖
	plan := Plan{Placements: []Placement{
		{NoteIdx: 0, Target: "Study/river/01-基础/03-切片.md", NewContent: "# 切片\n\n甲\n"},
		{NoteIdx: 1, Target: "Study/river/01-基础/03-切片.md", NewContent: "# 切片\n\n甲乙\n"},
	}}
	if err := Apply(root, plan, ns); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "river", "01-基础", "03-切片.md"))
	if string(data) != "# 切片\n\n甲乙\n" {
		t.Fatalf("应按序落盘为最终 content: %q", data)
	}
	rest, _ := Load(root)
	if len(rest) != 0 {
		t.Fatalf("两条都应移除: %+v", rest)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/notes/ -run "TestParsePlan|TestApply" -v`
Expected: 编译失败（plan.go 未定义）

- [ ] **Step 3: 实现 plan.go**

```go
// internal/notes/plan.go
package notes

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

// Placement 一条归位方案：便签下标（0 起，对应 Load 顺序）→ 目标文件完整新内容
type Placement struct {
	NoteIdx    int
	Target     string // 工作区相对路径
	NewContent string
}

// Plan tidy 整批方案；同目标多个 Placement 按序应用，content 须累计包含前序改动
type Plan struct {
	Placements []Placement
}

// planWire AI 输出线格式：note 为 1 起序号
type planWire struct {
	Placements []struct {
		Note    int    `json:"note"`
		Target  string `json:"target"`
		Content string `json:"content"`
	} `json:"placements"`
}

// ParsePlanJSON 解析并校验方案：note 序号在界、target 相对路径形态、content 非空；
// note 转 0 起 NoteIdx，content 统一 \n
func ParsePlanJSON(s string, ns []Note) (Plan, error) {
	var w planWire
	if err := json.Unmarshal([]byte(s), &w); err != nil {
		return Plan{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	var plan Plan
	for i, p := range w.Placements {
		switch {
		case p.Note < 1 || p.Note > len(ns):
			return Plan{}, fmt.Errorf("placement[%d]: 便签序号越界: %d", i, p.Note)
		case strings.TrimSpace(p.Target) == "":
			return Plan{}, fmt.Errorf("placement[%d]: target 为空", i)
		case strings.TrimSpace(p.Content) == "":
			return Plan{}, fmt.Errorf("placement[%d]: content 为空", i)
		}
		if err := checkRelPath(p.Target); err != nil {
			return Plan{}, fmt.Errorf("placement[%d]: %w", i, err)
		}
		plan.Placements = append(plan.Placements, Placement{
			NoteIdx:    p.Note - 1,
			Target:     strings.TrimSpace(p.Target),
			NewContent: strings.ReplaceAll(p.Content, "\r\n", "\n"),
		})
	}
	return plan, nil
}

// Apply 执行方案：按序逐目标原子写；任一失败即中止且不动 notes.md（便签仍在，幂等可重跑）；
// 全部成功后重写 notes.md 移除已归位便签
func Apply(root string, plan Plan, ns []Note) error {
	for i, p := range plan.Placements {
		abs := filepath.Join(root, filepath.FromSlash(p.Target))
		if err := workspace.WriteFileAtomic(abs, []byte(p.NewContent)); err != nil {
			return fmt.Errorf("placement[%d] 写入 %s 失败: %w", i, p.Target, err)
		}
	}
	placed := map[int]bool{}
	for _, p := range plan.Placements {
		placed[p.NoteIdx] = true
	}
	rest := make([]Note, 0, len(ns))
	for i, n := range ns {
		if !placed[i] {
			rest = append(rest, n)
		}
	}
	return Save(root, rest)
}
```

注意：`Apply` 写目标目录不存在时 `WriteFileAtomic` 会失败（tidy/move 的目标都是现存文件，目录理应存在，不需要 MkdirAll 兜底——这正是「目标必须是已有条目」的天然防线）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/notes/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交点（建议命令，开发者执行）**

```bash
git add internal/notes/
git commit -m "feat(notes): tidy 方案解析校验与执行"
```

---

### Task 4: 命令壳基础——/notes 路由、列表、录入、edit + 注册 + help 文本

**Files:**
- Create: `internal/command/notes.go`
- Test: `internal/command/notes_test.go`
- Modify: `internal/command/help.go`（Build 追加 `newNotes()`；helpText 增补说明）

- [ ] **Step 1: 写失败测试**

```go
// internal/command/notes_test.go
package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/notes"
)

func TestNotesRouteAdd(t *testing.T) {
	ws := wsForTest(t)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out, Args: []string{"slice、map", "channel 可以看作引用类型"}}
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	ns, err := notes.Load(ws.Root)
	if err != nil || len(ns) != 1 {
		t.Fatalf("应录入一条便签: %+v %v", ns, err)
	}
	if ns[0].Text != "slice、map channel 可以看作引用类型" {
		t.Fatalf("Args 应拼接: %q", ns[0].Text)
	}
	if !strings.Contains(out.String(), "已记录") {
		t.Fatalf("应回显确认: %s", out.String())
	}
}

func TestNotesRouteListEmptyAndNonEmpty(t *testing.T) {
	ws := wsForTest(t)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(out.String()), "暂无便签") {
		t.Fatalf("空应提示: %s", out.String())
	}
	if err := notes.Add(ws.Root, "甲", notesTestTime()); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[1]") || !strings.Contains(out.String(), "甲") {
		t.Fatalf("应列出便签: %s", out.String())
	}
}

func TestNotesEditCreatesSkeleton(t *testing.T) {
	ws := wsForTest(t)
	// editOpen 依赖编辑器配置，这里只验证骨架创建分支：直接调 Save 前 Load 不报错即可
	// 交互 shell 的编辑器拉起不在单测范围（与现有命令壳测试口径一致）
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if _, err := notes.Load(ws.Root); err != nil {
		t.Fatal(err)
	}
	_ = ctx
	if err := notes.Save(ws.Root, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(notes.Path(ws.Root)); err != nil {
		t.Fatalf("骨架应存在: %v", err)
	}
}

func notesTestTime() time.Time { return time.Date(2026, 10, 29, 14, 30, 0, 0, workspace.CST) }
```

等等——`TestNotesEditCreatesSkeleton` 没有测到命令本身，价值低。替换为对路由的空文本提示测试：

```go
func TestNotesRouteEmptyText(t *testing.T) {
	ws := wsForTest(t)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out, Args: []string{"   "}}
	if err := runNotes(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(out.String()), "用法") {
		t.Fatalf("空文本应提示用法: %s", out.String())
	}
}
```

最终 `notes_test.go` 顶部 import 需含 `time` 与 `github.com/bugzi/gs/internal/workspace`（notesTestTime 用）；`os`/`filepath` 仅在保留骨架测试时需要，按最终测试内容清理 import。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestNotes -v`
Expected: 编译失败（runNotes 未定义）

- [ ] **Step 3: 实现命令壳基础（internal/command/notes.go）**

```go
package command

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/notes"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

func newNotes() Command {
	return Command{
		Name:  "notes",
		Short: "便签：临时记录缓存/手动归位/AI 整理",
		Subs:  []string{"edit", "del", "move", "tidy"},
		Run:   func(ctx *Ctx) error { return runNotes(ctx) },
	}
}

// runNotes 首词路由：命中子命令分发，其余整行拼接视为录入
func runNotes(ctx *Ctx) error {
	if len(ctx.Args) == 0 {
		return runNotesList(ctx)
	}
	switch ctx.Args[0] {
	case "edit":
		return runNotesEdit(ctx)
	case "del":
		return runNotesDel(ctx, strings.Join(ctx.Args[1:], " "))
	case "move":
		return runNotesMove(ctx, strings.Join(ctx.Args[1:], " "))
	case "tidy":
		return runNotesTidy(ctx)
	}
	return runNotesAdd(ctx, strings.Join(ctx.Args, " "))
}

func runNotesList(ctx *Ctx) error {
	ns, err := notes.Load(ctx.WS.Root)
	if err != nil {
		return err
	}
	if len(ns) == 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("暂无便签，用 /notes <文本> 记录"))
		return nil
	}
	for i, n := range ns {
		fmt.Fprintf(ctx.Out, "[%d] %s %s\n", i+1, n.Time.In(workspace.CST).Format("060102 15:04"), n.Text)
	}
	return nil
}

func runNotesAdd(ctx *Ctx, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		fmt.Fprintln(ctx.Out, ui.Dim("用法：/notes <文本>；或 /notes edit|del|move|tidy"))
		return nil
	}
	if err := notes.Add(ctx.WS.Root, text, time.Now()); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s %s\n", ui.OK("已记录"), text)
	return nil
}

func runNotesEdit(ctx *Ctx) error {
	// 文件不存在先落骨架，编辑器里有的改
	if _, err := os.Stat(notes.Path(ctx.WS.Root)); os.IsNotExist(err) {
		if err := notes.Save(ctx.WS.Root, nil); err != nil {
			return err
		}
	}
	return editOpen(ctx, notes.Path(ctx.WS.Root))
}
```

- [ ] **Step 4: 注册命令 + help 文本**

`internal/command/help.go` 的 `Build()`：

```go
return []Command{newLogs(), newSea(), newShip(), newRiver(), newSpring(), newCode(), newNotes(), newPlan(), newSearch(), newShow(), newProject(), newReview(), newAI(), newHelp(), newExit()}
```

`helpText` 在 `/code` 行后插入：

```
  /notes [文本]           便签缓存：/notes <文本> 记一条，/notes 列出
  /notes edit|del         编辑便签文件 / 选择删除
  /notes move [关键词]    便签原文追加到 river/spring 条目并移出缓存
  /notes tidy             AI 整批归位到现有 river/spring（整批预览，一次确认）
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/command/ -run TestNotes -v && go build ./...`
Expected: PASS + 编译通过

- [ ] **Step 6: 提交点（建议命令，开发者执行）**

```bash
git add internal/command/notes.go internal/command/notes_test.go internal/command/help.go
git commit -m "feat(notes): /notes 路由、列表、录入与 edit 子命令"
```

---

### Task 5: del / move——便签选择器与手动归位

**Files:**
- Modify: `internal/command/notes.go`
- Test: `internal/command/notes_test.go`

- [ ] **Step 1: 写失败测试（纯核心：AppendNote + 移除保存已由 notes 包覆盖；这里测 move 的装配核心）**

```go
// moveNoteCore move 的非交互核心：原文追加目标 + 从缓存移除（可直测）
func TestMoveNoteCore(t *testing.T) {
	ws := wsForTest(t)
	target := filepath.Join(ws.Root, "Study", "spring", "切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片\n\n总结\n"), 0o644)
	if err := notes.Add(ws.Root, "扩容翻倍", notesTestTime()); err != nil {
		t.Fatal(err)
	}
	ns, _ := notes.Load(ws.Root)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if err := moveNoteCore(ctx, ns, 0, "Study/spring/切片.md"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "扩容翻倍") {
		t.Fatalf("目标应含便签原文: %q", data)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 0 {
		t.Fatalf("缓存应清空: %+v", rest)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestMoveNoteCore -v`
Expected: 编译失败（moveNoteCore 未定义）

- [ ] **Step 3: 实现选择器与 del/move（追加到 notes.go）**

```go
// pickNote 便签选择器：平铺选项（编号 + 时间 + 文本），关键词过滤文本，返回 Load 顺序下标
func pickNote(ctx *Ctx, kw string) (int, error) {
	ns, err := notes.Load(ctx.WS.Root)
	if err != nil {
		return 0, err
	}
	if len(ns) == 0 {
		return 0, fmt.Errorf("暂无便签")
	}
	kw = strings.ToLower(strings.TrimSpace(kw))
	var opts []menuOption
	for i, n := range ns {
		if kw != "" && !strings.Contains(strings.ToLower(n.Text), kw) {
			continue
		}
		opts = append(opts, option(fmt.Sprintf("[%d] %s %s", i+1, n.Time.In(workspace.CST).Format("060102 15:04"), n.Text), strconv.Itoa(i)))
	}
	if len(opts) == 0 {
		return 0, fmt.Errorf("没有匹配的便签")
	}
	var pick string
	if err := selectFrom(ctx, "选择便签", opts, &pick); err != nil {
		return 0, err
	}
	return strconv.Atoi(pick)
}

// dropNote 移除下标 i 的副本（不改原切片）
func dropNote(ns []notes.Note, i int) []notes.Note {
	out := make([]notes.Note, 0, len(ns)-1)
	out = append(out, ns[:i]...)
	return append(out, ns[i+1:]...)
}

// truncateRune confirm 标题里的便签预览截断
func truncateRune(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func runNotesDel(ctx *Ctx, kw string) error {
	i, err := pickNote(ctx, kw)
	if err != nil {
		return err
	}
	ns, _ := notes.Load(ctx.WS.Root)
	ok, err := confirm(ctx, "删除便签："+truncateRune(ns[i].Text, 30))
	if err != nil || !ok {
		return err
	}
	if err := notes.Save(ctx.WS.Root, dropNote(ns, i)); err != nil {
		return err
	}
	fmt.Fprintln(ctx.Out, ui.OK("已删除"))
	return nil
}

// moveNoteCore move 非交互核心：原文追加目标 + 从缓存移除（先目标后 notes.md，失败即停）
func moveNoteCore(ctx *Ctx, ns []notes.Note, i int, targetRel string) error {
	if err := notes.AppendNote(ctx.WS.Root, targetRel, ns[i].Text); err != nil {
		return err
	}
	if err := notes.Save(ctx.WS.Root, dropNote(ns, i)); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s 已归位到 %s\n", ui.OK("完成"), ui.Path(targetRel))
	return nil
}

func runNotesMove(ctx *Ctx, kw string) error {
	i, err := pickNote(ctx, kw)
	if err != nil {
		return err
	}
	ns, err := notes.Load(ctx.WS.Root)
	if err != nil {
		return err
	}
	station := "river"
	var pick string
	opts := []menuOption{option("河流 river", "river"), option("泉水 spring", "spring")}
	if err := selectFrom(ctx, "归位到哪一站？", opts, &pick); err != nil {
		return err
	}
	station = pick
	it, err := pickTierEntryGrouped(ctx, station, "", "选择归位目标条目")
	if err != nil {
		return err
	}
	ok, err := confirm(ctx, fmt.Sprintf("把「%s」追加到 %s 末尾？", truncateRune(ns[i].Text, 24), it.Path))
	if err != nil || !ok {
		return err
	}
	return moveNoteCore(ctx, ns, i, it.Path)
}
```

import 需补 `"strconv"`。确认取消（`!ok`）时 `return err` 里 err 为 nil，静默返回——与现有 runSpringAdd 等口径一致。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/command/ -run "TestNotes|TestMoveNoteCore" -v && go build ./...`
Expected: PASS + 编译通过

- [ ] **Step 5: 提交点（建议命令，开发者执行）**

```bash
git add internal/command/
git commit -m "feat(notes): del/move 便签选择器与手动归位"
```

---

### Task 6: /notes tidy——只读 agent 循环 + 整批方案 + 一次确认

**Files:**
- Modify: `internal/command/notes.go`
- Test: `internal/command/notes_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestBuildTidyPrompt(t *testing.T) {
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("Study/river/01-基础/03-切片.md", "# 切片\n")
	mk("Study/spring/切片.md", "# 切片总结\n")
	ns := []notes.Note{{Time: notesTestTime(), Text: "扩容翻倍"}}
	prompt, err := buildTidyPrompt(ws, ns)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Study/river/01-基础/03-切片.md",
		"Study/spring/切片.md",
		"1. 扩容翻倍",
		"placements",
		"不得新建文件",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("提示词缺 %q:\n%s", want, prompt)
		}
	}
}

func TestBuildTidyPromptNoCandidates(t *testing.T) {
	ws := wsForTest(t)
	if _, err := buildTidyPrompt(ws, []notes.Note{{Time: notesTestTime(), Text: "x"}}); err == nil {
		t.Fatal("无候选应报错")
	}
}

func TestExtractJSON(t *testing.T) {
	if got := extractJSON("```json\n{\"a\":1}\n```"); got != `{"a":1}` {
		t.Fatalf("围栏应剥离: %q", got)
	}
	if got := extractJSON(` {"a":1} `); got != `{"a":1}` {
		t.Fatalf("空白应去除: %q", got)
	}
}

func TestTidyPreviewApply(t *testing.T) {
	ws := wsForTest(t)
	target := filepath.Join(ws.Root, "Study", "river", "01-基础", "03-切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片\n\n原文\n"), 0o644)
	ns := []notes.Note{{Time: notesTestTime(), Text: "扩容翻倍"}}
	notes.Save(ws.Root, ns)
	plan, err := notes.ParsePlanJSON(
		`{"placements":[{"note":1,"target":"Study/river/01-基础/03-切片.md","content":"# 切片\n\n原文\n\n扩容翻倍\n"}]}`, ns)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	asked := ""
	if err := tidyPreviewApply(ctx, ns, plan, func(title string) bool { asked = title; return true }); err != nil {
		t.Fatal(err)
	}
	if asked == "" {
		t.Fatal("应弹出确认")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "# 切片\n\n原文\n\n扩容翻倍\n" {
		t.Fatalf("目标未更新: %q", data)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 0 {
		t.Fatalf("缓存应清空: %+v", rest)
	}
	if !strings.Contains(stripANSI(out.String()), "归位 1 条") {
		t.Fatalf("应汇报结果: %s", out.String())
	}
}

func TestTidyNoPlacementKeepsNotes(t *testing.T) {
	ws := wsForTest(t)
	ns := []notes.Note{{Time: notesTestTime(), Text: "无处可去的便签"}}
	notes.Save(ws.Root, ns)
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Out: &out}
	if err := tidyPreviewApply(ctx, ns, notes.Plan{}, func(string) bool { return true }); err != nil {
		t.Fatal(err)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 1 {
		t.Fatalf("空方案应原样保留: %+v", rest)
	}
	if !strings.Contains(stripANSI(out.String()), "没有找到合适的归宿") {
		t.Fatalf("应提示保留: %s", out.String())
	}
}

func TestRunNotesTidyBadJSON(t *testing.T) {
	st, ws, cfg := newTestAIState(t, "data: {\"choices\":[{\"delta\":{\"content\":\"不是JSON\"}}]}\n\n")
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("Study/river/a.md", "# A\n")
	notes.Add(ws.Root, "甲", notesTestTime())
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, AI: st}
	if err := runNotesTidy(ctx); err == nil {
		t.Fatal("解析失败应报错")
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 1 {
		t.Fatalf("解析失败便签不得动: %+v", rest)
	}
}

func TestRunNotesTidyEndToEnd(t *testing.T) {
	planContent := `{"placements":[{"note":1,"target":"Study/river/a.md","content":"# A\n\n融入甲\n"}]}`
	ev, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]any{"content": planContent}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	st, ws, cfg := newTestAIState(t, "data: "+string(ev)+"\n\n")
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	mk("Study/river/a.md", "# A\n")
	notes.Add(ws.Root, "甲", notesTestTime())
	var out bytes.Buffer
	ctx := &Ctx{WS: ws, Cfg: cfg, Out: &out, AI: st}
	// 注入确认桩：模拟用户选「应用」
	tidyAsk = func(ctx *Ctx, title string) bool { return true }
	defer func() { tidyAsk = nil }()
	if err := runNotesTidy(ctx); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "river", "a.md"))
	if string(data) != "# A\n\n融入甲\n" {
		t.Fatalf("目标未更新: %q", data)
	}
	rest, _ := notes.Load(ws.Root)
	if len(rest) != 0 {
		t.Fatalf("缓存应清空: %+v", rest)
	}
}
```

说明：`tidyAsk` 为包级可注入确认桩（nil 时走 `confirm`），模式对齐 `AIState.AskSea`；测试 import 需补 `encoding/json`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run "TestBuildTidy|TestExtractJSON|TestTidy|TestRunNotesTidy" -v`
Expected: 编译失败（tidy 函数未定义）

- [ ] **Step 3: 实现 tidy（追加到 notes.go）**

```go
// tidySystemTpl tidy 提示词：任务规则与 JSON 输出约定
const tidySystemTpl = `你是 gs 学习工作台的便签整理助手。用户有若干临时便签，需要把每条归位到已有的 river（沉淀笔记）或 spring（知识点总结）文件中。

规则：
1. 只能归位到候选清单中列出的现有文件，不得发明路径，不得建议新建文件。
2. 没有合适归宿的便签不要出现在方案里（会留在便签缓存中）。
3. content 必须是目标文件的完整新内容：保留原文，把便签内容改写融入最合适的位置，语言风格与原文一致。
4. 同一目标文件归位多条便签时，各 placement 的 content 累计包含前序 placement 的改动。
5. 先用工具检索阅读候选文件再判断；不确定就不归位。

最终只输出一个 JSON 对象，不要输出任何其他文字：
{"placements":[{"note":<便签序号,1 起>,"target":"<工作区相对路径>","content":"<目标文件完整新内容>"}]}
`

// tidyAsk 包级确认桩：nil 走 huh confirm（测试注入用，模式同 AIState.AskSea）
var tidyAsk func(ctx *Ctx, title string) bool

func tidyConfirm(ctx *Ctx, title string) bool {
	if tidyAsk != nil {
		return tidyAsk(ctx, title)
	}
	ok, err := confirm(ctx, title)
	return err == nil && ok
}

// buildTidyPrompt 组装 tidy 系统提示词；river/spring 无任何条目时报错
func buildTidyPrompt(ws *workspace.Workspace, ns []notes.Note) (string, error) {
	var cands strings.Builder
	for _, tier := range []string{"river", "spring"} {
		files, err := ListTierFiles(ws, tier)
		if err != nil {
			return "", err
		}
		for _, f := range files {
			fmt.Fprintf(&cands, "- %s\n", f)
		}
	}
	if cands.Len() == 0 {
		return "", fmt.Errorf("river/spring 还没有条目，无处可归位")
	}
	var nb strings.Builder
	for i, n := range ns {
		fmt.Fprintf(&nb, "%d. %s\n", i+1, n.Text)
	}
	return fmt.Sprintf(tidySystemTpl, cands.String(), nb.String()), nil
}
```

注意模板里 `%s` 只有两个（候选清单、便签清单）——工作区根路径不再注入，工具都以工作区为根，去掉设计稿中多余的根路径占位。

```go
// extractJSON 剥离 ``` 围栏取 JSON 体；无围栏原文去空白
func extractJSON(s string) string {
	if i := strings.Index(s, "```"); i >= 0 {
		body := s[i:]
		body = strings.TrimPrefix(body, "```json")
		body = strings.TrimPrefix(body, "```")
		if j := strings.Index(body, "```"); j >= 0 {
			body = body[:j]
		}
		return strings.TrimSpace(body)
	}
	return strings.TrimSpace(s)
}

// tidyPreviewApply 渲染整批方案预览（diffx 只显差异），ask 确认后一次应用
func tidyPreviewApply(ctx *Ctx, ns []notes.Note, plan notes.Plan, ask func(string) bool) error {
	if len(plan.Placements) == 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("AI 没有找到合适的归宿，便签原样保留"))
		return nil
	}
	for _, p := range plan.Placements {
		cur, err := os.ReadFile(filepath.Join(ctx.WS.Root, filepath.FromSlash(p.Target)))
		if err != nil {
			return fmt.Errorf("读取目标 %s 失败: %w", p.Target, err)
		}
		fmt.Fprintf(ctx.Out, "\n%s %s → %s\n", ui.OK("归位"), ui.Dim(truncateRune(ns[p.NoteIdx].Text, 40)), ui.Path(p.Target))
		fmt.Fprint(ctx.Out, truncateLines(diffx.Render(string(cur), p.NewContent), 30))
	}
	fmt.Fprintf(ctx.Out, "共 %d 条归位，%d 条保留\n", len(plan.Placements), len(ns)-len(plan.Placements))
	if !ask("应用该方案？") {
		fmt.Fprintln(ctx.Out, ui.Dim("已取消，便签未动"))
		return nil
	}
	if err := notes.Apply(ctx.WS.Root, plan, ns); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s 归位 %d 条，保留 %d 条\n", ui.OK("完成"), len(plan.Placements), len(ns)-len(plan.Placements))
	return nil
}

// runNotesTidy 便签 AI 整批整理：只读工具 agent 循环 → JSON 方案 → 预览确认 → Apply
func runNotesTidy(ctx *Ctx) error {
	ns, err := notes.Load(ctx.WS.Root)
	if err != nil {
		return err
	}
	if len(ns) == 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("没有便签可整理"))
		return nil
	}
	st, err := ensureAIState(ctx)
	if err != nil {
		return err
	}
	prompt, err := buildTidyPrompt(ctx.WS, ns)
	if err != nil {
		return err
	}
	fmt.Fprintln(ctx.Out, ui.Dim("（AI 整理中，可自主检索工作区…）"))
	ag := &agent.Agent{
		Client: st.Client,
		Tools: []agent.Tool{
			agent.SearchTool(ctx.WS.Root, ctx.Cfg.Scan.Exclude, ctx.Cfg.Scan.MaxFileKB, 20, 5),
			agent.ReadFileTool(ctx.WS.Root, 400),
			agent.ListDirTool(ctx.WS.Root),
		},
	}
	final, _, err := ag.Run(context.Background(),
		[]ai.Message{{Role: "system", Content: prompt}, {Role: "user", Content: "请整理以上便签。"}},
		agent.Hooks{OnToolCall: func(name, argsJSON string) {
			fmt.Fprintf(ctx.Out, "%s\n", ui.Dim("· "+name+" "+compactArgs(argsJSON)))
		}})
	if err != nil {
		return err
	}
	plan, err := notes.ParsePlanJSON(extractJSON(final.Content), ns)
	if err != nil {
		return fmt.Errorf("方案解析失败（便签未动，可重试）: %w", err)
	}
	return tidyPreviewApply(ctx, ns, plan, func(title string) bool { return tidyConfirm(ctx, title) })
}
```

import 需补：`"context"`、`"path/filepath"`、`"github.com/bugzi/gs/internal/agent"`、`"github.com/bugzi/gs/internal/ai"`、`"github.com/bugzi/gs/internal/edit/diffx"`（`truncateLines`/`compactArgs`/`ensureAIState` 为同包既有函数）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/command/ -run "TestNotes|TestMoveNoteCore|TestBuildTidy|TestExtractJSON|TestTidy|TestRunNotesTidy" -v && go build ./...`
Expected: PASS + 编译通过

- [ ] **Step 5: 提交点（建议命令，开发者执行）**

```bash
git add internal/command/
git commit -m "feat(notes): tidy 只读检索整批归位"
```

---

### Task 7: 全量验证与收尾（日志 + 归档）

**Files:**
- Modify: `docs/log/<当天YYMMDD>.md`（追加一条，≤40 字，附创建时间行）
- Modify: `docs/project/新功能开发区.md`（归档区顶部记本轮）

- [ ] **Step 1: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 全部通过（落地判据）

- [ ] **Step 2: 人工冒烟（建议开发者执行）**

```bash
go run ./cmd/gs
```

依次验证：`/notes 学习一下 channel 底层` → `/notes`（列表）→ `/notes move`（选 river 条目，确认追加）→ 再录一条 → `/notes tidy`（观察预览与确认；未配置 [ai] 时应报友好错误）→ `/notes del`。

- [ ] **Step 3: 记开发日志**

`docs/log/<YYMMDD>.md`（东八区当天）末尾追加（时间用 `date +"%Y-%m-%d %H:%M"` 实取）：

```markdown
## <小节标题>

> 创建于 <YYYY-MM-DD HH:MM>

- /notes 便签落地：缓存/手动归位/tidy 整批归位
```

- [ ] **Step 4: 归档新功能条目**

`docs/project/新功能开发区.md` 归档区顶部插入（轮号按现有最大轮 +1）：

```markdown
### 第N轮（YYMMDD）：notes 便签缓存

- /notes 便签：临时记录缓存；手动 move 归位 river/spring；/notes tidy AI 整批归位（无合适归宿留在缓存）。
  立项来源：开发者口头需求（2026-10-30 brainstorming）；设计文档 docs/project/notes/2026-10-30-notes便签-design.md
  （brainstorming 产出即开发依据，未另立 designs-specs）；计划 docs/project/notes/2026-10-30-notes便签-plan.md。
```

- [ ] **Step 5: 提交点（建议命令，开发者执行）**

```bash
git add docs/
git commit -m "docs: /notes 便签功能开发日志与新功能归档"
```

---

## 自审记录

- **Spec 覆盖**：存储格式（Task 1）、路径校验与 AppendNote（Task 2）、方案解析/执行（Task 3）、路由/列表/录入/edit/注册/help（Task 4）、del/move（Task 5）、tidy 全流程（Task 6）、日志与归档纪律（Task 7）。设计文档「边界处理」各条分别由 TestLoadSkipsUnknownLines（乱改容错）、TestApplyTargetFailureKeepsNotes（两步写失败不动便签）、checkRelPath（伪装/穿越）覆盖；「预览确认到落盘窗口」为设计声明项，无需测试。
- **占位符**：无 TBD/TODO；所有代码步骤给出完整代码。
- **类型一致性**：`notes.Note`/`Placement.NoteIdx`（0 起）与 AI 侧 `note`（1 起）转换仅在 `ParsePlanJSON` 一处；`dropNote`/`truncateRune`/`tidyAsk` 各 Task 间命名一致；`Format`/`headerLines` 未被外部引用处使用时命名一致。
