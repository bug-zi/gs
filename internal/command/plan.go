package command

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/bugzi/gs/internal/planview"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

type KV struct{ Key, Value string }

type State struct {
	UpdateLine  string   // 更新时间：开头那一行
	Breakpoints []KV     // 「## 当前断点」小节内的 - 键：值 行
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

func timeNowCST() time.Time { return time.Now().In(workspace.CST) }

// planSubCmds plan 子命令；其余首参视为主干过滤关键词
var planSubCmds = []string{"status", "next", "edit", "done", "doing", "todo", "add"}

func newPlan() Command {
	return Command{
		Name:  "plan",
		Short: "查看/更新学习路线图与断点",
		Subs:  planSubCmds,
		Run: func(ctx *Ctx) error {
			if len(ctx.Args) > 0 {
				switch ctx.Args[0] {
				case "status":
					return runPlanStatus(ctx)
				case "next":
					return runPlanNext(ctx, strings.Join(ctx.Args[1:], " "))
				case "edit":
					return runPlanEdit(ctx)
				case "done", "doing", "todo":
					return runPlanSetStatus(ctx, ctx.Args[0], strings.Join(ctx.Args[1:], " "))
				case "renumber":
				return runPlanRenumber(ctx)
			case "add":
					return runPlanAdd(ctx, ctx.Args[1:])
				}
			}
			return runPlanBrowser(ctx, strings.Join(ctx.Args, " "))
		},
	}
}

// runPlanStatus 学习状态速览（原默认视图迁入，逻辑不变）
func runPlanStatus(ctx *Ctx) error {
	return runViewPager(ctx,
		func() string {
			data, err := os.ReadFile(ctx.WS.StateFile())
			if err != nil {
				return "（读取失败: " + err.Error() + "）"
			}
			var out strings.Builder
			renderPlan(&out, ParseState(string(data)))
			return ui.RenderMD(out.String())
		},
		func() (string, bool) { return ctx.WS.StateFile(), true })
}

// runPlanNext 更新学习状态.md 的「下一步」（原 next 分支原样抽出）
func runPlanNext(ctx *Ctx, next string) error {
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
}

// runPlanEdit 编辑计划文件；roadmap.toml 存在时列入首位（DOCX 提示语保留）
func runPlanEdit(ctx *Ctx) error {
	entries, err := os.ReadDir(ctx.WS.PlanDir())
	if err != nil {
		return err
	}
	var mds []string
	if _, err := os.Stat(ctx.WS.RoadmapFile()); err == nil {
		mds = append(mds, "roadmap.toml")
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			mds = append(mds, e.Name())
		}
	}
	if len(mds) == 0 {
		return errors.New("plan 目录下没有可编辑文件")
	}
	var choice string
	opts := make([]menuOption, 0, len(mds))
	for _, m := range mds {
		opts = append(opts, option(m, m))
	}
	if err := selectFrom(ctx, "选择要编辑的计划文件（DOCX 请用 Office 打开）", opts, &choice); err != nil {
		return err
	}
	return editOpen(ctx, filepath.Join(ctx.WS.PlanDir(), choice))
}

// loadRoadmap 读入路线图；未生成时报错并给 status 替代入口，不自动创建
func loadRoadmap(ctx *Ctx) (*roadmap.Roadmap, error) {
	path := ctx.WS.RoadmapFile()
	if _, err := os.Stat(path); err != nil {
		return nil, errors.New("路线图尚未生成（Study/plan/roadmap.toml 不存在），可先用 /plan status 查看学习状态")
	}
	rm, err := roadmap.Load(path)
	if err != nil {
		return nil, fmt.Errorf("%v\n可用 /plan edit 修复 roadmap.toml", err)
	}
	return rm, nil
}

// runPlanBrowser /plan 默认入口：交互浏览器；kw 非空时定位主干；raw 不可用降级静态树
func runPlanBrowser(ctx *Ctx, kw string) error {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	if kw != "" && len(rm.MatchBranches(kw)) == 0 {
		names := make([]string, 0, len(rm.Branches))
		for i := range rm.Branches {
			names = append(names, rm.Branches[i].Title)
		}
		return fmt.Errorf("没有匹配 %q 的主干；可用主干：%s", kw, strings.Join(names, "、"))
	}
	path := ctx.WS.RoadmapFile()
	err = planview.Run(ctx.Out, rm, ctx.WS.Root, kw, planview.Hooks{
		OpenNote: func(rel string) {
			_ = openPlanNote(ctx, rel)
		},
		Save: func(rm *roadmap.Roadmap) error { return roadmap.Save(path, rm) },
	})
	if errors.Is(err, planview.ErrNonInteractive) {
		var st *State
		if data, err := os.ReadFile(ctx.WS.StateFile()); err == nil {
			s := ParseState(string(data))
			st = &s
		}
		fmt.Fprint(ctx.Out, planTreeText(rm, st, kw))
		return nil
	}
	return err
}

// openPlanNote plan 浏览器打开笔记（Enter/o 共用）；
// 渲染内容即文件本身（.md 走 Markdown、.go 走代码高亮），
// 长短分流复用 runViewPager（>20 行转交 TRAE 查阅+编辑）
func openPlanNote(ctx *Ctx, rel string) error {
	abs := filepath.Join(ctx.WS.Root, filepath.FromSlash(rel))
	return runViewPager(ctx,
		func() string {
			if strings.EqualFold(filepath.Ext(rel), ".go") {
				return ui.Path(rel) + "\n" + ui.RenderGo(readFileString(abs))
			}
			return ui.Path(rel) + "\n" + ui.RenderMD(readFileString(abs))
		},
		func() (string, bool) { return abs, true })
}

// loadRoadmapSoft 路线图不存在时返回 nil 不报错，供可选挂载路径
func loadRoadmapSoft(ctx *Ctx) (*roadmap.Roadmap, error) {
	if _, err := os.Stat(ctx.WS.RoadmapFile()); err != nil {
		return nil, nil
	}
	return roadmap.Load(ctx.WS.RoadmapFile())
}

// stateNext 取学习状态「下一步」键值，无则空串
func stateNext(st State) string {
	for _, kv := range st.Breakpoints {
		if kv.Key == "下一步" {
			return kv.Value
		}
	}
	return ""
}

// planColor 状态行配色：doing 金 220 / done 沙 144 / todo 灰 245（同族低饱和）
func planColor(status, line string) string {
	c := "245"
	switch status {
	case roadmap.StatusDoing:
		c = "220"
	case roadmap.StatusDone:
		c = "144"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(line)
}

// planTreeText 组装树视图文本（渲染 + 断点摘要降级），纯逻辑可测
func planTreeText(rm *roadmap.Roadmap, st *State, kw string) string {
	next := "（未找到学习状态.md）"
	if st != nil {
		if v := stateNext(*st); v != "" {
			next = "下一步：" + v
		} else {
			next = "（学习状态.md 没有「下一步」断点）"
		}
	}
	done, total := rm.DonePoints(), rm.TotalPoints()
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	var idx []int
	if kw != "" {
		idx = rm.MatchBranches(kw)
	}
	return roadmap.Render(rm, roadmap.RenderOpts{
		Title:     "Go 学习路线图",
		Progress:  fmt.Sprintf("进度 %d/%d (%d%%)", done, total, pct),
		Next:      next,
		BranchIdx: idx,
		Color:     planColor,
	})
}

// runPlanAdd 建树期录节点：plan add branch|point，缺参交互补全
func runPlanAdd(ctx *Ctx, args []string) error {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	kind := ""
	if len(args) > 0 {
		kind = args[0]
		args = args[1:]
	}
	switch kind {
	case "branch":
		title, err := takeArg(ctx, args, "主干标题")
		if err != nil {
			return err
		}
		if err := rm.AddBranch(title); err != nil {
			return err
		}
	case "point":
		bi, rest, err := pickBranchByArgs(ctx, rm, args)
		if err != nil {
			return err
		}
		title := ""
		group := ""
		if len(rest) > 0 {
			title = rest[0]
			group = strings.TrimSpace(strings.Join(rest[1:], " "))
		}
		if strings.TrimSpace(title) == "" {
			title, err = inputLine(ctx, "知识点标题")
			if err != nil {
				return err
			}
		}
		if err := rm.AddPoint(bi, strings.TrimSpace(title), group, nil); err != nil {
			return err
		}
	default:
		return fmt.Errorf("未知类型 %q（支持 branch / point）", kind)
	}
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return err
	}
	fmt.Fprintln(ctx.Out, "已添加并落盘 roadmap.toml")
	return nil
}

// takeArg 取剩余参数作标题；为空则交互输入
func takeArg(ctx *Ctx, args []string, label string) (string, error) {
	if s := strings.TrimSpace(strings.Join(args, " ")); s != "" {
		return s, nil
	}
	return inputLine(ctx, label)
}

// pickBranchByArgs 首参作主干关键词定位（唯一直达/多匹配选/零匹配报错）；无参交互选。
// 返回选中的主干下标与剩余参数。
func pickBranchByArgs(ctx *Ctx, rm *roadmap.Roadmap, args []string) (int, []string, error) {
	if len(args) == 0 {
		bi, err := pickBranchIdx(ctx, rm)
		return bi, nil, err
	}
	ms := rm.MatchBranches(args[0])
	switch len(ms) {
	case 0:
		return 0, nil, fmt.Errorf("没有匹配 %q 的主干", args[0])
	case 1:
		return ms[0], args[1:], nil
	}
	bi, err := pickBranchFrom(ctx, rm, ms, "匹配到多个主干")
	if err != nil {
		return 0, nil, err
	}
	return bi, args[1:], nil
}

// runPlanSetStatus 快捷标记：关键词唯一直达、多匹配弹选；浏览器内 d/s/t 才是主路径
func runPlanSetStatus(ctx *Ctx, status, kw string) error {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	if kw == "" {
		return errors.New("请给知识点关键词；或在 /plan 浏览器中用 d/s/t 原位标记")
	}
	matches := rm.MatchPoints(kw)
	var ref roadmap.PointRef
	switch len(matches) {
	case 0:
		return fmt.Errorf("没有匹配 %q 的知识点", kw)
	case 1:
		ref = matches[0]
	default:
		var pick string
		opts := make([]menuOption, 0, len(matches))
		for _, m := range matches {
			opts = append(opts, option(rm.PathOf(m)+"（当前 "+rm.PointAt(m).Status+"）", pointKey(m)))
		}
		if err := selectFrom(ctx, "多匹配，选择知识点", opts, &pick); err != nil {
			return err
		}
		ref = pointParse(pick)
	}
	if err := rm.SetStatus(ref, status); err != nil {
		return err
	}
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return err
	}
	done, total := rm.DonePoints(), rm.TotalPoints()
	fmt.Fprintf(ctx.Out, "已标记 [%s] %s（进度 %d/%d）\n", status, rm.PointAt(ref).Title, done, total)
	return nil
}

// locatePoint 知识点定位：关键词唯一直达、多匹配弹选、空关键词全量弹选
func locatePoint(ctx *Ctx, rm *roadmap.Roadmap, kw string) (roadmap.PointRef, error) {
	matches := rm.MatchPoints(kw)
	switch len(matches) {
	case 0:
		if kw == "" {
			return roadmap.PointRef{}, errors.New("路线图没有任何知识点，可用 /plan add point 添加")
		}
		return roadmap.PointRef{}, fmt.Errorf("没有匹配 %q 的知识点", kw)
	case 1:
		return matches[0], nil
	}
	var pick string
	opts := make([]menuOption, 0, len(matches))
	for _, m := range matches {
		opts = append(opts, option(rm.PathOf(m)+"（当前 "+rm.PointAt(m).Status+"）", pointKey(m)))
	}
	if err := selectFrom(ctx, "选择知识点", opts, &pick); err != nil {
		return roadmap.PointRef{}, err
	}
	return pointParse(pick), nil
}

func pointKey(r roadmap.PointRef) string { return fmt.Sprintf("%d/%d", r.Branch, r.Point) }

func pointParse(s string) roadmap.PointRef {
	var r roadmap.PointRef
	fmt.Sscanf(s, "%d/%d", &r.Branch, &r.Point)
	return r
}

// pickBranchIdx 无参纯交互选主干
func pickBranchIdx(ctx *Ctx, rm *roadmap.Roadmap) (int, error) {
	all := make([]int, len(rm.Branches))
	for i := range rm.Branches {
		all[i] = i
	}
	return pickBranchFrom(ctx, rm, all, "选择主干")
}

// pickBranchFrom 在候选主干下标中弹选择
func pickBranchFrom(ctx *Ctx, rm *roadmap.Roadmap, ids []int, label string) (int, error) {
	var pick string
	opts := make([]menuOption, 0, len(ids))
	for _, i := range ids {
		done, total := rm.BranchProgress(i)
		opts = append(opts, option(fmt.Sprintf("%s（%d/%d）", rm.Branches[i].Title, done, total), fmt.Sprintf("%d", i)))
	}
	if err := selectFrom(ctx, label, opts, &pick); err != nil {
		return 0, err
	}
	var bi int
	fmt.Sscanf(pick, "%d", &bi)
	return bi, nil
}
