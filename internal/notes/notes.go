// Package notes 便签缓存：Study/notes.md 的解析、保存与 tidy 方案执行。纯逻辑，不 import ui。
package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

// Note 单条便签：录入时刻（东八区）+ 文本
type Note struct {
	Time time.Time
	Text string
}

const (
	fileRel        = "Study/notes.md"
	noteTimeLayout = "060102 15:04"
)

// noteLineRe 锚定完整便签行格式，避免正文里 - [ 开头的行误判
var noteLineRe = regexp.MustCompile(`^- \[\d+\] (\d{6} \d{2}:\d{2}) (.*)$`)

// Path notes.md 绝对路径（路径知识唯一来源）
func Path(root string) string { return filepath.Join(root, filepath.FromSlash(fileRel)) }

// Load 解析 notes.md；文件不存在返回空切片；不识别的行跳过
func Load(root string) ([]Note, error) {
	data, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Note
	for _, line := range strings.Split(string(data), "\n") {
		m := noteLineRe.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		tm, perr := time.ParseInLocation(noteTimeLayout, m[1], workspace.CST)
		if perr != nil {
			continue
		}
		out = append(out, Note{Time: tm, Text: m[2]})
	}
	return out, nil
}

// Format 渲染整份文件：骨架 + 重编号便签
func Format(ns []Note) string {
	lines := []string{"# 便签", "", "> 临时记录，待整理进 river/spring", ""}
	for i, n := range ns {
		lines = append(lines, fmt.Sprintf("- [%d] %s %s", i+1, n.Time.In(workspace.CST).Format(noteTimeLayout), n.Text))
	}
	return strings.Join(lines, "\n") + "\n"
}

// Save 重编号并原子写；目录不存在自动创建
func Save(root string, ns []Note) error {
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		return err
	}
	return workspace.WriteFileAtomic(Path(root), []byte(Format(ns)))
}

// Add 追加一条便签
func Add(root, text string, now time.Time) error {
	ns, err := Load(root)
	if err != nil {
		return err
	}
	ns = append(ns, Note{Time: now.In(workspace.CST), Text: text})
	return Save(root, ns)
}

// checkRelPath 校验 target 为工作区相对路径形态（拒绝绝对路径、rooted 路径与 .. 穿越）
func checkRelPath(target string) error {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(target)))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") || strings.HasPrefix(clean, "..") || clean == "." {
		return fmt.Errorf("必须是工作区相对路径: %s", target)
	}
	return nil
}

// AppendNote 把便签文本原文追加到目标文件末尾（补尾换行 + 空行分隔），原子写
func AppendNote(root, targetRel, text string) error {
	if err := checkRelPath(targetRel); err != nil {
		return err
	}
	abs := filepath.Join(root, filepath.FromSlash(strings.TrimSpace(targetRel)))
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += "\n" + strings.TrimRight(text, "\n") + "\n"
	return workspace.WriteFileAtomic(abs, []byte(s))
}
