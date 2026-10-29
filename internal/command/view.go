package command

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bugzi/gs/internal/edit"
	"github.com/bugzi/gs/internal/termkey"
	"github.com/bugzi/gs/internal/ui"
)

// previewOf 读文件前 n 行作预览
func previewOf(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "（读取失败）"
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// pagerKeyString raw 按键 → pager 命令串；ok=false 表示该键不参与分发（方向键等忽略）
func pagerKeyString(k termkey.Key) (string, bool) {
	switch k.Kind {
	case termkey.KindRune:
		return strings.ToLower(string(k.R)), true
	case termkey.KindEnter:
		return "", true
	case termkey.KindEsc:
		return "esc", true
	case termkey.KindCtrlC:
		return "ctrl+c", true
	case termkey.KindCtrlD:
		return "ctrl+d", true
	}
	return "", false
}

// pagerKeySource 统一取键源：raw 单键（sess 非 nil）或降级行读取；next 阻塞取一条可分发命令
type pagerKeySource struct {
	sess *keySession
	br   *bufio.Reader
}

func (p *pagerKeySource) next() (string, bool) {
	if p.sess != nil {
		for {
			k, ok := p.sess.kr.Read()
			if !ok {
				return "", false
			}
			if s, dispatch := pagerKeyString(k); dispatch {
				return s, true
			}
		}
	}
	line, err := p.br.ReadString('\n')
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(line), true
}

// newPagerKeySource raw 可用走单键会话，否则降级行读取；调用方 defer sess.close() 还原终端
func newPagerKeySource() (*pagerKeySource, *keySession) {
	sess := &keySession{}
	if sess.resume() {
		return &pagerKeySource{sess: sess}, sess
	}
	return &pagerKeySource{br: bufio.NewReader(os.Stdin)}, sess
}

// pagerMode 查阅内容打印策略
type pagerMode int

const (
	pagerPaged pagerMode = iota // 终端尺寸已知时按键分页（空格/回车下翻，其余键跳完剩余）
	pagerFull                   // 整篇直出不分页（logs 显示策略：不论行数完整呈现）
)

// runViewPager 查阅态循环：非文档文件直开外部编辑器（默认解析链命中 TRAE）；
// 文档逾行（>20 行）先询问打开方式（gs 全屏查阅/编辑 或 外部编辑器），
// 短文件维持分页渲染 → [e]编辑 [r]刷新 [q]退出。
// 渲染内容就是 editPath 文件本身时的标准入口；不转交且整篇直出的显示策略场景（logs）直接调 pagerLoop(pagerFull)。
func runViewPager(ctx *Ctx, render func() string, editPath func() (string, bool)) error {
	if path, ok := editPath(); ok {
		if !edit.IsDocFile(path) { // 非文档文件不进查阅/逾行菜单，直开外部
			return openExternalBlocked(ctx, path)
		}
		if data, err := os.ReadFile(path); err == nil && !edit.ShouldCliEdit(data) {
			return runLargeFile(ctx, path)
		}
	}
	return pagerLoop(ctx, render, editPath, pagerPaged)
}

// openExternalBlocked 非文档文件直开外部编辑器：打印提示并阻塞至编辑器关闭
func openExternalBlocked(ctx *Ctx, path string) error {
	fmt.Fprintf(ctx.Out, "正在打开 %s ...（编辑器关闭后回到 gs）\n", ui.Path(path))
	return edit.OpenExternal(path, ctx.Cfg)
}

// pagerShouldPage 本次渲染是否分页：logs 整篇直出；短文件（≤20 行，与外部编辑器
// 分流同判）也不分页——分页按终端页高切页，矮窗口下短笔记会被静默切半屏等按键，
// 表现为「显示不完整」，违背短文件完整显示的约定
func pagerShouldPage(mode pagerMode, editPath func() (string, bool)) bool {
	if mode != pagerPaged {
		return false
	}
	if path, ok := editPath(); ok {
		if data, err := os.ReadFile(path); err == nil && edit.ShouldCliEdit(data) {
			return false
		}
	}
	return true
}

// runViewPagerDel 条目查阅入口：非文档直开外部；长短分流同 runViewPager，两路都带 [x] 删除钩子
func runViewPagerDel(ctx *Ctx, render func() string, editPath func() (string, bool), onDelete func() bool) error {
	if path, ok := editPath(); ok {
		if !edit.IsDocFile(path) {
			return openExternalBlocked(ctx, path)
		}
		if data, err := os.ReadFile(path); err == nil && !edit.ShouldCliEdit(data) {
			return runLargeFileEx(ctx, path, onDelete)
		}
	}
	return pagerLoopEx(ctx, render, editPath, pagerPaged, onDelete)
}

// pagerLoop 委托无删除钩子的查阅循环
func pagerLoop(ctx *Ctx, render func() string, editPath func() (string, bool), mode pagerMode) error {
	return pagerLoopEx(ctx, render, editPath, mode, nil)
}

// pagerLoopEx 带 [x] 删除钩子的查阅循环；onDelete 在退出 raw 后调用（内含预览确认），
// 返回 true = 条目已删除，结束查阅。nil = 无删除键，提示行不显示 [x]。
func pagerLoopEx(ctx *Ctx, render func() string, editPath func() (string, bool), mode pagerMode, onDelete func() bool) error {
	src, sess := newPagerKeySource()
	defer sess.close()
	hint := ui.Dim("[e]编辑 [r]刷新 [q/Esc]退出 > ")
	if onDelete != nil {
		hint = ui.Dim("[e]编辑 [x]删除 [r]刷新 [q/Esc]退出 > ")
	}
	for {
		body := render()
		fmt.Fprint(ctx.Out, "\n")
		if h, ok := ui.PageSize(); pagerShouldPage(mode, editPath) && ok {
			ui.PagedPrint(ctx.Out, src.next, body, h)
		} else {
			fmt.Fprintf(ctx.Out, "%s\n", body)
		}
		fmt.Fprint(ctx.Out, hint)
		line, ok := src.next()
		if !ok {
			return nil // Ctrl-D / 读取失败结束查看
		}
		switch line {
		case "q", "", "esc", "ctrl+c", "ctrl+d":
			return nil
		case "r":
			continue
		case "x":
			if onDelete == nil {
				continue
			}
			if src.sess != nil {
				sess.close() // 确认交互需要普通终端，先退出 raw
			}
			del := onDelete()
			if src.sess != nil && !sess.resume() {
				return nil
			}
			if del {
				return nil
			}
		case "e":
			path, ok := editPath()
			if !ok {
				fmt.Fprintln(ctx.Out, ui.Dim("本视图没有可编辑的文件"))
				continue
			}
			if src.sess != nil {
				sess.close() // 编辑器需要普通终端，先退出 raw
			}
			fmt.Fprintf(ctx.Out, "正在打开 %s ...（编辑器退出后自动刷新）\n", ui.Path(path))
			if err := editOpen(ctx, path); err != nil {
				return err
			}
			if src.sess != nil && !sess.resume() {
				return nil
			}
		}
	}
}
