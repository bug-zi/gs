package command

import (
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

func testTierRoadmap() *roadmap.Roadmap {
	return &roadmap.Roadmap{Version: 2, Branches: []roadmap.Branch{
		{Title: "基础", Points: []roadmap.Point{
			{Title: "变量", Group: "语法", Spring: "Study/spring/变量.md"},
			{Title: "切片", Group: "语法", Spring: "Study/spring/切片.md"},
			{Title: "接口", Group: "接口", Spring: "Study/spring/幽灵.md"}, // 幽灵绑定：文件不存在
			{Title: "未建spring", Group: "接口"},                          // 未绑定
		}},
		{Title: "并发", Points: []roadmap.Point{
			{Title: "channel", Spring: "Study/spring/channel.md"},
			{Title: "select", River: []string{"Study/river/摘录A.md", "Study/river/摘录A.md"}}, // 同组重复绑定应去重
		}},
		{Title: "运行时", Points: []roadmap.Point{
			{Title: "g0", River: []string{"Study/river/摘录A.md"}}, // 跨板块多绑：A 应同时出现在并发与运行时
		}},
		{Title: "空板块", Points: []roadmap.Point{
			{Title: "x", Spring: "Study/spring/不存在.md"}, // 全幽灵 → 板块不出现
		}},
	}}
}

func TestBuildRoadmapGroupsSpring(t *testing.T) {
	files := []string{
		"Study/spring/散件.md",
		"Study/spring/channel.md",
		"Study/spring/变量.md",
		"Study/spring/切片.md",
	}
	groups := buildRoadmapGroups(testTierRoadmap(), files, springBinds, "spring")
	if len(groups) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Name != "基础" || groups[1].Name != "并发" || groups[2].Name != "未挂载" {
		t.Fatalf("板块顺序错误: %+v", groups)
	}
	if len(groups[0].Items) != 2 || groups[0].Items[0].Title != "变量" || groups[0].Items[1].Title != "切片" {
		t.Fatalf("基础组条目错误（幽灵绑定不应展示）: %+v", groups[0].Items)
	}
	if groups[2].Items[0].Title != "散件" {
		t.Fatalf("未挂载组应按名排序: %+v", groups[2].Items)
	}
}

func TestBuildRoadmapGroupsNoRoadmap(t *testing.T) {
	groups := buildRoadmapGroups(nil, []string{"Study/spring/b.md", "Study/spring/a.md"}, springBinds, "spring")
	if len(groups) != 1 || groups[0].Name != "spring" || len(groups[0].Items) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Items[0].Title != "a" {
		t.Fatalf("应按名排序: %+v", groups[0].Items)
	}
}

func TestBuildRoadmapGroupsRiverMultiBind(t *testing.T) {
	files := []string{"Study/river/摘录A.md", "Study/river/散件.md"}
	groups := buildRoadmapGroups(testTierRoadmap(), files, riverBinds, "river")
	// 基础主干无 river 绑定 → 不出现；应为 [并发, 运行时, 未挂载]
	if len(groups) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
	// 并发组：同组重复绑定去重后 1 条
	if groups[0].Name != "并发" || len(groups[0].Items) != 1 || groups[0].Items[0].Path != "Study/river/摘录A.md" {
		t.Fatalf("并发组应去重: %+v", groups[0])
	}
	// 运行时组：跨板块多绑的 A 再次出现
	if groups[1].Name != "运行时" || len(groups[1].Items) != 1 || groups[1].Items[0].Path != "Study/river/摘录A.md" {
		t.Fatalf("运行时组缺多绑条目: %+v", groups[1])
	}
	// A 已绑定，未挂载组只剩散件
	if last := groups[2]; last.Name != "未挂载" || len(last.Items) != 1 || last.Items[0].Title != "散件" {
		t.Fatalf("未挂载组错误: %+v", last)
	}
}

func TestBuildDirGroups(t *testing.T) {
	files := []string{
		"Study/sea/interview/b.md",
		"Study/sea/Gin与Gee/a2.md",
		"Study/sea/Gin与Gee/a1.md",
		"Study/sea/根目录散件.md",
	}
	groups := buildDirGroups("sea", files, "未归类")
	if len(groups) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
	// 组按名排序：Gin与Gee < interview < 未归类
	if groups[0].Name != "Gin与Gee" || groups[1].Name != "interview" || groups[2].Name != "未归类" {
		t.Fatalf("组序错误: %+v", groups)
	}
	if groups[0].Items[0].Title != "a1" || groups[0].Items[1].Title != "a2" {
		t.Fatalf("组内应按名排序: %+v", groups[0].Items)
	}
	if groups[2].Items[0].Title != "根目录散件" {
		t.Fatalf("散件组错误: %+v", groups[2])
	}
}

func TestFilterTierGroups(t *testing.T) {
	files := []string{"Study/spring/变量.md", "Study/spring/切片.md", "Study/spring/channel.md"}
	rm := testTierRoadmap()

	if all := filterTierGroups(buildRoadmapGroups(rm, files, springBinds, "spring"), ""); len(all) != 2 {
		t.Fatalf("空关键词应返回全部: %+v", all)
	}
	if g := filterTierGroups(buildRoadmapGroups(rm, files, springBinds, "spring"), "并发"); len(g) != 1 || len(g[0].Items) != 1 || g[0].Items[0].Title != "channel" {
		t.Fatalf("组名命中应保留整组: %+v", g)
	}
	if g := filterTierGroups(buildRoadmapGroups(rm, files, springBinds, "spring"), "语法"); len(g) != 1 || len(g[0].Items) != 2 {
		t.Fatalf("小标签命中: %+v", g)
	}
	if g := filterTierGroups(buildRoadmapGroups(rm, files, springBinds, "spring"), "chan"); len(g) != 1 || len(g[0].Items) != 1 {
		t.Fatalf("标题命中: %+v", g)
	}
	if g := filterTierGroups(buildRoadmapGroups(rm, files, springBinds, "spring"), "mysql"); len(g) != 0 {
		t.Fatalf("零命中应为空: %+v", g)
	}
}

func TestTierItemLabel(t *testing.T) {
	g := TierGroup{Name: "基础", Items: []TierItem{
		{Title: "变量", Sub: "语法"},
		{Title: "接口", Sub: "接口"},
	}}
	if s := tierItemLabel(g, g.Items[0], false); s != "语法 · 变量" {
		t.Fatalf("多小标签应带「标签 · 」前缀: %s", s)
	}
	if s := tierItemLabel(g, g.Items[0], true); s != "【基础】语法 · 变量" {
		t.Fatalf("多组应带「【组名】」前缀: %s", s)
	}
	g1 := TierGroup{Name: "并发", Items: []TierItem{{Title: "channel", Sub: "原语"}}}
	if s := tierItemLabel(g1, g1.Items[0], false); s != "channel" {
		t.Fatalf("单小标签不应带前缀: %s", s)
	}
}

func TestTierRelPath(t *testing.T) {
	if s := tierRelPath("spring", "Study/spring/变量.md"); s != "变量.md" {
		t.Fatalf("tierRelPath = %s", s)
	}
	if s := tierRelPath("sea", "Study/sea/Go语言基础/1.1.0 x.md"); s != "Go语言基础/1.1.0 x.md" {
		t.Fatalf("tierRelPath = %s", s)
	}
	if s := tierRelPath("sea", "其他/路径.md"); s != "其他/路径.md" {
		t.Fatalf("非本层前缀应原样返回: %s", s)
	}
}
