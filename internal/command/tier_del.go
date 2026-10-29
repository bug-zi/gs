package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/tier"
	"github.com/bugzi/gs/internal/ui"
)

// runTierDel /sea|ship|river|spring|code del 入口：选择条目 → 删除全流程
func runTierDel(ctx *Ctx, tierName, kw string) error {
	it, err := pickTierEntryGrouped(ctx, tierName, kw, "选择要删除的条目")
	if err != nil {
		return err
	}
	_, _ = deleteTierEntry(ctx, tierName, it.Path)
	return nil
}

// deleteTierEntry 条目删除全流程：预览影响 → y/N 确认 → 落盘绑定 → 回收站 → 引用清理。
// 返回是否已删除；输入失败（Esc 等）与非 y 一律视为取消不报错——查阅态 [x] 键复用本流程，
// 取消应静默回到查阅。
func deleteTierEntry(ctx *Ctx, tierName, rel string) (bool, error) {
	printTierDeletePreview(ctx, tierName, rel)
	ans, err := inputLine(ctx, "确认删除？（y/N）")
	if err != nil || strings.TrimSpace(ans) != "y" && strings.TrimSpace(ans) != "Y" {
		fmt.Fprintln(ctx.Out, "已取消")
		return false, nil
	}
	if err := applyTierDelete(ctx, tierName, rel); err != nil {
		return false, err
	}
	return true, nil
}

// printTierDeletePreview 输出删除影响预览：文件迁移、绑定清理、引用统计
func printTierDeletePreview(ctx *Ctx, tierName, rel string) {
	fmt.Fprintf(ctx.Out, "删除 %s 条目：%s\n", tierName, ui.Path(rel))
	fmt.Fprintln(ctx.Out, ui.Dim("  文件将移入回收站（.gs/trash/<时间戳>/ 下保留原相对结构）"))
	owners := []roadmap.PointRef{}
	if rm, err := loadRoadmapSoft(ctx); err == nil && rm != nil {
		owners = rm.OwnersOf(rel)
		if len(owners) > 0 {
			fmt.Fprintln(ctx.Out, "绑定清理：")
			for _, o := range owners {
				fmt.Fprintf(ctx.Out, "  - %s（%s）\n", rm.PathOf(o), tierName)
			}
		}
	}
	refs := false
	if n := countTopicsRefs(ctx, rel); n > 0 {
		refs = true
		fmt.Fprintf(ctx.Out, "引用统计：topics.toml 将清除 %d 条\n", n)
	}
	if n := tier.CountChainRefs(ctx.WS.Root, rel); n > 0 {
		refs = true
		fmt.Fprintf(ctx.Out, "引用统计：出处链引用 %d 处将失效（原文保留不动）\n", n)
	}
	if len(owners) == 0 && !refs {
		fmt.Fprintln(ctx.Out, ui.Dim("无绑定与引用，仅移入回收站"))
	}
}

// applyTierDelete 删除执行：清绑定落盘 → 回收站迁移 → topics 清理。
// 出处链指向被删文件的一律只警告不改写（不擅自改内容）；迁移失败时绑定已清、
// 文件原地保留，重跑 del 幂等修复。
func applyTierDelete(ctx *Ctx, tierName, rel string) error {
	rm, err := loadRoadmapSoft(ctx)
	if err != nil {
		return err
	}
	if rm != nil {
		if owners := unbindEntry(rm, tierName, rel); len(owners) > 0 {
			if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
				return fmt.Errorf("绑定落盘失败（文件未迁移）: %w", err)
			}
			for _, o := range owners {
				fmt.Fprintf(ctx.Out, "%s 已解绑 %s\n", ui.OK("绑定"), rm.PathOf(o))
			}
		}
	}
	trashRel, err := tier.MoveToTrash(ctx.WS.Root, rel, timeNowCST())
	if err != nil {
		return fmt.Errorf("回收站迁移失败（绑定已清，文件原地保留，可重跑 del 幂等修复）: %w", err)
	}
	fmt.Fprintf(ctx.Out, "%s 已移入回收站 %s\n", ui.OK("删除"), ui.Dim(trashRel))
	pruneTopics(ctx, rel)
	if n := tier.CountChainRefs(ctx.WS.Root, rel); n > 0 {
		fmt.Fprintf(ctx.Out, ui.Dim("注意：出处链引用 %d 处已失效（原文保留）")+"\n", n)
	}
	return nil
}

// unbindEntry 从全部挂载点清除该条目的绑定；返回受影响知识点
func unbindEntry(rm *roadmap.Roadmap, tierName, rel string) []roadmap.PointRef {
	owners := rm.OwnersOf(rel)
	for _, o := range owners {
		switch tierName {
		case "spring":
			rm.UnbindSpring(o)
		case "code":
			rm.UnbindCode(o)
		case "sea":
			rm.UnbindList(o, roadmap.SeaList, rel)
		case "ship":
			rm.UnbindList(o, roadmap.ShipList, rel)
		case "river":
			rm.UnbindList(o, roadmap.RiverList, rel)
		}
	}
	return owners
}

// countTopicsRefs topics.toml 中指向 rel 的条目数；索引不存在或读取失败视为 0
func countTopicsRefs(ctx *Ctx, rel string) int {
	if _, err := os.Stat(ctx.WS.IndexPath()); err != nil {
		return 0
	}
	ix, err := index.Load(ctx.WS.IndexPath())
	if err != nil {
		return 0
	}
	return ix.CountPaths(map[string]bool{rel: true})
}

// pruneTopics 清除 topics.toml 中指向 rel 的条目并落盘；索引不存在时 no-op
func pruneTopics(ctx *Ctx, rel string) {
	if _, err := os.Stat(ctx.WS.IndexPath()); err != nil {
		return
	}
	ix, err := index.Load(ctx.WS.IndexPath())
	if err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("topics.toml 读取失败: "+err.Error()))
		return
	}
	if n := ix.PrunePaths(map[string]bool{rel: true}); n > 0 {
		if err := ix.Save(ctx.WS.IndexPath()); err != nil {
			fmt.Fprintln(ctx.Out, ui.Err("topics.toml 落盘失败: "+err.Error()))
			return
		}
		fmt.Fprintf(ctx.Out, "topics.toml 清除 %d 条\n", n)
	}
}

// applyFileMoves 执行文件迁移并同步改写 topics 与出处链；返回出处链改写处数。
// planview 删点收容与 /plan del point 共用；迁移语义同 renumber（ApplyRenumber 幂等可重跑）。
func applyFileMoves(ctx *Ctx, moves []roadmap.FileMove) (int, error) {
	if len(moves) == 0 {
		return 0, nil
	}
	if err := tier.ApplyRenumber(ctx.WS.Root, &roadmap.RenumberPlan{Moves: moves}); err != nil {
		return 0, err
	}
	if n, err := rewriteTopics(ctx.WS, moves); err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("topics.toml 改写失败: "+err.Error()))
	} else if n > 0 {
		fmt.Fprintf(ctx.Out, "topics.toml 改写 %d 处\n", n)
	}
	chainN := 0
	for _, m := range moves {
		if n, err := tier.RewriteSourceChains(ctx.WS.Root, m.From, m.To); err == nil {
			chainN += n
		}
	}
	return chainN, nil
}
