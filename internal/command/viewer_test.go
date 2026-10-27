package command

import "testing"

func TestClampView(t *testing.T) {
	cases := []struct {
		name                string
		total, off, h, want int
	}{
		{"内容不足一页回到顶", 10, 3, 20, 0},
		{"范围内不动", 100, 30, 20, 30},
		{"越底夹回", 100, 95, 20, 80},
		{"越顶夹回", 100, -5, 20, 0},
		{"恰好到底", 100, 80, 20, 80},
		{"空文档", 0, 5, 20, 0},
		{"页高为1", 5, 10, 1, 4},
	}
	for _, c := range cases {
		if got := clampView(c.total, c.off, c.h); got != c.want {
			t.Errorf("%s: clampView(%d,%d,%d) = %d，want %d",
				c.name, c.total, c.off, c.h, got, c.want)
		}
	}
}
