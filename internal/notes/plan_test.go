package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func planFixture(t *testing.T) (string, []Note) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "Study", "river", "01-基础", "03-切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片\n\n原文\n"), 0o644)
	ns := []Note{{Time: ts(30), Text: "甲"}, {Time: ts(40), Text: "乙"}}
	if err := Save(root, ns); err != nil {
		t.Fatal(err)
	}
	return root, ns
}

func TestParsePlanJSONOK(t *testing.T) {
	_, ns := planFixture(t)
	plan, err := ParsePlanJSON(`{"placements":[{"note":2,"target":"Study/river/01-基础/03-切片.md","content":"# 切片\n\n原文\n\n乙\n"}]}`, ns)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Placements) != 1 || plan.Placements[0].NoteIdx != 1 || plan.Placements[0].Target != "Study/river/01-基础/03-切片.md" {
		t.Fatalf("解析结果不对: %+v", plan.Placements)
	}
	if strings.Contains(plan.Placements[0].NewContent, "\\n") {
		t.Fatalf("\\n 应已解码为真实换行: %q", plan.Placements[0].NewContent)
	}
}

func TestParsePlanJSONInvalid(t *testing.T) {
	_, ns := planFixture(t)
	cases := []struct {
		name, json string
	}{
		{"坏JSON", `{"placements":`},
		{"序号越界", `{"placements":[{"note":3,"target":"a.md","content":"x"}]}`},
		{"序号为零", `{"placements":[{"note":0,"target":"a.md","content":"x"}]}`},
		{"空content", `{"placements":[{"note":1,"target":"a.md","content":"  "}]}`},
		{"空target", `{"placements":[{"note":1,"target":"  ","content":"x"}]}`},
		{"路径穿越", `{"placements":[{"note":1,"target":"../x.md","content":"x"}]}`},
		{"绝对路径", `{"placements":[{"note":1,"target":"C:/x.md","content":"x"}]}`},
	}
	for _, c := range cases {
		if _, err := ParsePlanJSON(c.json, ns); err == nil {
			t.Fatalf("%s 应报错", c.name)
		}
	}
}

func TestApply(t *testing.T) {
	root, ns := planFixture(t)
	plan := Plan{Placements: []Placement{{
		NoteIdx:    0,
		Target:     "Study/river/01-基础/03-切片.md",
		NewContent: "# 切片\n\n原文\n\n融入甲\n",
	}}}
	if err := Apply(root, plan, ns); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "river", "01-基础", "03-切片.md"))
	if string(data) != "# 切片\n\n原文\n\n融入甲\n" {
		t.Fatalf("目标未更新: %q", data)
	}
	rest, err := Load(root)
	if err != nil || len(rest) != 1 || rest[0].Text != "乙" {
		t.Fatalf("已归位便签未移除: %+v %v", rest, err)
	}
}

func TestApplyTargetFailureKeepsNotes(t *testing.T) {
	root, ns := planFixture(t)
	// 目标目录不存在 → 写失败 → notes.md 必须原样
	plan := Plan{Placements: []Placement{{
		NoteIdx:    0,
		Target:     "Study/river/不存在/xx.md",
		NewContent: "x",
	}}}
	if err := Apply(root, plan, ns); err == nil {
		t.Fatal("目标写失败应报错")
	}
	rest, err := Load(root)
	if err != nil || len(rest) != 2 {
		t.Fatalf("失败时不得动 notes.md: %+v %v", rest, err)
	}
}

func TestApplySameTargetInOrder(t *testing.T) {
	root, ns := planFixture(t)
	// 同目标两个 placement，content 累计，按序应用后者覆盖
	plan := Plan{Placements: []Placement{
		{NoteIdx: 0, Target: "Study/river/01-基础/03-切片.md", NewContent: "# 切片\n\n甲\n"},
		{NoteIdx: 1, Target: "Study/river/01-基础/03-切片.md", NewContent: "# 切片\n\n甲乙\n"},
	}}
	if err := Apply(root, plan, ns); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, "Study", "river", "01-基础", "03-切片.md"))
	if string(data) != "# 切片\n\n甲乙\n" {
		t.Fatalf("应按序落盘为最终 content: %q", data)
	}
	rest, _ := Load(root)
	if len(rest) != 0 {
		t.Fatalf("两条都应移除: %+v", rest)
	}
}
