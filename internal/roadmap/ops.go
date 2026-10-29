package roadmap

import (
	"errors"
	"fmt"
	"strings"
)

// PointRef 知识点定位：主干/知识点两段下标
type PointRef struct{ Branch, Point int }

// PointAt 取知识点指针
func (rm *Roadmap) PointAt(r PointRef) *Point {
	return &rm.Branches[r.Branch].Points[r.Point]
}

// PathOf 知识点全路径「主干 › 知识点」
func (rm *Roadmap) PathOf(r PointRef) string {
	return rm.Branches[r.Branch].Title + " › " + rm.PointAt(r).Title
}

// MatchPoints 关键词匹配全树知识点标题（大小写不敏感的包含匹配），按树序返回
func (rm *Roadmap) MatchPoints(kw string) []PointRef {
	kw = strings.ToLower(kw)
	var out []PointRef
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			if strings.Contains(strings.ToLower(rm.Branches[bi].Points[pi].Title), kw) {
				out = append(out, PointRef{bi, pi})
			}
		}
	}
	return out
}

// MatchBranches 主干标题含关键词的下标
func (rm *Roadmap) MatchBranches(kw string) []int {
	kw = strings.ToLower(kw)
	var out []int
	for i := range rm.Branches {
		if strings.Contains(strings.ToLower(rm.Branches[i].Title), kw) {
			out = append(out, i)
		}
	}
	return out
}

// SetStatus 标记知识点状态
func (rm *Roadmap) SetStatus(r PointRef, status string) error {
	switch status {
	case StatusTodo, StatusDoing, StatusDone:
	default:
		return fmt.Errorf("非法状态 %q", status)
	}
	rm.PointAt(r).Status = status
	return nil
}

// TotalPoints 全树知识点数
func (rm *Roadmap) TotalPoints() int {
	n := 0
	for bi := range rm.Branches {
		n += len(rm.Branches[bi].Points)
	}
	return n
}

// DonePoints 全树 done 知识点数
func (rm *Roadmap) DonePoints() int {
	n := 0
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			if rm.Branches[bi].Points[pi].Status == StatusDone {
				n++
			}
		}
	}
	return n
}

// BranchProgress 主干内 done/total
func (rm *Roadmap) BranchProgress(bi int) (done, total int) {
	for pi := range rm.Branches[bi].Points {
		total++
		if rm.Branches[bi].Points[pi].Status == StatusDone {
			done++
		}
	}
	return done, total
}

// AddBranch 新主干，重名报错
func (rm *Roadmap) AddBranch(title string) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("标题不能为空")
	}
	for i := range rm.Branches {
		if rm.Branches[i].Title == title {
			return fmt.Errorf("主干已存在: %s", title)
		}
	}
	rm.Branches = append(rm.Branches, Branch{Title: title})
	return nil
}

// AddPoint 主干下新知识点（status=todo）；标题全树唯一（spring 文件名约定依赖此约束）
func (rm *Roadmap) AddPoint(bi int, title, group string, tags []string) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("标题不能为空")
	}
	for bi2 := range rm.Branches {
		for pi := range rm.Branches[bi2].Points {
			if rm.Branches[bi2].Points[pi].Title == title {
				return fmt.Errorf("知识点 %q 已存在于 %s——v2 要求全树唯一", title, rm.Branches[bi2].Title)
			}
		}
	}
	rm.Branches[bi].Points = append(rm.Branches[bi].Points, Point{Title: title, Group: group, Status: StatusTodo, Tags: tags})
	return nil
}

// BindSpring 绑定唯一 spring（覆盖语义，0/1）
func (rm *Roadmap) BindSpring(r PointRef, rel string) {
	rm.PointAt(r).Spring = rel
}

// BindCode 绑定唯一演示代码（覆盖语义，0/1）
func (rm *Roadmap) BindCode(r PointRef, rel string) {
	rm.PointAt(r).Code = rel
}

// BindRiver / BindSea 追加绑定（幂等）
func (rm *Roadmap) BindRiver(r PointRef, rel string) { bindList(&rm.PointAt(r).River, rel) }
func (rm *Roadmap) BindSea(r PointRef, rel string)   { bindList(&rm.PointAt(r).Sea, rel) }

func bindList(list *[]string, rel string) {
	for _, s := range *list {
		if s == rel {
			return
		}
	}
	*list = append(*list, rel)
}

// SeaList / ShipList / RiverList 绑定数组访问器（MoveTierBinding 的 from/to 参数）
func SeaList(p *Point) *[]string   { return &p.Sea }
func ShipList(p *Point) *[]string  { return &p.Ship }
func RiverList(p *Point) *[]string { return &p.River }

// CopyTierBinding 全树把 from 列表中含有 oldRel 的知识点，向其 to 列表追加挂载 newRel；
// from 中 oldRel 原样保留（装船语义：sea 绑定不动）。幂等：to 侧 bindList 去重，无命中返回空。
func (rm *Roadmap) CopyTierBinding(oldRel, newRel string, from, to func(*Point) *[]string) []PointRef {
	var hits []PointRef
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			p := &rm.Branches[bi].Points[pi]
			found := false
			for _, s := range *from(p) {
				if s == oldRel {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			bindList(to(p), newRel)
			hits = append(hits, PointRef{bi, pi})
		}
	}
	return hits
}

// MoveTierBinding 全树把 from 列表中的 oldRel 迁入 to 列表并更新为 newRel；返回受影响知识点（树序）。
// from 与 to 为同一访问器时即原地改名。幂等：无命中返回空。
func (rm *Roadmap) MoveTierBinding(oldRel, newRel string, from, to func(*Point) *[]string) []PointRef {
	var hits []PointRef
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			p := &rm.Branches[bi].Points[pi]
			fl, tl := from(p), to(p)
			idx := -1
			for i, s := range *fl {
				if s == oldRel {
					idx = i
					break
				}
			}
			if idx < 0 {
				continue
			}
			*fl = append((*fl)[:idx], (*fl)[idx+1:]...)
			bindList(tl, newRel)
			hits = append(hits, PointRef{bi, pi})
		}
	}
	return hits
}

// UnbindSpring 清空 spring 绑定
func (rm *Roadmap) UnbindSpring(r PointRef) { rm.PointAt(r).Spring = "" }

// UnbindCode 清空 code 绑定
func (rm *Roadmap) UnbindCode(r PointRef) { rm.PointAt(r).Code = "" }

// BindShip 追加绑定（幂等）
func (rm *Roadmap) BindShip(r PointRef, rel string) { bindList(&rm.PointAt(r).Ship, rel) }

// UnbindList 从指定绑定数组移除一条；返回是否命中
func (rm *Roadmap) UnbindList(r PointRef, list func(*Point) *[]string, rel string) bool {
	l := list(rm.PointAt(r))
	for i, s := range *l {
		if s == rel {
			*l = append((*l)[:i], (*l)[i+1:]...)
			return true
		}
	}
	return false
}

// MoveBinding 把 src 清单中的一条移挂到 dst 同类清单（卸载+挂载）；src 未命中返回 false
func (rm *Roadmap) MoveBinding(src PointRef, list func(*Point) *[]string, rel string, dst PointRef) bool {
	if !rm.UnbindList(src, list, rel) {
		return false
	}
	bindList(list(rm.PointAt(dst)), rel)
	return true
}

// IsBound 路径是否被任一知识点绑定（spring/code/river/ship/sea 任一）
func (rm *Roadmap) IsBound(rel string) bool {
	_, ok := rm.OwnerOf(rel)
	return ok
}

// OwnerOf 反查绑定路径所属知识点（树序首个命中）
func (rm *Roadmap) OwnerOf(rel string) (PointRef, bool) {
	owners := rm.OwnersOf(rel)
	if len(owners) == 0 {
		return PointRef{}, false
	}
	return owners[0], true
}

// SpringPath spring 文件名约定路径（知识点标题全局唯一保证不冲突）
func SpringPath(title string) string { return "Study/spring/" + title + ".md" }

// CheckTitleUnique 新标题全树查重；空标题报错（供改名/拆分流程先验证再逐步应用）
func (rm *Roadmap) CheckTitleUnique(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("标题不能为空")
	}
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			if rm.Branches[bi].Points[pi].Title == title {
				return fmt.Errorf("知识点 %q 已存在于 %s——v2 要求全树唯一", title, rm.Branches[bi].Title)
			}
		}
	}
	return nil
}

// RenamePoint 知识点改名：查重后改标题；spring 字段符合约定路径时跟随改为新约定路径
// （文件改名由壳层负责）。返回改前 spring 路径：非空表示发生了约定跟随。
func (rm *Roadmap) RenamePoint(r PointRef, newTitle string) (string, error) {
	newTitle = strings.TrimSpace(newTitle)
	p := rm.PointAt(r)
	if newTitle == "" {
		return "", errors.New("标题不能为空")
	}
	if newTitle == p.Title {
		return "", nil
	}
	if err := rm.CheckTitleUnique(newTitle); err != nil {
		return "", err
	}
	old := p.Title
	p.Title = newTitle
	if p.Spring == SpringPath(old) {
		oldSpring := p.Spring
		p.Spring = SpringPath(newTitle)
		return oldSpring, nil
	}
	return "", nil
}

// maxStatus 状态进度取高：todo < doing < done
func maxStatus(a, b string) string {
	rank := map[string]int{StatusTodo: 0, StatusDoing: 1, StatusDone: 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// appendUnion 追加去重
func appendUnion(dst, src []string) []string {
	for _, s := range src {
		found := false
		for _, d := range dst {
			if d == s {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, s)
		}
	}
	return dst
}

// MergePoints 把 absorb 并入 keep 后删除 absorb：数组绑定合并去重；spring/code 空则补
//（双方都有时由壳层先裁决 keep 侧取值）；状态取进度高者；tags 并集；note 拼接。
// 调用后传入的 PointRef 下标失效，需按标题重新定位。
func (rm *Roadmap) MergePoints(keep, absorb PointRef) {
	kp, ap := rm.PointAt(keep), rm.PointAt(absorb)
	for _, rel := range ap.Sea {
		bindList(&kp.Sea, rel)
	}
	for _, rel := range ap.Ship {
		bindList(&kp.Ship, rel)
	}
	for _, rel := range ap.River {
		bindList(&kp.River, rel)
	}
	if kp.Spring == "" {
		kp.Spring = ap.Spring
	}
	if kp.Code == "" {
		kp.Code = ap.Code
	}
	kp.Status = maxStatus(kp.Status, ap.Status)
	kp.Tags = appendUnion(kp.Tags, ap.Tags)
	if ap.Note != "" {
		if kp.Note != "" {
			kp.Note += "\n---\n"
		}
		kp.Note += ap.Note
	}
	b := &rm.Branches[absorb.Branch]
	b.Points = append(b.Points[:absorb.Point], b.Points[absorb.Point+1:]...)
}

// SplitPoint 拆分：在 r 正后方插入同分组新知识点（todo、无绑定）；标题全树查重
func (rm *Roadmap) SplitPoint(r PointRef, newTitle string) (PointRef, error) {
	if err := rm.CheckTitleUnique(newTitle); err != nil {
		return PointRef{}, err
	}
	src := rm.PointAt(r)
	np := Point{Title: strings.TrimSpace(newTitle), Group: src.Group, Status: StatusTodo}
	b := &rm.Branches[r.Branch]
	b.Points = append(b.Points[:r.Point+1], append([]Point{np}, b.Points[r.Point+1:]...)...)
	return PointRef{r.Branch, r.Point + 1}, nil
}

// MovePointToBranch 换主干：挪到目标主干末尾；同主干为空操作
func (rm *Roadmap) MovePointToBranch(r PointRef, bi int) {
	if r.Branch == bi {
		return
	}
	pt := *rm.PointAt(r) // 先值拷贝：删除会使源切片内存移位
	rm.Branches[r.Branch].Points = append(rm.Branches[r.Branch].Points[:r.Point], rm.Branches[r.Branch].Points[r.Point+1:]...)
	rm.Branches[bi].Points = append(rm.Branches[bi].Points, pt)
}

// SetGroup 换分组标签（空串清除）
func (rm *Roadmap) SetGroup(r PointRef, group string) { rm.PointAt(r).Group = group }

// PointEmpty 知识点五类绑定全空
func (rm *Roadmap) PointEmpty(r PointRef) bool {
	p := rm.PointAt(r)
	return p.Spring == "" && p.Code == "" && len(p.Sea) == 0 && len(p.Ship) == 0 && len(p.River) == 0
}

// DeletePoint 删除知识点；仍有任一绑定时拒绝（壳层先提示清理）
func (rm *Roadmap) DeletePoint(r PointRef) error {
	if !rm.PointEmpty(r) {
		return errors.New("知识点仍有绑定，先卸载后再删除")
	}
	b := &rm.Branches[r.Branch]
	b.Points = append(b.Points[:r.Point], b.Points[r.Point+1:]...)
	return nil
}

// OwnersOf 反查绑定路径所属的全部知识点（树序；spring/code 单值与 sea/ship/river 清单均查）
func (rm *Roadmap) OwnersOf(rel string) []PointRef {
	var out []PointRef
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			p := &rm.Branches[bi].Points[pi]
			if p.Spring == rel || p.Code == rel {
				out = append(out, PointRef{bi, pi})
				continue
			}
			hit := false
			for _, list := range [][]string{p.Sea, p.Ship, p.River} {
				for _, s := range list {
					if s == rel {
						hit = true
						break
					}
				}
				if hit {
					break
				}
			}
			if hit {
				out = append(out, PointRef{bi, pi})
			}
		}
	}
	return out
}

// DetachPoint 清空知识点五类绑定，返回清出的 river 绑定清单（供壳层收容迁移）。
// 只动内存不落盘；清空后 PointEmpty 为真，DeletePoint 可直接调用。
func (rm *Roadmap) DetachPoint(r PointRef) []string {
	p := rm.PointAt(r)
	river := append([]string{}, p.River...)
	p.Spring, p.Code = "", ""
	p.Sea, p.Ship, p.River = nil, nil, nil
	return river
}

// DeleteBranch 删除空主干；仍有知识点或仅剩最后一个主干时拒绝（壳层先提示处理）
func (rm *Roadmap) DeleteBranch(bi int) error {
	b := &rm.Branches[bi]
	if len(b.Points) > 0 {
		return fmt.Errorf("主干「%s」仍有 %d 个知识点，先删除或移走后再删除主干", b.Title, len(b.Points))
	}
	if len(rm.Branches) <= 1 {
		return errors.New("至少保留一个主干")
	}
	rm.Branches = append(rm.Branches[:bi], rm.Branches[bi+1:]...)
	return nil
}
