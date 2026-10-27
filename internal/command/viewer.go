package command

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/bugzi/gs/internal/edit"
	"github.com/bugzi/gs/internal/termkey"
	"github.com/bugzi/gs/internal/ui"
)

// clampView 把视口顶部行号 off 夹回 [0, max(0,total-h)]；h<1 恒回 0
func clampView(total, off, h int) int {
	if h < 1 {
		return 0
	}
	max := total - h
	if max < 0 {
		max = 0
	}
	if off > max {
		off = max
	}
	if off < 0 {
		off = 0
	}
	return off
}

// loadLines 读文件并渲染成显示行：RenderMD 出渲染态，再按渲染宽度 ANSI 感知强折，
// 防超宽行（代码块/表格）撑破整帧；读失败显示占位错误行
func loadLines(path string) []string {
	width := min(ui.TermWidth(90), 100)
	data, err := os.ReadFile(path)
	body := string(data)
	if err != nil {
		body = fmt.Sprintf("（读取失败：%v）", err)
	}
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(ui.RenderMD(body), "\n"), "\n") {
		lines = append(lines, strings.Split(ansi.Hardwrap(ln, width, true), "\n")...)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// drawViewerFrame 重绘查阅帧：状态行 + 视口内容 + 键位提示；提示行不带尾换行，
// 写完 \x1b[A 归位帧首供下次整体覆盖，整帧恰好占满屏不触发终端滚动
func drawViewerFrame(out io.Writer, path string, lines []string, off, h int) {
	end := min(off+h, len(lines))
	view := lines[off:end]
	pos := fmt.Sprintf("%d-%d/%d 行", off+1, end, len(lines))
	if len(lines) == 0 {
		pos = "空文档"
	}
	var b strings.Builder
	b.WriteString("\r\x1b[J")
	fmt.Fprintf(&b, "\x1b[1m%s\x1b[22m · %s\r\n", ui.Path(path), pos)
	for _, ln := range view {
		b.WriteString(ln + "\r\n")
	}
	b.WriteString(ui.Dim("[↑↓/jk]行 [PgUp/PgDn/空格]页 [g/G]首尾 [e]gs编辑 [o]TRAE [r]刷新 [q/Esc]退出"))
	fmt.Fprintf(out, "%s\x1b[%dA\r", b.String(), len(view)+1)
}

// runMDViewer 逾行文件全屏查阅器：渲染态滚动浏览，e 进 gs 内编辑、o 交外部编辑器，
// 两者返回后重读重渲染；q/Esc/Ctrl-C/D 退出回 gs>
func runMDViewer(ctx *Ctx, path string) error {
	sess := &keySession{}
	defer sess.close()
	if !sess.resume() {
		return edit.OpenExternal(path, ctx.Cfg)
	}
	quit := func() {
		sess.close()
		fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
	}
	lines := loadLines(path)
	off := 0
	for {
		h, ok := ui.PageSize()
		if !ok {
			h = 20
		}
		h -= 2 // 状态行 + 提示行
		if h < 1 {
			h = 1
		}
		off = clampView(len(lines), off, h)
		drawViewerFrame(ctx.Out, path, lines, off, h)
		k, ok := sess.kr.Read()
		if !ok {
			break
		}
		switch k.Kind {
		case termkey.KindUp:
			off--
		case termkey.KindDown:
			off++
		case termkey.KindPgUp:
			off -= h
		case termkey.KindPgDown:
			off += h
		case termkey.KindHome:
			off = 0
		case termkey.KindEnd:
			off = len(lines)
		case termkey.KindEsc, termkey.KindCtrlC, termkey.KindCtrlD:
			quit()
			return nil
		case termkey.KindRune:
			switch k.R {
			case 'j':
				off++
			case 'k':
				off--
			case 'g':
				off = 0
			case 'G':
				off = len(lines)
			case ' ':
				off += h
			case 'q':
				quit()
				return nil
			case 'r':
				lines = loadLines(path)
			case 'e', 'o':
				sess.close() // 编辑器需要普通终端，先退出 raw
				fmt.Fprint(ctx.Out, "\r\x1b[J")
				if k.R == 'e' {
					if data, rerr := os.ReadFile(path); rerr == nil {
						if err := edit.EditInline(path, data); err != nil {
							fmt.Fprintln(ctx.Out, ui.Dim("内嵌编辑启动失败，转交外部编辑器…"))
							_ = edit.OpenExternal(path, ctx.Cfg)
						}
					}
				} else {
					fmt.Fprintf(ctx.Out, "正在调起外部编辑器打开 %s ...（关闭文件后回到查阅）\n", ui.Path(path))
					_ = edit.OpenExternal(path, ctx.Cfg)
				}
				if !sess.resume() {
					return nil
				}
				lines = loadLines(path)
			}
		}
	}
	quit()
	return nil
}

// runLargeFile 逾行文件打开方式选择：gs 全屏查阅/编辑 或 TRAE/外部编辑器。
// 非 TTY 不询问维持旧路；Esc 取消（ErrCancelled 由 REPL 静默）= 放弃本次打开
func runLargeFile(ctx *Ctx, path string) error {
	if _, ok := ui.PageSize(); !ok {
		fmt.Fprintf(ctx.Out, "长文件：正在调起外部编辑器打开 %s ...（关闭文件后回到 gs）\n", ui.Path(path))
		return edit.OpenExternal(path, ctx.Cfg)
	}
	var pick string
	err := selectFrom(ctx, "逾行文件，选择打开方式", []menuOption{
		option("gs 查阅/编辑（全屏 TUI）", "gs"),
		option("TRAE/外部编辑器打开", "ext"),
	}, &pick)
	if err != nil {
		return err
	}
	if pick == "ext" {
		fmt.Fprintf(ctx.Out, "长文件：正在调起外部编辑器打开 %s ...（关闭文件后回到 gs）\n", ui.Path(path))
		return edit.OpenExternal(path, ctx.Cfg)
	}
	return runMDViewer(ctx, path)
}
