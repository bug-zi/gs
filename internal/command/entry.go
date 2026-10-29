package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

// validateTopic 主题即文件名：禁止路径分隔符与特殊字符
func validateTopic(topic string) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("主题不能为空")
	}
	if strings.ContainsAny(topic, `/\:*?"<>|`) || strings.Contains(topic, "..") {
		return fmt.Errorf("主题不能包含路径分隔符或特殊字符")
	}
	return nil
}

// CreateEntry 在指定层创建条目，返回工作区相对路径
func CreateEntry(ws *workspace.Workspace, tier, topic, source, body string) (string, error) {
	dirFn, ok := tierDirs[tier]
	if !ok {
		return "", fmt.Errorf("未知层: %s", tier)
	}
	return CreateEntryIn(ws, dirFn(ws), topic, source, body)
}

// CreateEntryIn 在指定目录创建条目：# 主题 + 出处引用行 + 正文，原子写。
// source 为空则不写出处行。
func CreateEntryIn(ws *workspace.Workspace, dir, topic, source, body string) (string, error) {
	if err := validateTopic(topic); err != nil {
		return "", err
	}
	path := filepath.Join(dir, topic+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("条目已存在: %s", ws.Rel(path))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", topic)
	if source != "" {
		fmt.Fprintf(&b, "> 出处：%s\n\n", source)
	}
	b.WriteString(body)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := workspace.WriteFileAtomic(path, []byte(b.String())); err != nil {
		return "", err
	}
	return ws.Rel(path), nil
}

// CodeTemplate 知识点演示代码初始模板：仅可编译骨架，无占位演示代码
func CodeTemplate(topic string) string {
	return fmt.Sprintf("// 知识点「%s」演示代码\n\npackage main\n\nfunc main() {}\n", topic)
}

// CreateCodeEntry 创建知识点演示代码 Study/code/<标题>.go，返回工作区相对路径，原子写
func CreateCodeEntry(ws *workspace.Workspace, topic string) (string, error) {
	if err := validateTopic(topic); err != nil {
		return "", err
	}
	path := filepath.Join(ws.CodeDir(), topic+".go")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("条目已存在: %s", ws.Rel(path))
	}
	if err := os.MkdirAll(ws.CodeDir(), 0o755); err != nil {
		return "", err
	}
	if err := workspace.WriteFileAtomic(path, []byte(CodeTemplate(topic))); err != nil {
		return "", err
	}
	return ws.Rel(path), nil
}
