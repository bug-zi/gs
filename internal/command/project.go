package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/edit"
	"github.com/bugzi/gs/internal/structure"
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
func projectOptions(projects []workspace.Project) ([]menuOption, bool) {
	hasGS := false
	opts := make([]menuOption, 0, len(projects)+1)
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
		opts := []menuOption{
			option("README", "README"),
			option("框架树", "框架树"),
			option("项目日志", "项目日志"),
			option("学习笔记", "学习笔记"),
			option("升级计划", "升级计划"),
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
		case "框架树":
			if err := showProjectTree(ctx, p); err != nil {
				return err
			}
		case "项目日志":
			if err := showProjectLog(ctx, p); err != nil {
				return err
			}
		case "学习笔记":
			if err := showProjectNotes(ctx, p); err != nil {
				return err
			}
		case "升级计划":
			if err := showProjectPlan(ctx, p); err != nil {
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
	if latest := latestLearnDate(dates, info); latest != "" {
		fmt.Fprintf(&b, "- 最近学习：%s\n", latest)
	} else {
		b.WriteString("- 最近学习：（无记录）\n")
	}
	if len(dates) > 0 {
		fmt.Fprintf(&b, "- 学习日志：%d 篇（最近 %s）\n", len(dates), dates[0].Display)
	} else {
		b.WriteString("- 学习日志：0 篇\n")
	}
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
	opts := []menuOption{option("返回", "")}
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

// openInIDE 用 IDE 打开文件：未显式配置 [project] opener 时走编辑器解析链
// （[ui] editor > $EDITOR > trae > code > notepad，trae/code 自动 --wait），阻塞到编辑器关闭；
// 显式配置的 opener 按原语义异步启动（其可用性由配置自担）。
// 默认 trae 曾因 PATH 上的失效包装脚本（指向旧安装路径）静默无响应，故改为解析链兜底。
func openInIDE(ctx *Ctx, path string) error {
	if opener := ctx.Cfg.Project.Opener; opener != "" {
		if _, err := exec.LookPath(opener); err != nil {
			return fmt.Errorf("未找到 %s 命令，请安装 IDE 或在 ~/.gs/config.toml [project] opener 配置", opener)
		}
		return exec.Command(opener, path).Start()
	}
	return edit.OpenExternal(path, ctx.Cfg)
}

// showProjectNotes 两处聚合笔记浏览：阅读 / 编辑 / 打开对应源文件
func showProjectNotes(ctx *Ctx, p workspace.Project) error {
	projNotes, seaNotes := structure.CollectNotes(p.Path, filepath.Join(ctx.WS.SeaDir(), p.Name))
	if len(projNotes)+len(seaNotes) == 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("（无学习笔记）"))
		return nil
	}
	var opts []menuOption
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
	var actOpts []menuOption
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

// filterSubtree 只保留 dir 子树内（含 dir 自身）的节点
func filterSubtree(entries []structure.Entry, dir string) []structure.Entry {
	prefix := dir + "/"
	kept := entries[:0]
	for _, e := range entries {
		if e.Rel == dir || strings.HasPrefix(e.Rel, prefix) {
			kept = append(kept, e)
		}
	}
	return kept
}

// splitBatches 缺失节点按父目录分组；返回自上而下排序的父目录列表与分组映射
// （字典序即自上而下：父路径是子路径前缀，必排在前）
func splitBatches(missing []structure.Entry) ([]string, map[string][]structure.Entry) {
	byDir := map[string][]structure.Entry{}
	for _, t := range missing {
		parent := parentDir(t.Rel)
		byDir[parent] = append(byDir[parent], t)
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, byDir
}

// renderTree 当前目录整棵子树的树状文本（纯函数）：多层缩进排版 + 介绍 + 笔记标记。
// 整体包进代码围栏，防止 markdown 渲染把换行折叠成段落。
func renderTree(projectName, dir, projectRoot string, exclude []string,
	cache *structure.StructureCache, matched map[string][]structure.Note) string {
	var b strings.Builder
	b.WriteString("框架树 · " + projectName)
	if dir != "" {
		b.WriteString("　" + dir + "/")
	}
	b.WriteString("\n\n```text\n")
	var walk func(prefix, indent string)
	walk = func(prefix, indent string) {
		entries := structure.Scan(filepath.Join(projectRoot, filepath.FromSlash(prefix)), prefix, exclude)
		for i, e := range entries {
			branch, child := "├── ", "│   "
			if i == len(entries)-1 {
				branch, child = "└── ", "    "
			}
			name := e.Name
			if e.IsDir {
				name += "/"
			}
			line := indent + branch + name
			if d := cache.Find(e.Rel); d != "" {
				line += "　" + d
			}
			if n := len(matched[e.Rel]); n > 0 {
				line += fmt.Sprintf("　·笔%d", n)
			}
			b.WriteString(line + "\n")
			if e.IsDir {
				walk(e.Rel, indent+child)
			}
		}
	}
	walk(dir, "")
	b.WriteString("```\n")
	return b.String()
}

// allProjectNotes 聚合项目两处笔记（供树标记与跳转共用）
func allProjectNotes(ctx *Ctx, p workspace.Project) []structure.Note {
	a, b := structure.CollectNotes(p.Path, filepath.Join(ctx.WS.SeaDir(), p.Name))
	return append(append([]structure.Note{}, a...), b...)
}

// openerName 菜单展示用的打开器名称：显式 opener 原样；否则解析链命中的编辑器名
func openerName(ctx *Ctx) string {
	if opener := ctx.Cfg.Project.Opener; opener != "" {
		return opener
	}
	if ed := edit.Resolve(ctx.Cfg); ed != "" {
		return strings.TrimSuffix(filepath.Base(ed), filepath.Ext(ed))
	}
	return "外部编辑器"
}

// indentBlock 多行文本整体缩进
func indentBlock(s, pad string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

// showProjectTree 框架树浏览：缓存缺介绍时 AI 按批续生成（断点续传，缓存非空也续），进入交互树视图
func showProjectTree(ctx *Ctx, p workspace.Project) error {
	cache, err := structure.LoadStructureCache(structure.StructurePath(ctx.WS, p.Name))
	if err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("介绍缓存解析失败，可重新生成覆盖"))
		cache = &structure.StructureCache{}
	}
	if aiConfigured(ctx) {
		if n := len(cache.Missing(structure.CollectTargets(p.Path, ctx.Cfg.Scan.Exclude))); n > 0 {
			fmt.Fprintln(ctx.Out, ui.Dim(fmt.Sprintf("（续生成介绍… 缺 %d 节点）", n)))
			failed, err := genTreeDescriptions(ctx, p, cache, "")
			if err != nil {
				fmt.Fprintln(ctx.Out, ui.Err("AI 生成介绍失败："+err.Error()))
			} else if len(failed) > 0 {
				fmt.Fprintln(ctx.Out, ui.Err(fmt.Sprintf("%d 批介绍生成失败，可按 r 重试", len(failed))))
			} else {
				fmt.Fprintln(ctx.Out, ui.Dim("已缓存至 " + ctx.WS.Rel(structure.StructurePath(ctx.WS, p.Name))))
			}
		}
	}
	return runTreeBrowser(ctx, p, cache)
}

// fileActions 文件节点操作：IDE 打开 / 查看相关笔记 / 返回
func fileActions(ctx *Ctx, p workspace.Project, rel string) error {
	abs := filepath.Join(p.Path, filepath.FromSlash(rel))
	matched := structure.MatchNotes(structure.WalkGoFiles(p.Path, ctx.Cfg.Scan.Exclude), allProjectNotes(ctx, p))
	var opts []menuOption
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
		var noteOpts []menuOption
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

// genTreeDescriptions 生成 dir 子树内缺失节点的介绍：按目录分批，只询问缓存缺失者，
// 单批失败记录后继续其余批次，每批成功即并入缓存保存；返回失败批次目录列表。
func genTreeDescriptions(ctx *Ctx, p workspace.Project, cache *structure.StructureCache, dir string) ([]string, error) {
	st, err := ensureAIState(ctx)
	if err != nil {
		return nil, err
	}
	missing := cache.Missing(structure.CollectTargets(p.Path, ctx.Cfg.Scan.Exclude))
	if dir != "" {
		missing = filterSubtree(missing, dir)
	}
	if len(missing) == 0 {
		return nil, nil
	}
	dirs, byDir := splitBatches(missing)
	var failed []string
	for _, d := range dirs {
		label := d
		if label == "" {
			label = "（根目录）"
		}
		fmt.Fprintln(ctx.Out, ui.Dim("（AI 分析中… "+label+"）"))
		nodes, err := genLevelDescriptions(ctx, st, p, d, byDir[d])
		if err != nil {
			failed = append(failed, d)
			continue
		}
		cache.Merge(nodes)
		if err := cache.Save(structure.StructurePath(ctx.WS, p.Name)); err != nil {
			return failed, err
		}
	}
	return failed, nil
}

// genLevelDescriptions AI 生成单层各节点的一两句话介绍
func genLevelDescriptions(ctx *Ctx, st *AIState, p workspace.Project,
	dir string, entries []structure.Entry) ([]structure.Node, error) {
	budget := 16000
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "- %s\n", e.Rel)
		if !e.IsDir {
			if head := fileHead(filepath.Join(p.Path, filepath.FromSlash(e.Rel)), 40, &budget); head != "" {
				fmt.Fprintf(&b, "  文件开头:\n%s\n", indentBlock(head, "    "))
			}
		}
	}
	sys := "你是代码导览助手。清单即全部待介绍节点：目录按所含内容写概览，.go 文件结合文件开头内容写职责。为每个节点写一两句话功能介绍：中文、不超过 60 字。只输出 JSON 数组 [{\"path\":\"相对路径\",\"desc\":\"介绍\"}]，path 与清单一致，覆盖每个节点，不要输出其他内容。"
	label := dir
	if label == "" {
		label = "（根目录）"
	}
	reply, err := st.Client.Chat(context.Background(), []ai.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: "项目: " + p.Name + "\n当前目录: " + label + "\n文件清单:\n" + b.String()},
	})
	if err != nil {
		return nil, err
	}
	return parseDescJSON(reply)
}

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
	var opts []menuOption
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
	opts := make([]menuOption, 0, len(docs))
	for _, d := range docs {
		opts = append(opts, option(d, d))
	}
	if err := selectFrom(ctx, "选择要编辑的文档", opts, &pick); err != nil {
		return err
	}
	return editOpen(ctx, filepath.Join(p.Path, filepath.FromSlash(pick)))
}
