package repl

import "testing"

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"logs", "logs", 0},
		{"logs", "log", 1},
		{"notes", "note", 1},
		{"plan", "plans", 1},
		{"abc", "xyz", 3},
		{"", "ab", 2},
	}
	for _, c := range cases {
		if got := Levenshtein(c.a, c.b); got != c.want {
			t.Fatalf("Levenshtein(%q,%q) = %d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	names := []string{"logs", "log", "notes", "plan", "search", "help", "exit"}
	if got := suggest(names, "logz"); got != "logs" {
		t.Fatalf("got %q", got)
	}
	if got := suggest(names, "完全不相关的长命令"); got != "" {
		t.Fatalf("远距命令不应建议, got %q", got)
	}
}
