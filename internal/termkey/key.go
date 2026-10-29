package termkey

import (
	"os"

	"golang.org/x/term"
)

// KeyReader 原始模式单键读取器：NewKeyReader 进 raw、Close 还原，
// 供交互视图（树浏览等）逐键消费。
type KeyReader struct {
	raw    *term.State
	parser Parser
}

// NewKeyReader 进入原始输入模式；失败（如 stdin 非终端）返回 error，调用方自行降级。
// 成功后清一次输入缓冲：上个会话（bubbletea 编辑器退出不清缓冲、连按 Esc 的残留）
// 漏下的字节会被误当首键——一个残留 Esc 就会让交互视图瞬间退出。
func NewKeyReader() (*KeyReader, error) {
	raw, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	DrainConsoleInput()
	return &KeyReader{raw: raw}, nil
}

// Read 阻塞读一个按键；终端读失败返回 ok=false（视作结束信号）
func (k *KeyReader) Read() (Key, bool) {
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return Key{}, false
		}
		if keys := k.parser.Feed(buf[:n]); len(keys) > 0 {
			return keys[0], true
		}
	}
}

// Close 还原终端并冲掉残留输入；可重复调用
func (k *KeyReader) Close() {
	if k.raw == nil {
		return
	}
	_ = term.Restore(int(os.Stdin.Fd()), k.raw)
	DrainConsoleInput()
	k.raw = nil
}
