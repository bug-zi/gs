package roadmap

import (
	"strings"
	"testing"
)

func renderFixture(t *testing.T) *Roadmap { return opsFixture(t) }

func TestRenderStatic(t *testing.T) {
	rm := renderFixture(t)
	got := Render(rm, RenderOpts{Title: "Go 学习路线图", Progress: "进度 1/4 (25%)", Next: "下一步：走读 map 源码"})
	for _, want := range []string{
		"进度 1/4 (25%)",
		"下一步：走读 map 源码",
		"语言基础 ·", // dot 引导线
		"1/3",
		"语法核心", // 分组小标题
		"├─ [x] slice 底层结构与扩容", // done 符号
		"└─ [ ] iface/eface 底层",
		"└─ [ ] atomic",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("缺少 %q\n输出:\n%s", want, got)
		}
	}
	// 同组连续知识点只出一次分组小标题
	if strings.Count(got, "语法核心") != 1 {
		t.Fatalf("分组小标题应去重:\n%s", got)
	}
}

func TestRenderFilter(t *testing.T) {
	rm := renderFixture(t)
	got := Render(rm, RenderOpts{Title: "T", BranchIdx: []int{1}})
	if strings.Contains(got, "语言基础") {
		t.Fatalf("过滤后不应含语言基础:\n%s", got)
	}
	if !strings.Contains(got, "atomic") {
		t.Fatalf("应含并发主干:\n%s", got)
	}
}

func TestRenderColor(t *testing.T) {
	rm := renderFixture(t)
	got := Render(rm, RenderOpts{Title: "T", Color: func(status, line string) string {
		return "<" + status + ">" + line
	}})
	// 颜色回调包装整行（含树形前缀），只断言包装发生
	if !strings.Contains(got, "<done>│") {
		t.Fatalf("颜色回调未生效:\n%s", got)
	}
}

func TestRenderEmptyBranch(t *testing.T) {
	rm, err := Parse([]byte("version = 2\n\n[[branch]]\ntitle = \"算法与面试\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := Render(rm, RenderOpts{Title: "T"})
	if !strings.Contains(got, "└─ 算法与面试\n") {
		t.Fatalf("空主干不应挂进度:\n%s", got)
	}
}
