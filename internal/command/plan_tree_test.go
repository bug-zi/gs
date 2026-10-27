package command

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
	"github.com/bugzi/gs/internal/workspace"
)

const planV2Fixture = `version = 2

[[branch]]
title = "语言基础"

  [[branch.point]]
  title = "变量与常量"
  group = "语法核心"

  [[branch.point]]
  title = "map 底层与负载因子"
  group = "语法核心"

[[branch]]
title = "并发编程"

  [[branch.point]]
  title = "atomic"
  group = "同步原语"
`

func writeRoadmapV2(t *testing.T, ws *workspace.Workspace) {
	t.Helper()
	if err := os.WriteFile(ws.RoadmapFile(), []byte(planV2Fixture), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rmV2(t *testing.T) *roadmap.Roadmap {
	t.Helper()
	rm, err := roadmap.Parse([]byte(planV2Fixture))
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

func TestPlanTreeTextV2(t *testing.T) {
	st := &State{Breakpoints: []KV{{Key: "下一步", Value: "走读 map 源码。"}}}
	out := planTreeText(rmV2(t), st, "")
	if !strings.Contains(out, "进度 0/3 (0%)") || !strings.Contains(out, "下一步：走读 map 源码。") {
		t.Fatalf("静态树不符:\n%s", out)
	}
	if !strings.Contains(out, "语法核心") {
		t.Fatalf("分组小标题缺失:\n%s", out)
	}
}

func TestPlanTreeTextFilterV2(t *testing.T) {
	out := planTreeText(rmV2(t), nil, "并发")
	if !strings.Contains(out, "atomic") || strings.Contains(out, "变量与常量") {
		t.Fatalf("过滤失败:\n%s", out)
	}
}

// 测试进程 stdin 非终端：浏览器应降级静态输出
func TestRunPlanBrowserDegrades(t *testing.T) {
	ws := wsForTest(t)
	writeRoadmapV2(t, ws)
	var out bytes.Buffer
	if err := newPlan().Run(&Ctx{WS: ws, Out: &out, Args: []string{"并发"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Go 学习路线图") {
		t.Fatalf("降级应输出静态树:\n%s", out.String())
	}
}

func TestRunPlanMissingRoadmap(t *testing.T) {
	ws := wsForTest(t)
	err := newPlan().Run(&Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"done", "x"}})
	if err == nil || !strings.Contains(err.Error(), "尚未生成") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPlanSetStatusUnique(t *testing.T) {
	ws := wsForTest(t)
	writeRoadmapV2(t, ws)
	var out bytes.Buffer
	if err := newPlan().Run(&Ctx{WS: ws, Out: &out, Args: []string{"doing", "atomic"}}); err != nil {
		t.Fatal(err)
	}
	rm, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	m := rm.MatchPoints("atomic")
	if len(m) != 1 || rm.PointAt(m[0]).Status != "doing" {
		t.Fatalf("落盘状态不符: %+v", rm.PointAt(m[0]))
	}
	if !strings.Contains(out.String(), "已标记 [doing]") {
		t.Fatalf("输出不符: %s", out.String())
	}
}

func TestRunPlanSetStatusZeroMatch(t *testing.T) {
	ws := wsForTest(t)
	writeRoadmapV2(t, ws)
	err := newPlan().Run(&Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"done", "不存在"}})
	if err == nil || !strings.Contains(err.Error(), "没有匹配") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunPlanAddPoint(t *testing.T) {
	ws := wsForTest(t)
	writeRoadmapV2(t, ws)
	if err := newPlan().Run(&Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"add", "point", "并发", "sync.Cond", "同步原语"}}); err != nil {
		t.Fatal(err)
	}
	rm, err := roadmap.Load(ws.RoadmapFile())
	if err != nil {
		t.Fatal(err)
	}
	m := rm.MatchPoints("sync.Cond")
	if len(m) != 1 || rm.PointAt(m[0]).Group != "同步原语" {
		t.Fatalf("新知识点不符: %+v", rm.MatchPoints("sync.Cond"))
	}
	// v2 全局唯一：跨主干重名报错
	err = newPlan().Run(&Ctx{WS: ws, Out: &bytes.Buffer{}, Args: []string{"add", "point", "语言基础", "atomic"}})
	if err == nil || !strings.Contains(err.Error(), "唯一") {
		t.Fatalf("err = %v", err)
	}
}

func TestLocatePointUnique(t *testing.T) {
	rm := rmV2(t)
	ref, err := locatePoint(&Ctx{Out: &bytes.Buffer{}}, rm, "atomic")
	if err != nil || ref != (roadmap.PointRef{Branch: 1, Point: 0}) {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	if _, err := locatePoint(&Ctx{Out: &bytes.Buffer{}}, rm, "不存在"); err == nil {
		t.Fatal("零匹配应报错")
	}
}
