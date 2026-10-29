package command

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/planview"
	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/tier"
	"github.com/bugzi/gs/internal/ui"
)

// runPlanDel /plan del branch|point <关键词>：非交互直达删除
func runPlanDel(ctx *Ctx, args []string) error {
	if len(args) == 0 {
		return errors.New("用法：/plan del branch|point <关键词>")
	}
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	switch args[0] {
	case "branch":
		return runPlanDelBranch(ctx, rm, strings.Join(args[1:], " "))
	case "point":
		return runPlanDelPoint(ctx, rm, strings.Join(args[1:], " "))
	default:
		return fmt.Errorf("未知类型 %q（支持 branch / point）", args[0])
	}
}

// runPlanDelPoint 删除知识点：定位 → 预览 → 确认 → 执行（与浏览器 o 菜单「删除」同语义）
func runPlanDelPoint(ctx *Ctx, rm *roadmap.Roadmap, kw string) error {
	ref, err := locatePoint(ctx, rm, kw)
	if err != nil {
		return err
	}
	p := rm.PointAt(ref)
	river := append([]string{}, p.River...)
	fmt.Fprintf(ctx.Out, "删除知识点：%s\n", ui.Path(rm.PathOf(ref)))
	if !rm.PointEmpty(ref) {
		fmt.Fprintf(ctx.Out, "  将解绑：%s\n", planview.BindingsBrief(p))
	}
	for _, rel := range river {
		fmt.Fprintf(ctx.Out, "  收容：%s → 99-未挂载\n", ui.Dim(rel))
	}
	ans, err := inputLine(ctx, "确认删除？（y/N）")
	if err != nil || strings.TrimSpace(ans) != "y" && strings.TrimSpace(ans) != "Y" {
		fmt.Fprintln(ctx.Out, "已取消")
		return nil
	}
	return applyPointDelete(ctx, rm, ref, river)
}

// applyPointDelete 删点执行核心（可直测）：清绑定 → 落盘 → river 收容 → 引用改写。
// 顺序落盘先行、文件迁移在后；迁移失败残留态可 /plan renumber 收容修复。
func applyPointDelete(ctx *Ctx, rm *roadmap.Roadmap, ref roadmap.PointRef, river []string) error {
	rm.DetachPoint(ref)
	if err := rm.DeletePoint(ref); err != nil {
		return err
	}
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return fmt.Errorf("roadmap 落盘失败（文件未迁移）: %w", err)
	}
	fmt.Fprintf(ctx.Out, "%s 知识点已删除并落盘\n", ui.OK("完成"))
	if len(river) == 0 {
		return nil
	}
	moves := roadmap.BuildUnmountMoves(river, roadmap.ListUnmounted(ctx.WS.Root))
	chainN, err := applyFileMoves(ctx, moves)
	if err != nil {
		return fmt.Errorf("river 收容迁移失败（绑定已清，可 /plan renumber 收容修复）: %w", err)
	}
	if chainN > 0 {
		fmt.Fprintf(ctx.Out, "出处链改写 %d 处\n", chainN)
	}
	return nil
}

// runPlanDelBranch 删除空主干：定位 → 校验 → 确认 → 落盘 → 空板块目录入回收站
func runPlanDelBranch(ctx *Ctx, rm *roadmap.Roadmap, kw string) error {
	bi, err := locateBranch(ctx, rm, kw)
	if err != nil {
		return err
	}
	b := rm.Branches[bi]
	if len(b.Points) > 0 {
		return fmt.Errorf("主干「%s」仍有 %d 个知识点，先逐个删除或移走", b.Title, len(b.Points))
	}
	if len(rm.Branches) <= 1 {
		return errors.New("至少保留一个主干")
	}
	ans, err := inputLine(ctx, fmt.Sprintf("删除空主干「%s」？（y/N，删除后可 /plan renumber 重排编号）", b.Title))
	if err != nil || strings.TrimSpace(ans) != "y" && strings.TrimSpace(ans) != "Y" {
		fmt.Fprintln(ctx.Out, "已取消")
		return nil
	}
	title := b.Title
	if err := rm.DeleteBranch(bi); err != nil {
		return err
	}
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return fmt.Errorf("roadmap 落盘失败: %w", err)
	}
	fmt.Fprintf(ctx.Out, "%s 主干「%s」已删除\n", ui.OK("完成"), title)
	dirRel := fmt.Sprintf("%s/%02d-%s", roadmap.RiverDirRel, bi+1, title)
	if entries, err := os.ReadDir(filepath.Join(ctx.WS.Root, filepath.FromSlash(dirRel))); err == nil && len(entries) == 0 {
		if trashRel, err := tier.MoveToTrash(ctx.WS.Root, dirRel, timeNowCST()); err == nil {
			fmt.Fprintf(ctx.Out, "空板块目录已入回收站 %s\n", ui.Dim(trashRel))
		}
	}
	fmt.Fprintln(ctx.Out, ui.Dim("可运行 /plan renumber 重排其余板块编号"))
	return nil
}

// locateBranch 主干定位：空关键词全量弹选；关键词唯一直达、多匹配弹选
func locateBranch(ctx *Ctx, rm *roadmap.Roadmap, kw string) (int, error) {
	if kw == "" {
		return pickBranchIdx(ctx, rm)
	}
	ms := rm.MatchBranches(kw)
	switch len(ms) {
	case 0:
		return 0, fmt.Errorf("没有匹配 %q 的主干", kw)
	case 1:
		return ms[0], nil
	}
	return pickBranchFrom(ctx, rm, ms, "匹配到多个主干")
}
