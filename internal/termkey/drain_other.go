//go:build !windows

package termkey

// DrainConsoleInput 非 Windows 平台暂无对应实现（gs 主战场是 Windows）
func DrainConsoleInput() {}
