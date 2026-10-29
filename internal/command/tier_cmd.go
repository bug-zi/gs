package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/tier"
	"github.com/bugzi/gs/internal/ui"
)

// runSeaImport inbox → clip 整理：逐个预览、起标题、迁移
func runSeaImport(ctx *Ctx) error {
	inbox := filepath.Join(ctx.WS.SeaDir(), "inbox")
	entries, err := os.ReadDir(inbox)
	if err != nil {
		fmt.Fprintln(ctx.Out, ui.Dim("没有 inbox 目录（放文件到 Study/sea/inbox/ 后再执行）"))
		return nil
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		src := filepath.Join(inbox, e.Name())
		fmt.Fprintf(ctx.Out, "\n%s\n%s\n", ui.Path("Study/sea/inbox/"+e.Name()), previewOf(src, 10))
		def := strings.TrimSuffix(e.Name(), ".md")
		title, err := inputLine(ctx, "导入标题（回车默认 "+def+"）")
		if err != nil {
			return err
		}
		title = strings.TrimSpace(title)
		if title == "" {
			title = def
		}
		dst := filepath.Join(ctx.WS.SeaDir(), "clip", title+".md")
		if _, err := os.Stat(dst); err == nil {
			fmt.Fprintln(ctx.Out, ui.Dim("跳过: clip 下已存在 "+title+".md"))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			return err
		}
		fmt.Fprintf(ctx.Out, "%s %s\n", ui.OK("已导入"), ui.Path(ctx.WS.Rel(dst)))
		n++
	}
	if n == 0 {
		fmt.Fprintln(ctx.Out, ui.Dim("inbox 中没有待导入的 md"))
	} else {
		fmt.Fprintf(ctx.Out, "导入完成：%d 个\n", n)
	}
	return nil
}

// newTierCmd 生成三层命令的板块化入口：默认查阅、edit 编辑、extraSubs 各自特例（add/import）优先。
// 条目按板块分组选择（spring/river 按 roadmap 主干、sea 按一级子目录），关键词平铺直选。
func newTierCmd(name, short string, extraSubs []string, extra func(ctx *Ctx, sub string) (bool, error)) Command {
	subs := append([]string{"edit", "del"}, extraSubs...)
	return Command{
		Name:  name,
		Short: short,
		Subs:  subs,
		Run: func(ctx *Ctx) error {
			if len(ctx.Args) > 0 {
				if handled, err := extra(ctx, ctx.Args[0]); handled || err != nil {
					return err
				}
				if ctx.Args[0] == "del" {
					return runTierDel(ctx, name, strings.Join(ctx.Args[1:], " "))
				}
				if ctx.Args[0] == "edit" {
					it, err := pickTierEntryGrouped(ctx, name, strings.Join(ctx.Args[1:], " "), "选择要编辑的条目")
					if err != nil {
						return err
					}
					return editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(it.Path)))
				}
			}
			it, err := pickTierEntryGrouped(ctx, name, strings.Join(ctx.Args, " "), "选择条目")
			if err != nil {
				return err
			}
			return runTierItemView(ctx, name, it)
		},
	}
}

func newSea() Command {
	return newTierCmd("sea", "大海：外部知识查阅/导入/编辑", []string{"import"}, func(ctx *Ctx, sub string) (bool, error) {
		if sub == "import" {
			return true, runSeaImport(ctx)
		}
		return false, nil
	})
}

// runShipCopy 装船命令入口（/ship copy）：选择器 → 装船核心 → 打开编辑器
func runShipCopy(ctx *Ctx) error {
	it, err := pickTierEntryGrouped(ctx, "sea", strings.Join(ctx.Args[1:], " "), "选择要装船（开始加工）的大海条目")
	if err != nil {
		return err
	}
	dstRel, err := moveToShip(ctx, it.Path)
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "已装船 %s，开始加工\n", ui.Path(dstRel))
	return editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(dstRel)))
}

// moveToShip 装船核心（不含选择器与编辑器，可直测）：复制入 ship/（sea 原件与其绑定原样保留）→
// 绑定复制 sea[]→ship[]。装船不动 sea、不改写出处链；ship→river 才是真实迁移。
func moveToShip(ctx *Ctx, srcRel string) (string, error) {
	dstRel := "Study/ship/" + filepath.Base(srcRel)
	if err := tier.CopyFile(ctx.WS.Root, srcRel, dstRel); err != nil {
		return "", err
	}
	if rm, err := loadRoadmapSoft(ctx); err == nil && rm != nil {
		if hits := rm.CopyTierBinding(srcRel, dstRel, roadmap.SeaList, roadmap.ShipList); len(hits) > 0 {
			if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
				fmt.Fprintln(ctx.Out, ui.Err("绑定落盘失败（文件已复制，可重跑 /ship copy 幂等修复）: "+err.Error()))
			} else {
				for _, h := range hits {
					fmt.Fprintf(ctx.Out, "绑定挂载：%s\n", ui.Path(rm.PathOf(h)))
				}
			}
		}
	}
	return dstRel, nil
}

// newShip 船坞命令：copy 装船（sea→ship 复制）、move 入河（ship→river 真实迁移）
func newShip() Command {
	return newTierCmd("ship", "船坞：加工中条目查阅/装船/入河", []string{"copy", "move"}, func(ctx *Ctx, sub string) (bool, error) {
		switch sub {
		case "copy":
			return true, runShipCopy(ctx)
		case "move":
			return true, runRiverMove(ctx)
		}
		return false, nil
	})
}

// askTopic 起主题名：输入框预填默认值
func askTopic(ctx *Ctx, def string) (string, error) {
	s, err := inputLine(ctx, "条目主题（回车默认 "+def+"）")
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = def
	}
	return s, nil
}

// findShipBinding 反查 ship 路径挂在哪个知识点
func findShipBinding(rm *roadmap.Roadmap, rel string) (roadmap.PointRef, bool) {
	for bi := range rm.Branches {
		for pi := range rm.Branches[bi].Points {
			for _, s := range rm.Branches[bi].Points[pi].Ship {
				if s == rel {
					return roadmap.PointRef{Branch: bi, Point: pi}, true
				}
			}
		}
	}
	return roadmap.PointRef{}, false
}

// askMountPoint 追问挂载知识点（关键词过滤，回车跳过）；多匹配弹选择器。返回是否命中。
// 命中后由调用方决定绑定写入哪个列表（各层流转语义不同）。
func askMountPoint(ctx *Ctx, rm *roadmap.Roadmap) (roadmap.PointRef, bool) {
	kw, err := inputLine(ctx, "挂到知识点？（输入关键词过滤，回车跳过）")
	if err != nil {
		return roadmap.PointRef{}, false
	}
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return roadmap.PointRef{}, false
	}
	matches := rm.MatchPoints(kw)
	switch {
	case len(matches) == 0:
		fmt.Fprintln(ctx.Out, ui.Dim("没有匹配的知识点，未挂载"))
		return roadmap.PointRef{}, false
	case len(matches) == 1:
		return matches[0], true
	default:
		var pick string
		opts := make([]menuOption, 0, len(matches))
		for _, m := range matches {
			opts = append(opts, option(rm.PathOf(m), pointKey(m)))
		}
		if err := selectFrom(ctx, "多匹配，挂到哪个知识点？", opts, &pick); err != nil {
			return roadmap.PointRef{}, false
		}
		return pointParse(pick), true
	}
}

// runRiverMove 入河命令入口（/ship move）：选择器 → 挂载解析 → riverLanding 核心 → 绑定迁移 → 出处链改写。
func runRiverMove(ctx *Ctx) error {
	it, err := pickTierEntryGrouped(ctx, "ship", strings.Join(ctx.Args[1:], " "), "选择要入河（加工完成）的船坞条目")
	if err != nil {
		return err
	}
	rm, _ := loadRoadmapSoft(ctx)
	var ref roadmap.PointRef
	bound := false
	if rm != nil {
		ref, bound = findShipBinding(rm, it.Path)
		if !bound {
			if r, ok := askMountPoint(ctx, rm); ok {
				ref, bound = r, true
			}
		}
	}
	dstRel, err := riverLanding(ctx, rm, it.Path, ref, bound)
	if err != nil {
		return err
	}
	if bound && rm != nil {
		rm.MoveTierBinding(it.Path, dstRel, roadmap.ShipList, roadmap.RiverList)
		if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
			fmt.Fprintln(ctx.Out, ui.Err("绑定落盘失败（文件已移动）: "+err.Error()))
		}
	}
	if n, err := tier.RewriteSourceChains(ctx.WS.Root, it.Path, dstRel); err == nil && n > 0 {
		fmt.Fprintf(ctx.Out, "出处链改写 %d 处\n", n)
	}
	fmt.Fprintf(ctx.Out, "已入河 %s\n  （原 %s）\n", ui.Path(dstRel), ui.Dim(it.Path))
	return nil
}

// riverLanding 入河落点核心（不含选择器/追问/绑定/出处链，可直测）：
// 已挂载领号改名入河（目标冲突篇序自动顺延）；未挂载（或无路线图）入 99-未挂载收容区不领号。
func riverLanding(ctx *Ctx, rm *roadmap.Roadmap, srcRel string, ref roadmap.PointRef, bound bool) (string, error) {
	if !bound || rm == nil {
		dstRel := "Study/river/99-未挂载/" + filepath.Base(srcRel)
		return dstRel, tier.MoveFile(ctx.WS.Root, srcRel, dstRel)
	}
	b := &rm.Branches[ref.Branch]
	dirRel := fmt.Sprintf("Study/river/%02d-%s", ref.Branch+1, b.Title)
	pointNum := ref.Point + 1
	title := roadmap.StripNumPrefix(filepath.Base(srcRel))
	dup := roadmap.NextDupSeq(rm.PointAt(ref).River, pointNum)
	for {
		dstRel := dirRel + "/" + roadmap.RiverFileName(pointNum, dup, title)
		err := tier.MoveFile(ctx.WS.Root, srcRel, dstRel)
		if err == nil {
			return dstRel, nil
		}
		if !strings.Contains(err.Error(), "目标已存在") {
			return "", err
		}
		dup++ // 篇序自动顺延重试
	}
}

// runSpringAdd 创建/打开知识点唯一的 spring 总结并绑定：
// 定位知识点 → Study/spring/<标题>.md 已存在则直接打开；否则创建占位条目并 BindSpring 落盘
func runSpringAdd(ctx *Ctx, kw string) error {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	ref, err := locatePoint(ctx, rm, kw)
	if err != nil {
		return err
	}
	p := rm.PointAt(ref)
	if p.Spring != "" {
		abs := filepath.Join(ctx.WS.Root, filepath.FromSlash(p.Spring))
		fmt.Fprintf(ctx.Out, "已存在 %s，直接打开\n", ui.Path(p.Spring))
		return editOpen(ctx, abs)
	}
	rel, err := CreateEntry(ctx.WS, "spring", p.Title, "", "（用自己的话总结这个知识点，写完删掉本行）\n")
	if err != nil {
		return err
	}
	rm.BindSpring(ref, rel)
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s %s\n", ui.OK("已创建并绑定"), ui.Path(rel))
	return editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(rel)))
}

// runCodeAdd 创建/打开知识点唯一的演示代码并绑定（镜像 runSpringAdd）：
// 定位知识点 → Study/code/<标题>.go 已绑定则直接打开；否则创建模板文件并 BindCode 落盘
func runCodeAdd(ctx *Ctx, kw string) error {
	rm, err := loadRoadmap(ctx)
	if err != nil {
		return err
	}
	ref, err := locatePoint(ctx, rm, kw)
	if err != nil {
		return err
	}
	p := rm.PointAt(ref)
	if p.Code != "" {
		abs := filepath.Join(ctx.WS.Root, filepath.FromSlash(p.Code))
		fmt.Fprintf(ctx.Out, "已存在 %s，直接打开\n", ui.Path(p.Code))
		return editOpen(ctx, abs)
	}
	rel, err := CreateCodeEntry(ctx.WS, p.Title)
	if err != nil {
		return err
	}
	rm.BindCode(ref, rel)
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "%s %s\n", ui.OK("已创建并绑定"), ui.Path(rel))
	return editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(rel)))
}

func newCode() Command {
	return newTierCmd("code", "代码：知识点演示代码（唯一）/按板块查阅/编辑", []string{"add"}, func(ctx *Ctx, sub string) (bool, error) {
		if sub == "add" {
			return true, runCodeAdd(ctx, strings.Join(ctx.Args[1:], " "))
		}
		return false, nil
	})
}

func newRiver() Command {
	return newTierCmd("river", "河流：沉淀查阅/编辑", nil, func(ctx *Ctx, sub string) (bool, error) {
		return false, nil
	})
}

func newSpring() Command {
	return newTierCmd("spring", "泉水：知识点总结（唯一）/按板块查阅/编辑", []string{"add"}, func(ctx *Ctx, sub string) (bool, error) {
		if sub == "add" {
			return true, runSpringAdd(ctx, strings.Join(ctx.Args[1:], " "))
		}
		return false, nil
	})
}
