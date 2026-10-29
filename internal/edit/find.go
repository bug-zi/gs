package edit

import "strings"

// match 一处查找匹配:逻辑行 + [col,end) rune 区间
type match struct {
	line, col, end int
}

// findMatches 忽略大小写全量扫描;空词返回 nil。
// 用 rune 级 ToLower 折叠(CJK 无影响;个别 Unicode 大写在折叠后 rune 数变化,
// 属可接受近似,md 笔记场景不出现)
func findMatches(lines [][]rune, q string) []match {
	qr := []rune(strings.ToLower(q))
	if len(qr) == 0 {
		return nil
	}
	var out []match
	for li, lr := range lines {
		low := []rune(strings.ToLower(string(lr)))
		for i := 0; i+len(qr) <= len(low); i++ {
			if string(low[i:i+len(qr)]) == string(qr) {
				out = append(out, match{line: li, col: i, end: i + len(qr)})
			}
		}
	}
	return out
}
