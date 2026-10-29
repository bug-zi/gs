# 四站流转与 river 编号管理 实施计划

> 状态：**代码已落地（2026-10-27）**——T1-T12 全部完成，全仓 go build + vet + test 19 包全绿；T13 真机冒烟进行中（入口关键词 bug 已修）。
- 已归档：261027 随 四站流转与river编号 落地（机制判定 build+test 通过即归档，不等冒烟）。执行偏差见文末「执行偏差记录」。

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 spec `docs/project/全局/archive/2026-10-27-四站流转与river编号-design.md`——sea→ship→river→spring 四站流转（`/ship move`、`/river move`）+ river 按树序编号与 `/plan renumber` 全量重排。

**Architecture:** roadmap 包新增 `Point.Ship` 字段与 `MoveTierBinding` 绑定迁移、renumber 纯函数计划生成；新包 `internal/tier` 承担文件迁移与出处链改写（纯逻辑）；command 层只做选择器/确认/提示装配。流转统一管线：文件 rename → TOML 绑定原子写 → 全库出处链改写。

**Tech Stack:** Go 1.22+；BurntSushi/toml；无新依赖。

**硬性纪律（执行者必读）：**

1. **禁止执行任何 git 命令**——所有提交由开发者手动完成；任务末尾的「提交检查点」只给出建议命令并暂停等待。
2. 源码注释用中文、仅写非直观处；文件写入一律走 `workspace.WriteFileAtomic`。
3. 每个任务以 `go build ./...` + `go test ./...` 全绿收口。
4. 工作区根 = `D:\Code\Go`；gs 源码根 = `D:\Code\Go\Project\gs`。以下路径均相对 gs 源码根（除注明「工作区相对」的运行期路径字符串）。

**代码事实备忘（已探明，勿再探）：**

- `Point` 在 `internal/roadmap/roadmap.go:26`，含 `River []string` / `Sea []string`（toml tag omitempty）。
- 绑定写入 `internal/roadmap/ops.go`：`bindList(list *[]string, rel string)` 幂等追加（未导出，可复用）。
- 出处行格式：`> 出处：<工作区相对路径>`（`internal/command/entry.go:45` 写入、`internal/command/review_stats.go:35` 解析，前缀常量 `"> 出处："`）。
- 选择器：`pickTierEntryGrouped(ctx, tier, kw, label)`（`tier_view.go:182`），按 `tierDirs[tier]` 列文件；spring/river/code 按主干分组（binds 函数），sea 按一级子目录。
- 交互原语：`inputLine(ctx, label)`、`selectFrom(ctx, label, opts, &pick)`、`option(label, value)`、`pointKey(ref)`/`pointParse(s)`（plan.go:481）、`editOpen(ctx, abs)`、`loadRoadmap/loadRoadmapSoft(ctx)`。
- `/plan` 子命令分发在 `plan.go:132` 的 `switch ctx.Args[0]`（status/next/edit/done|doing|todo/add）。
- 命令注册：`help.go:63` `Build()`；帮助文案 `help.go:8` `helpText`。
- `workspace.Workspace`（`workspace.go:15`）：`Root`、`SeaDir/RiverDir/SpringDir/CodeDir`、`Rel`、`RoadmapFile`、`IndexPath`（.gs/topics.toml）、`WriteFileAtomic`。
- planview 详情页：`state.go:112` `pointRows()`，现有四段 spring/code/river/sea。
- **测试基建**：命令层工作区 fixture 为 `wsForTest(t)`（`logs_test.go:15`，TempDir + Study/logs + Study/plan）；`Ctx` 构造 `&Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{...}}`；`inputLine`/`plainSelect` 直接读 `os.Stdin`（`prompt.go:131`），测试用 `withStdin`（os.Pipe 换流）注入；`editOpen` 会拉起真实编辑器——含编辑器的命令入口拆出核心函数（不带 editOpen）直测。
- 单文件选择直达：`pickTierEntryGrouped` 在候选总数为 1 时不走选择器，测试夹具各留单个候选即可绕开交互。

---

## 阶段 1 · 主链路（/ship move、/river move、/ship 列表）

### Task 1: schema 与绑定迁移（workspace + roadmap）

**Files:**
- Modify: `internal/workspace/workspace.go:44-47`（目录方法区）
- Modify: `internal/roadmap/roadmap.go:26-36`（Point 结构）
- Modify: `internal/roadmap/ops.go`（文件末尾追加）
- Test: `internal/roadmap/ops_ship_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/roadmap/ops_ship_test.go`

```go
package roadmap

import "testing"

func shipFixture() *Roadmap {
	return &Roadmap{Version: Version, Branches: []Branch{{
		Title:  "并发编程",
		Points: []Point{{Title: "select", Status: StatusDoing, Sea: []string{"Study/sea/a.md", "Study/sea/b.md"}}},
	}}}
}

func TestMoveTierBindingSeaToShip(t *testing.T) {
	rm := shipFixture()
	hits := rm.MoveTierBinding("Study/sea/a.md", "Study/ship/a.md", SeaList, ShipList)
	if len(hits) != 1 || hits[0] != (PointRef{0, 0}) {
		t.Fatalf("hits = %v, want [{0 0}]", hits)
	}
	p := rm.PointAt(PointRef{0, 0})
	if len(p.Sea) != 1 || p.Sea[0] != "Study/sea/b.md" {
		t.Fatalf("Sea = %v, want [Study/sea/b.md]", p.Sea)
	}
	if len(p.Ship) != 1 || p.Ship[0] != "Study/ship/a.md" {
		t.Fatalf("Ship = %v, want [Study/ship/a.md]", p.Ship)
	}
}

func TestMoveTierBindingSameListRename(t *testing.T) {
	rm := shipFixture()
	rm.PointAt(PointRef{0, 0}).River = []string{"Study/river/old.md"}
	rm.MoveTierBinding("Study/river/old.md", "Study/river/new.md", RiverList, RiverList)
	if got := rm.PointAt(PointRef{0, 0}).River; len(got) != 1 || got[0] != "Study/river/new.md" {
		t.Fatalf("River = %v, want [Study/river/new.md]", got)
	}
}

func TestMoveTierBindingMissingAndIdempotent(t *testing.T) {
	rm := shipFixture()
	if hits := rm.MoveTierBinding("Study/sea/none.md", "Study/ship/x.md", SeaList, ShipList); len(hits) != 0 {
		t.Fatalf("未命中应返回空，hits = %v", hits)
	}
	rm.MoveTierBinding("Study/sea/a.md", "Study/ship/a.md", SeaList, ShipList)
	if hits := rm.MoveTierBinding("Study/sea/a.md", "Study/ship/a.md", SeaList, ShipList); len(hits) != 0 {
		t.Fatalf("二次迁移应无命中，hits = %v", hits)
	}
}

func TestParseShipField(t *testing.T) {
	data := "version = 2\n\n[[branch]]\ntitle = \"A\"\n\n  [[branch.point]]\n  title = \"x\"\n  ship = [\"Study/ship/x.md\"]\n"
	rm, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := rm.Branches[0].Points[0].Ship; len(got) != 1 || got[0] != "Study/ship/x.md" {
		t.Fatalf("Ship = %v", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/roadmap/ -run TestMoveTierBinding -v`（在 `D:\Code\Go\Project\gs` 下）
Expected: 编译失败（`MoveTierBinding`/`SeaList` 等未定义）

- [ ] **Step 3: 最小实现**

`internal/roadmap/roadmap.go` Point 结构（River 之后插一行）：

```go
	River  []string `toml:"river,omitempty"`
	Ship   []string `toml:"ship,omitempty"` // 加工中（船坞）；可选字段，缺省空
	Sea    []string `toml:"sea,omitempty"`
```

`internal/roadmap/ops.go` 文件末尾追加：

```go
// SeaList / ShipList / RiverList 绑定数组访问器（MoveTierBinding 的 from/to 参数）
func SeaList(p *Point) *[]string   { return &p.Sea }
func ShipList(p *Point) *[]string  { return &p.Ship }
func RiverList(p *Point) *[]string { return &p.River }

// MoveTierBinding 全树把 from 列表中的 oldRel 迁入 to 列表并更新为 newRel；返回受影响知识点（树序）。
// from 与 to 为同一访问器时即原地改名。幂等：无命中返回空。
func (rm *Roadmap) MoveTierBinding(oldRel, newRel string, from, to func(*Point) *[]string) []PointRef {
	var hits []PointRef
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			p := &rm.Branches[bi].Points[pi]
			fl, tl := from(p), to(p)
			idx := -1
			for i, s := range *fl {
				if s == oldRel {
					idx = i
					break
				}
			}
			if idx < 0 {
				continue
			}
			*fl = append((*fl)[:idx], (*fl)[idx+1:]...)
			bindList(tl, newRel)
			hits = append(hits, PointRef{bi, pi})
		}
	}
	return hits
}
```

`internal/workspace/workspace.go`（SpringDir 行后插入）：

```go
func (w *Workspace) ShipDir() string   { return filepath.Join(w.Root, "Study", "ship") }
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./internal/roadmap/ ./internal/workspace/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(roadmap): Point.Ship 字段与 MoveTierBinding 绑定迁移"`

---

### Task 2: river 编号纯函数（roadmap/renumber.go 第一部分）

**Files:**
- Create: `internal/roadmap/renumber.go`
- Test: `internal/roadmap/renumber_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/roadmap/renumber_test.go`

```go
package roadmap

import "testing"

func TestStripNumPrefix(t *testing.T) {
	cases := map[string]string{
		"5 中间件Middleware":   "中间件Middleware",
		"2.1 前置知识补充":       "前置知识补充",
		"1-3 整理：五级跳的演进时间线": "整理：五级跳的演进时间线",
		"03.1-切片陷阱汇总":       "切片陷阱汇总",
		"05-slice底层结构与扩容":   "slice底层结构与扩容",
		"12、超时控制":           "超时控制",
		"select":             "select", // 无前缀原样
		"7 ":                 "7",      // 全被剥掉回退原名（去 .md 后）
	}
	for in, want := range cases {
		if got := StripNumPrefix(in); got != want {
			t.Errorf("StripNumPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextDupSeq(t *testing.T) {
	const pn = 3
	cases := []struct {
		existing []string
		want     int
	}{
		{nil, 0},
		{[]string{"Study/river/03-首篇.md"}, 1},
		{[]string{"Study/river/03-首篇.md", "Study/river/03.1-第二篇.md"}, 2},
		{[]string{"Study/river/03.1-a.md", "Study/river/03.3-c.md"}, 4},
		{[]string{"Study/river/13-别的点.md"}, 0}, // 其他点不干扰
	}
	for _, c := range cases {
		if got := NextDupSeq(c.existing, pn); got != c.want {
			t.Errorf("NextDupSeq(%v, %d) = %d, want %d", c.existing, pn, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/roadmap/ -run "TestStripNumPrefix|TestNextDupSeq" -v`
Expected: 编译失败（函数未定义）

- [ ] **Step 3: 实现** `internal/roadmap/renumber.go`

```go
// Package roadmap 内：river 编号与重排纯逻辑。TOML 数组顺序是树序唯一权威。
package roadmap

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// numPrefixRe 源材料编号前缀：1 、2.1 、1-3 、03.1- 、05- 、12、 等（数字开头可含 . 与 - 段，后接分隔符）
var numPrefixRe = regexp.MustCompile(`^[0-9][0-9.\-]*[\s\-_·.]*`)

// StripNumPrefix 剥掉文件名标题中的源编号前缀；全被剥掉时回退原名（防误剥纯数字标题）
func StripNumPrefix(name string) string {
	base := strings.TrimSuffix(name, ".md")
	s := strings.Trim(numPrefixRe.ReplaceAllString(base, ""), " -_·.")
	if s == "" {
		return base
	}
	return s
}

// FormatPointNum 点位号两位补零（1-based）
func FormatPointNum(n int) string { return fmt.Sprintf("%02d", n) }

// NextDupSeq 该知识点现有 river 文件名中的下一个篇序 N：
// 无文件 → 0（首篇不带 .N）；仅有首篇 → 1；已有 .k → max(k)+1
func NextDupSeq(existing []string, pointNum int) int {
	dupPrefix := fmt.Sprintf("%02d.", pointNum)
	basePrefix := fmt.Sprintf("%02d-", pointNum)
	max, hasBase := 0, false
	for _, name := range existing {
		base := strings.TrimSuffix(filepath.Base(name), ".md")
		switch {
		case strings.HasPrefix(base, dupPrefix):
			var k int
			if n, _ := fmt.Sscanf(strings.TrimPrefix(base, dupPrefix), "%d", &k); n == 1 && k > max {
				max = k
			}
			hasBase = true
		case strings.HasPrefix(base, basePrefix):
			hasBase = true
		}
	}
	switch {
	case !hasBase:
		return 0
	case max == 0:
		return 1
	default:
		return max + 1
	}
}

// RiverFileName 知识点 river 文件名：dup=0 首篇 `<点两位>-<标题>.md`；dup≥1 同点第 dup 篇 `<点两位>.<dup>-<标题>.md`
func RiverFileName(pointNum, dup int, title string) string {
	if dup == 0 {
		return fmt.Sprintf("%s-%s.md", FormatPointNum(pointNum), title)
	}
	return fmt.Sprintf("%s.%d-%s.md", FormatPointNum(pointNum), dup, title)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./internal/roadmap/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(roadmap): river 编号纯函数 StripNumPrefix/NextDupSeq/RiverFileName"`

---

### Task 3: 出处链扫描与改写（internal/tier 新包）

**Files:**
- Create: `internal/tier/chain.go`
- Test: `internal/tier/chain_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/tier/chain_test.go`

```go
package tier

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chainFixture(t *testing.T) string {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Study/river/a.md", "# A\n\n> 出处：Study/sea/x.md\n\n正文\n")
	write("Study/river/b.md", "# B\n\n> 出处：Study/sea/y.md\n")
	write("Study/spring/c.md", "> 出处：Study/sea/x.md")                   // 文件尾无换行
	write("Study/sea/x.md", "# X\n> 出处：Study/sea/x.md\n")                // 自引用仅验证匹配
	write(".gs/topics.toml", "topic = 1\n")                                // 点目录跳过
	return root
}

func TestCountAndRewriteSourceChains(t *testing.T) {
	root := chainFixture(t)
	if n := CountChainRefs(root, "Study/sea/x.md"); n != 3 {
		t.Fatalf("CountChainRefs = %d, want 3", n)
	}
	changed, err := RewriteSourceChains(root, "Study/sea/x.md", "Study/ship/x.md")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 3 {
		t.Fatalf("changed = %d, want 3", changed)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "river", "a.md"))
	if !strings.Contains(string(data), "> 出处：Study/ship/x.md") {
		t.Fatalf("改写未生效，文件内容：\n%s", data)
	}
	if n := CountChainRefs(root, "Study/sea/x.md"); n != 0 {
		t.Fatalf("改写后仍有残留引用 %d 处", n)
	}
	if changed, _ := RewriteSourceChains(root, "Study/sea/none.md", "Study/ship/none.md"); changed != 0 {
		t.Fatalf("无关路径 changed = %d, want 0", changed)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tier/ -v`
Expected: 编译失败（包不存在）

- [ ] **Step 3: 实现** `internal/tier/chain.go`

```go
// Package tier 四站流转执行器（sea/ship/river 文件迁移、出处链改写）。纯逻辑，不 import ui。
package tier

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

const chainPrefix = "> 出处："

// chainRel 从「> 出处：X」行提取 X；非出处行返回 ""
func chainRel(line string) string {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, chainPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(t, chainPrefix))
}

// walkMD 遍历 root 下全部 .md（跳过点开头的目录），对每个文件调用 fn
func walkMD(root string, fn func(path string, data []byte)) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		fn(path, data)
		return nil
	})
}

// CountChainRefs 全库出处行指向 rel 的次数
func CountChainRefs(root, rel string) int {
	n := 0
	walkMD(root, func(_ string, data []byte) {
		for _, line := range strings.Split(string(data), "\n") {
			if chainRel(line) == rel {
				n++
			}
		}
	})
	return n
}

// RewriteSourceChains 全库把出处行中的 oldRel 改写为 newRel（行内容精确匹配），
// 逐文件原子写；返回被改写的文件数。
func RewriteSourceChains(root, oldRel, newRel string) (int, error) {
	changed := 0
	var firstErr error
	walkMD(root, func(path string, data []byte) {
		lines := strings.Split(string(data), "\n")
		hit := false
		for i, line := range lines {
			if chainRel(line) == oldRel {
				lines[i] = chainPrefix + newRel
				hit = true
			}
		}
		if !hit {
			return
		}
		if err := workspace.WriteFileAtomic(path, []byte(strings.Join(lines, "\n"))); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return
		}
		changed++
	})
	return changed, firstErr
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./internal/tier/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(tier): 出处链全库扫描计数与原子改写"`

---

### Task 4: 文件迁移 MoveFile（internal/tier）

**Files:**
- Create: `internal/tier/move.go`
- Test: `internal/tier/move_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/tier/move_test.go`

```go
package tier

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFile(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "Study", "sea", "g", "a.md")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("# A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(root, "Study/sea/g/a.md", "Study/ship/a.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("源文件应已不存在，err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "Study", "ship", "a.md"))
	if err != nil || string(data) != "# A\n" {
		t.Fatalf("目标内容异常 data=%q err=%v", data, err)
	}
	// 目标已存在报错
	if err := os.WriteFile(filepath.Join(root, "Study", "ship", "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MoveFile(root, "Study/sea/g/a.md", "Study/ship/a.md"); err == nil {
		t.Fatal("目标已存在应报错")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tier/ -run TestMoveFile -v`
Expected: 编译失败

- [ ] **Step 3: 实现** `internal/tier/move.go`

```go
package tier

import (
	"fmt"
	"os"
	"path/filepath"
)

// MoveFile 同盘 rename 迁移；目标目录不存在则创建。rel 为工作区相对正斜杠路径。
func MoveFile(root, srcRel, dstRel string) error {
	src := filepath.Join(root, filepath.FromSlash(srcRel))
	dst := filepath.Join(root, filepath.FromSlash(dstRel))
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("目标已存在: %s", dstRel)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./internal/tier/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(tier): MoveFile 同盘原子迁移"`

---

### Task 5: `/ship move` 装船 + `/ship` 列表 + 注册

**Files:**
- Modify: `internal/command/knowledge.go:13-23`（tierDirs/tierExts 加 ship；ListKnowledge 顺序 sea→ship→river→spring）
- Modify: `internal/command/tier_view.go:29-44`（加 shipBinds）、`tier_view.go:192-199`（分组 binds switch 加 ship）
- Modify: `internal/command/tier_cmd.go`（runShipMove + newShip；插在 newSea 之后）
- Modify: `internal/command/help.go:63-65`（Build 注册）、`help.go:21-24`（helpText 三层命令段）
- Test: `internal/command/ship_move_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/command/ship_move_test.go`

```go
package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func shipMoveFixture(t *testing.T) (*workspace.Workspace, *Ctx, string) {
	t.Helper()
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/sea/g/a.md", "# A\n\n> 出处：Study/sea/src.md\n\n正文\n")
	mk("Study/river/r.md", "# R\n\n> 出处：Study/sea/g/a.md\n")
	rmToml := "version = 2\n\n[[branch]]\ntitle = \"并发编程\"\n\n  [[branch.point]]\n  title = \"select\"\n  sea = [\"Study/sea/g/a.md\"]\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := &Ctx{WS: ws, Out: &bytes.Buffer{}}
	return ws, ctx, "Study/sea/g/a.md"
}

func TestMoveToShip(t *testing.T) {
	ws, ctx, src := shipMoveFixture(t)
	dst, err := moveToShip(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	if dst != "Study/ship/a.md" {
		t.Fatalf("dst = %s", dst)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "ship", "a.md")); err != nil {
		t.Fatalf("ship 文件缺失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "sea", "g", "a.md")); !os.IsNotExist(err) {
		t.Fatal("sea 原文件应已不在")
	}
	rm, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	p := rm.Branches[0].Points[0]
	if len(p.Ship) != 1 || p.Ship[0] != "Study/ship/a.md" {
		t.Fatalf("Ship 绑定未迁移: %v", p.Ship)
	}
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "river", "r.md"))
	if !strings.Contains(string(data), "> 出处：Study/ship/a.md") {
		t.Fatalf("出处链未改写：\n%s", data)
	}
}
```

（若 `Ctx` 字段与上面构造不符，以 `internal/command/command.go:11` 实际定义为准调整。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestMoveToShip -v`
Expected: 编译失败（moveToShip 未定义）

- [ ] **Step 3: 实现**

`internal/command/knowledge.go` 两张表加 ship，ListKnowledge 顺序插 ship：

```go
var tierDirs = map[string]func(*workspace.Workspace) string{
	"sea":    (*workspace.Workspace).SeaDir,
	"ship":   (*workspace.Workspace).ShipDir,
	"river":  (*workspace.Workspace).RiverDir,
	"spring": (*workspace.Workspace).SpringDir,
	"code":   (*workspace.Workspace).CodeDir,
}

var tierExts = map[string]string{
	"sea": ".md", "ship": ".md", "river": ".md", "spring": ".md", "code": ".go",
}
```

```go
	for _, tier := range []string{"sea", "ship", "river", "spring"} {
```

`internal/command/tier_view.go`（riverBinds 之后加）：

```go
// shipBinds 取知识点 ship 层绑定（加工中条目分组用）
func shipBinds(p *roadmap.Point) []string { return p.Ship }
```

`pickTierEntryGrouped` 的 binds switch 加一例：

```go
		binds := riverBinds
		switch tier {
		case "spring":
			binds = springBinds
		case "code":
			binds = codeBinds
		case "ship":
			binds = shipBinds
		}
```

`internal/command/tier_cmd.go`（newSea 之后插入）：

```go
// runShipMove 大海→船坞命令入口：选择器 → 装船核心 → 打开编辑器
func runShipMove(ctx *Ctx) error {
	it, err := pickTierEntryGrouped(ctx, "sea", strings.Join(ctx.Args, " "), "选择要装船（开始加工）的大海条目")
	if err != nil {
		return err
	}
	dstRel, err := moveToShip(ctx, it.Path)
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "已装船 %s，开始加工\n", ui.Path(dstRel))
	return editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(dstRel)))
}

// moveToShip 装船核心（不含选择器与编辑器，可直测）：移入 ship/ → 绑定迁移 sea[]→ship[] →
// 出处链改写。统一迁移管线：rename → TOML → 出处链。
func moveToShip(ctx *Ctx, srcRel string) (string, error) {
	dstRel := "Study/ship/" + filepath.Base(srcRel)
	if err := tier.MoveFile(ctx.WS.Root, srcRel, dstRel); err != nil {
		return "", err
	}
	if rm, err := loadRoadmapSoft(ctx); err == nil && rm != nil {
		if hits := rm.MoveTierBinding(srcRel, dstRel, roadmap.SeaList, roadmap.ShipList); len(hits) > 0 {
			if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
				fmt.Fprintln(ctx.Out, ui.Err("绑定落盘失败（文件已移动，可重跑 /ship move 同路径幂等修复）: "+err.Error()))
			} else {
				for _, h := range hits {
					fmt.Fprintf(ctx.Out, "绑定迁移：%s\n", ui.Path(rm.PathOf(h)))
				}
			}
		}
	}
	if n, err := tier.RewriteSourceChains(ctx.WS.Root, srcRel, dstRel); err == nil && n > 0 {
		fmt.Fprintf(ctx.Out, "出处链改写 %d 处\n", n)
	}
	return dstRel, nil
}

func newShip() Command {
	return newTierCmd("ship", "船坞：加工中条目查阅/装船/入河", []string{"move"}, func(ctx *Ctx, sub string) (bool, error) {
		if sub == "move" {
			return true, runShipMove(ctx)
		}
		return false, nil
	})
}
```

import 增补：`"github.com/bugzi/gs/internal/tier"`。

`internal/command/help.go`：Build 的命令表 newSea() 后插 `newShip()`；helpText 对应段替换为：

```
  /sea [关键词]           大海条目查阅；/sea import 整理 inbox，/sea edit 编辑
  /ship [关键词]          船坞（加工中）查阅；/ship move 装船开始加工（sea→ship，装船即开编辑器）
  /river [关键词]         河流条目查阅；/river move 宣告加工完成入河（ship→river，领号挂载）
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./... -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(command): /ship move 装船流转与 /ship 列表"`

---

### Task 6: `/river move` 入河（领号/挂载追问/99 收容）+ 删除旧 `/river add`

**Files:**
- Modify: `internal/command/tier_cmd.go`（runRiverAdd 改写为 runRiverMove；新增 findShipBinding、askMountPoint；删 bindAndReport；newRiver 重接线）
- Test: `internal/command/river_move_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/command/river_move_test.go`

```go
package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

// withStdin 用管道替换 os.Stdin 注入模拟输入（inputLine 直读 os.Stdin）
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
}

func riverMoveFixture(t *testing.T) (*workspace.Workspace, *Ctx) {
	t.Helper()
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 分支 0「并发编程」3 个知识点，point 下标 2 为目标（点位号 03）
	rmToml := "version = 2\n\n[[branch]]\ntitle = \"并发编程\"\n\n" +
		"  [[branch.point]]\n  title = \"goroutine\"\n\n" +
		"  [[branch.point]]\n  title = \"channel\"\n\n" +
		"  [[branch.point]]\n  title = \"select\"\n" +
		"  river = [\"Study/river/02-并发编程/03-首篇.md\"]\n" +
		"  ship = [\"Study/ship/b.md\"]\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	mk("Study/river/02-并发编程/03-首篇.md", "# 首篇\n")
	mk("Study/ship/b.md", "# B\n\n> 出处：Study/sea/x.md\n")
	return ws, &Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"b"}}
}

func TestRunRiverMoveBoundNumbering(t *testing.T) {
	ws, ctx := riverMoveFixture(t)
	if err := runRiverMove(ctx); err != nil { // 单候选直达，不走选择器
		t.Fatal(err)
	}
	want := "Study/river/02-并发编程/03.1-b.md"
	if _, err := os.Stat(filepath.Join(ws.Root, filepath.FromSlash(want))); err != nil {
		t.Fatalf("目标缺失 %s: %v", want, err)
	}
	rm, _ := roadmap.Load(ws.RoadmapFile())
	p := rm.Branches[0].Points[2]
	if len(p.River) != 2 || p.River[1] != want {
		t.Fatalf("River = %v, want 含 %s", p.River, want)
	}
	if len(p.Ship) != 0 {
		t.Fatalf("Ship 应已清空: %v", p.Ship)
	}
}

func TestRunRiverMoveUnmountedSkip(t *testing.T) {
	ws, ctx := riverMoveFixture(t)
	// 覆盖为无绑定的孤儿 ship 文件
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/ship/u.md", "# U\n")
	ctx.Args = []string{"u"}
	withStdin(t, "\n") // 挂载追问直接回车跳过
	if err := runRiverMove(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "river", "99-未挂载", "u.md")); err != nil {
		t.Fatalf("应落入收容区: %v", err)
	}
	rm, _ := roadmap.Load(ws.RoadmapFile())
	if len(rm.Branches[0].Points[2].Ship) == 0 {
		t.Fatal("原 ship 绑定不应被无关迁移影响") // 孤儿迁移不动任何绑定
	}
}
```

注意：`TestRunRiverMoveBoundNumbering` 里 ship 文件已被挂载（`ship = [...]`），走「已挂载领号」分支，**不触发** `askMountPoint`，无需 stdin 注入；单候选 ship 文件直达选择器。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestRunRiverMove -v`
Expected: 编译失败（runRiverMove 未定义）

- [ ] **Step 3: 实现——改写 tier_cmd.go**

删除 `runRiverAdd`（含其「全文摘录」注释）与 `bindAndReport`；新增：

```go
// findShipBinding 反查 ship 路径挂在哪个知识点
func findShipBinding(rm *roadmap.Roadmap, rel string) (roadmap.PointRef, bool) {
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			for _, s := range rm.Branches[bi].Points[pi].Ship {
				if s == rel {
					return roadmap.PointRef{bi, pi}, true
				}
			}
		}
	}
	return roadmap.PointRef{}, false
}

// askMountPoint 追问挂载知识点（关键词过滤，回车跳过）；多匹配弹选择器。返回是否命中。
// 命中后由调用方决定绑定写入哪个列表（sea/ship/river 各自的流转语义不同）。
func askMountPoint(ctx *Ctx, rm *roadmap.Roadmap) (roadmap.PointRef, bool) {
	kw, err := inputLine(ctx, "挂到知识点？（输入关键词过滤，回车跳过）")
	if err != nil {
		return roadmap.PointRef{}, false
	}
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return roadmap.PointRef{}, false
	}
	matches := rm.MatchPoints(kw)
	switch {
	case len(matches) == 0:
		fmt.Fprintln(ctx.Out, ui.Dim("没有匹配的知识点，未挂载"))
		return roadmap.PointRef{}, false
	case len(matches) == 1:
		return matches[0], true
	default:
		var pick string
		opts := make([]menuOption, 0, len(matches))
		for _, m := range matches {
			opts = append(opts, option(rm.PathOf(m), pointKey(m)))
		}
		if err := selectFrom(ctx, "多匹配，挂到哪个知识点？", opts, &pick); err != nil {
			return roadmap.PointRef{}, false
		}
		return pointParse(pick), true
	}
}

// runRiverMove 船坞→河流（宣告加工完成）：选 ship 条目 → 已挂知识点则领号改名入河；
// 未挂载追问挂载（跳过落 99-未挂载 不领号）→ 绑定迁移 ship[]→river[] → 出处链改写。
func runRiverMove(ctx *Ctx) error {
	it, err := pickTierEntryGrouped(ctx, "ship", strings.Join(ctx.Args, " "), "选择要入河（加工完成）的船坞条目")
	if err != nil {
		return err
	}
	rm, _ := loadRoadmapSoft(ctx)
	var ref roadmap.PointRef
	bound := false
	if rm != nil {
		ref, bound = findShipBinding(rm, it.Path)
		if !bound {
			if r, ok := askMountPoint(ctx, rm); ok {
				ref, bound = r, true
			}
		}
	}

	var dstRel string
	if !bound || rm == nil {
		dstRel = "Study/river/99-未挂载/" + filepath.Base(it.Path)
		if err := tier.MoveFile(ctx.WS.Root, it.Path, dstRel); err != nil {
			return err
		}
	} else {
		b := &rm.Branches[ref.Branch]
		dirRel := fmt.Sprintf("Study/river/%02d-%s", ref.Branch+1, b.Title)
		pointNum := ref.Point + 1
		title := roadmap.StripNumPrefix(filepath.Base(it.Path))
		dup := tier.NextDupSeq(rm.PointAt(ref).River, pointNum)
		var mvErr error
		for {
			dstRel = dirRel + "/" + roadmap.RiverFileName(pointNum, dup, title)
			mvErr = tier.MoveFile(ctx.WS.Root, it.Path, dstRel)
			if mvErr == nil || !strings.Contains(mvErr.Error(), "目标已存在") {
				break
			}
			dup++ // 篇序自动顺延重试
		}
		if mvErr != nil {
			return mvErr
		}
	}

	if bound && rm != nil {
		rm.MoveTierBinding(it.Path, dstRel, roadmap.ShipList, roadmap.RiverList)
		if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
			fmt.Fprintln(ctx.Out, ui.Err("绑定落盘失败（文件已移动）: "+err.Error()))
		}
	}
	if n, err := tier.RewriteSourceChains(ctx.WS.Root, it.Path, dstRel); err == nil && n > 0 {
		fmt.Fprintf(ctx.Out, "出处链改写 %d 处\n", n)
	}
	fmt.Fprintf(ctx.Out, "已入河 %s\n  （原 %s）\n", ui.Path(dstRel), ui.Dim(it.Path))
	return nil
}

func newRiver() Command {
	return newTierCmd("river", "河流：沉淀查阅/入河/编辑", []string{"move"}, func(ctx *Ctx, sub string) (bool, error) {
		if sub == "move" {
			return true, runRiverMove(ctx)
		}
		return false, nil
	})
}
```

helpText 的 `/river` 行已在 Task 5 更新。全库检索确认 `runRiverAdd`/`bindAndReport` 无残余引用（`grep -rn "runRiverAdd\|bindAndReport" internal/`）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./... -v`
Expected: 全部 PASS（旧 river add 相关测试如存在则一并删除）

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(command): /river move 入河领号流转；删除旧摘录流程"`

---

## 阶段 2 · 编号与重排（/plan renumber）

### Task 7: BuildRenumberPlan 重排计划纯函数

**Files:**
- Modify: `internal/roadmap/renumber.go`（追加）
- Test: `internal/roadmap/renumber_test.go`（追加）

- [ ] **Step 1: 写失败测试**（追加进 renumber_test.go）

```go
func TestBuildRenumberPlan(t *testing.T) {
	rm := &Roadmap{Version: Version, Branches: []Branch{
		{Title: "语言基础", Points: []Point{
			{Title: "变量", River: []string{"Study/river/01-变量.md"}},
			{Title: "切片", River: []string{
				"Study/river/01-旧目录/02-切片底层.md", // 主干序已变（该点现处 branch 0 point 1 → 应为 02-）
				"Study/river/01-旧目录/02.1-切片陷阱.md",
			}},
		}},
		{Title: "并发编程", Points: []Point{
			{Title: "select", River: []string{"Study/river/并发/select.md"}}, // 无号名 → 01-select.md
		}},
	}}
	existing := []string{
		"Study/river/01-变量.md",
		"Study/river/01-旧目录/02-切片底层.md",
		"Study/river/01-旧目录/02.1-切片陷阱.md",
		"Study/river/并发/select.md",
		"Study/river/孤儿.md", // 无绑定 → 99
		"Study/river/99-未挂载/已在收容区.md", // 已在 99 → 不动
	}
	plan := BuildRenumberPlan(rm, existing)

	wantTo := map[string]string{
		"Study/river/01-变量.md":                 "Study/river/01-语言基础/01-变量.md",
		"Study/river/01-旧目录/02-切片底层.md":        "Study/river/01-语言基础/02-切片底层.md",
		"Study/river/01-旧目录/02.1-切片陷阱.md":      "Study/river/01-语言基础/02.1-切片陷阱.md",
		"Study/river/并发/select.md":              "Study/river/02-并发编程/01-select.md",
		"Study/river/孤儿.md":                     "Study/river/99-未挂载/孤儿.md",
	}
	got := map[string]string{}
	for _, m := range plan.Moves {
		got[m.From] = m.To
	}
	for _, m := range plan.Orphans {
		got[m.From] = m.To
	}
	if len(got) != len(wantTo) {
		t.Fatalf("计划条目数 = %d, want %d\nplan = %+v", len(got), len(wantTo), plan)
	}
	for from, to := range wantTo {
		if got[from] != to {
			t.Errorf("%s → %s, want %s", from, got[from], to)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/roadmap/ -run TestBuildRenumberPlan -v`
Expected: 编译失败

- [ ] **Step 3: 实现**（renumber.go 追加）

```go
// FileMove 重排计划单次迁移（工作区相对正斜杠）
type FileMove struct{ From, To string }

// RenumberPlan 按树序全量归位 river 的确定性计划
type RenumberPlan struct {
	Moves   []FileMove // 挂载文件归位/改名（含篇序紧凑化）
	Orphans []FileMove // 未挂载文件 → 99-未挂载（保留原名）
}

// RiverDirRel / UnmountedDirName river 层与收容区（工作区相对）
const (
	RiverDirRel      = "Study/river"
	UnmountedDirName = "99-未挂载"
)

// BuildRenumberPlan 依 TOML 当前树序生成 river 全量归位计划。
// existing 为 river 层现有文件清单（ListTierFiles 输出）。纯函数，不触碰文件系统；
// From==To 的文件不进计划（幂等，可重跑）。
func BuildRenumberPlan(rm *Roadmap, existing []string) *RenumberPlan {
	bound := map[string]PointRef{}
	per := map[PointRef][]string{}
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			for _, rel := range rm.Branches[bi].Points[pi].River {
				bound[rel] = PointRef{bi, pi}
				per[PointRef{bi, pi}] = append(per[PointRef{bi, pi}], rel)
			}
		}
	}
	plan := &RenumberPlan{}
	used := map[string]bool{"": true}

	add := func(from, to string) {
		if from == to || used[to] {
			return
		}
		used[to] = true
		plan.Moves = append(plan.Moves, FileMove{from, to})
	}

	for bi := range rm.Branches {
		dirName := fmt.Sprintf("%02d-%s", bi+1, rm.Branches[bi].Title)
		for pi := range rm.Branches[bi].Points {
			rels := per[PointRef{bi, pi}]
			sort.Strings(rels)
			pointNum := pi + 1
			for k, rel := range rels {
				title := StripNumPrefix(filepath.Base(rel))
				dup := k // 树序紧凑化：0=首篇，1..=篇序
				to := RiverDirRel + "/" + dirName + "/" + RiverFileName(pointNum, dup, title)
				for used[to] { // 计划内撞名（同点同名多文件）→ 篇序顺延
					dup++
					to = RiverDirRel + "/" + dirName + "/" + RiverFileName(pointNum, dup, title)
				}
				add(rel, to)
			}
		}
	}
	for _, rel := range existing {
		if _, ok := bound[rel]; ok {
			continue
		}
		to := RiverDirRel + "/" + UnmountedDirName + "/" + filepath.Base(rel)
		if rel == to || used[to] {
			continue
		}
		used[to] = true
		plan.Orphans = append(plan.Orphans, FileMove{rel, to})
	}
	return plan
}
```

import 增补 `"sort"`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/roadmap/ -run TestBuildRenumberPlan -v`
Expected: PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(roadmap): BuildRenumberPlan 重排计划纯函数"`

---

### Task 8: ApplyRenumber 执行器 + topics.toml 改写

**Files:**
- Create: `internal/tier/apply.go`
- Test: `internal/tier/apply_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/tier/apply_test.go`

```go
package tier

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

func TestApplyRenumber(t *testing.T) {
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
	mk("Study/river/并发/select.md", "s")
	mk("Study/river/孤儿.md", "o")

	plan := &roadmap.RenumberPlan{
		Moves:   []roadmap.FileMove{{"Study/river/并发/select.md", "Study/river/02-并发编程/01-select.md"}},
		Orphans: []roadmap.FileMove{{"Study/river/孤儿.md", "Study/river/99-未挂载/孤儿.md"}},
	}
	if err := ApplyRenumber(root, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "02-并发编程", "01-select.md")); err != nil {
		t.Fatalf("归位失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "99-未挂载", "孤儿.md")); err != nil {
		t.Fatalf("收容失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Study", "river", "并发")); !os.IsNotExist(err) {
		t.Fatal("空目录应被清理")
	}
	// 幂等：重跑不报错
	if err := ApplyRenumber(root, plan); err != nil {
		t.Fatalf("重跑应幂等: %v", err)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/tier/ -run TestApplyRenumber -v`
Expected: 编译失败

- [ ] **Step 3: 实现** `internal/tier/apply.go`

```go
package tier

import (
	"os"
	"path/filepath"

	"github.com/bugzi/gs/internal/roadmap"
)

// ApplyRenumber 执行重排计划：逐条迁移文件（目标目录自动创建），迁移后清理 river 下变空的
// 一级目录。幂等：From 不存在的条目跳过，可中断重跑。
func ApplyRenumber(root string, plan *roadmap.RenumberPlan) error {
	all := append(append([]roadmap.FileMove{}, plan.Moves...), plan.Orphans...)
	for _, m := range all {
		src := filepath.Join(root, filepath.FromSlash(m.From))
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue // 已迁移过（重跑）
		}
		if err := MoveFile(root, m.From, m.To); err != nil {
			return err
		}
	}
	cleanupEmptyRiverDirs(root)
	return nil
}

// cleanupEmptyRiverDirs 删除 river 下变空的一级子目录（不动 river 根与文件）
func cleanupEmptyRiverDirs(root string) {
	riverDir := filepath.Join(root, filepath.FromSlash(roadmap.RiverDirRel))
	entries, err := os.ReadDir(riverDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(riverDir, e.Name())
		if sub, err := os.ReadDir(p); err == nil && len(sub) == 0 {
			os.Remove(p)
		}
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./internal/tier/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(tier): ApplyRenumber 重排执行与空目录清理"`

---

### Task 9: `/plan renumber` 装配（dry-run → 确认 → 执行 → 引用改写）

**Files:**
- Modify: `internal/command/plan.go:132-141`（分发 switch 加 case）
- Create: `internal/command/plan_renumber.go`（runPlanRenumber + rewriteTopics）
- Test: `internal/command/plan_renumber_test.go`（新建）

- [ ] **Step 1: 写失败测试** `internal/command/plan_renumber_test.go`

```go
package command

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

func renumberFixture(t *testing.T) (*workspace.Workspace, *Ctx) {
	t.Helper()
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rmToml := "version = 2\n\n[[branch]]\ntitle = \"并发编程\"\n\n  [[branch.point]]\n  title = \"select\"\n  river = [\"Study/river/并发/select.md\"]\n"
	if err := os.WriteFile(ws.RoadmapFile(), []byte(rmToml), 0o644); err != nil {
		t.Fatal(err)
	}
	mk("Study/river/并发/select.md", "# select\n\n> 出处：Study/sea/s.md\n")
	mk("Study/river/孤儿.md", "# 孤儿\n")
	mk(".gs/topics.toml", "[[topics]]\nname = \"演示\"\ncreated = \"2026-10-27\"\n\n[[topics.files]]\npath = \"Study/river/并发/select.md\"\nhint = \"\"\n")
	return ws, &Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"renumber"}}
}

func TestRunPlanRenumberEndToEnd(t *testing.T) {
	ws, ctx := renumberFixture(t)
	withStdin(t, "y\n") // 确认执行
	if err := runPlanRenumber(ctx); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"Study/river/02-并发编程/01-select.md", "Study/river/99-未挂载/孤儿.md"} {
		if _, err := os.Stat(filepath.Join(ws.Root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s 缺失: %v", rel, err)
		}
	}
	rm, _ := roadmap.Load(ws.RoadmapFile())
	if got := rm.Branches[0].Points[0].River; len(got) != 1 || got[0] != "Study/river/02-并发编程/01-select.md" {
		t.Fatalf("River 绑定未改写: %v", got)
	}
	ix, _ := index.Load(ws.IndexPath())
	found := false
	for _, tp := range ix.Topics {
		for _, f := range tp.Files {
			if f.Path == "Study/river/02-并发编程/01-select.md" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("topics.toml 未改写")
	}
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "river", "02-并发编程", "01-select.md"))
	if !strings.Contains(string(data), "> 出处：Study/sea/s.md") && !strings.Contains(string(data), "# select") {
		t.Fatal("文件内容丢失")
	}
	if !strings.Contains(ctx.Out.String(), "重排完成") {
		t.Fatal("应输出完成汇报")
	}
	// 幂等：重跑应报告无需重排
	withStdin(t, "y\n")
	ctx2 := &Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"renumber"}}
	if err := runPlanRenumber(ctx2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ctx2.Out.String(), "无需重排") {
		t.Fatalf("重跑应无需重排，输出：\n%s", ctx2.Out.String())
	}
}

func TestRewriteTopicsNoFileNoop(t *testing.T) {
	ws := wsForTest(t)
	n, err := rewriteTopics(ws, []roadmap.FileMove{{From: "a", To: "b"}})
	if err != nil || n != 0 {
		t.Fatalf("无索引文件应 no-op: n=%d err=%v", n, err)
	}
	if _, err := os.Stat(ws.IndexPath()); !os.IsNotExist(err) {
		t.Fatal("不应创建 topics.toml")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run "TestRunPlanRenumber|TestRewriteTopics" -v`
Expected: 编译失败

- [ ] **Step 3: 实现**

`internal/command/plan_renumber.go`（新建）：

```go
package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/tier"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// runPlanRenumber 全量重排 river 编号：dry-run 预览 → 确认 → 文件迁移 →
// roadmap river 绑定改写 → topics.toml 改写 → 全库出处链改写。幂等可重跑。
func runPlanRenumber(ctx *Ctx) error {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	existing, err := ListTierFiles(ctx.WS, "river")
	if err != nil {
		return err
	}
	plan := roadmap.BuildRenumberPlan(rm, existing)
	if len(plan.Moves) == 0 && len(plan.Orphans) == 0 {
		fmt.Fprintln(ctx.Out, "编号已与树序一致，无需重排")
		return nil
	}

	// dry-run 预览
	fmt.Fprintf(ctx.Out, "重排预览（共 %d 项迁移 + %d 项收容）：\n", len(plan.Moves), len(plan.Orphans))
	for _, m := range plan.Moves {
		fmt.Fprintf(ctx.Out, "  %s\n    → %s\n", ui.Dim(m.From), ui.Path(m.To))
	}
	for _, m := range plan.Orphans {
		fmt.Fprintf(ctx.Out, "  %s\n    → %s（未挂载收容）\n", ui.Dim(m.From), ui.Path(m.To))
	}
	if n := countPlanChainRefs(ctx.WS.Root, plan); n > 0 {
		fmt.Fprintf(ctx.Out, "出处链将同步改写 %d 处\n", n)
	}
	fmt.Fprintln(ctx.Out, ui.Dim("注意：工作区外引用（思源/Obsidian 等）不会同步"))
	ans, err := inputLine(ctx, "执行重排？（y/N）")
	if err != nil {
		return err
	}
	if ans = strings.TrimSpace(ans); ans != "y" && ans != "Y" {
		fmt.Fprintln(ctx.Out, "已取消")
		return nil
	}

	if err := tier.ApplyRenumber(ctx.WS.Root, plan); err != nil {
		return err
	}
	for _, m := range plan.Moves {
		rm.MoveTierBinding(m.From, m.To, roadmap.RiverList, roadmap.RiverList)
	}
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return fmt.Errorf("roadmap 落盘失败（文件已迁移）: %w", err)
	}
	if n, err := rewriteTopics(ctx.WS, plan.Moves); err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("topics.toml 改写失败: "+err.Error()))
	} else if n > 0 {
		fmt.Fprintf(ctx.Out, "topics.toml 改写 %d 处\n", n)
	}
	chainN := 0
	for _, m := range plan.Moves {
		if n, err := tier.RewriteSourceChains(ctx.WS.Root, m.From, m.To); err == nil {
			chainN += n
		}
	}
	for _, m := range plan.Orphans {
		if n, err := tier.RewriteSourceChains(ctx.WS.Root, m.From, m.To); err == nil {
			chainN += n
		}
	}
	if chainN > 0 {
		fmt.Fprintf(ctx.Out, "出处链改写 %d 处\n", chainN)
	}
	fmt.Fprintf(ctx.Out, "重排完成：%d 项迁移 + %d 项收容\n", len(plan.Moves), len(plan.Orphans))
	return nil
}

// countPlanChainRefs 预览阶段统计计划涉及的出处链引用总数
func countPlanChainRefs(root string, plan *roadmap.RenumberPlan) int {
	n := 0
	for _, m := range plan.Moves {
		n += tier.CountChainRefs(root, m.From)
	}
	for _, m := range plan.Orphans {
		n += tier.CountChainRefs(root, m.From)
	}
	return n
}

// rewriteTopics 沉淀索引 topics.toml 路径改写；文件不存在时 no-op（不创建）
func rewriteTopics(ws *workspace.Workspace, moves []roadmap.FileMove) (int, error) {
	if _, err := os.Stat(ws.IndexPath()); err != nil {
		return 0, nil
	}
	ix, err := index.Load(ws.IndexPath())
	if err != nil {
		return 0, err
	}
	replace := map[string]string{}
	for _, m := range moves {
		replace[m.From] = m.To
	}
	n := 0
	for ti := range ix.Topics {
		for fi := range ix.Topics[ti].Files {
			if to, ok := replace[ix.Topics[ti].Files[fi].Path]; ok {
				ix.Topics[ti].Files[fi].Path = to
				n++
			}
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, ix.Save(ws.IndexPath())
}
```

注：`rewriteTopics` 已含 `workspace` import（见上方 import 块）。

`internal/command/plan.go:132` 分发 switch 加：

```go
			case "renumber":
				return runPlanRenumber(ctx)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./... -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(command): /plan renumber 全量重排（dry-run 预览+引用同步改写）"`

---

### Task 10: 帮助文案收口 + 存量归位冒烟

**Files:**
- Modify: `internal/command/help.go`（helpText /plan 段加 renumber 行）

- [ ] **Step 1: helpText 的 /plan 段加一行**

```
  /plan renumber          按树序全量重排 river 编号（dry-run 预览，确认执行，引用同步改写）
```

- [ ] **Step 2: 全量验证**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: 全绿无告警

- [ ] **Step 3: 真机存量归位（开发者手动，AI 给指引）**

开发者在 `D:\Code\Go` 工作区执行 `gs` 后：
1. `/river move` 选中现有孤儿文件 `Study/river/02-Go核心/slice 底层结构与扩容.md` → 追问挂载 → 输入「slice」挂到 Go 语言基础 › slice 底层结构与扩容 → 应落 `Study/river/01-Go语言基础/03-slice底层结构与扩容.md`（该点 river 首篇）。
2. `/plan renumber` → 预览应显示 0 项迁移（上一步已归位）→ 确认编号无空洞。
3. `/plan` 浏览器进入该知识点，river 段可见新路径且可打开。

- [ ] **Step 4: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "docs: help 文案补 /plan renumber"`

---

## 阶段 3 · 视图增强

### Task 11: planview 详情页四段（加 ship 段）

**Files:**
- Modify: `internal/planview/state.go:21`（注释）、`state.go:127-140`（pointRows river/sea 段间插 ship 段）
- Test: `internal/planview/state_test.go`（追加用例）

- [ ] **Step 1: 写失败测试**（state_test.go 追加；fixture 沿用该文件既有 stateFixture，给某 point 预置 `Ship: []string{"Study/ship/x.md"}`）

```go
func TestPointRowsShipSection(t *testing.T) {
	m := stateFixture() // 该 fixture 中使某 point.Ship = ["Study/ship/x.md"]（不存在的文件 → [失效]）
	m.View, m.Branch, m.Point = viewPoint, 0, 0
	rows := m.pointRows()
	found, broken := false, false
	for _, r := range rows {
		if r.Kind == rowSectionHead && strings.Contains(r.Text, "ship · 加工中") {
			found = true
		}
		if r.Kind == rowNote && strings.Contains(r.Text, "Study/ship/x.md") && strings.Contains(r.Text, "[失效]") {
			broken = true
		}
	}
	if !found {
		t.Fatal("缺 ship 段")
	}
	if !broken {
		t.Fatal("失效标注缺失")
	}
}
```

（若 stateFixture 的 Exists 注入为全存在，则断言只查 ship 段与 note 行存在。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/planview/ -run TestPointRowsShipSection -v`
Expected: FAIL（缺 ship 段）

- [ ] **Step 3: 实现**

`state.go:21` 注释更新：`viewPoint // 知识点详情：spring/river/ship/sea 清单`。
`pointRows()` 中 river 段与 sea 段之间插入：

```go
	rows = append(rows, row{Kind: rowSectionHead, Text: fmt.Sprintf("ship · 加工中 ×%d", len(p.Ship))})
	for _, rel := range p.Ship {
		rows = append(rows, m.noteRow(rel))
	}
	if len(p.Ship) == 0 {
		rows = append(rows, row{Kind: rowHint, Text: "  （无；/ship move 装船后开始加工）"})
	}
```

同时 river 空段提示文案 `"  （无；/river add 摘录后可挂载）"` 改为 `"  （无；/river move 入河后自动挂载）"`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go build ./... && go test ./internal/planview/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(planview): 知识点详情页 ship 加工中段"`

---

### Task 12: /review 加工中统计

**Files:**
- Modify: `internal/command/review_stats.go`（TierStats 加字段；GatherTierStats 统计 ship；renderTierStats 输出）
- Test: `internal/command/review_stats_test.go`（追加）

- [ ] **Step 1: 写失败测试**（review_stats_test.go 追加，沿既有 fixture）

```go
func TestGatherTierStatsShip(t *testing.T) {
	/* fixture：Study/ship/ 下放 2 个 .md */
	st := GatherTierStats(ws)
	if st.ShipTotal != 2 {
		t.Fatalf("ShipTotal = %d, want 2", st.ShipTotal)
	}
	s := renderTierStats(st)
	if !strings.Contains(s, "船坞 2") {
		t.Fatalf("概览行缺船坞计数：\n%s", s)
	}
	if !strings.Contains(s, "加工中（待入河）") {
		t.Fatal("缺加工中提示段")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/command/ -run TestGatherTierStatsShip -v`
Expected: 编译失败

- [ ] **Step 3: 实现**

TierStats 加字段：

```go
	ShipTotal    int
	ShipFiles    []string // 加工中（待入河）
```

GatherTierStats 末尾（spring 统计前）加：

```go
	shipFiles, _ := ListTierFiles(ws, "ship")
	st.ShipFiles = shipFiles
	st.ShipTotal = len(shipFiles)
```

renderTierStats：概览行改为

```go
	fmt.Fprintf(&b, "三层概览：大海 %d（未摘录 %d）· 船坞 %d（加工中）· 河流 %d（待改写 %d）· 泉水 %d\n",
		st.SeaTotal, len(st.SeaUncited), st.ShipTotal, st.RiverTotal, len(st.RiverUncited), st.SpringTotal)
```

尾部追加提示段：

```go
	if st.ShipTotal > 0 {
		b.WriteString("\n加工中（待入河，/river move 宣告完成）：\n")
		for _, f := range st.ShipFiles {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
```

（概览行首词「三层概览」措辞保留，避免 review.go 调用方文案连锁改动；含义扩展为四站。）

- [ ] **Step 4: 跑测试确认通过 + 全量收口**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: 全绿

- [ ] **Step 5: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "feat(review): 加工中统计注入体检输出"`

---

### Task 13: 端到端真机冒烟（开发者主导，AI 指引）

- [ ] **Step 1: 冒烟清单**（在 `D:\Code\Go` 工作区，`gs` REPL 内）

1. `/ship move` 挑一篇 sea 文件 → 确认移入 `Study/ship/`、树上该点（若已绑定）ship 段可见、编辑器自动打开
2. 编辑器内加工保存退出 → `/plan` 浏览器该点 ship 段仍在
3. `/river move` 选该文件 → 已挂载则领号入河；未挂载则追问挂载或跳过入 99
4. `/plan` 详情页 river 段可见新路径、`Enter` 打开正常；`o` 键出处回溯若指向已迁移文件应已改写可跳
5. `/plan renumber` → 预览清单合理 → y 执行 → 重跑一次应显示「无需重排」（幂等）
6. `/review` 输出含船坞计数
7. 手动把一个 ship 文件移回 sea + 手编 TOML 清 ship[]（范围外兜底路径验证可选）

- [ ] **Step 2: 全量回归**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: 全绿

- [ ] **Step 3: 提交检查点（开发者手动执行）**

建议：`git add -A && git commit -m "chore: 四站流转与 river 编号管理收尾"`

---

## 执行偏差记录

- **T5/T6/T9 测试方案调整（2026-10-27 实施中）**：计划原定 `withStdin`（os.Pipe）注入 `inputLine`——实施发现 `inputLine` 走 huh/bubbletea TUI，管道 stdin 下 Program 挂死（现有测试从未覆盖交互流）。改为**核心函数拆分直测**：`runShipMove` 拆 `moveToShip`、`runRiverMove` 拆 `riverLanding`（落点计算+迁移，无 UI）、`runPlanRenumber` 拆 `buildRenumberPreview`+`applyRenumberPlan`；交互门（追问挂载、y/N 确认）留在薄壳，由 T13 真机冒烟覆盖。与项目规则 4「命令层纯逻辑与交互壳分离」一致。
- **包名修正**：T6 代码 `tier.StripNumPrefix`/`tier.NextDupSeq` 实为 roadmap 包函数（计划笔误），实施中已按 `roadmap.` 落地。
- **并行会话注意**：实施期间检测到 `internal/repl/repl.go` 被并行会话编辑中（portCheck/drainPort）；本计划范围与其零重叠，构建门改为按包验证，全量 `go build ./...` 留待 T13 收口。

## 自审记录（计划完成后已核）

- Spec 覆盖：ship 字段（T1）、/ship move 与列表（T5）、/river move 领号/挂载/99（T6）、renumber dry-run+引用改写（T7-9）、存量归位（T10）、四段视图（T11）、review 统计（T12）、范围外兜底（T13 冒烟 7）。
- 绑定迁移管线三步（rename→TOML→出处链）在 T5/T6/T9 一致落地。
- 命名一致性：`MoveTierBinding`/`SeaList|ShipList|RiverList`/`StripNumPrefix`/`NextDupSeq`/`RiverFileName`（roadmap 包）/`MoveFile`/`RewriteSourceChains`/`CountChainRefs`/`ApplyRenumber`（tier 包）/`BuildRenumberPlan`（roadmap 包）/`rewriteTopics`（command 包）全计划统一；T6 已修正为 `roadmap.StripNumPrefix`。
- 测试可落地性：命令层测试统一用 `wsForTest` + `withStdin`（os.Pipe）+ 单候选直达选择器；`runShipMove` 拆出 `moveToShip` 核心绕开 `editOpen`；`runRiverMove`/`runPlanRenumber` 无 editOpen 可整命令直测。
- 交互边界：挂载追问仅在未绑定分支触发；多匹配弹 `plainSelect`（非 TTY 降级编号列表）在测试中不覆盖（单匹配夹具）。
