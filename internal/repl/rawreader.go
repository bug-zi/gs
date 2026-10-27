package repl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bugzi/gs/internal/complete"
	"github.com/bugzi/gs/internal/termkey"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// ErrEscBack ai 提示符下按 Esc：返回主模式信号（REPL Run 捕获后走 backFromAI）
var ErrEscBack = errors.New("esc back")

// rawReader 无框架自绘输入行：x/term.MakeRaw 自己读字节、自己解析、自己渲染。
// 终端状态进出完全由本文件负责，无任何框架接管，规避 bubbletea 帧生命周期与
// 普通输出交错的兼容性问题（trae/VSCode 终端实测多次失败后重写）。
type rawReader struct {
	prompt   string
	tree     complete.Tree
	hist     *History
	fallback Reader
}

func newRawReader(prompt string, tree complete.Tree, hist *History) *rawReader {
	return &rawReader{prompt: prompt, tree: tree, hist: hist}
}

func (r *rawReader) SetPrompt(p string) { r.prompt = p }
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
	defer func() {
		_ = term.Restore(int(os.Stdin.Fd()), raw)
		termkey.DrainConsoleInput()
	}()

	st := NewLineState()
	var ghost, hint string
	var cands []string
	panel, sel, noPanel := false, -1, false
	histIdx, typed := -1, ""

	// draw 重绘整个帧：清输入行及下方旧内容 → 输入行 → 面板 → 光标复位。
	// 输入行经水平滚动窗口裁剪，保证永不超出终端宽度（超出会换行、
	// 换行后 \x1b[J 清不掉上方旧行，导致每键重打全文的叠影）。
	draw := func() {
		res := complete.For(r.tree, st.Text())
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
		}
		avail := terminalWidth() - runewidth.StringWidth(r.prompt) - 1
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
	draw()
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return finishRead(r, st)
		}
		chunk := buf[:n]
		keys := parser.Feed(chunk)
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
				if strings.HasPrefix(r.prompt, "ai") {
					os.Stdout.WriteString("\r\x1b[J\r\n") // 清输入帧（面板+ghost）后交还普通终端
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
					return "", io.EOF
				}
				st.Delete()
			case termkey.KindUp:
				if panel && len(cands) > 0 {
					if sel < 0 {
						sel = 0
					} else if sel > 0 {
						sel--
					}
				} else {
					histMove(-1)
				}
			case termkey.KindDown:
				if panel && len(cands) > 0 {
					if sel < len(cands)-1 {
						sel++
					}
				} else {
					histMove(1)
				}
			case termkey.KindEnter:
				if panel && sel >= 0 && sel < len(cands) {
					st = FromText(acceptCandidateText(st.Text(), cands[sel]))
					st.Pos = st.Len()
					noPanel, sel = false, -1
					draw()
					continue
				}
				r.hist.Add(st.Text())
				return finishRead(r, st)
			}
		}
		draw()
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
