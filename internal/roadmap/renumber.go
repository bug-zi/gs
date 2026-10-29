// river 编号与重排纯逻辑。TOML 数组顺序是树序唯一权威。
package roadmap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// numPrefixRe 源材料编号前缀：1 、2.1 、1-3 、03.1- 、05- 、12、 等（数字开头可含 . 与 - 段，后接分隔符）
var numPrefixRe = regexp.MustCompile(`^[0-9][0-9.\-]*[\s\-_·.、]*`)

// StripNumPrefix 剥掉文件名标题中的源编号前缀；全被剥掉时回退原名（防误剥纯数字标题）
func StripNumPrefix(name string) string {
	base := strings.TrimSuffix(name, ".md")
	s := strings.Trim(numPrefixRe.ReplaceAllString(base, ""), " -_·.、")
	if s == "" {
		return strings.Trim(base, " -_·.、")
	}
	return s
}

// FormatPointNum 点位号两位补零（1-based）
func FormatPointNum(n int) string { return fmt.Sprintf("%02d", n) }

// NextDupSeq 该知识点现有 river 文件名中的下一个篇序 N：
// 无文件 → 0（首篇不带 .N）；仅有首篇 → 1；已有 .k → max(k)+1
func NextDupSeq(existing []string, pointNum int) int {
	dupPrefix := fmt.Sprintf("%02d.", pointNum)
	basePrefix := fmt.Sprintf("%02d-", pointNum)
	max, hasBase := 0, false
	for _, name := range existing {
		base := strings.TrimSuffix(filepath.Base(name), ".md")
		switch {
		case strings.HasPrefix(base, dupPrefix):
			var k int
			if n, _ := fmt.Sscanf(strings.TrimPrefix(base, dupPrefix), "%d", &k); n == 1 && k > max {
				max = k
			}
			hasBase = true
		case strings.HasPrefix(base, basePrefix):
			hasBase = true
		}
	}
	switch {
	case !hasBase:
		return 0
	case max == 0:
		return 1
	default:
		return max + 1
	}
}

// RiverFileName 知识点 river 文件名：dup=0 首篇 `<点两位>-<标题>.md`；dup≥1 同点第 dup 篇 `<点两位>.<dup>-<标题>.md`
func RiverFileName(pointNum, dup int, title string) string {
	if dup == 0 {
		return fmt.Sprintf("%s-%s.md", FormatPointNum(pointNum), title)
	}
	return fmt.Sprintf("%s.%d-%s.md", FormatPointNum(pointNum), dup, title)
}

// FileMove 重排计划单次迁移（工作区相对正斜杠）
type FileMove struct{ From, To string }

// RenumberPlan 按树序全量归位 river 的确定性计划
type RenumberPlan struct {
	Moves   []FileMove // 挂载文件归位/改名（含篇序紧凑化）
	Orphans []FileMove // 未挂载文件 → 99-未挂载（保留原名）
}

// RiverDirRel / UnmountedDirName river 层与收容区（工作区相对）
const (
	RiverDirRel      = "Study/river"
	UnmountedDirName = "99-未挂载"
)

// BuildRenumberPlan 依 TOML 当前树序生成 river 全量归位计划。
// existing 为 river 层现有文件清单（ListTierFiles 输出）。纯函数，不触碰文件系统；
// From==To 的文件不进计划（幂等，可重跑）。
func BuildRenumberPlan(rm *Roadmap, existing []string) *RenumberPlan {
	bound := map[string]PointRef{}
	per := map[PointRef][]string{}
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			for _, rel := range rm.Branches[bi].Points[pi].River {
				bound[rel] = PointRef{bi, pi}
				per[PointRef{bi, pi}] = append(per[PointRef{bi, pi}], rel)
			}
		}
	}
	plan := &RenumberPlan{}
	used := map[string]bool{"": true}

	add := func(m FileMove) {
		if m.From == m.To || used[m.To] {
			return
		}
		used[m.To] = true
		plan.Moves = append(plan.Moves, m)
	}

	for bi := range rm.Branches {
		dirName := fmt.Sprintf("%02d-%s", bi+1, rm.Branches[bi].Title)
		for pi := range rm.Branches[bi].Points {
			rels := per[PointRef{bi, pi}]
			sort.Strings(rels)
			pointNum := pi + 1
			for k, rel := range rels {
				title := StripNumPrefix(filepath.Base(rel))
				dup := k // 树序紧凑化：0=首篇，1..=篇序
				target := func() string {
					return RiverDirRel + "/" + dirName + "/" + RiverFileName(pointNum, dup, title)
				}
				for used[target()] { // 计划内撞名（同点同名多文件）→ 篇序顺延
					dup++
				}
				add(FileMove{rel, target()})
			}
		}
	}
	for _, rel := range existing {
		if _, ok := bound[rel]; ok {
			continue
		}
		to := RiverDirRel + "/" + UnmountedDirName + "/" + filepath.Base(rel)
		if rel == to || used[to] {
			continue
		}
		used[to] = true
		plan.Orphans = append(plan.Orphans, FileMove{rel, to})
	}
	return plan
}

// BuildUnmountMoves 把 rels 收容到 99-未挂载（保留原名；与收容区现有文件撞名时基名后插 .k 顺延）。
// existingUnmounted 为收容区现有文件名清单，纯函数不触碰文件系统。
func BuildUnmountMoves(rels []string, existingUnmounted []string) []FileMove {
	used := map[string]bool{}
	for _, e := range existingUnmounted {
		used[e] = true
	}
	moves := make([]FileMove, 0, len(rels))
	for _, rel := range rels {
		base := filepath.Base(rel)
		if used[base] {
			ext := filepath.Ext(base)
			stem := strings.TrimSuffix(base, ext)
			for k := 1; ; k++ {
				cand := fmt.Sprintf("%s.%d%s", stem, k, ext)
				if !used[cand] {
					base = cand
					break
				}
			}
		}
		used[base] = true
		moves = append(moves, FileMove{rel, RiverDirRel + "/" + UnmountedDirName + "/" + base})
	}
	return moves
}

// ListUnmounted 列出收容区现有文件名（目录缺失视为空；子目录不计入）
func ListUnmounted(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, RiverDirRel, UnmountedDirName))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}
