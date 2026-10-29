package edit

import "testing"

func TestOsc52(t *testing.T) {
	got := osc52("hi")
	want := "\x1b]52;c;aGk=\x07"
	if got != want {
		t.Fatalf("osc52: %q", got)
	}
	if osc52("") != "\x1b]52;c;\x07" {
		t.Fatal("空串应仍成合法序列")
	}
}
