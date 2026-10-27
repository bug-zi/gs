// Package termkey 终端原始按键底层：字节流解析、raw 模式单键读取、控制台缓冲清理。
// 供 repl（输入行）与 command（交互视图）共同依赖，不含任何业务逻辑。
package termkey

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// Kind 按键类别
type Kind int

const (
	KindNone Kind = iota
	KindRune
	KindEnter
	KindBackspace
	KindDelete
	KindLeft
	KindRight
	KindUp
	KindDown
	KindHome
	KindEnd
	KindPgUp
	KindPgDown
	KindTab
	KindEsc
	KindCtrlC
	KindCtrlD
)

func (k Kind) String() string {
	switch k {
	case KindRune:
		return "rune"
	case KindEnter:
		return "enter"
	case KindBackspace:
		return "backspace"
	case KindDelete:
		return "delete"
	case KindLeft:
		return "left"
	case KindRight:
		return "right"
	case KindUp:
		return "up"
	case KindDown:
		return "down"
	case KindHome:
		return "home"
	case KindEnd:
		return "end"
	case KindPgUp:
		return "pgup"
	case KindPgDown:
		return "pgdown"
	case KindTab:
		return "tab"
	case KindEsc:
		return "esc"
	case KindCtrlC:
		return "ctrl+c"
	case KindCtrlD:
		return "ctrl+d"
	}
	return "none"
}

// Key 单次按键事件
type Key struct {
	Kind Kind
	R    rune
}

// Parser 原始字节流 → 按键序列；跨块残缺序列缓存在内部
type Parser struct {
	pending []byte
}

// Feed 解析一块原始字节，返回按键序列，并按 GS_KEYLOG 记录诊断
func (p *Parser) Feed(b []byte) []Key {
	keys := p.parse(b)
	logKeys(b, keys)
	return keys
}

// fmtBytes 字节诊断表示
func fmtBytes(b []byte) string { return fmt.Sprintf("% x", b) }

// parse 解析主体；残缺序列留待下一块
func (p *Parser) parse(b []byte) []Key {
	data := append(p.pending, b...)
	p.pending = nil
	var keys []Key
	i := 0
	for i < len(data) {
		c := data[i]
		switch {
		case c == 0x1b:
			// ESC 序列：ESC [ A/B/C/D/H/F；残包留待下一块
			if i+1 < len(data) && data[i+1] == '[' {
				if i+2 >= len(data) {
					p.pending = append(p.pending, data[i:]...)
					return keys
				}
				switch data[i+2] {
				case 'A':
					keys = append(keys, Key{Kind: KindUp})
				case 'B':
					keys = append(keys, Key{Kind: KindDown})
				case 'C':
					keys = append(keys, Key{Kind: KindRight})
				case 'D':
					keys = append(keys, Key{Kind: KindLeft})
				case 'H':
					keys = append(keys, Key{Kind: KindHome})
				case 'F':
					keys = append(keys, Key{Kind: KindEnd})
				case '5':
					if i+3 >= len(data) {
						p.pending = append(p.pending, data[i:]...)
						return keys
					}
					if data[i+3] == '~' {
						keys = append(keys, Key{Kind: KindPgUp})
						i += 4
						continue
					}
					keys = append(keys, Key{Kind: KindEsc})
					i += 3
					continue
				case '6':
					if i+3 >= len(data) {
						p.pending = append(p.pending, data[i:]...)
						return keys
					}
					if data[i+3] == '~' {
						keys = append(keys, Key{Kind: KindPgDown})
						i += 4
						continue
					}
					keys = append(keys, Key{Kind: KindEsc})
					i += 3
					continue
				default:
					keys = append(keys, Key{Kind: KindEsc})
				}
				i += 3
				continue
			}
			keys = append(keys, Key{Kind: KindEsc})
			i++
		case c == 0x0d:
			// ConPTY 对 Enter 可能交付 \r\n：吞掉紧随的 \n，避免残留污染下一次读取
			if i+1 < len(data) && data[i+1] == 0x0a {
				i++
			}
			keys = append(keys, Key{Kind: KindEnter})
			i++
		case c == 0x0a:
			keys = append(keys, Key{Kind: KindEnter})
			i++
		case c == 0x7f || c == 0x08:
			keys = append(keys, Key{Kind: KindBackspace})
			i++
		case c == 0x09:
			keys = append(keys, Key{Kind: KindTab})
			i++
		case c == 0x03:
			keys = append(keys, Key{Kind: KindCtrlC})
			i++
		case c == 0x04:
			keys = append(keys, Key{Kind: KindCtrlD})
			i++
		case c < 0x20:
			// 其余控制字符（含杂散 \x00）忽略
			i++
		default:
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 && !utf8.FullRune(data[i:]) {
				// 残缺 UTF-8：等下一块拼齐
				p.pending = append(p.pending, data[i:]...)
				return keys
			}
			keys = append(keys, Key{Kind: KindRune, R: r})
			i += size
		}
	}
	return keys
}

// logKeys GS_KEYLOG 诊断：记录原始字节与解析出的按键
func logKeys(chunk []byte, keys []Key) {
	path := os.Getenv("GS_KEYLOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	var names []string
	for _, k := range keys {
		if k.Kind == KindRune {
			names = append(names, string(k.R))
			continue
		}
		names = append(names, k.Kind.String())
	}
	_, _ = f.WriteString("bytes=" + fmtBytes(chunk) + " → " + strings.Join(names, " ") + "\n")
}
