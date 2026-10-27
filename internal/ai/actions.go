package ai

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

type Action struct {
	File    string `json:"file"`
	Action  string `json:"action"`
	Content string `json:"content"`
}

var fenceRe = regexp.MustCompile("(?s)```[a-zA-Z-]*\\s*\n(.*?)```")

var allowedExt = map[string]bool{".md": true, ".go": true, ".toml": true}

// SplitReply 从回复中提取动作块，返回去除动作块后的正文与动作列表。
// 代码块内容须为 {"actions":[...]} 且每项 action 为 edit/create、file 非空。
func SplitReply(reply string) (string, []Action) {
	prose := reply
	var actions []Action
	for _, m := range fenceRe.FindAllStringSubmatch(reply, -1) {
		var holder struct {
			Actions []Action `json:"actions"`
		}
		if json.Unmarshal([]byte(m[1]), &holder) != nil || len(holder.Actions) == 0 {
			continue
		}
		valid := true
		for _, a := range holder.Actions {
			if (a.Action != "edit" && a.Action != "create") || strings.TrimSpace(a.File) == "" {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		actions = append(actions, holder.Actions...)
		prose = strings.ReplaceAll(prose, m[0], "")
	}
	return strings.TrimSpace(prose), actions
}

// SafePath 校验动作目标必须位于 root 内且扩展名在白名单中
func SafePath(root, file string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(file)))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", errors.New("路径越界: " + file)
	}
	if !allowedExt[strings.ToLower(filepath.Ext(clean))] {
		return "", errors.New("扩展名不允许: " + file)
	}
	return filepath.Join(root, clean), nil
}
