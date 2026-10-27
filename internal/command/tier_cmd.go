package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/roadmap"
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
		fmt.Fprintf(ctx.Out, "已导入 %s\n", ui.Path(ctx.WS.Rel(dst)))
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
	subs := append([]string{"edit"}, extraSubs...)
	return Command{
		Name:  name,
		Short: short,
		Subs:  subs,
		Run: func(ctx *Ctx) error {
			if len(ctx.Args) > 0 {
				if handled, err := extra(ctx, ctx.Args[0]); handled || err != nil {
					return err
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

// runRiverAdd 大海→河流 摘录：选 sea 条目 → 创建预填「出处+全文」的 river 条目 → 打开编辑器
// （全文摘录=直接保存退出；选段摘录=编辑器内删减后保存）
func runRiverAdd(ctx *Ctx) error {
	it, err := pickTierEntryGrouped(ctx, "sea", strings.Join(ctx.Args, " "), "选择要摘录的大海条目")
	if err != nil {
		return err
	}
	src := it.Path
	data, err := os.ReadFile(filepath.Join(ctx.WS.Root, filepath.FromSlash(src)))
	if err != nil {
		return err
	}
	def := strings.TrimSuffix(filepath.Base(src), ".md")
	topic, err := askTopic(ctx, def)
	if err != nil {
		return err
	}
	rel, err := CreateEntry(ctx.WS, "river", topic, src, string(data))
	if err != nil {
		return err
	}
	fmt.Fprintf(ctx.Out, "已创建 %s（可在编辑器内删减为选段）\n", ui.Path(rel))
	// 尾部追问：挂到知识点（可回车跳过）；路线图不存在时静默跳过
	if rm, err := loadRoadmapSoft(ctx); err == nil && rm != nil {
		kw, err := inputLine(ctx, "挂到知识点？（输入关键词过滤，回车跳过）")
		if err == nil {
			if kw = strings.TrimSpace(kw); kw != "" {
				matches := rm.MatchPoints(kw)
				switch {
				case len(matches) == 0:
					fmt.Fprintln(ctx.Out, ui.Dim("没有匹配的知识点，未挂载"))
				case len(matches) == 1:
					bindAndReport(ctx, rm, matches[0], rel)
				default:
					var pick string
					opts := make([]menuOption, 0, len(matches))
					for _, m := range matches {
						opts = append(opts, option(rm.PathOf(m), pointKey(m)))
					}
					if err := selectFrom(ctx, "多匹配，挂到哪个知识点？", opts, &pick); err == nil {
						bindAndReport(ctx, rm, pointParse(pick), rel)
					}
				}
			}
		}
	}
	return editOpen(ctx, filepath.Join(ctx.WS.Root, filepath.FromSlash(rel)))
}

// bindAndReport 挂载 river 绑定并落盘汇报
func bindAndReport(ctx *Ctx, rm *roadmap.Roadmap, ref roadmap.PointRef, rel string) {
	rm.BindRiver(ref, rel)
	if err := roadmap.Save(ctx.WS.RoadmapFile(), rm); err != nil {
		fmt.Fprintln(ctx.Out, ui.Err("绑定落盘失败: "+err.Error()))
		return
	}
	fmt.Fprintf(ctx.Out, "已挂到 %s\n", ui.Path(rm.PathOf(ref)))
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
	fmt.Fprintf(ctx.Out, "已创建并绑定 %s\n", ui.Path(rel))
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
	fmt.Fprintf(ctx.Out, "已创建并绑定 %s\n", ui.Path(rel))
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
	return newTierCmd("river", "河流：摘录查阅/新增/编辑", []string{"add"}, func(ctx *Ctx, sub string) (bool, error) {
		if sub == "add" {
			return true, runRiverAdd(ctx)
		}
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
