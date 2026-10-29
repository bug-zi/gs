package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

func testTierRoadmap() *roadmap.Roadmap {
	return &roadmap.Roadmap{Version: 2, Branches: []roadmap.Branch{
		{Title: "基础", Points: []roadmap.Point{
			{Title: "变量", Group: "语法", Spring: "Study/spring/变量.md"},
			{Title: "切片", Group: "语法", Spring: "Study/spring/切片.md"},
			{Title: "接口", Group: "接口", Spring: "Study/spring/幽灵.md"}, // 幽灵绑定：文件不存在
			{Title: "未建spring", Group: "接口"},                         // 未绑定
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

func TestBuildRoadmapGroupsShipShowsFileName(t *testing.T) {
	// 同一知识点挂多篇 ship 文件时标题必须用文件原名，否则条目无法区分
	files := []string{
		"Study/ship/1.2.0 Go语言基础之数组.md",
		"Study/ship/1.2.1 Go语言基础之切片.md",
	}
	rm := &roadmap.Roadmap{Version: 2, Branches: []roadmap.Branch{
		{Title: "基础", Points: []roadmap.Point{
			{Title: "数组与切片", Group: "语法核心", Ship: files},
		}},
	}}
	groups := buildRoadmapGroups(rm, files, shipBinds, "ship")
	if len(groups) != 1 || len(groups[0].Items) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Items[0].Title != "1.2.0 Go语言基础之数组" || groups[0].Items[1].Title != "1.2.1 Go语言基础之切片" {
		t.Fatalf("ship 条目应显示文件原名而非知识点标题: %+v", groups[0].Items)
	}
}

func TestBuildDirGroups(t *testing.T) {
	files := []string{
		"Study/sea/interview/b.md",
		"Study/sea/Gin与Gee/a2.md",
		"Study/sea/Gin与Gee/a1.md",
		"Study/sea/根目录散件.md",
	}
	groups := buildDirGroups("sea", files, "未归类", nil)
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

func TestShippedSeaSet(t *testing.T) {
	ws := wsForTest(t)
	mk := func(rel string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/sea/a/在加工.md")
	mk("Study/sea/a/已入河.md")
	mk("Study/sea/b/未装船.md")
	mk("Study/sea/b/1.2.0 数组.md")
	mk("Study/ship/在加工.md")
	mk("Study/river/01-并发/03-已入河.md") // 领号入河，去编号后应命中
	mk("Study/river/01-并发/04-数组.md")  // 源编号与领号不同款，归一后应命中

	set := shippedSeaSet(ws)
	if !set["在加工"] {
		t.Fatalf("ship 同名副本应命中: %v", set)
	}
	if !set["已入河"] {
		t.Fatalf("river 去编号名应命中: %v", set)
	}
	if !set["数组"] {
		t.Fatalf("源编号与领号归一后应命中: %v", set)
	}
	if set["未装船"] {
		t.Fatalf("无 ship 副本且无 river 条目不应命中: %v", set)
	}
}

func TestBuildDirGroupsShipped(t *testing.T) {
	files := []string{
		"Study/sea/g/1-已装船.md",
		"Study/sea/g/2-新材料.md",
		"Study/sea/g/3-也装船.md",
	}
	// shipped 键为编号归一后的名（与 shippedSeaSet 输出同款）
	shipped := map[string]bool{"已装船": true, "也装船": true}
	groups := buildDirGroups("sea", files, "未归类", shipped)
	items := groups[0].Items
	want := []struct {
		title   string
		shipped bool
	}{
		{"2-新材料", false}, // 未装船在前（即使名字比已装船的大）
		{"1-已装船", true},  // 已装船在后，组内仍按名升序
		{"3-也装船", true},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v", items)
	}
	for i, w := range want {
		if items[i].Title != w.title || items[i].Shipped != w.shipped {
			t.Fatalf("items[%d] = %+v, want %+v", i, items[i], w)
		}
	}
}
