package complete

import "strings"

// Result 输入行当前应展示的补全状态
type Result struct {
	Ghost      string   // 可接受续写（Tab/→ 上屏）；空 = 无
	Candidates []string // 面板候选；空 = 不弹
	Hint       string   // 面板顶部参数提示；空 = 无
}

// For 计算输入行的补全状态。匹配大小写不敏感；
// 别名与其命令名同时匹配前缀时只保留命令名。
func For(tree Tree, line string) Result {
	if !strings.HasPrefix(line, "/") {
		return Result{}
	}
	body := line[1:]
	if body == "" {
		return Result{Candidates: allNames(tree)}
	}
	fields := strings.Split(body, " ")
	first := fields[0]
	if len(fields) == 1 {
		cands := matchNames(tree, first)
		if len(cands) == 1 {
			return Result{Ghost: ghostOf(cands[0], first)}
		}
		if len(cands) > 1 {
			return Result{Candidates: cands}
		}
		return Result{}
	}
	cmd := findCmd(tree, first)
	if cmd == nil {
		return Result{}
	}
	if len(fields) == 2 && fields[1] != "" {
		second := fields[1]
		var cands []string
		for _, s := range cmd.Subs {
			if strings.HasPrefix(strings.ToLower(s), strings.ToLower(second)) {
				cands = append(cands, s)
			}
		}
		if len(cands) == 1 {
			return Result{Ghost: cands[0][len(second):], Hint: cmd.Hint}
		}
		if len(cands) > 1 {
			return Result{Candidates: cands, Hint: cmd.Hint}
		}
		return Result{Hint: cmd.Hint}
	}
	if len(fields) > 2 {
		// 更深的参数位：仅参数提示
		return Result{Hint: cmd.Hint}
	}
	// 尾随空格：列子命令 + 参数提示
	return Result{Candidates: append([]string(nil), cmd.Subs...), Hint: cmd.Hint}
}

// ghostOf 返回 full 相对已打部分 typed 的剩余；大小写不敏感校验前缀
func ghostOf(full, typed string) string {
	if !strings.HasPrefix(strings.ToLower(full), strings.ToLower(typed)) {
		return ""
	}
	return full[len(typed):]
}

// allNames 全部命令名（不含别名，避免面板冗余）
func allNames(tree Tree) []string {
	var out []string
	for _, c := range tree {
		out = append(out, c.Name)
	}
	return out
}

// matchNames 前缀匹配命令名；别名仅在其命令名不匹配时参与
func matchNames(tree Tree, prefix string) []string {
	low := strings.ToLower(prefix)
	var out []string
	seen := map[string]bool{}
	add := func(n string) {
		if !seen[n] && strings.HasPrefix(strings.ToLower(n), low) {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, c := range tree {
		add(c.Name)
	}
	for _, c := range tree {
		if strings.HasPrefix(strings.ToLower(c.Name), low) {
			continue // 命令名已匹配，别名不重复出现
		}
		for _, a := range c.Alias {
			add(a)
		}
	}
	return out
}

// findCmd 按名称或别名查找（大小写不敏感）
func findCmd(tree Tree, name string) *Cmd {
	low := strings.ToLower(name)
	for i := range tree {
		if strings.ToLower(tree[i].Name) == low {
			return &tree[i]
		}
	}
	for i := range tree {
		for _, a := range tree[i].Alias {
			if strings.ToLower(a) == low {
				return &tree[i]
			}
		}
	}
	return nil
}
