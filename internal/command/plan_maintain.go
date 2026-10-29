package command

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bugzi/gs/internal/planview"
	"github.com/bugzi/gs/internal/roadmap"
)

// tierRoot 待归类扫描的 Study 子目录
var tierRoot = map[planview.Tier]string{
	planview.TierSea:    "sea",
	planview.TierShip:   "ship",
	planview.TierRiver:  "river",
	planview.TierSpring: "spring",
	planview.TierCode:   "code",
}

// scanUnbound 扫描 Study/<tier> 下未被任何知识点绑定的文件（code 认 .go，其余 .md），按路径排序；
// 目录缺失按 0 计（不算错误）
func scanUnbound(root string, rm *roadmap.Roadmap, t planview.Tier) []string {
	dir := filepath.Join(root, "Study", tierRoot[t])
	ext := ".md"
	if t == planview.TierCode {
		ext = ".go"
	}
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 目录不存在/无权限：跳过
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(d.Name()), ext) {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !rm.IsBound(rel) {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return out
}

// selectPointFlow 关键词过滤选知识点：输入关键词（空=全部）→ 唯一直达/多选/零匹配重试
func selectPointFlow(ctx *Ctx, rm *roadmap.Roadmap, prompt string) (roadmap.PointRef, bool) {
	for {
		kw, err := inputLine(ctx, prompt+"（关键词过滤，空=全部，Esc 取消）")
		if err != nil {
			return roadmap.PointRef{}, false
		}
		hits := rm.MatchPoints(strings.TrimSpace(kw))
		switch len(hits) {
		case 0:
			fmt.Fprintln(ctx.Out, "没有匹配的知识点，换个关键词或 Esc 取消")
			continue
		case 1:
			return hits[0], true
		}
		opts := make([]menuOption, 0, len(hits))
		for _, h := range hits {
			opts = append(opts, option(rm.PathOf(h), strconv.Itoa(h.Branch)+":"+strconv.Itoa(h.Point)))
		}
		var choice string
		if err := selectFrom(ctx, "选择知识点", opts, &choice); err != nil {
			return roadmap.PointRef{}, false
		}
		var ref roadmap.PointRef
		fmt.Sscanf(choice, "%d:%d", &ref.Branch, &ref.Point)
		return ref, true
	}
}

// selectOneFlow 单选菜单，返回下标
func selectOneFlow(ctx *Ctx, prompt string, opts []string) (int, bool) {
	mo := make([]menuOption, 0, len(opts))
	for i, o := range opts {
		mo = append(mo, option(o, strconv.Itoa(i)))
	}
	var choice string
	if err := selectFrom(ctx, prompt, mo, &choice); err != nil {
		return 0, false
	}
	i, err := strconv.Atoi(choice)
	if err != nil {
		return 0, false
	}
	return i, true
}
