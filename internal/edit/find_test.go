package edit

import "testing"

func TestFindMatches(t *testing.T) {
	lines := [][]rune{[]rune("Go语言 go GoGo"), []rune("golang")}
	ms := findMatches(lines, "go")
	want := []match{{0, 0, 2}, {0, 5, 7}, {0, 8, 10}, {0, 10, 12}, {1, 0, 2}}
	if len(ms) != len(want) {
		t.Fatalf("匹配数: %v", ms)
	}
	for i := range want {
		if ms[i] != want[i] {
			t.Fatalf("第 %d 处: got %v want %v", i, ms[i], want[i])
		}
	}
	if findMatches(lines, "") != nil {
		t.Fatal("空词应返回 nil")
	}
	if findMatches(lines, "xyz") != nil {
		t.Fatal("无匹配应返回 nil")
	}
}
