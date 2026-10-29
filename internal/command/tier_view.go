package command

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// TierItem 层内条目：Title 展示标题，Sub 组内小标签（如知识点分组，可空），Path 工作区相对路径；
// Shipped 标记已装船（sea 选择器置灰排后用，其余层恒为零值）
type TierItem struct {
	Title   string
	Sub     string
	Path    string
	Shipped bool
}

// TierGroup 板块及其条目；roadmap 组序即由浅入深，目录组按名排序
type TierGroup struct {
	Name  string
	Items []TierItem
}

// springBinds / riverBinds 取知识点在该层的绑定（spring 0/1；river 多绑）；sea 走目录分组不用绑定
func springBinds(p *roadmap.Point) []string {
	if p.Spring == "" {
		return nil
	}
	return []string{p.Spring}
}

func riverBinds(p *roadmap.Point) []string { return p.River }

// shipBinds 取知识点 ship 层绑定（加工中条目分组用）
func shipBinds(p *roadmap.Point) []string { return p.Ship }

// codeBinds 取知识点 code 层绑定（0/1）
func codeBinds(p *roadmap.Point) []string {
	if p.Code == "" {
		return nil
	}
	return []string{p.Code}
}

// buildRoadmapGroups 按 roadmap 主干聚合绑定条目；rm 为 nil 时归入单一平铺组（组名用层名）。
// 以磁盘文件为准：绑定指向不存在文件的幽灵绑定不展示；未绑定文件归入尾部「未挂载」组。
func buildRoadmapGroups(rm *roadmap.Roadmap, files []string, binds func(*roadmap.Point) []string, tier string) []TierGroup {
	ext := ".md"
	if tier == "code" {
		ext = ".go"
	}
	// 多绑层（ship/river）同一点可挂多篇，标题须用文件原名区分；单绑层（spring/code）
	// 文件名随知识点生成，用知识点标题更直观
	multiBind := tier == "ship" || tier == "river"
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}
	var groups []TierGroup
	bound := map[string]bool{}
	if rm != nil {
		for bi := range rm.Branches {
			b := &rm.Branches[bi]
			g := TierGroup{Name: b.Title}
			seen := map[string]bool{}
			for pi := range b.Points {
				p := &b.Points[pi]
				for _, rel := range binds(p) {
					if !fileSet[rel] || seen[rel] {
						continue
					}
					seen[rel] = true
					bound[rel] = true
					title := p.Title
					if multiBind {
						title = strings.TrimSuffix(filepath.Base(rel), ext)
					}
					g.Items = append(g.Items, TierItem{Title: title, Sub: p.Group, Path: rel})
				}
			}
			if len(g.Items) > 0 {
				groups = append(groups, g)
			}
		}
	}
	var loose []TierItem
	for _, f := range files {
		if !bound[f] {
			loose = append(loose, TierItem{Title: strings.TrimSuffix(filepath.Base(f), ext), Path: f})
		}
	}
	sort.Slice(loose, func(i, j int) bool { return loose[i].Title < loose[j].Title })
	if len(loose) > 0 {
		name := "未挂载"
		if rm == nil {
			name = tier
		}
		groups = append(groups, TierGroup{Name: name, Items: loose})
	}
	return groups
}

// buildDirGroups 按一级子目录聚合（sea）；根目录散件归 looseName 组，组与组内条目均按名排序。
// shipped 为已装船基名集合（nil 合法，键为编号归一后的名）：命中的条目带 Shipped 标记，
// 组内未装船排前、已装船排后。
func buildDirGroups(tier string, files []string, looseName string, shipped map[string]bool) []TierGroup {
	prefix := "Study/" + tier + "/"
	byDir := map[string][]TierItem{}
	var loose []TierItem
	for _, f := range files {
		rel := strings.TrimPrefix(f, prefix)
		name := strings.TrimSuffix(filepath.Base(rel), ".md")
		it := TierItem{Title: name, Path: f, Shipped: shipped[roadmap.StripNumPrefix(name)]}
		if i := strings.Index(rel, "/"); i >= 0 {
			dir := rel[:i]
			byDir[dir] = append(byDir[dir], it)
		} else {
			loose = append(loose, it)
		}
	}
	var groups []TierGroup
	for dir, items := range byDir {
		sortShippedFirst(items)
		groups = append(groups, TierGroup{Name: dir, Items: items})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	sortShippedFirst(loose)
	if len(loose) > 0 {
		groups = append(groups, TierGroup{Name: looseName, Items: loose})
	}
	return groups
}

// sortShippedFirst 组内排序：未装船在前，各自按名升序
func sortShippedFirst(items []TierItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Shipped != items[j].Shipped {
			return !items[i].Shipped
		}
		return items[i].Title < items[j].Title
	})
}

// filterTierGroups 关键词过滤：命中组名保留整组，否则按小标签/标题筛成员；空关键词原样返回
func filterTierGroups(groups []TierGroup, kw string) []TierGroup {
	kw = strings.ToLower(strings.TrimSpace(kw))
	if kw == "" {
		return groups
	}
	var out []TierGroup
	for _, g := range groups {
		if strings.Contains(strings.ToLower(g.Name), kw) {
			out = append(out, g)
			continue
		}
		var items []TierItem
		for _, it := range g.Items {
			if strings.Contains(strings.ToLower(it.Title), kw) || strings.Contains(strings.ToLower(it.Sub), kw) {
				items = append(items, it)
			}
		}
		if len(items) > 0 {
			out = append(out, TierGroup{Name: g.Name, Items: items})
		}
	}
	return out
}

// distinctSubs 组内不同小标签数（空标签不计）
func distinctSubs(g TierGroup) int {
	seen := map[string]bool{}
	n := 0
	for _, it := range g.Items {
		if it.Sub != "" && !seen[it.Sub] {
			seen[it.Sub] = true
			n++
		}
	}
	return n
}

// tierItemLabel 条目展示：多组时带「【组名】」前缀；组内含多小标签时以「标签 · 」辅助定位
func tierItemLabel(g TierGroup, it TierItem, multi bool) string {
	s := it.Title
	if distinctSubs(g) >= 2 && it.Sub != "" {
		s = it.Sub + " · " + s
	}
	if multi {
		s = "【" + g.Name + "】" + s
	}
	return s
}

// tierRelPath 层内相对展示路径（去 Study/<层>/ 前缀）
func tierRelPath(tier, rel string) string {
	return strings.TrimPrefix(rel, "Study/"+tier+"/")
}

// pickTierEntryGrouped 板块化选择层内条目：spring/river 按 roadmap 主干聚合，sea 按一级子目录聚合。
// 多组且无关键词走两级（板块→条目）；有关键词平铺直选；仅一条也展示选择列表（回车确认，Esc 取消）。返回选中条目。
func pickTierEntryGrouped(ctx *Ctx, tier, kw, label string) (TierItem, error) {
	files, err := ListTierFiles(ctx.WS, tier)
	if err != nil {
		return TierItem{}, err
	}
	var groups []TierGroup
	switch tier {
	case "sea":
		groups = buildDirGroups(tier, files, "未归类", shippedSeaSet(ctx.WS))
	default:
		rm, _ := loadRoadmapSoft(ctx)
		binds := riverBinds
		switch tier {
		case "spring":
			binds = springBinds
		case "code":
			binds = codeBinds
		case "ship":
			binds = shipBinds
		}
		groups = buildRoadmapGroups(rm, files, binds, tier)
	}
	groups = filterTierGroups(groups, kw)
	total := 0
	for _, g := range groups {
		total += len(g.Items)
	}
	if total == 0 {
		return TierItem{}, fmt.Errorf("没有匹配的 %s 条目", tier)
	}
	if len(groups) > 1 && strings.TrimSpace(kw) == "" {
		var pick string
		opts := make([]menuOption, 0, len(groups))
		for i, g := range groups {
			opts = append(opts, option(fmt.Sprintf("%s（%d 篇）", g.Name, len(g.Items)), strconv.Itoa(i)))
		}
		if err := selectFrom(ctx, "选择板块", opts, &pick); err != nil {
			return TierItem{}, err
		}
		var gi int
		fmt.Sscanf(pick, "%d", &gi)
		if gi < 0 || gi >= len(groups) {
			return TierItem{}, fmt.Errorf("板块选择异常")
		}
		groups = groups[gi : gi+1]
	}
	multi := len(groups) > 1
	var opts []menuOption
	for _, g := range groups {
		for _, it := range g.Items {
			opts = append(opts, menuOption{Label: tierItemLabel(g, it, multi), Value: it.Path, Dim: it.Shipped})
		}
	}
	var pick string
	if err := selectFrom(ctx, label, opts, &pick); err != nil {
		return TierItem{}, err
	}
	for _, g := range groups {
		for _, it := range g.Items {
			if it.Path == pick {
				return it, nil
			}
		}
	}
	return TierItem{}, fmt.Errorf("条目选择异常")
}

// runTierItemView 条目查阅态；标题显示层内相对路径，不带 Study/<层>/ 前缀；.go 走代码高亮。
// 两路查阅（短文件 pager / 长文件全屏 TUI）都带 [x] 删除当前条目。
func runTierItemView(ctx *Ctx, tierName string, it TierItem) error {
	abs := filepath.Join(ctx.WS.Root, filepath.FromSlash(it.Path))
	title := ui.Path(tierRelPath(tierName, it.Path))
	del := func() bool {
		deleted, err := deleteTierEntry(ctx, tierName, it.Path)
		if err != nil {
			fmt.Fprintln(ctx.Out, ui.Err(err.Error()))
		}
		return deleted
	}
	return runViewPagerDel(ctx,
		func() string {
			data, err := os.ReadFile(abs)
			if err != nil {
				return "（读取失败: " + err.Error() + "）"
			}
			if strings.EqualFold(filepath.Ext(it.Path), ".go") {
				return title + "\n" + ui.RenderGo(string(data))
			}
			return title + "\n" + ui.RenderMD(string(data))
		},
		func() (string, bool) { return abs, true },
		del)
}

// shippedSeaSet 已装船基名集合（键为编号归一后的名）：ship 同名副本（加工中）∪ river 条目。
// 键与查询统一走 StripNumPrefix：sea 原件带源编号（1.2.0 xxx）、装船/入河后改名或领号
// （02-xxx），原始基名比对会漏标。层目录列举失败时静默跳过该层——宁漏标、不阻塞浏览。
func shippedSeaSet(ws *workspace.Workspace) map[string]bool {
	set := map[string]bool{}
	for _, tier := range []string{"ship", "river"} {
		if files, err := ListTierFiles(ws, tier); err == nil {
			for _, f := range files {
				set[roadmap.StripNumPrefix(filepath.Base(f))] = true
			}
		}
	}
	return set
}
