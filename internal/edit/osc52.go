package edit

import "encoding/base64"

// osc52 构造 OSC52 写系统剪贴板转义序列(BEL 结尾)。
// 需终端支持(Windows Terminal 支持;conhost 等不支持时复制静默无效)
func osc52(s string) string {
	return "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(s)) + "\x07"
}
