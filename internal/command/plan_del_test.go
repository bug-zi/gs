package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/index"
	"github.com/bugzi/gs/internal/roadmap"
)

func TestApplyPointDelete(t *testing.T) {
	ws, ctx := tierDelFixture(t)
	rm, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	ref := roadmap.PointRef{Branch: 0, Point: 0}
	river := []string{delRiverRel}
	if err := applyPointDelete(ctx, rm, ref, river); err != nil {
		t.Fatal(err)
	}
	// 知识点已删除
	rm2, _ := roadmap.Load(ws.RoadmapFile())
	if len(rm2.Branches[0].Points) != 0 {
		t.Fatal("知识点未删除")
	}
	// river 收容 99-未挂载
	if _, err := os.Stat(filepath.Join(ws.Root, "Study", "river", "99-未挂载", "01-首篇.md")); err != nil {
		t.Fatalf("应收容到 99-未挂载: %v", err)
	}
	// 出处链改写指向新位置
	data, _ := os.ReadFile(filepath.Join(ws.Root, "Study", "spring", "select.md"))
	if !strings.Contains(string(data), "> 出处：Study/river/99-未挂载/01-首篇.md") {
		t.Fatal("出处链未改写")
	}
	// topics 路径改写
	ix, _ := index.Load(ws.IndexPath())
	if n := ix.CountPaths(map[string]bool{delRiverRel: true}); n != 0 {
		t.Fatal("topics 旧路径未改写")
	}
}
