package notes

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

// Placement 一条归位方案：便签下标（0 起，对应 Load 顺序）→ 目标文件完整新内容
type Placement struct {
	NoteIdx    int
	Target     string // 工作区相对路径
	NewContent string
}

// Plan tidy 整批方案；同目标多个 Placement 按序应用，content 须累计包含前序改动
type Plan struct {
	Placements []Placement
}

// planWire AI 输出线格式：note 为 1 起序号
type planWire struct {
	Placements []struct {
		Note    int    `json:"note"`
		Target  string `json:"target"`
		Content string `json:"content"`
	} `json:"placements"`
}

// ParsePlanJSON 解析并校验方案：note 序号在界、target 相对路径形态、content 非空；
// note 转 0 起 NoteIdx，content 统一 \n
func ParsePlanJSON(s string, ns []Note) (Plan, error) {
	var w planWire
	if err := json.Unmarshal([]byte(s), &w); err != nil {
		return Plan{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	var plan Plan
	for i, p := range w.Placements {
		switch {
		case p.Note < 1 || p.Note > len(ns):
			return Plan{}, fmt.Errorf("placement[%d]: 便签序号越界: %d", i, p.Note)
		case strings.TrimSpace(p.Target) == "":
			return Plan{}, fmt.Errorf("placement[%d]: target 为空", i)
		case strings.TrimSpace(p.Content) == "":
			return Plan{}, fmt.Errorf("placement[%d]: content 为空", i)
		}
		if err := checkRelPath(p.Target); err != nil {
			return Plan{}, fmt.Errorf("placement[%d]: %w", i, err)
		}
		plan.Placements = append(plan.Placements, Placement{
			NoteIdx:    p.Note - 1,
			Target:     strings.TrimSpace(p.Target),
			NewContent: strings.ReplaceAll(p.Content, "\r\n", "\n"),
		})
	}
	return plan, nil
}

// Apply 执行方案：按序逐目标原子写；任一失败即中止且不动 notes.md（便签仍在，幂等可重跑）；
// 全部成功后重写 notes.md 移除已归位便签。目标目录不存在时写失败即报错——
// 这正是「目标必须是已有条目」的天然防线，不做 MkdirAll 兜底
func Apply(root string, plan Plan, ns []Note) error {
	for i, p := range plan.Placements {
		abs := filepath.Join(root, filepath.FromSlash(p.Target))
		if err := workspace.WriteFileAtomic(abs, []byte(p.NewContent)); err != nil {
			return fmt.Errorf("placement[%d] 写入 %s 失败: %w", i, p.Target, err)
		}
	}
	placed := map[int]bool{}
	for _, p := range plan.Placements {
		placed[p.NoteIdx] = true
	}
	rest := make([]Note, 0, len(ns))
	for i, n := range ns {
		if !placed[i] {
			rest = append(rest, n)
		}
	}
	return Save(root, rest)
}
