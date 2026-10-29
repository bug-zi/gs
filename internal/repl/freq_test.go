package repl

import (
	"reflect"
	"testing"

	"github.com/bugzi/gs/internal/command"
)

func TestFreqFromHistory(t *testing.T) {
	cmds := []command.Command{
		{Name: "logs", Alias: []string{"log"}},
		{Name: "sea"},
		{Name: "help", Alias: []string{"h"}},
	}
	items := []string{
		"/sea foo",
		"/sea bar",
		"/log x",     // 别名归一 logs
		"sea 裸行忽略",
		"/",          // 空命令忽略
		"/HELP",      // 大小写归一
		"/unknown y", // 未知命令忽略
		"/sea",       // 无参数也计数
	}
	freq := freqFromHistory(items, cmds)
	if freq["sea"] != 3 || freq["logs"] != 1 || freq["help"] != 1 {
		t.Fatalf("freq = %v, want sea:3 logs:1 help:1", freq)
	}
}

func TestSortCmdsByFreq(t *testing.T) {
	cmds := []command.Command{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}
	freq := map[string]int{"b": 3, "d": 1}
	got := sortCmdsByFreq(cmds, freq)
	want := []string{"b", "d", "a", "c"} // 降序；零次 a/c 稳定保持原序
	names := make([]string, len(got))
	for i, c := range got {
		names[i] = c.Name
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("sortCmdsByFreq = %v, want %v", names, want)
	}
}

func TestSortCmdsByFreqPins(t *testing.T) {
	cmds := []command.Command{
		{Name: "sea"}, {Name: "exit"}, {Name: "help"}, {Name: "logs"}, {Name: "plan"},
	}
	freq := map[string]int{"sea": 9, "help": 5} // logs/exit 零次也不影响档位
	got := sortCmdsByFreq(cmds, freq)
	names := make([]string, len(got))
	for i, c := range got {
		names[i] = c.Name
	}
	want := []string{"logs", "sea", "help", "plan", "exit"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("sortCmdsByFreq = %v, want %v", names, want)
	}
}
