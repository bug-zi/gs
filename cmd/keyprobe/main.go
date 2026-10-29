// keyprobe 终端按键探针:raw 模式逐键回显原始字节与解析结果,诊断各终端
// (TRAE/VS Code 内置终端/conhost/Windows Terminal)的修饰键序列与 OSC52 支持。
// 用法:在待测终端里 go run ./cmd/keyprobe → 逐个按下待测按键 → Ctrl+D 退出。
// 按 o 发一条 OSC52 写剪贴板测试串(内容 gs-osc52-ok),到别处粘贴即可验证。
// 全部按键记录追加写入当前目录 keyprobe.log。
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/bugzi/gs/internal/termkey"
)

func main() {
	log, err := os.OpenFile("keyprobe.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Println("无法写 keyprobe.log:", err)
		os.Exit(1)
	}
	defer log.Close()

	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Println("stdin 非终端:", err)
		os.Exit(1)
	}
	defer term.Restore(int(os.Stdin.Fd()), old)

	fmt.Print("按键探针:逐个按下待测键(Shift+方向 / Ctrl+C,X,Y,Z,F,S / Alt+上下 / F3 / Shift+F3 / Tab / Shift+Tab)\r\n")
	fmt.Print("按 o 发 OSC52 剪贴板测试串;Ctrl+D 退出。每行=原始字节 | 解析结果\r\n\r\n")

	p := &termkey.Parser{}
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			break
		}
		chunk := buf[:n]
		kinds := ""
		for _, k := range p.Feed(chunk) {
			if k.Kind == termkey.KindRune {
				kinds += fmt.Sprintf("rune(%q) ", k.R)
			} else {
				kinds += k.Kind.String() + " "
			}
		}
		entry := fmt.Sprintf("% x  ->  %s\r\n", chunk, kinds)
		os.Stdout.WriteString(entry)
		log.WriteString(strings.ReplaceAll(entry, "\r\n", "\n") + "\n")

		if len(chunk) == 1 && chunk[0] == 0x04 { // Ctrl+D
			fmt.Print("bye\r\n")
			break
		}
		if len(chunk) == 1 && chunk[0] == 'o' {
			b64 := base64.StdEncoding.EncodeToString([]byte("gs-osc52-ok"))
			os.Stdout.WriteString("\x1b]52;c;" + b64 + "\x07")
			fmt.Print("已发 OSC52(内容 gs-osc52-ok),到别处粘贴验证是否写入剪贴板\r\n")
		}
	}
}
