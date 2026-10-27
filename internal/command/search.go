package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bugzi/gs/internal/search"
	"github.com/bugzi/gs/internal/ui"
)

func newSearch() Command {
	return Command{
		Name:  "search",
		Alias: []string{"s"},
		Short: "全局搜索 md 与 go 源码",
		Run: func(ctx *Ctx) error {
			query := strings.Join(ctx.Args, " ")
			if query == "" {
				return errors.New("用法: /search <关键词> [更多词…]（空格分隔为 AND）")
			}
			results, words, err := search.Search(ctx.WS.Root, ctx.Cfg, query)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintf(ctx.Out, "未找到含 %q 的内容\n", query)
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(ctx.Out, "\n%s\n", ui.Path(r.Path))
				for _, h := range r.Hits {
					loc := ""
					if h.Unit != "" {
						loc = h.Unit + " "
					}
					fmt.Fprintf(ctx.Out, "%s\n", ui.Dim(fmt.Sprintf("  %s:%d", loc, h.Line)))
					for _, l := range h.Context {
						fmt.Fprintf(ctx.Out, "  | %s\n", ui.HighlightWords(strings.TrimRight(l, "\r"), words))
					}
				}
			}
			fmt.Fprintf(ctx.Out, "\n共 %d 个文件命中\n", len(results))
			return nil
		},
	}
}
