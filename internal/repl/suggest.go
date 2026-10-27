package repl

// Levenshtein 按 rune 计算编辑距离
func Levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		curr[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// suggest 返回编辑距离 ≤2 的最近命令名；没有则返回空串
func suggest(names []string, input string) string {
	best := ""
	bestD := 3
	for _, n := range names {
		d := Levenshtein(n, input)
		if d < bestD {
			bestD = d
			best = n
		}
	}
	if bestD <= 2 {
		return best
	}
	return ""
}
