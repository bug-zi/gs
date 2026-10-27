package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/tier"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// runPlanRenumber 全量重排 river 编号：dry-run 预览 → y/N 确认 → 执行（文件迁移 →
// roadmap river 绑定改写 → topics.toml 改写 → 全库出处链改写）。幂等可重跑。
func runPlanRenumber(ctx *Ctx) error {
	plan, err := buildRenumberPreview(ctx)
	if err != nil {
		return err
	}
	if plan == nil {
		return nil
	}
	ans, err := inputLine(ctx, "执行重排？（y/N）")
	if err != nil {
		return err
	}
	if ans = strings.TrimSpace(ans); ans != "y" && ans != "Y" {
		fmt.Fprintln(ctx.Out, "已取消")
		return nil
	}
	return applyRenumberPlan(ctx, plan)
}

// buildRenumberPreview 生成重排计划并输出 dry-run 预览；无可做项时返回 nil（含幂等重跑）。
func buildRenumberPreview(ctx *Ctx) (*roadmap.RenumberPlan, error) {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return nil, err
	}
	existing, err := ListTierFiles(ctx.WS, "river")
	if err != nil {
		return nil, err
	}
	plan := roadmap.BuildRenumberPlan(rm, existing)
	if len(plan.Moves) == 0 && len(plan.Orphans) == 0 {
		fmt.Fprintln(ctx.Out, "编号已与树序一致，无需重排")
		return nil, nil
	}
	fmt.Fprintf(ctx.Out, "重排预览（共 %d 项迁移 + %d 项收容）：\n", len(plan.Moves), len(plan.Orphans))
	for _, m := range plan.Moves {
		fmt.Fprintf(ctx.Out, "  %s\n    → %s\n", ui.Dim(m.From), ui.Path(m.To))
	}
	for _, m := range plan.Orphans {
		fmt.Fprintf(ctx.Out, "  %s\n    → %s（未挂载收容）\n", ui.Dim(m.From), ui.Path(m.To))
	}
	if n := countPlanChainRefs(ctx.WS.Root, plan); n > 0 {
		fmt.Fprintf(ctx.Out, "出处链将同步改写 %d 处\n", n)
	}
	fmt.Fprintln(ctx.Out, ui.Dim("注意：工作区外引用（思源/Obsidian 等）不会同步"))
	return plan, nil
}

// applyRenumberPlan 执行重排计划与全部引用同步改写
func applyRenumberPlan(ctx *Ctx, plan *roadmap.RenumberPlan) error {
	if err := tier.ApplyRenumber(ctx.WS.Root, plan); err != nil {
		return err
	}
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return fmt.Errorf("roadmap 读取失败（文件已迁移）: %w", err)
	}
	for _, m := range plan.Moves {
		rm.MoveTierBinding(m.From, m.To, roadmap.RiverList, roadmap.RiverList)
	}
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return fmt.Errorf("roadmap 落盘失败（文件已迁移）: %w", err)
	}
	if n, err := rewriteTopics(ctx.WS, plan.Moves); err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("topics.toml 改写失败: "+err.Error()))
	} else if n > 0 {
		fmt.Fprintf(ctx.Out, "topics.toml 改写 %d 处\n", n)
	}
	chainN := 0
	for _, m := range plan.Moves {
		if n, err := tier.RewriteSourceChains(ctx.WS.Root, m.From, m.To); err == nil {
			chainN += n
		}
	}
	for _, m := range plan.Orphans {
		if n, err := tier.RewriteSourceChains(ctx.WS.Root, m.From, m.To); err == nil {
			chainN += n
		}
	}
	if chainN > 0 {
		fmt.Fprintf(ctx.Out, "出处链改写 %d 处\n", chainN)
	}
	fmt.Fprintf(ctx.Out, "重排完成：%d 项迁移 + %d 项收容\n", len(plan.Moves), len(plan.Orphans))
	return nil
}

// countPlanChainRefs 预览阶段统计计划涉及的出处链引用总数
func countPlanChainRefs(root string, plan *roadmap.RenumberPlan) int {
	n := 0
	for _, m := range plan.Moves {
		n += tier.CountChainRefs(root, m.From)
	}
	for _, m := range plan.Orphans {
		n += tier.CountChainRefs(root, m.From)
	}
	return n
}

// rewriteTopics 沉淀索引 topics.toml 路径改写；文件不存在时 no-op（不创建）
func rewriteTopics(ws *workspace.Workspace, moves []roadmap.FileMove) (int, error) {
	if _, err := os.Stat(ws.IndexPath()); err != nil {
		return 0, nil
	}
	ix, err := index.Load(ws.IndexPath())
	if err != nil {
		return 0, err
	}
	replace := map[string]string{}
	for _, m := range moves {
		replace[m.From] = m.To
	}
	n := 0
	for ti := range ix.Topics {
		for fi := range ix.Topics[ti].Files {
			if to, ok := replace[ix.Topics[ti].Files[fi].Path]; ok {
				ix.Topics[ti].Files[fi].Path = to
				n++
			}
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, ix.Save(ws.IndexPath())
}
