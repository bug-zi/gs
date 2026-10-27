package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PageSize 返回 (每页行数, 是否分页)。stdout 非终端 → (0, false)；
// 终端场景页高 = 终端高 - 1（预留提示行），最低保 5 行。
func PageSize() (int, bool) {
	_, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || h <= 0 {
		return 0, false
	}
	h--
	if h < 5 {
		h = 5
	}
	return h, true
}

// TermWidth 返回 stdout 终端宽度；非终端或取不到返回 def。
func TermWidth(def int) int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return def
	}
	return w
}

// SplitPages 按每页 height 行把行切片切页；height<=0 视为单页
func SplitPages(lines []string, height int) [][]string {
	if height <= 0 {
		if len(lines) == 0 {
			return nil
		}
		return [][]string{lines}
	}
	var pages [][]string
	for i := 0; i < len(lines); i += height {
		end := i + height
		if end > len(lines) {
			end = len(lines)
		}
		pages = append(pages, lines[i:end])
	}
	return pages
}

// PagedPrint 分页写出 content：满 height 行停一次读键。
// 空输入或空格=下一页；b=上一页（首页原地重打）；其他输入=跳过剩余页直接打完；
// 读键 EOF=补齐剩余页。height<=0 或内容不足一页时一次打完。
func PagedPrint(out io.Writer, nextKey func() (string, bool), content string, height int) {
	content = strings.TrimRight(content, "\n")
	if height <= 0 {
		fmt.Fprintln(out, content)
		return
	}
	pages := SplitPages(strings.Split(content, "\n"), height)
	for cur := 0; cur < len(pages); {
		fmt.Fprintln(out, strings.Join(pages[cur], "\n"))
		cur++
		if cur == len(pages) {
			return
		}
		key, ok := nextKey()
		if !ok {
			fmt.Fprintln(out, strings.Join(flatten(pages[cur:]), "\n"))
			return
		}
		switch strings.TrimSpace(key) {
		case "", " ":
		case "b":
			if cur > 0 {
				cur--
			}
		default:
			fmt.Fprintln(out, strings.Join(flatten(pages[cur:]), "\n"))
			return
		}
	}
}

// flatten 把多页拍平回单行切片
func flatten(pages [][]string) []string {
	var out []string
	for _, p := range pages {
		out = append(out, p...)
	}
	return out
}
