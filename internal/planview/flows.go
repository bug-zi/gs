package planview

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bugzi/gs/internal/roadmap"
)

// 结构操作菜单项（browser.go 'o' 键入口，下标即选项）
var structureMenu = []string{"合并", "拆分", "换主干", "换分组", "删除"}

// branchMenu 主干行维护菜单（全局树主干行 o 键）
var branchMenu = []string{"删除分支"}

// listAcc 数组型三类绑定访问器
func listAcc(t Tier) func(*roadmap.Point) *[]string {
	switch t {
	case TierSea:
		return roadmap.SeaList
	case TierShip:
		return roadmap.ShipList
	case TierRiver:
		return roadmap.RiverList
	}
	return nil
}

// bindSingle 把 rel 绑定挂到 ref 的对应清单
func bindSingle(m *Model, ref roadmap.PointRef, t Tier, rel string) {
	switch t {
	case TierSpring:
		m.RM.BindSpring(ref, rel)
	case TierCode:
		m.RM.BindCode(ref, rel)
	case TierSea:
		m.RM.BindSea(ref, rel)
	case TierShip:
		m.RM.BindShip(ref, rel)
	case TierRiver:
		m.RM.BindRiver(ref, rel)
	}
}

// bindMove 五类通用移动；spring/code 目标已占用返回 false
func bindMove(m *Model, src roadmap.PointRef, t Tier, rel string, dst roadmap.PointRef) bool {
	switch t {
	case TierSpring:
		if m.RM.PointAt(dst).Spring != "" {
			return false
		}
		m.RM.UnbindSpring(src)
		m.RM.BindSpring(dst, rel)
	case TierCode:
		if m.RM.PointAt(dst).Code != "" {
			return false
		}
		m.RM.UnbindCode(src)
		m.RM.BindCode(dst, rel)
	default:
		return m.RM.MoveBinding(src, listAcc(t), rel, dst)
	}
	return true
}

// save 落盘并刷新待归类缓存；失败写错误行不中断
func (m *Model) save(h Hooks) {
	if h.Save != nil {
		if err := h.Save(m.RM); err != nil {
			m.Err = "落盘失败：" + err.Error()
		}
	}
	m.refreshUnbound()
}

// locate 结构操作后按标题重新定位（标题全树唯一）；找不到不动
func (m *Model) locate(title string) {
	for bi := range m.RM.Branches {
		for pi := range m.RM.Branches[bi].Points {
			if m.RM.Branches[bi].Points[pi].Title == title {
				m.Branch, m.Point = bi, pi
				m.Expanded[bi] = true
				m.Normalize()
				return
			}
		}
	}
}

// doUnbind 卸载光标绑定（详情页笔记行）；只动记录，文件留磁盘
func doUnbind(m *Model, h Hooks) {
	if m.View != viewPoint {
		m.Err = "详情页才能卸载绑定"
		return
	}
	r := m.rowAt(m.Cursor)
	if r == nil || r.Kind != rowNote {
		m.Err = "光标不在绑定条目上"
		return
	}
	ref := m.curRef()
	switch r.Tier {
	case TierSpring:
		m.RM.UnbindSpring(ref)
	case TierCode:
		m.RM.UnbindCode(ref)
	default:
		if !m.RM.UnbindList(ref, listAcc(r.Tier), r.Note) {
			m.Err = "绑定不存在"
			return
		}
	}
	m.save(h)
}

// doMove 移动光标绑定到其他知识点（详情页笔记行；可跨主干）
func doMove(m *Model, h Hooks) {
	r := m.rowAt(m.Cursor)
	if m.View != viewPoint || r == nil || r.Kind != rowNote {
		m.Err = "光标不在绑定条目上"
		return
	}
	if h.SelectPoint == nil {
		m.Err = "该操作不可用"
		return
	}
	dst, ok := h.SelectPoint("移动到哪个知识点？")
	if !ok {
		return
	}
	src := m.curRef()
	if src == dst {
		m.Err = "目标就是当前知识点"
		return
	}
	if r.Tier == TierSpring || r.Tier == TierCode {
		if !bindMove(m, src, r.Tier, r.Note, dst) {
			m.Err = "目标知识点已有该项，先卸载后再移动"
			return
		}
	} else if !m.RM.MoveBinding(src, listAcc(r.Tier), r.Note, dst) {
		m.Err = "绑定不存在"
		return
	}
	m.save(h)
}

// doAttach 挂载未归类文件：详情页=挂到光标清单节；待归类区=挂到选定知识点
func doAttach(m *Model, h Hooks) {
	switch m.View {
	case viewPoint:
		tier, ok := m.TierAtCursor()
		if !ok {
			m.Err = "光标不在清单节内"
			return
		}
		ref := m.curRef()
		if (tier == TierSpring && m.RM.PointAt(ref).Spring != "") ||
			(tier == TierCode && m.RM.PointAt(ref).Code != "") {
			m.Err = "本知识点已有该项，先卸载再挂载"
			return
		}
		m.refreshUnbound()
		rel, ok := pickUnbound(m, h, tier)
		if !ok {
			return
		}
		bindSingle(m, ref, tier, rel)
		m.save(h)
	case viewUnbound:
		r := m.rowAt(m.Cursor)
		if r == nil || r.Kind != rowNote {
			m.Err = "光标不在文件行上"
			return
		}
		if h.SelectPoint == nil {
			m.Err = "该操作不可用"
			return
		}
		dst, ok := h.SelectPoint("挂载到哪个知识点？")
		if !ok {
			return
		}
		if m.RM.IsBound(r.Note) { // 快照后已被他处挂载：转为移动
			owner, _ := m.RM.OwnerOf(r.Note)
			if !h.Confirm("该文件已挂载于「" + m.RM.PathOf(owner) + "」，改为移动？") {
				return
			}
			if !bindMove(m, owner, r.Tier, r.Note, dst) {
				m.Err = "目标知识点已有该项，操作取消"
				return
			}
		} else {
			if (r.Tier == TierSpring && m.RM.PointAt(dst).Spring != "") ||
				(r.Tier == TierCode && m.RM.PointAt(dst).Code != "") {
				m.Err = "目标知识点已有该项，先卸载再挂载"
				return
			}
			bindSingle(m, dst, r.Tier, r.Note)
		}
		m.refreshUnbound()
		m.save(h)
	default:
		m.Err = "该视图不支持挂载"
	}
}

// pickUnbound 从未归类清单挑一个文件；空清单报错不弹菜单
func pickUnbound(m *Model, h Hooks, t Tier) (string, bool) {
	files := m.unboundCache[t]
	if len(files) == 0 {
		m.Err = "没有未归类的 " + t.Label() + " 文件"
		return "", false
	}
	if h.SelectOne == nil {
		m.Err = "该操作不可用"
		return "", false
	}
	idx, ok := h.SelectOne("挂载哪个文件？", files)
	if !ok {
		return "", false
	}
	return files[idx], true
}

// doRename 知识点改名；spring 文件按约定跟随（文件改名经壳层回调）
func doRename(m *Model, h Hooks) {
	ref, ok := m.PointAtCursor()
	if !ok {
		m.Err = "光标不在知识点上"
		return
	}
	if h.Input == nil {
		m.Err = "该操作不可用"
		return
	}
	title, ok := h.Input("新标题（当前：" + m.RM.PointAt(ref).Title + "）")
	if !ok {
		return
	}
	oldSpring, err := m.RM.RenamePoint(ref, title)
	if err != nil {
		m.Err = err.Error()
		return
	}
	if oldSpring != "" && m.exists(oldSpring) && h.RenameFile != nil {
		if err := h.RenameFile(oldSpring, roadmap.SpringPath(m.RM.PointAt(ref).Title)); err != nil {
			m.Err = "spring 文件改名失败：" + err.Error()
		}
	}
	m.save(h)
}

// doMenu 结构操作菜单分发：知识点行 → structureMenu；全局树主干行 → branchMenu
func doMenu(m *Model, h Hooks) {
	if _, ok := m.PointAtCursor(); ok {
		if h.SelectOne == nil {
			m.Err = "该操作不可用"
			return
		}
		idx, ok := h.SelectOne("对知识点执行什么操作？", structureMenu)
		if !ok {
			return
		}
		switch structureMenu[idx] {
		case "合并":
			doMerge(m, h)
		case "拆分":
			doSplit(m, h)
		case "换主干":
			doMoveBranch(m, h)
		case "换分组":
			doSetGroup(m, h)
		case "删除":
			doDelete(m, h)
		}
		return
	}
	if r := m.rowAt(m.Cursor); r != nil && r.Kind == rowBranchHead && m.View == viewGlobal {
		doBranchMenu(m, h, r.Branch)
		return
	}
	m.Err = "光标不在知识点或主干上"
}

// doBranchMenu 主干行操作菜单分发
func doBranchMenu(m *Model, h Hooks, bi int) {
	if h.SelectOne == nil {
		m.Err = "该操作不可用"
		return
	}
	idx, ok := h.SelectOne("对主干「"+m.RM.Branches[bi].Title+"」执行什么操作？", branchMenu)
	if !ok {
		return
	}
	if branchMenu[idx] == "删除分支" {
		doDeleteBranch(m, h, bi)
	}
}

// doDeleteBranch 删除空主干；对应空 river 板块目录移入回收站（renumber 提示并入确认文案）
func doDeleteBranch(m *Model, h Hooks, bi int) {
	b := m.RM.Branches[bi]
	if len(b.Points) > 0 {
		m.Err = "主干仍有知识点，先删除或移走后再删除"
		return
	}
	if !h.Confirm(fmt.Sprintf("删除空主干「%s」？（删除后可 /plan renumber 重排编号）", b.Title)) {
		return
	}
	title := b.Title
	if err := m.RM.DeleteBranch(bi); err != nil {
		m.Err = err.Error()
		return
	}
	m.save(h)
	if h.TrashRel != nil {
		dirRel := fmt.Sprintf("%s/%02d-%s", roadmap.RiverDirRel, bi+1, title)
		_ = h.TrashRel(dirRel) // 目录不存在时由壳层实现静默，不阻断
	}
}

// doMerge 光标点 A ← 选点 B 并入 A（B 删除）
func doMerge(m *Model, h Hooks) {
	keep, ok := m.PointAtCursor()
	if !ok {
		m.Err = "光标不在知识点上"
		return
	}
	if h.SelectPoint == nil {
		m.Err = "该操作不可用"
		return
	}
	absorb, ok := h.SelectPoint("把哪个知识点并入「" + m.RM.PointAt(keep).Title + "」？")
	if !ok {
		return
	}
	if absorb == keep {
		m.Err = "不能并入自身"
		return
	}
	kp, ap := m.RM.PointAt(keep), m.RM.PointAt(absorb)
	if kp.Spring != "" && ap.Spring != "" {
		idx, ok := h.SelectOne("双方都有 spring，保留哪个？",
			[]string{"保留「" + kp.Title + "」的", "保留「" + ap.Title + "」的"})
		if !ok {
			return
		}
		if idx == 1 {
			kp.Spring = ap.Spring
		}
	}
	if kp.Code != "" && ap.Code != "" {
		idx, ok := h.SelectOne("双方都有 code，保留哪个？",
			[]string{"保留「" + kp.Title + "」的", "保留「" + ap.Title + "」的"})
		if !ok {
			return
		}
		if idx == 1 {
			kp.Code = ap.Code
		}
	}
	if !h.Confirm(fmt.Sprintf("把「%s」并入「%s」？绑定将合并，其节点将被删除", ap.Title, kp.Title)) {
		return
	}
	keepTitle := kp.Title
	m.RM.MergePoints(keep, absorb)
	m.locate(keepTitle)
	m.save(h)
}

// doSplit 拆分：新点插正后方（同组、todo）；sea/ship/river 逐清单挑选移入；
// 选择阶段不改 RM，放弃（Esc）即整体取消，不产生半截变更
func doSplit(m *Model, h Hooks) {
	src, ok := m.PointAtCursor()
	if !ok {
		m.Err = "光标不在知识点上"
		return
	}
	if h.Input == nil || h.SelectOne == nil {
		m.Err = "该操作不可用"
		return
	}
	title, ok := h.Input("新知识点标题（拆自「" + m.RM.PointAt(src).Title + "」）")
	if !ok {
		return
	}
	if err := m.RM.CheckTitleUnique(title); err != nil {
		m.Err = err.Error()
		return
	}
	type allocItem struct {
		t   Tier
		rel string
	}
	var allocs []allocItem
	picked := map[string]bool{}
	for _, t := range []Tier{TierSea, TierShip, TierRiver} {
		for {
			var remain []string
			for _, rel := range *listAcc(t)(m.RM.PointAt(src)) {
				if !picked[rel] {
					remain = append(remain, rel)
				}
			}
			if len(remain) == 0 {
				break
			}
			opts := append(append([]string{}, remain...), "（完成本清单）")
			idx, ok := h.SelectOne("「"+t.Label()+"」哪些移到新知识点？（Esc 取消整个拆分）", opts)
			if !ok {
				return
			}
			if idx == len(remain) {
				break
			}
			picked[remain[idx]] = true
			allocs = append(allocs, allocItem{t, remain[idx]})
		}
	}
	srcTitle := m.RM.PointAt(src).Title
	np, err := m.RM.SplitPoint(src, title)
	if err != nil {
		m.Err = err.Error()
		return
	}
	for _, a := range allocs {
		m.RM.MoveBinding(src, listAcc(a.t), a.rel, np)
	}
	m.locate(srcTitle)
	m.save(h)
}

// doMoveBranch 换主干：连带全部绑定挪到目标主干末尾
func doMoveBranch(m *Model, h Hooks) {
	ref, ok := m.PointAtCursor()
	if !ok {
		m.Err = "光标不在知识点上"
		return
	}
	if h.SelectOne == nil {
		m.Err = "该操作不可用"
		return
	}
	names := make([]string, 0, len(m.RM.Branches))
	for i := range m.RM.Branches {
		names = append(names, m.RM.Branches[i].Title)
	}
	idx, ok := h.SelectOne("换到哪个主干？", names)
	if !ok {
		return
	}
	if idx == ref.Branch {
		m.Err = "已在该主干"
		return
	}
	title := m.RM.PointAt(ref).Title
	m.RM.MovePointToBranch(ref, idx)
	m.locate(title)
	m.save(h)
}

// doSetGroup 换分组标签（空清除）
func doSetGroup(m *Model, h Hooks) {
	ref, ok := m.PointAtCursor()
	if !ok {
		m.Err = "光标不在知识点上"
		return
	}
	if h.Input == nil {
		m.Err = "该操作不可用"
		return
	}
	group, ok := h.Input("分组标签（留空清除）")
	if !ok {
		return
	}
	m.RM.SetGroup(ref, strings.TrimSpace(group))
	m.save(h)
}

// doDelete 删除知识点：预览解绑与 river 收容影响 → 确认 → 清绑定落盘 → river 收容迁移。
// 顺序：落盘先行、文件迁移在后；迁移失败残留态可 /plan renumber 收容修复。
func doDelete(m *Model, h Hooks) {
	ref, ok := m.PointAtCursor()
	if !ok {
		m.Err = "光标不在知识点上"
		return
	}
	p := m.RM.PointAt(ref)
	river := append([]string{}, p.River...)
	summary := "删除知识点「" + p.Title + "」？"
	if !m.RM.PointEmpty(ref) {
		summary += "\n将解绑：" + BindingsBrief(p)
	}
	for _, rel := range river {
		summary += "\n收容：" + rel + " → 99-未挂载"
	}
	if !h.Confirm(summary) {
		return
	}
	m.RM.DetachPoint(ref)
	if err := m.RM.DeletePoint(ref); err != nil {
		m.Err = err.Error()
		return
	}
	m.save(h)
	if len(river) > 0 {
		if h.ApplyMoves == nil {
			m.Err = "该操作不可用"
			return
		}
		moves := roadmap.BuildUnmountMoves(river, roadmap.ListUnmounted(m.Root))
		if err := h.ApplyMoves(moves); err != nil {
			m.Err = "river 收容失败（绑定已清，可 /plan renumber 收容修复）：" + err.Error()
			return
		}
		m.refreshUnbound()
	}
}

// BindingsBrief 非空绑定摘要（删除预览用）
func BindingsBrief(p *roadmap.Point) string {
	var parts []string
	if p.Spring != "" {
		parts = append(parts, "spring")
	}
	if p.Code != "" {
		parts = append(parts, "code")
	}
	if len(p.River) > 0 {
		parts = append(parts, "river×"+strconv.Itoa(len(p.River)))
	}
	if len(p.Ship) > 0 {
		parts = append(parts, "ship×"+strconv.Itoa(len(p.Ship)))
	}
	if len(p.Sea) > 0 {
		parts = append(parts, "sea×"+strconv.Itoa(len(p.Sea)))
	}
	return strings.Join(parts, "、")
}

// doShipCopy 装船：详情页 sea 行按 c —— 复制入 ship/（sea 原件与 sea 绑定保留），
// 当前知识点 ship 追加绑定后落盘；成功后光标跳到下一个未消费 sea 行便于连按
func doShipCopy(m *Model, h Hooks) {
	if m.View != viewPoint {
		m.Err = "详情页才能装船"
		return
	}
	r := m.rowAt(m.Cursor)
	if r == nil || r.Kind != rowNote || r.Tier != TierSea {
		m.Err = "光标不在 sea 条目上"
		return
	}
	if h.CopyFileRel == nil {
		m.Err = "该操作不可用"
		return
	}
	if !m.exists(r.Note) {
		m.Err = "文件已失效：" + r.Note
		return
	}
	dstRel := "Study/ship/" + filepath.Base(r.Note)
	if err := h.CopyFileRel(r.Note, dstRel); err != nil {
		m.Err = "装船失败：" + err.Error()
		return
	}
	m.RM.BindShip(m.curRef(), dstRel)
	m.save(h)
	m.refocusAfterShip(r.Note)
}

// refocusAfterShip 装船后 ship 节 +1 行使行号整体偏移，按 rel 重定位本行再跳到
// 下一个未消费 sea 行；无未消费行则留在原地
func (m *Model) refocusAfterShip(rel string) {
	rows := m.Rows()
	start := -1
	for i, r := range rows {
		if r.Kind == rowNote && r.Tier == TierSea && r.Note == rel {
			start = i
			break
		}
	}
	if start < 0 {
		m.Normalize()
		return
	}
	p := m.RM.PointAt(m.curRef())
	for d := 1; d <= len(rows); d++ {
		i := (start + d) % len(rows)
		if r := rows[i]; r.Kind == rowNote && r.Tier == TierSea && !seaShipped(p, r.Note) {
			m.Cursor = i
			return
		}
	}
	m.Cursor = start
}

// doShipRiver 入河：详情页 ship 行按 r —— 领号改名迁入 river（与 /ship move 的
// riverLanding 同口径：剥源编号、按主干/点位领号、磁盘重名篇序顺延），
// 绑定 ship[]→river[] 落盘后改写全库出处链；出处链失败仅提示不回滚
func doShipRiver(m *Model, h Hooks) {
	if m.View != viewPoint {
		m.Err = "详情页才能入河"
		return
	}
	r := m.rowAt(m.Cursor)
	if r == nil || r.Kind != rowNote || r.Tier != TierShip {
		m.Err = "光标不在 ship 条目上"
		return
	}
	if h.MoveFileRel == nil || h.RewriteChainsRel == nil {
		m.Err = "该操作不可用"
		return
	}
	if !m.exists(r.Note) {
		m.Err = "文件已失效：" + r.Note
		return
	}
	srcRel := r.Note
	ref := m.curRef()
	dirRel := fmt.Sprintf("Study/river/%02d-%s", ref.Branch+1, m.RM.Branches[ref.Branch].Title)
	pointNum := ref.Point + 1
	title := roadmap.StripNumPrefix(filepath.Base(srcRel))
	dup := roadmap.NextDupSeq(m.RM.PointAt(ref).River, pointNum)
	for {
		dstRel := dirRel + "/" + roadmap.RiverFileName(pointNum, dup, title)
		err := h.MoveFileRel(srcRel, dstRel)
		if err == nil {
			m.RM.MoveTierBinding(srcRel, dstRel, roadmap.ShipList, roadmap.RiverList)
			m.save(h)
			if cerr := h.RewriteChainsRel(srcRel, dstRel); cerr != nil {
				m.Err = "出处链改写失败（迁移已完成）：" + cerr.Error()
			}
			return
		}
		if !strings.Contains(err.Error(), "目标已存在") {
			m.Err = "入河失败：" + err.Error()
			return
		}
		dup++ // 磁盘残留同名，篇序顺延重试
	}
}
