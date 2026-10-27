// Package roadmap Go 学习路线图的纯逻辑：TOML 正本解析/写回、关键词匹配、状态与绑定变更、静态树渲染。
// 本包不 import ui/lipgloss，颜色由调用方注入。
package roadmap

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/bugzi/gs/internal/workspace"
)

const (
	// Version 路线图文件格式版本（v2：branch→point，topic 层废止）
	Version = 2

	StatusTodo  = "todo"
	StatusDoing = "doing"
	StatusDone  = "done"
)

// Point 知识点（二级）；绑定路径一律工作区相对路径，Spring/Code 为 0/1 深度绑定
type Point struct {
	Title  string   `toml:"title"`
	Group  string   `toml:"group,omitempty"`
	Status string   `toml:"status"` // todo(缺省) | doing | done
	Tags   []string `toml:"tags,omitempty"`
	Spring string   `toml:"spring,omitempty"`
	Code   string   `toml:"code,omitempty"`
	River  []string `toml:"river,omitempty"`
	Ship   []string `toml:"ship,omitempty"` // 加工中（船坞）；可选字段，缺省空
	Sea    []string `toml:"sea,omitempty"`
	Note   string   `toml:"note,omitempty"`
}

// Branch 主干（一级）
type Branch struct {
	Title  string  `toml:"title"`
	Points []Point `toml:"point,omitempty"`
}

// Roadmap 树状路线图正本
type Roadmap struct {
	Version  int      `toml:"version"`
	Branches []Branch `toml:"branch"`
}

// Load 读入并校验 roadmap.toml
func Load(path string) (*Roadmap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse 解析；TOML 语法错误带行列号返回
func Parse(data []byte) (*Roadmap, error) {
	var rm Roadmap
	if err := toml.Unmarshal(data, &rm); err != nil {
		var perr toml.ParseError
		if errors.As(err, &perr) {
			return nil, fmt.Errorf("roadmap.toml 解析失败：\n%s", perr.ErrorWithPosition())
		}
		return nil, err
	}
	if err := rm.Validate(); err != nil {
		return nil, err
	}
	return &rm, nil
}

// Validate 结构校验：版本、主干非空、标题非空、知识点全局唯一、status 合法（缺省归一化）
func (rm *Roadmap) Validate() error {
	switch rm.Version {
	case Version:
	case 1:
		return errors.New("检测到 v1 格式（branch.topic.node）：roadmap.toml 已升级 v2（branch.point），请用 v2 初稿覆盖（初稿见 docs/project/plan/2026-10-26-路线图v2内容清单.md）")
	default:
		return fmt.Errorf("不支持的版本 %d（期望 %d）", rm.Version, Version)
	}
	if len(rm.Branches) == 0 {
		return errors.New("路线图没有任何主干")
	}
	seen := map[string]string{} // 知识点标题 → 主干，全树唯一（spring 文件名约定依赖此约束）
	for bi := range rm.Branches {
		b := &rm.Branches[bi]
		if b.Title == "" {
			return fmt.Errorf("第 %d 个主干缺少标题", bi+1)
		}
		for pi := range b.Points {
			p := &b.Points[pi]
			if p.Title == "" {
				return fmt.Errorf("主干 %q 存在缺少标题的知识点", b.Title)
			}
			if prev, dup := seen[p.Title]; dup {
				return fmt.Errorf("知识点 %q 重复（%s 与 %s）——v2 要求全树唯一", p.Title, prev, b.Title)
			}
			seen[p.Title] = b.Title
			switch p.Status {
			case "":
				p.Status = StatusTodo
			case StatusTodo, StatusDoing, StatusDone:
			default:
				return fmt.Errorf("知识点 %q 状态非法 %q（todo/doing/done）", p.Title, p.Status)
			}
		}
	}
	return nil
}

// Save 原子写回（重编码会丢弃手写注释，命令维护为主属预期）
func Save(path string, rm *Roadmap) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(rm); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(path, buf.Bytes())
}
