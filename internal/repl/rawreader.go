package repl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bugzi/gs/internal/complete"
	"github.com/bugzi/gs/internal/termkey"
	"github.com/bugzi/gs/internal/ui"
	"golang.org/x/term"
)

// ErrEscBack ai 提示符下按 Esc：返回主模式信号（REPL Run 捕获后走 backFromAI）
var ErrEscBack = errors.New("esc back")

// rawReader 无框架自绘输入行：x/term.MakeRaw 自己读字节、自己解析、自己渲染。
// 终端状态进出完全由本文件负责，无任何框架接管，规避 bubbletea 帧生命周期与
// 普通输出交错的兼容性问题（trae/VSCode 终端实测多次失败后重写）。
type rawReader struct {
	prompt     string
	promptName string // 素文本首词（gs/ai），供 Esc 返回判定
	tree       complete.Tree
	hist       *History
	fallback   Reader
}

func newRawReader(prompt string, tree complete.Tree, hist *History) *rawReader {
	p, name := brandPrompt(prompt)
	return &rawReader{prompt: p, promptName: name, tree: tree, hist: hist}
}

func (r *rawReader) SetPrompt(p string) {
	r.prompt, r.promptName = brandPrompt(p)
}

// brandPrompt 提示符品牌色：gs 金粗、ai 赭粗，其余（箭头段）暗灰；返回带色串与素文本首词
func brandPrompt(p string) (string, string) {
	i := strings.Index(p, " ")
	if i <= 0 {
		return p, p
	}
	name, rest := p[:i], p[i:]
	color := ui.ANSIGold
	if name == "ai" {
		color = ui.ANSIOchreBold
	}
	return color + name + ui.ANSIReset + ui.ANSIIdle + rest + ui.ANSIReset, name
}
func (r *rawReader) Close() error       { return nil }

func (r *rawReader) Read() (string, error) {
	if r.fallback != nil {
		return r.fallback.Read()
	}
	raw, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "（无法进入原始输入模式，已降级为普通输入）")
		r.fallback = newPlainReader(r.prompt)
		return r.fallback.Read()
	}
	// 入口清一次输入缓冲：上个 TUI（尤其 bubbletea 系，退出不清缓冲）或快速连按
	// Esc 逐层退出漏下的残留 0x1b 会被当作首键 Esc，把候选面板压制成整个会话不弹
	termkey.DrainConsoleInput()
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), raw)
		termkey.DrainConsoleInput()
	}()

	st := NewLineState()
	var res complete.Result
	var ghost, hint string
	var cands []string
	panel, sel, noPanel := false, -1, false
	histIdx, typed := -1, ""

	var mu sync.Mutex // 守护以下状态与重绘：定时器回调与主循环并发触达
	var hintTimer *time.Timer
	var hintText string // 非空 = 输入行下方延迟提示显示中

	stopHint := func() {
		if hintTimer != nil {
			hintTimer.Stop()
			hintTimer = nil
		}
		hintText = ""
	}

	// drawLocked 重绘整个帧（须持 mu）：清输入行及下方旧内容 → 输入行 → 面板/延迟提示 → 光标复位。
	// 输入行经水平滚动窗口裁剪，保证永不超出终端宽度（超出会换行、
	// 换行后 \x1b[J 清不掉上方旧行，导致每键重打全文的叠影）。
	drawLocked := func() {
		res = complete.For(r.tree, st.Text())
		ghost, hint, cands = res.Ghost, res.Hint, res.Candidates
		panel = len(cands) >= 2 && !noPanel
		if !panel {
			sel = -1
		} else if sel >= len(cands) {
			sel = -1
		}
		var lines []string
		if panel {
			lines = buildPanelLines(cands, hint, sel)
		} else if hintText != "" {
			lines = []string{"  " + ui.ANSIIdle + ui.ANSIItalic + hintText + ui.ANSIReset} // 延迟提示：斜体暗灰
		}
		avail := terminalWidth() - plainWidth(r.prompt) - 1
		line, curCol := buildInputLineFit(r.prompt, st, ghost, avail)
		var b strings.Builder
		b.WriteString("\r\x1b[J")
		b.WriteString(line)
		for _, l := range lines {
			b.WriteString("\r\n\x1b[K")
			b.WriteString(l)
		}
		if len(lines) > 0 {
			fmt.Fprintf(&b, "\x1b[%dA", len(lines))
		}
		fmt.Fprintf(&b, "\r\x1b[%dC", curCol)
		os.Stdout.WriteString(b.String())
	}

	// settle 每轮按键后的收尾（须持 mu）：清提示与定时器 → 重绘 → 唯一命中且面板未弹时设 1s 定时器
	settle := func() {
		stopHint()
		drawLocked()
		text := delayedHintText(res.Single, res.SingleHint)
		if text == "" || noPanel {
			return
		}
		line := st.Text()
		hintTimer = time.AfterFunc(time.Second, func() {
			mu.Lock()
			defer mu.Unlock()
			if st.Text() != line || noPanel { // 行已变或 Esc 压制面板：放弃
				return
			}
			hintText = text
			drawLocked()
		})
	}

	histMove := func(delta int) {
		items := r.hist.Items()
		if len(items) == 0 {
			return
		}
		if histIdx < 0 {
			if delta > 0 {
				return
			}
			typed = st.Text()
			histIdx = len(items) - 1
		} else {
			histIdx += delta
			if histIdx < 0 {
				histIdx = -1
				st = FromText(typed)
				st.Pos = st.Len()
				return
			}
			if histIdx > len(items)-1 {
				histIdx = len(items) - 1
			}
		}
		st = FromText(items[histIdx])
		st.Pos = st.Len()
	}

	var parser termkey.Parser
	mu.Lock()
	drawLocked()
	mu.Unlock()
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			mu.Lock()
			stopHint()
			mu.Unlock()
			return finishRead(r, st)
		}
		chunk := buf[:n]
		keys := parser.Feed(chunk)
		mu.Lock()
		for _, key := range keys {
			switch key.Kind {
			case termkey.KindRune:
				st.Insert(key.R)
			case termkey.KindBackspace:
				st.Backspace()
			case termkey.KindDelete:
				st.Delete()
			case termkey.KindLeft:
				st.Left()
			case termkey.KindRight:
				if st.Pos == st.Len() && ghost != "" {
					st.InsertText(ghost) // 行尾 → 接受 ghost
				} else {
					st.Right()
				}
			case termkey.KindHome:
				st.Pos = 0
			case termkey.KindEnd:
				st.Pos = st.Len()
			case termkey.KindTab:
				if ghost != "" && st.Pos == st.Len() {
					st.InsertText(ghost)
				}
			case termkey.KindEsc:
				if r.promptName == "ai" {
					os.Stdout.WriteString("\r\x1b[J\r\n") // 清输入帧（面板+ghost+提示）后交还普通终端
					stopHint()
					mu.Unlock()
					return "", ErrEscBack
				}
				noPanel = true
				panel = false
				sel = -1
			case termkey.KindCtrlC:
				st = NewLineState()
				histIdx, typed, noPanel, sel = -1, "", false, -1
			case termkey.KindCtrlD:
				if st.Len() == 0 {
					os.Stdout.WriteString("\r\x1b[J\r\n")
					stopHint()
					mu.Unlock()
					return "", io.EOF
				}
				st.Delete()
			case termkey.KindUp:
				if panel && len(cands) > 0 {
					sel = wrapSel(sel, -1, len(cands))
				} else {
					histMove(-1)
				}
			case termkey.KindDown:
				if panel && len(cands) > 0 {
					sel = wrapSel(sel, 1, len(cands))
				} else {
					histMove(1)
				}
			case termkey.KindEnter:
				if panel && sel >= 0 && sel < len(cands) {
					st = FromText(acceptCandidateText(st.Text(), cands[sel]))
					st.Pos = st.Len()
					noPanel, sel = false, -1
					drawLocked()
					continue
				}
				stopHint()
				r.hist.Add(st.Text())
				mu.Unlock()
				return finishRead(r, st)
			}
		}
		settle()
		mu.Unlock()
	}
}

// terminalWidth 终端列数，取不到时按 80 处理
func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// finishRead 收尾：清面板、回显提交行，交还普通终端
func finishRead(r *rawReader, st LineState) (string, error) {
	line := st.Text()
	os.Stdout.WriteString("\r\x1b[J\r")
	if line != "" {
		fmt.Fprintf(os.Stdout, "%s%s\r\n", r.prompt, line)
	} else {
		os.Stdout.WriteString("\r\n")
	}
	return line, nil
}

// acceptCandidateText 用候选替换当前词：第一词阶段补 "/"+词，第二词阶段保留第一词
func acceptCandidateText(line, cand string) string {
	if i := strings.Index(line, " "); i >= 0 {
		return line[:i+1] + cand
	}
	return "/" + cand
}

// delayedHintText 延迟提示行内容：命令名 + 参数提示；任一为空则不显示
func delayedHintText(single, singleHint string) string {
	if single == "" || singleHint == "" {
		return ""
	}
	return single + " " + singleHint
}

// wrapSel 面板选中项循环移动：↑ 在首行回绕到末行，↓ 在末行回绕到首行；
// 未选中（-1）时就近进入列表第一行；n<=0 无候选维持未选中
func wrapSel(sel, delta, n int) int {
	if n <= 0 {
		return -1
	}
	if delta < 0 {
		switch {
		case sel < 0:
			return 0
		case sel == 0:
			return n - 1
		default:
			return sel - 1
		}
	}
	if sel >= n-1 {
		return 0
	}
	return sel + 1
}
