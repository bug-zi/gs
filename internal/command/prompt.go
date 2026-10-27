package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-runewidth"

	"github.com/bugzi/gs/internal/termkey"
	"github.com/bugzi/gs/internal/ui"
)

// menuOption 单选项：Label 展示、Value 提交值
type menuOption struct {
	Label string
	Value string
}

func option(label, value string) menuOption { return menuOption{Label: label, Value: value} }

// ErrCancelled 用户取消选择（Esc/Ctrl-C/Ctrl-D），REPL 对其静默
var ErrCancelled = errors.New("已取消")

// selectState 选择菜单光标状态（纯逻辑可测）
type selectState struct{ sel int }

// move 移动光标，环形回绕
func (s *selectState) move(n, d int) {
	s.sel = ((s.sel+d)%n + n) % n
}

// selectFrom 单选菜单。raw 自绘替代 huh：bubbletea 首帧在 trae/ConPTY 下渲染不全
// （选项列表需按一次键才完整），全 gs 的选择菜单统一走 termkey。stdin 非 TTY 降级编号列表。
func selectFrom(ctx *Ctx, title string, opts []menuOption, out *string) error {
	if len(opts) == 0 {
		return errors.New("没有可选项")
	}
	kr, err := termkey.NewKeyReader()
	if err != nil {
		return plainSelect(ctx, title, opts, out)
	}
	defer kr.Close()
	st := &selectState{}
	for {
		drawSelectFrame(ctx.Out, title, opts, st)
		k, ok := kr.Read()
		if !ok {
			return ErrCancelled
		}
		switch k.Kind {
		case termkey.KindUp, termkey.KindDown:
			d := -1
			if k.Kind == termkey.KindDown {
				d = 1
			}
			st.move(len(opts), d)
		case termkey.KindEnter:
			fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
			*out = opts[st.sel].Value
			return nil
		case termkey.KindEsc, termkey.KindCtrlC, termkey.KindCtrlD:
			fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
			return ErrCancelled
		case termkey.KindRune:
			switch k.R {
			case 'j':
				st.move(len(opts), 1)
			case 'k':
				st.move(len(opts), -1)
			}
		}
	}
}

// drawSelectFrame 重绘选择菜单帧：标题 + 键位提示 + 视口内选项（光标行反白 ❯）+ 越界提示；
// 写完光标移回帧首，供下次整体重绘覆盖
func drawSelectFrame(out io.Writer, title string, opts []menuOption, st *selectState) {
	hint := ui.Dim("↑↓/jk 选择　Enter 确认　Esc 取消")
	avail := 0
	if h, ok := ui.PageSize(); ok {
		avail = h - 4
	}
	lo, hi := treeWindow(len(opts), st.sel, avail)
	width := ui.TermWidth(90)
	if width > 100 {
		width = 100
	}
	var lines []string
	if lo > 0 {
		lines = append(lines, ui.Dim(fmt.Sprintf("　…（上方还有 %d 项）", lo)))
	}
	for i := lo; i < hi; i++ {
		label := opts[i].Label
		if w := runewidth.StringWidth(label); w > width-2 {
			label = runewidth.Truncate(label, width-2, "…")
		}
		if i == st.sel {
			lines = append(lines, "\x1b[7m❯ "+label+"\x1b[27m")
		} else {
			lines = append(lines, "  "+label)
		}
	}
	if hi < len(opts) {
		lines = append(lines, ui.Dim(fmt.Sprintf("　…（下方还有 %d 项）", len(opts)-hi)))
	}
	var b strings.Builder
	b.WriteString("\r\x1b[J")
	b.WriteString("\x1b[1m" + title + "\x1b[22m\r\n")
	b.WriteString(hint + "\r\n")
	for _, l := range lines {
		b.WriteString(l + "\r\n")
	}
	fmt.Fprintf(out, "%s\x1b[%dA\r", b.String(), 2+len(lines)) // 上移行数 = 标题+提示+选项，差一会使旧帧残留堆叠
}

// plainSelect 非交互降级：编号列表 + 读行选择
func plainSelect(ctx *Ctx, title string, opts []menuOption, out *string) error {
	var b strings.Builder
	b.WriteString(title + "\n")
	for i, o := range opts {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, o.Label)
	}
	b.WriteString("输入编号选择（空/0 取消）> ")
	fmt.Fprint(ctx.Out, b.String())
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return ErrCancelled
	}
	n, perr := strconv.Atoi(strings.TrimSpace(line))
	if perr != nil || n < 1 || n > len(opts) {
		return ErrCancelled
	}
	*out = opts[n-1].Value
	return nil
}

// confirm 是/否确认
func confirm(ctx *Ctx, title string) (bool, error) {
	var ok bool
	err := huh.NewConfirm().Title(title).Value(&ok).Run()
	return ok, err
}

// inputLine 单行输入
func inputLine(ctx *Ctx, title string) (string, error) {
	var s string
	err := huh.NewInput().Title(title).Value(&s).Run()
	return s, err
}
