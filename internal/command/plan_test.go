package command

import (
	"strings"
	"testing"
)

const stateFixture = `# 学习状态

更新时间：2026-10-20（东八区）。本文件记录执行断点与已确认变更。

## 当前断点

- 当前优先模块：KillCourse 学习总览核对。
- 下一步：用户核对总览后设置入口。
- 当前疑点：版本提示误报。

## 已确认的顺序与节点

1. KillCourse 速成
2. Go 核心补缺

## 其他

无关内容。
`

func TestParseState(t *testing.T) {
	st := ParseState(stateFixture)
	if len(st.Breakpoints) != 3 {
		t.Fatalf("Breakpoints = %+v", st.Breakpoints)
	}
	if st.Breakpoints[0].Key != "当前优先模块" || st.Breakpoints[0].Value != "KillCourse 学习总览核对。" {
		t.Fatalf("首条 = %+v", st.Breakpoints[0])
	}
	if len(st.Phases) != 2 || st.Phases[0] != "KillCourse 速成" {
		t.Fatalf("Phases = %+v", st.Phases)
	}
}

func TestUpdateNextReplace(t *testing.T) {
	out, err := UpdateNext(stateFixture, "开始登录模块走读", "2026-10-24")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- 下一步：开始登录模块走读") {
		t.Fatalf("未替换: %s", out)
	}
	if strings.Contains(out, "用户核对总览后设置入口") {
		t.Fatal("旧内容应被替换")
	}
	if !strings.Contains(out, "更新时间：2026-10-24") {
		t.Fatal("更新时间应刷新")
	}
	if strings.Count(out, "## 当前断点") != 1 {
		t.Fatal("结构不应被破坏")
	}
}

func TestUpdateNextAppend(t *testing.T) {
	in := strings.ReplaceAll(stateFixture, "- 下一步：用户核对总览后设置入口。\n", "")
	out, err := UpdateNext(in, "新建断点", "2026-10-24")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- 下一步：新建断点") {
		t.Fatalf("未追加: %s", out)
	}
}
