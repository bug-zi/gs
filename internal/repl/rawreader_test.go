package repl

import "testing"

func TestWrapSel(t *testing.T) {
	cases := []struct {
		name       string
		sel, delta int
		n          int
		want       int
	}{
		{"未选中上移选首行", -1, -1, 5, 0},
		{"首行上移回绕末行", 0, -1, 5, 4},
		{"中间上移", 2, -1, 5, 1},
		{"未选中下移选首行", -1, 1, 5, 0},
		{"末行下移回绕首行", 4, 1, 5, 0},
		{"中间下移", 2, 1, 5, 3},
		{"空面板维持未选中", -1, -1, 0, -1},
	}
	for _, tc := range cases {
		if got := wrapSel(tc.sel, tc.delta, tc.n); got != tc.want {
			t.Errorf("%s: wrapSel(%d,%d,%d) = %d, want %d", tc.name, tc.sel, tc.delta, tc.n, got, tc.want)
		}
	}
}

func TestDelayedHintText(t *testing.T) {
	cases := []struct {
		name               string
		single, singleHint string
		want               string
	}{
		{"有提示", "sea", "<关键词>", "sea <关键词>"},
		{"命令无提示不显示", "help", "", ""},
		{"空命令", "", "<关键词>", ""},
	}
	for _, tc := range cases {
		if got := delayedHintText(tc.single, tc.singleHint); got != tc.want {
			t.Errorf("%s: delayedHintText(%q,%q) = %q, want %q", tc.name, tc.single, tc.singleHint, got, tc.want)
		}
	}
}
