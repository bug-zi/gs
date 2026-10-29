package edit

import "strings"

// leadIndent 行首空白 rune 数(空格/制表各计 1)
func leadIndent(lr []rune) int {
	n := 0
	for _, r := range lr {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
}

// isListItem 是否列表行(无序/有序)
func isListItem(lr []rune) bool {
	s := string(lr)
	return reBullet.MatchString(s) || reOrdered.MatchString(s)
}

// itemSpan 行 i 所属块 [start,end):列表项含缩进更深的连续续行
// (止于空行或同深/更浅行);普通行块=单逻辑行
func itemSpan(lines [][]rune, i int) (int, int) {
	if !isListItem(lines[i]) {
		return i, i + 1
	}
	ind := leadIndent(lines[i])
	end := i + 1
	for end < len(lines) {
		l := lines[end]
		if strings.TrimSpace(string(l)) == "" || leadIndent(l) <= ind {
			break
		}
		end++
	}
	return i, end
}

func concatBlock(parts ...[][]rune) [][]rune {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([][]rune, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// moveUpDown 计算块上/下移后的新行表与新光标行;到边界不可移返回 false。
// 有序列表编号不重排(设计决策:原样移动)
func moveUpDown(lines [][]rune, cur int, up bool) ([][]rune, int, bool) {
	s, e := itemSpan(lines, cur)
	var out [][]rune
	newStart := 0
	switch {
	case up && s == 0, !up && e >= len(lines):
		return nil, 0, false
	case up:
		ps, _ := itemSpan(lines, s-1)
		out = concatBlock(lines[:ps], lines[s:e], lines[ps:s], lines[e:])
		newStart = ps
	default:
		_, ne := itemSpan(lines, e)
		out = concatBlock(lines[:s], lines[e:ne], lines[s:e], lines[ne:])
		newStart = ne - (e - s)
	}
	return out, newStart + (cur - s), true
}
