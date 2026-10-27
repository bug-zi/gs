package roadmap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validTOML = `version = 2

[[branch]]
title = "并发编程"

  [[branch.point]]
  title = "select"
  group = "基础原语"
  status = "doing"
  tags = ["高频"]
  spring = "Study/spring/select.md"
  code = "Study/code/select.go"
  river = ["Study/river/select语义.md"]
  sea = ["Study/sea/ai/2026-10-26-1430.md"]

  [[branch.point]]
  title = "atomic"
`

func TestParseValid(t *testing.T) {
	rm, err := Parse([]byte(validTOML))
	if err != nil {
		t.Fatal(err)
	}
	if len(rm.Branches) != 1 || len(rm.Branches[0].Points) != 2 {
		t.Fatalf("结构不符: %+v", rm.Branches)
	}
	p := rm.Branches[0].Points[0]
	if p.Group != "基础原语" || p.Status != "doing" || len(p.Tags) != 1 || p.Tags[0] != "高频" {
		t.Fatalf("p0 = %+v", p)
	}
	if p.Spring != "Study/spring/select.md" || len(p.River) != 1 || len(p.Sea) != 1 {
		t.Fatalf("绑定字段不符: %+v", p)
	}
	if p.Code != "Study/code/select.go" {
		t.Fatalf("code 绑定字段不符: %+v", p)
	}
	if rm.Branches[0].Points[1].Status != StatusTodo {
		t.Fatalf("缺省状态应归一化为 todo: %+v", rm.Branches[0].Points[1])
	}
}

func TestParseRejectsV1(t *testing.T) {
	_, err := Parse([]byte("version = 1\n[[branch]]\ntitle = \"b\"\n[[branch.topic]]\ntitle = \"t\"\n[[branch.topic.node]]\ntitle = \"n\"\n"))
	if err == nil || !strings.Contains(err.Error(), "v1") || !strings.Contains(err.Error(), "v2") {
		t.Fatalf("v1 应报废弃错: %v", err)
	}
}

func TestParseGlobalDupTitle(t *testing.T) {
	_, err := Parse([]byte("version = 2\n[[branch]]\ntitle = \"A\"\n[[branch.point]]\ntitle = \"x\"\n[[branch]]\ntitle = \"B\"\n[[branch.point]]\ntitle = \"x\"\n"))
	if err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("跨主干重名应报错: %v", err)
	}
}

func TestParseInvalidStatus(t *testing.T) {
	_, err := Parse([]byte("version = 2\n[[branch]]\ntitle = \"b\"\n[[branch.point]]\ntitle = \"n\"\nstatus = \"paused\"\n"))
	if err == nil || !strings.Contains(err.Error(), "paused") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseSyntaxErrorHasPosition(t *testing.T) {
	_, err := Parse([]byte("version = 2\n[[branch]\ntitle = x\n"))
	if err == nil {
		t.Fatal("应报错")
	}
	if !strings.Contains(err.Error(), "line") {
		t.Fatalf("缺行列信息: %v", err)
	}
}

func TestParseNoBranch(t *testing.T) {
	if _, err := Parse([]byte("version = 2\n")); err == nil {
		t.Fatal("无主干应报错")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	rm, err := Parse([]byte(validTOML))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "roadmap.toml")
	if err := Save(path, rm); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".gs-tmp"); !os.IsNotExist(err) {
		t.Fatal("不应残留临时文件")
	}
	rm2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if rm2.Branches[0].Points[0].Spring != "Study/spring/select.md" {
		t.Fatalf("回读不符: %+v", rm2.Branches[0].Points[0])
	}
	if rm2.Branches[0].Points[0].Code != "Study/code/select.go" {
		t.Fatalf("code 回读不符: %+v", rm2.Branches[0].Points[0])
	}
}
