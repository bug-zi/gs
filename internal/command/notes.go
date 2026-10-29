package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/agent"
	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/edit/diffx"
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
	var pick string
	opts := []menuOption{option("河流 river", "river"), option("泉水 spring", "spring")}
	if err := selectFrom(ctx, "归位到哪一站？", opts, &pick); err != nil {
		return err
	}
	it, err := pickTierEntryGrouped(ctx, pick, "", "选择归位目标条目")
	if err != nil {
		return err
	}
	ok, err := confirm(ctx, fmt.Sprintf("把「%s」追加到 %s 末尾？", truncateRune(ns[i].Text, 24), it.Path))
	if err != nil || !ok {
		return err
	}
	return moveNoteCore(ctx, ns, i, it.Path)
}

// tidySystemTpl tidy 提示词：任务规则与 JSON 输出约定
const tidySystemTpl = `你是 gs 学习工作台的便签整理助手。用户有若干临时便签，需要把每条归位到已有的 river（沉淀笔记）或 spring（知识点总结）文件中。

规则：
1. 只能归位到候选清单中列出的现有文件，不得发明路径，不得新建文件。
2. 没有合适归宿的便签不要出现在方案里（会留在便签缓存中）。
3. content 必须是目标文件的完整新内容：保留原文，把便签内容改写融入最合适的位置，语言风格与原文一致。
4. 同一目标文件归位多条便签时，各 placement 的 content 累计包含前序 placement 的改动。
5. 先用工具检索阅读候选文件再判断；不确定就不归位。

候选文件清单：
%s

待归位便签：
%s

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
