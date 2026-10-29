package repl

import (
	"sort"
	"strings"

	"github.com/bugzi/gs/internal/command"
)

// freqFromHistory 统计历史中各命令使用次数：/ 开头行取首词，
// 小写化后经 command.Find 归一别名到正式命令名；非命令行忽略
func freqFromHistory(items []string, cmds []command.Command) map[string]int {
	freq := make(map[string]int)
	for _, line := range items {
		if !strings.HasPrefix(line, "/") {
			continue
		}
		fields := strings.Fields(line[1:])
		if len(fields) == 0 {
			continue
		}
		if cmd := command.Find(cmds, strings.ToLower(fields[0])); cmd != nil {
			freq[cmd.Name]++
		}
	}
	return freq
}

// sortCmdsByFreq 排序：logs 固定第一、exit 固定最后，其余按使用次数稳定降序；
// 并列与零次保持原序（默认序垫底）
func sortCmdsByFreq(cmds []command.Command, freq map[string]int) []command.Command {
	out := append([]command.Command(nil), cmds...)
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := pinRank(out[i].Name), pinRank(out[j].Name)
		if pi != pj {
			return pi < pj
		}
		return freq[out[i].Name] > freq[out[j].Name]
	})
	return out
}

// pinRank 固定档位：logs=0（第一）、exit=2（最后）、其他=1（按频率）
func pinRank(name string) int {
	switch name {
	case "logs":
		return 0
	case "exit":
		return 2
	}
	return 1
}
