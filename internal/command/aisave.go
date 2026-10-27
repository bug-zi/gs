package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// SaveAIChat 把当前 /AI 会话整理成 md 存入 sea/ai/，返回新文件的工作区相对路径。
// 标题取首轮提问前 30 字（放引用行）；文件名用存档时刻，规避主题非法字符问题。
func SaveAIChat(ws *workspace.Workspace, st *AIState) (string, error) {
	if st == nil || len(st.Hist) == 0 {
		return "", fmt.Errorf("当前会话没有可存档的内容")
	}
	title := "AI 问答"
	for _, m := range st.Hist {
		if m.Role == "user" {
			r := []rune(strings.TrimSpace(m.Content))
			if len(r) > 30 {
				r = r[:30]
			}
			if n := strings.TrimSpace(strings.ReplaceAll(string(r), "\n", " ")); n != "" {
				title = n
			}
			break
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "> 存档自 /AI 会话 · %s · %s\n\n", time.Now().In(workspace.CST).Format("2006-01-02 15:04"), title)
	for _, m := range st.Hist {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "## 问\n\n%s\n\n", strings.TrimSpace(m.Content))
		case "assistant":
			fmt.Fprintf(&b, "## 答\n\n%s\n\n", strings.TrimSpace(m.Content))
		}
	}
	name := time.Now().In(workspace.CST).Format("2006-01-02-1504")
	path, err := CreateEntryIn(ws, filepath.Join(ws.SeaDir(), "ai"), name, "", b.String())
	if err == nil {
		st.Saved = true
	}
	return path, err
}

// MaybeSaveAIChat 离开 AI 模式时询问是否存档；空会话或已存档静默跳过
func MaybeSaveAIChat(ctx *Ctx) error {
	st := ctx.AI
	if st == nil || len(st.Hist) == 0 || st.Saved {
		return nil
	}
	ok, err := confirm(ctx, "本次 AI 会话存入大海（sea/ai）？")
	if err != nil || !ok {
		return err
	}
	path, err := SaveAIChat(ctx.WS, st)
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "已存档 %s\n", ui.Path(path))
	return nil
}

// InboxCount 统计 sea/inbox 下待导入的 md 数量
func InboxCount(ws *workspace.Workspace) int {
	entries, err := os.ReadDir(filepath.Join(ws.SeaDir(), "inbox"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			n++
		}
	}
	return n
}
