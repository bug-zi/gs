package command

import (
	"bytes"
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

// viewLog 渲染单日日志；create=true 时对不存在的文件先建空文件。
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
		fmt.Fprintf(out, "%s（新文件）已创建\n", ui.Path(ws.Rel(path)))
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
			if strings.HasPrefix(strings.TrimSpace(l), "#") {
				fmt.Fprint(out, ui.RenderMD(l))
				continue
			}
			fmt.Fprintln(out, ui.HighlightWords(l, strings.Fields(kw)))
		}
	}
	if !found {
		fmt.Fprintf(out, "全部日志中未找到含 %q 的记录\n", kw)
	}
	return nil
}

// viewByDayArg 按日期参数定位并渲染单日日志
func viewByDayArg(out io.Writer, ws *workspace.Workspace, arg string) (bool, error) {
	d, ok := findDate(ws, arg)
	if !ok {
		fmt.Fprintf(out, "没有 %s 的日志（可用 /logs 直接看最近）\n", arg)
		return true, nil
	}
	return viewLog(out, ws, d, false)
}

// dayArgMatch 按参数长度匹配日志日期：4 位比月日，6 位比年月日
func dayArgMatch(d time.Time, arg string) bool {
	switch len(arg) {
	case 4:
		return d.Format("0102") == arg
	case 6:
		return d.Format("060102") == arg
	}
	return false
}

func wsRel(ws *workspace.Workspace, t time.Time) string {
	return ws.Rel(filepath.Join(ws.LogsDir(), workspace.LogFileName(t)))
}

// dayArgOK 日期参数：4 位月日（1025）或 6 位年月日（261025）
func dayArgOK(s string) bool {
	if len(s) != 4 && len(s) != 6 {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func findDate(ws *workspace.Workspace, arg string) (time.Time, bool) {
	dates, err := ws.ListLogs()
	if err != nil {
		return time.Time{}, false
	}
	for _, d := range dates {
		if dayArgMatch(d, arg) {
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

func newLogs() Command {
	return Command{
		Name:  "logs",
		Alias: []string{"log"},
		Short: "查看/追加/编辑学习日志",
		Subs:  []string{"add", "edit"},
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
						ok, err := confirm(ctx, "超过 40 字（学习日志应一句话概括），仍要记录？")
						if err != nil {
							return err
						}
						if !ok {
							fmt.Fprintln(ctx.Out, "已取消")
							return nil
						}
					}
					if err := AppendEntry(ctx.WS, today, entry); err != nil {
						return err
					}
					fmt.Fprintf(ctx.Out, "已记入 %s\n", ui.Path(wsRel(ctx.WS, today)))
					return nil
				case "edit":
					t := today
					if len(ctx.Args) > 1 {
						if dayArgOK(ctx.Args[1]) {
							d, ok := findDate(ctx.WS, ctx.Args[1])
							if !ok {
								return fmt.Errorf("没有 %s 的日志", ctx.Args[1])
							}
							t = d
						} else {
							return fmt.Errorf("日期格式应为 MMDD 或 YYMMDD，如 1024 / 261024")
						}
					}
					return editLogFile(ctx, t)
				}
				if dayArgOK(ctx.Args[0]) {
					d, ok := findDate(ctx.WS, ctx.Args[0])
					if !ok {
						fmt.Fprintf(ctx.Out, "没有 %s 的日志（可用 /logs 直接看最近）\n", ctx.Args[0])
						return nil
					}
					return runDayPager(ctx, d)
				}
				return viewByKeyword(ctx.Out, ctx.WS, strings.Join(ctx.Args, " "))
			}
			// 无参：今天，无则最近一天
			day := today
			if _, err := os.Stat(filepath.Join(ctx.WS.LogsDir(), workspace.LogFileName(today))); err != nil {
				dates, err := ctx.WS.ListLogs()
				if err != nil || len(dates) == 0 {
					fmt.Fprintln(ctx.Out, "还没有任何日志，用 /log add \"...\" 记录第一条吧")
					return err
				}
				fmt.Fprintln(ctx.Out, ui.Dim("今天还没有日志，显示最近一次："))
				day = dates[0]
			}
			return runDayPager(ctx, day)
		},
	}
}

// runDayPager 单日日志的查阅态：logs 专项规则——不论行数整篇完整打印在 CLI，
// 不转交外部编辑器也不按键分页；e 编辑该日日志（仍走长短分流），编辑器退出后重渲染
func runDayPager(ctx *Ctx, day time.Time) error {
	return pagerLoop(ctx,
		func() string {
			var b bytes.Buffer
			_, _ = viewLog(&b, ctx.WS, day, false)
			return b.String()
		},
		func() (string, bool) {
			return filepath.Join(ctx.WS.LogsDir(), workspace.LogFileName(day)), true
		},
		pagerFull)
}
