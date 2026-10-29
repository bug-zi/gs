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

// Hooks 浏览器对外回调：OpenNote/Save 沿用；其余为维护操作交互子流程
// （壳层退 raw 后以行式菜单/输入完成，planview 状态机不碰终端交互）。
// 任一为 nil 时对应操作提示不可用。
type Hooks struct {
	OpenNote         func(rel string)
	Save             func(rm *roadmap.Roadmap) error
	SelectPoint      func(prompt string) (roadmap.PointRef, bool)   // 关键词过滤选知识点
	Input            func(prompt string) (string, bool)             // 单行文本输入
	Confirm          func(summary string) bool                      // 确认（破坏性操作前）
	SelectOne        func(prompt string, opts []string) (int, bool) // 单选菜单，返回下标
	Unbound          func(tier Tier) []string                       // 未挂载扫描（惰性，可重复调用）
	RenameFile       func(oldRel, newRel string) error              // 工作区内文件改名
	ApplyMoves       func(moves []roadmap.FileMove) error           // 文件迁移+引用同步改写（river 收容）
	TrashRel         func(rel string) error                         // 文件/空目录移入回收站
	CopyFileRel      func(srcRel, dstRel string) error              // 装船复制（sea 原件保留，原子写）
	MoveFileRel      func(srcRel, dstRel string) error              // 入河迁移（同盘 rename + 建目录）
	RewriteChainsRel func(srcRel, dstRel string) error              // 全库出处链改写（迁移后调用）
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
	m.Unbound = h.Unbound
	m.refreshUnbound()
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
				r := m.rowAt(m.Cursor)
				if r != nil && r.Kind == rowNote { // 笔记行沿用打开语义
					if act, rel := m.Enter(); act == ActOpenNote {
						if err := pauseAndOpen(out, sess, h, rel); err != nil {
							return err
						}
					}
					break
				}
				if err := pauseAndAct(out, sess, func() { doMenu(m, h) }); err != nil {
					return err
				}
			case 'x':
				doUnbind(m, h)
			case 'm', 'a', 'n', 'c', 'r':
				var fn func()
				switch k.R {
				case 'm':
					fn = func() { doMove(m, h) }
				case 'a':
					fn = func() { doAttach(m, h) }
				case 'n':
					fn = func() { doRename(m, h) }
				case 'c':
					fn = func() { doShipCopy(m, h) }
				case 'r':
					fn = func() { doShipRiver(m, h) }
				}
				if err := pauseAndAct(out, sess, fn); err != nil {
					return err
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

// pauseAndAct 退 raw → 清帧 → 执行交互流程（阻塞）→ 重进 raw；重进失败降级
func pauseAndAct(out io.Writer, sess *keySession, fn func()) error {
	sess.close()
	fmt.Fprint(out, "\r\x1b[J\r\n")
	fn()
	if !sess.resume() {
		return ErrNonInteractive
	}
	return nil
}
