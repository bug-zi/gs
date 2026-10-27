package edit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldCliEdit(t *testing.T) {
	cases := []struct {
		name string
		data string
		want bool
	}{
		{"空文件", "", true},
		{"单行无换行", "abc", true},
		{"整20行", strings.Repeat("a\n", 20), true},
		{"20行末行无换行", strings.Repeat("a\n", 19) + "a", true},
		{"21行", strings.Repeat("a\n", 21), false},
		{"21行末行无换行", strings.Repeat("a\n", 20) + "a", false},
	}
	for _, c := range cases {
		if got := ShouldCliEdit([]byte(c.data)); got != c.want {
			t.Fatalf("%s: ShouldCliEdit = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSaveFileKeepsEOL(t *testing.T) {
	dir := t.TempDir()
	pCRLF := filepath.Join(dir, "crlf.md")
	if err := saveFile(pCRLF, "- a\n- b\n", true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(pCRLF)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "- a\r\n- b\r\n" {
		t.Fatalf("CRLF 风格应写回 CRLF, got %q", got)
	}
	pLF := filepath.Join(dir, "lf.md")
	if err := saveFile(pLF, "- a\n- b\n", false); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(pLF)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "- a\n- b\n" {
		t.Fatalf("LF 风格应写回 LF, got %q", got)
	}
}

// newEditor 识别 CRLF 并在缓冲内归一为 \n
func TestEditorDetectsCRLF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.md")
	e := newEditor(p, []byte("a\r\nb\r\n"))
	if !e.crlf {
		t.Fatal("应识别 CRLF")
	}
	if v := e.buf.Value(); v != "a\nb\n" {
		t.Fatalf("载入应归一为 LF: %q", v)
	}
}
