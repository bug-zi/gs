//go:build windows

package termkey

import (
	"os"

	"golang.org/x/sys/windows"
)

// DrainConsoleInput 清空控制台输入缓冲。
// raw 程序以 VT 模式消费按键，退出时会把 Enter 的残留 \n 留在缓冲里，
// 下一位读取者（查阅分页器/下一步输入）会立刻读到它而误判按键，必须冲掉。
func DrainConsoleInput() {
	_ = windows.FlushConsoleInputBuffer(windows.Handle(os.Stdin.Fd()))
}
