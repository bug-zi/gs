package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

const reviewPromptTpl = `请基于以下学习材料，给出建议，用 Markdown 分节输出：

## 知识整理建议
（结构优化、过期内容、缺失主题）

## 合并建议
（主题重叠、应互相引用的条目）

## 三层流动建议
（结合三层统计：大海未摘录条目优先摘录哪些、河流待改写哪些、泉水产出节奏）

## 学习规划建议
（结合当前断点与实习面试目标，给出接下来 1-2 周的重点）

只对确有必要修改的文件输出 gs-actions 动作块（如合并条目）；建议本身用文字表达即可。

=== 学习状态 ===
%s

=== 阶段清单 ===
%s

=== 三层统计 ===
%s

=== 知识库（sea/river/spring） ===
%s

=== 近 30 天日志 ===
%s`

// gatherReviewMaterials 收集体检材料，各部分限量防止超长
func gatherReviewMaterials(ws *workspace.Workspace) (state, phases, stats, notes, logs string) {
	data, err := os.ReadFile(ws.StateFile())
	if err == nil {
		st := ParseState(string(data))
		var b1, b2 strings.Builder
		for _, kv := range st.Breakpoints {
			fmt.Fprintf(&b1, "- %s：%s\n", kv.Key, kv.Value)
		}
		for i, p := range st.Phases {
			fmt.Fprintf(&b2, "%d. %s\n", i+1, p)
		}
		state, phases = b1.String(), b2.String()
	}
	var nb strings.Builder
	if files, err := ListKnowledge(ws); err == nil {
		for _, f := range files {
			raw, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(f)))
			if err != nil {
				continue
			}
			fmt.Fprintf(&nb, "【%s】\n%s\n", f, truncateLines(string(raw), 60))
		}
	}
	notes = nb.String()
	var lb strings.Builder
	cutoff := time.Now().In(workspace.CST).AddDate(0, 0, -30)
	if dates, err := ws.ListLogs(); err == nil {
		for _, d := range dates {
			if d.Before(cutoff) {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(ws.LogsDir(), workspace.LogFileName(d)))
			if err != nil {
				continue
			}
			fmt.Fprintf(&lb, "【%s】%s\n", d.Format("01-02"), truncateLines(strings.TrimSpace(string(raw)), 40))
		}
	}
	logs = lb.String()
	stats = renderTierStats(GatherTierStats(ws))
	return state, phases, stats, notes, logs
}

func newReview() Command {
	return Command{
		Name:  "review",
		Short: "AI 体检：知识整理/合并/学习规划建议",
		Run: func(ctx *Ctx) error {
			state, phases, stats, notes, logs := gatherReviewMaterials(ctx.WS)
			fmt.Fprintln(ctx.Out, renderTierStats(GatherTierStats(ctx.WS)))
			st, err := ensureAIState(ctx)
			if err != nil {
				return err
			}
			prompt := fmt.Sprintf(reviewPromptTpl, state, phases, stats, notes, logs)
			fmt.Fprintln(ctx.Out, ui.Dim("（AI 正在体检学习资料…）"))
			reply, err := st.Client.Chat(context.Background(), []ai.Message{
				{Role: "system", Content: "你是严格的 Go 学习教练，只给可执行的建议。"},
				{Role: "user", Content: prompt},
			})
			if err != nil {
				return err
			}
			prose, actions := ai.SplitReply(reply)
			if prose != "" {
				fmt.Fprintln(ctx.Out, ui.RenderMD(prose))
			}
			if len(actions) > 0 {
				return applyActions(ctx, actions)
			}
			return nil
		},
	}
}
