package planview

import (
	"errors"
	"fmt"
	"io"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/termkey"
)

// ErrNonInteractive raw 模式不可用（非 TTY 等）；调用方降级静态输出
var ErrNonInteractive = errors.New("非交互终端，降级静态输出")

// Hooks 浏览器对外回调：打开笔记（壳层退 raw 后调 pager）与状态落盘
type Hooks struct {
	OpenNote func(rel string)
	Save     func(rm *roadmap.Roadmap) error
}

// keySession raw 按键会话：暂停（让给 pager）后可重开，退出保证还原终端
type keySession struct{ kr *termkey.KeyReader }

func (s *keySession) resume() bool {
	if s.kr != nil {
		return true
	}
	kr, err := termkey.NewKeyReader()
	if err != nil {
		return false
	}
	s.kr = kr
	return true
}

func (s *keySession) close() {
	if s.kr != nil {
		s.kr.Close()
		s.kr = nil
	}
}

// Run 浏览器主循环；原地变更 rm。raw 不可用返回 ErrNonInteractive。
func Run(out io.Writer, rm *roadmap.Roadmap, root, focusBranch string, h Hooks) error {
	sess := &keySession{}
	defer sess.close()
	if !sess.resume() {
		return ErrNonInteractive
	}
	m := &Model{RM: rm, Root: root, View: viewGlobal, Expanded: map[int]bool{}}
	if focusBranch != "" {
		m.Focus(focusBranch)
	}
	for {
		m.Normalize()
		drawFrame(out, m)
		k, ok := sess.kr.Read()
		if !ok {
			break
		}
		m.Err = ""
		switch k.Kind {
		case termkey.KindUp:
			m.Move(-1)
		case termkey.KindDown:
			m.Move(1)
		case termkey.KindTab:
			m.Toggle()
		case termkey.KindEnter:
			if act, rel := m.Enter(); act == ActOpenNote {
				if err := pauseAndOpen(out, sess, h, rel); err != nil {
					return err
				}
			}
		case termkey.KindEsc:
			if !m.Back() {
				sess.close()
				fmt.Fprint(out, "\r\x1b[J\r\n")
				return nil
			}
		case termkey.KindCtrlC, termkey.KindCtrlD:
			sess.close()
			fmt.Fprint(out, "\r\x1b[J\r\n")
			return nil
		case termkey.KindRune:
			switch k.R {
			case 'q':
				sess.close()
				fmt.Fprint(out, "\r\x1b[J\r\n")
				return nil
			case 'j':
				m.Move(1)
			case 'k':
				m.Move(-1)
			case 'o':
				if act, rel := m.Enter(); act == ActOpenNote {
					if err := pauseAndOpen(out, sess, h, rel); err != nil {
						return err
					}
				}
			case 'd', 's', 't':
				status := roadmap.StatusDone
				if k.R == 's' {
					status = roadmap.StatusDoing
				} else if k.R == 't' {
					status = roadmap.StatusTodo
				}
				if _, ok := m.Mark(status); ok && h.Save != nil {
					if err := h.Save(m.RM); err != nil {
						m.Err = "落盘失败：" + err.Error()
					}
				}
			}
		}
	}
	return nil
}

// pauseAndOpen 退 raw → 清帧 → 打开笔记（阻塞至阅读结束）→ 重进 raw；重进失败降级
func pauseAndOpen(out io.Writer, sess *keySession, h Hooks, rel string) error {
	sess.close()
	fmt.Fprint(out, "\r\x1b[J\r\n")
	if h.OpenNote != nil {
		h.OpenNote(rel)
	}
	if !sess.resume() {
		return ErrNonInteractive
	}
	return nil
}
