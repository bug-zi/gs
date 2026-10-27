package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitPages(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		h     int
		want  [][]string
	}{
		{"空内容", nil, 5, nil},
		{"单页不足额", []string{"a", "b"}, 5, [][]string{{"a", "b"}}},
		{"恰好整除", []string{"a", "b", "c", "d"}, 2, [][]string{{"a", "b"}, {"c", "d"}}},
		{"末页余行", []string{"a", "b", "c"}, 2, [][]string{{"a", "b"}, {"c"}}},
		{"高度0不分页", []string{"a", "b"}, 0, [][]string{{"a", "b"}}},
	}
	for _, c := range cases {
		got := SplitPages(c.lines, c.h)
		if len(got) != len(c.want) {
			t.Fatalf("%s: 页数 got %d want %d (%v)", c.name, len(got), len(c.want), got)
		}
		for i := range got {
			if strings.Join(got[i], "|") != strings.Join(c.want[i], "|") {
				t.Fatalf("%s: 第%d页 got %v want %v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// keyFeed 顺序吐出 keys，耗尽后返回 EOF
func keyFeed(keys []string) func() (string, bool) {
	i := 0
	return func() (string, bool) {
		if i >= len(keys) {
			return "", false
		}
		k := keys[i]
		i++
		return k, true
	}
}

func TestPagedPrint(t *testing.T) {
	content := "l1\nl2\nl3\nl4\nl5\nl6"
	cases := []struct {
		name string
		keys []string
	}{
		{"逐页翻完", []string{"", ""}},
		{"b回翻后翻完", []string{"", "b", "", ""}},
		{"其他键跳末", []string{"x"}},
		{"EOF补齐剩余页", nil},
	}
	for _, c := range cases {
		var out bytes.Buffer
		PagedPrint(&out, keyFeed(c.keys), content, 2)
		got := out.String()
		for _, want := range []string{"l1", "l6"} {
			if !strings.Contains(got, want) {
				t.Fatalf("%s: 输出应含 %s, got %q", c.name, want, got)
			}
		}
	}
}

func TestPagedPrintSinglePage(t *testing.T) {
	var out bytes.Buffer
	PagedPrint(&out, func() (string, bool) { t.Fatal("单页不应读键"); return "", false }, "a\nb", 5)
	if out.String() != "a\nb\n" {
		t.Fatalf("单页应原样一次打完: %q", out.String())
	}
}
