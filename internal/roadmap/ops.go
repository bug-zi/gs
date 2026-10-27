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
