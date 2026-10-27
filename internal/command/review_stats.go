package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/workspace"
)

// TierStats 三层流动统计：出处行反查「未摘录 / 待改写」
type TierStats struct {
	SeaTotal     int
	SeaUncited   []string // 未被任何 river 条目引用的大海条目
	RiverTotal   int
	RiverUncited []string // 未被任何 spring 条目引用的河流条目
	SpringTotal  int
}

// citedPaths 扫描某层全部条目的「> 出处：X」行，返回被引用路径集合
func citedPaths(ws *workspace.Workspace, tier string) map[string]bool {
	cited := map[string]bool{}
	files, err := ListTierFiles(ws, tier)
	if err != nil {
		return cited
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(ws.Root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "> 出处：") {
				cited[strings.TrimSpace(strings.TrimPrefix(t, "> 出处："))] = true
			}
		}
	}
	return cited
}

// GatherTierStats 确定性统计三层流动状态
func GatherTierStats(ws *workspace.Workspace) TierStats {
	var st TierStats
	riverCited := citedPaths(ws, "river")
	seaFiles, _ := ListTierFiles(ws, "sea")
	for _, f := range seaFiles {
		st.SeaTotal++
		if !riverCited[f] {
			st.SeaUncited = append(st.SeaUncited, f)
		}
	}
	springCited := citedPaths(ws, "spring")
	riverFiles, _ := ListTierFiles(ws, "river")
	for _, f := range riverFiles {
		st.RiverTotal++
		if !springCited[f] {
			st.RiverUncited = append(st.RiverUncited, f)
		}
	}
	springFiles, _ := ListTierFiles(ws, "spring")
	st.SpringTotal = len(springFiles)
	return st
}

// renderTierStats 渲染三层概览（本地展示与 AI 提示词共用）
func renderTierStats(st TierStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "三层概览：大海 %d（未摘录 %d）· 河流 %d（待改写 %d）· 泉水 %d\n",
		st.SeaTotal, len(st.SeaUncited), st.RiverTotal, len(st.RiverUncited), st.SpringTotal)
	if len(st.SeaUncited) > 0 {
		b.WriteString("\n大海中尚未摘录的条目：\n")
		for _, f := range st.SeaUncited {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	if len(st.RiverUncited) > 0 {
		b.WriteString("\n河流中尚未改写的条目：\n")
		for _, f := range st.RiverUncited {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	return b.String()
}
