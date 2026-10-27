package command

import (
	"fmt"
	"os"
	"path/filepath"
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

func newView(topic string, indexed bool) *showView {
	v := &showView{Topic: topic, Indexed: indexed}
	for _, t := range []string{"概览", "知识", "代码", "日志", "面试题"} {
		v.Sections = append(v.Sections, &showSection{Title: t})
	}
	return v
}

// buildShowView 索引优先，否则实时扫描
func buildShowView(ws *workspace.Workspace, cfg *config.Config, ix *index.Index, topic string) *showView {
	if tp := ix.Find(topic); tp != nil {
		return buildIndexedView(ws, tp)
	}
	return buildScanView(ws, cfg, topic)
}

func buildScanView(ws *workspace.Workspace, cfg *config.Config, topic string) *showView {
	v := newView(topic, false)
	results, _, err := search.Search(ws.Root, cfg, topic)
	if err != nil {
		v.Sections[0].Body = "扫描失败: " + err.Error()
		return v
	}
	var coreFiles, seaFiles, codeFiles, logFiles, interviewFiles []showFile
	var coreBody, seaBody, codeBodyB, logBody, interviewBody strings.Builder
	for _, r := range results {
		lp := strings.ToLower(r.Path)
		f := showFile{Path: r.Path}
		switch {
		case strings.HasSuffix(lp, ".md") && (strings.Contains(lp, "interview") || strings.Contains(lp, "面试")):
			// 面试题优先于知识节，保证 sea/interview 正本落面试题节
			interviewFiles = append(interviewFiles, f)
			fmt.Fprintf(&interviewBody, "- %s（%d 处命中）\n", r.Path, len(r.Hits))
		case strings.HasPrefix(lp, "study/river/") || strings.HasPrefix(lp, "study/spring/"):
			coreFiles = append(coreFiles, f)
			if data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(r.Path))); err == nil {
				fmt.Fprintf(&coreBody, "【%s】\n%s\n", r.Path, truncateLines(string(data), 60))
			}
		case strings.HasPrefix(lp, "study/sea/"):
			seaFiles = append(seaFiles, f)
			if data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(r.Path))); err == nil {
				fmt.Fprintf(&seaBody, "【%s】\n%s\n", r.Path, truncateLines(string(data), 60))
			}
		case strings.HasSuffix(lp, ".go"):
			codeFiles = append(codeFiles, f)
			fmt.Fprintf(&codeBodyB, "- %s（%d 处命中）\n", r.Path, len(r.Hits))
		case strings.HasPrefix(lp, "study/logs/") && strings.HasSuffix(lp, ".md"):
			logFiles = append(logFiles, f)
			if data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(r.Path))); err == nil {
				for _, l := range strings.Split(string(data), "\n") {
					if strings.Contains(strings.ToLower(l), strings.ToLower(topic)) {
						fmt.Fprintln(&logBody, l)
					}
				}
			}
		}
	}
	knowFiles := append(append([]showFile{}, coreFiles...), seaFiles...)
	knowBody := coreBody.String() + seaBody.String()
	v.Sections[0].Body = fmt.Sprintf("主题 %q（实时扫描）：知识 %d · 代码 %d · 日志 %d · 面试题 %d\n按 s 可沉淀为索引，下次直达。",
		topic, len(knowFiles), len(codeFiles), len(logFiles), len(interviewFiles))
	if len(codeFiles) > 0 {
		codeBodyB.WriteString("\n（演示时用 o 打开对应文件，结合行号讲解）\n")
	}
	v.Sections[1].Body, v.Sections[1].Files = knowBody, knowFiles
	v.Sections[2].Body, v.Sections[2].Files = codeBodyB.String(), codeFiles
	v.Sections[3].Body, v.Sections[3].Files = logBody.String(), logFiles
	v.Sections[4].Body, v.Sections[4].Files = interviewBody.String(), interviewFiles
	return v
}

func buildIndexedView(ws *workspace.Workspace, tp *index.Topic) *showView {
	v := newView(tp.Name, true)
	var overview strings.Builder
	fmt.Fprintf(&overview, "沉淀主题 %q（建立于 %s）\n\n演示提示：\n", tp.Name, tp.Created)
	var knowFiles, codeFiles []showFile
	var knowBody, codeBodyB strings.Builder
	for _, f := range tp.Files {
		item := showFile{Path: f.Path, Hint: f.Hint}
		if f.Hint != "" {
			fmt.Fprintf(&overview, "- %s：%s\n", f.Path, f.Hint)
		}
		if strings.HasSuffix(f.Path, ".go") {
			codeFiles = append(codeFiles, item)
			fmt.Fprintf(&codeBodyB, "- %s%s\n", f.Path, hintSuffix(f.Hint))
		} else {
			knowFiles = append(knowFiles, item)
			data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(f.Path)))
			if err != nil {
				fmt.Fprintf(&knowBody, "【%s】（读取失败）\n", f.Path)
				continue
			}
			fmt.Fprintf(&knowBody, "【%s】%s\n%s\n", f.Path, hintSuffix(f.Hint), truncateLines(string(data), 60))
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
	v.Sections[1].Body, v.Sections[1].Files = knowBody.String(), knowFiles
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

// showSave 把视图沉淀为索引条目
func showSave(ws *workspace.Workspace, v *showView) error {
	ix, err := index.Load(ws.IndexPath())
	if err != nil {
		return err
	}
	if ix.Find(v.Topic) != nil {
		return fmt.Errorf("主题 %q 已在索引中（/show edit 可维护）", v.Topic)
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
	ix.Topics = append(ix.Topics, tp)
	return ix.Save(ws.IndexPath())
}

// runShowPager 逐节浏览；o 打开当前节文件，s 沉淀，q/Esc 退出
func runShowPager(ctx *Ctx, v *showView) error {
	cur := 0
	src, sess := newPagerKeySource()
	defer sess.close()
	for {
		sec := v.Sections[cur]
		body := sec.Body
		if strings.TrimSpace(body) == "" {
			body = "（无相关内容）"
		} else {
			body = ui.RenderMD(body)
		}
		fmt.Fprintf(ctx.Out, "\n═══ [%d/5] %s ═══\n", cur+1, sec.Title)
		if h, ok := ui.PageSize(); ok {
			ui.PagedPrint(ctx.Out, src.next, body, h)
		} else {
			fmt.Fprintf(ctx.Out, "%s\n", body)
		}
		fmt.Fprint(ctx.Out, ui.Dim("[n]下一节 [p]上一节 [o]打开文件 [s]沉淀为索引 [q/Esc]退出 > "))
		line, ok := src.next()
		if !ok {
			return nil
		}
		switch line {
		case "n", "":
			if cur < 4 {
				cur++
			}
		case "p":
			if cur > 0 {
				cur--
			}
		case "q", "esc", "ctrl+c", "ctrl+d":
			return nil
		case "o":
			if len(sec.Files) == 0 {
				fmt.Fprintln(ctx.Out, ui.Dim("本节没有可打开的文件"))
				continue
			}
			opts := make([]menuOption, 0, len(sec.Files))
			for _, f := range sec.Files {
				opts = append(opts, option(f.Path+hintSuffix(f.Hint), f.Path))
			}
			var pick string
			if src.sess != nil {
				sess.close() // 子交互（选择菜单/编辑器）前让出终端
			}
			if err := selectFrom(ctx, "打开哪个文件", opts, &pick); err != nil {
				return err
			}
			if err := editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(pick))); err != nil {
				return err
			}
			if src.sess != nil && !sess.resume() {
				return nil
			}
		case "s":
			if src.sess != nil {
				sess.close() // 沉淀输出在普通终端打印，避免 raw 帧残留
			}
			if v.Indexed {
				fmt.Fprintln(ctx.Out, ui.Dim("该主题已在索引中，/show edit 可维护"))
			} else {
				if err := showSave(ctx.WS, v); err != nil {
					return err
				}
				fmt.Fprintf(ctx.Out, "已沉淀 %q 到 %s\n", v.Topic, ui.Path(".gs/topics.toml"))
			}
			if src.sess != nil && !sess.resume() {
				return nil
			}
		}
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
	opts := make([]menuOption, 0, len(names))
	for _, n := range names {
		opts = append(opts, option(n, n))
	}
	if err := selectFrom(ctx, "选择演示主题", opts, &pick); err != nil {
		return "", err
	}
	return strings.TrimSuffix(pick, "〔沉淀〕"), nil
}

func newShow() Command {
	return Command{
		Name:  "show",
		Short: "按主题生成验收演示视图",
		Subs:  []string{"edit"},
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
	}
}
