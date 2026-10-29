package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// chatTitle 取提问前 30 字作标题（换行转空格）；空白输入回退「AI 问答」
func chatTitle(text string) string {
	r := []rune(strings.TrimSpace(text))
	if len(r) > 30 {
		r = r[:30]
	}
	if n := strings.TrimSpace(strings.ReplaceAll(string(r), "\n", " ")); n != "" {
		return n
	}
	return "AI 问答"
}

// turnArchiveBody 单轮问答的存档正文（格式与 SaveAIChat 一致；时间精确到秒）
func turnArchiveBody(title, ask, ans string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "> 存档自 /AI 会话 · %s · %s\n\n", time.Now().In(workspace.CST).Format("2006-01-02 15:04:05"), title)
	fmt.Fprintf(&b, "## 问\n\n%s\n\n## 答\n\n%s\n\n", strings.TrimSpace(ask), strings.TrimSpace(ans))
	return b.String()
}

// SaveAIChat 把当前 /AI 会话整理成 md 存入 sea/ai/，返回新文件的工作区相对路径。
// 标题取首轮提问前 30 字（放引用行）；文件名用存档时刻，规避主题非法字符问题。
func SaveAIChat(ws *workspace.Workspace, st *AIState) (string, error) {
	if st == nil || len(st.Hist) == 0 {
		return "", fmt.Errorf("当前会话没有可存档的内容")
	}
	title := "AI 问答"
	for _, m := range st.Hist {
		if m.Role == "user" {
			title = chatTitle(m.Content)
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
	return CreateEntryIn(ws, filepath.Join(ws.SeaDir(), "ai"), name, "", b.String())
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

// chatClient 分类所需的最小 AI 能力（*ai.Client 满足；测试注入 fake）
type chatClient interface {
	Chat(ctx context.Context, msgs []ai.Message) (string, error)
}

// pointCand 分类候选：知识点标题与归属上下文
type pointCand struct {
	Title  string
	Branch string
	Group  string
}

// pointCandidates roadmap 全部知识点作为分类候选
func pointCandidates(rm *roadmap.Roadmap) []pointCand {
	var out []pointCand
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			p := &rm.Branches[bi].Points[pi]
			out = append(out, pointCand{Title: p.Title, Branch: rm.Branches[bi].Title, Group: p.Group})
		}
	}
	return out
}

const pointClassifyPrompt = `你是学习笔记归档分类器。根据一轮 AI 问答的内容，从候选知识点中选出最合适的一个。
只能输出候选清单中的知识点标题原话（不含主干与分组前缀）；没有合适的输出空字符串。
只输出一个 JSON 对象，不要输出任何其他文字，格式：{"知识点":"<候选标题或空字符串>"}`

// clipRunes 按 rune 截断到 n，超长补截断提示
func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "\n…（截断）"
	}
	return s
}

// classifyPoint 一次 AI 调用选知识点标题；AI 失败、响应无法解析、返回不在候选内一律返回 ""
func classifyPoint(ctx context.Context, cc chatClient, cands []pointCand, ask, ans string) string {
	if cc == nil || len(cands) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("候选知识点：\n")
	for _, c := range cands {
		if c.Group != "" {
			fmt.Fprintf(&b, "- %s（%s / %s）\n", c.Title, c.Branch, c.Group)
		} else {
			fmt.Fprintf(&b, "- %s（%s）\n", c.Title, c.Branch)
		}
	}
	fmt.Fprintf(&b, "\n【问】\n%s\n\n【答】\n%s\n", clipRunes(ask, 2000), clipRunes(ans, 2000))
	reply, err := cc.Chat(ctx, []ai.Message{
		{Role: "system", Content: pointClassifyPrompt},
		{Role: "user", Content: b.String()},
	})
	if err != nil {
		return ""
	}
	choice, err := parsePointChoice(reply)
	if err != nil {
		return ""
	}
	choice = strings.TrimSpace(choice)
	for _, c := range cands {
		if strings.EqualFold(c.Title, choice) {
			return c.Title
		}
	}
	return ""
}

// parsePointChoice 从回复中提取 {"知识点":"..."}；容忍围栏与前后杂文本（同 seaport.parseClassify 风格）
func parsePointChoice(reply string) (string, error) {
	i := strings.Index(reply, "{")
	j := strings.LastIndex(reply, "}")
	if i < 0 || j <= i {
		return "", errors.New("回复中没有 JSON")
	}
	var wrapper struct {
		Point string `json:"知识点"`
	}
	if err := json.Unmarshal([]byte(reply[i:j+1]), &wrapper); err != nil {
		return "", fmt.Errorf("JSON 解析失败: %w", err)
	}
	return strings.TrimSpace(wrapper.Point), nil
}

// resolvePoint 按标题回查知识点（忽略大小写精确匹配）
func resolvePoint(rm *roadmap.Roadmap, title string) (roadmap.PointRef, bool) {
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			if strings.EqualFold(rm.Branches[bi].Points[pi].Title, title) {
				return roadmap.PointRef{Branch: bi, Point: pi}, true
			}
		}
	}
	return roadmap.PointRef{}, false
}

// archiveTurnToSea 每轮存档一条龙：AI 分类知识点 → 落盘 sea/ai/（未命中落 sea/未分类/）→ 绑定 roadmap。
// cc 传入温度置 0 的客户端副本；结果行打印到 ctx.Out，绑定失败不吞错（文件已存，可后续 /plan 浏览器 a 键补挂）。
func archiveTurnToSea(ctx *Ctx, cc chatClient, ask, ans string) error {
	title := chatTitle(ask)
	name := time.Now().In(workspace.CST).Format("2006-01-02-150405")
	body := turnArchiveBody(title, ask, ans)

	rm, rmErr := roadmap.Load(ctx.WS.RoadmapFile())
	var cands []pointCand
	if rmErr == nil {
		cands = pointCandidates(rm)
	}
	var ref roadmap.PointRef
	matched, pointTitle := false, ""
	if len(cands) > 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("（正在归类…）"))
		if choice := classifyPoint(context.Background(), cc, cands, ask, ans); choice != "" {
			if r, ok := resolvePoint(rm, choice); ok {
				matched, ref, pointTitle = true, r, choice
			}
		}
	}
	dest := filepath.Join(ctx.WS.SeaDir(), "未分类")
	if matched {
		dest = filepath.Join(ctx.WS.SeaDir(), "ai")
	}
	rel, err := CreateEntryIn(ctx.WS, dest, name, "", body)
	if err != nil {
		return err
	}
	if !matched {
		fmt.Fprintf(ctx.Out, "%s %s\n", ui.OK("已存入"), ui.Path(rel))
		return nil
	}
	rm.BindSea(ref, rel)
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		fmt.Fprintf(ctx.Out, "%s %s %s\n", ui.OK("已存入"), ui.Path(rel), ui.Err("知识点绑定失败："+err.Error()))
		return nil
	}
	fmt.Fprintf(ctx.Out, "%s %s（绑定：%s）\n", ui.OK("已存入"), ui.Path(rel), pointTitle)
	return nil
}
