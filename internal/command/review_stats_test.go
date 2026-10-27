package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatherTierStats(t *testing.T) {
	ws := wsForTest(t)
	mk := func(rel, content string) {
		p := filepath.Join(ws.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("Study/sea/clip/已摘录.md", "# a\n")
	mk("Study/sea/clip/未摘录.md", "# b\n")
	mk("Study/river/要点.md", "# r\n\n> 出处：Study/sea/clip/已摘录.md\n\n正文\n")
	mk("Study/spring/我的理解.md", "# s\n\n> 出处：Study/river/要点.md\n\n正文\n")

	st := GatherTierStats(ws)
	if st.SeaTotal != 2 || len(st.SeaUncited) != 1 || st.SeaUncited[0] != "Study/sea/clip/未摘录.md" {
		t.Fatalf("sea: total=%d uncited=%v", st.SeaTotal, st.SeaUncited)
	}
	if st.RiverTotal != 1 || len(st.RiverUncited) != 0 {
		t.Fatalf("river: total=%d uncited=%v", st.RiverTotal, st.RiverUncited)
	}
	if st.SpringTotal != 1 {
		t.Fatalf("spring total = %d", st.SpringTotal)
	}
}

func TestRenderTierStats(t *testing.T) {
	s := renderTierStats(TierStats{
		SeaTotal: 2, SeaUncited: []string{"Study/sea/clip/未摘录.md"},
		RiverTotal: 1, SpringTotal: 0,
	})
	if !strings.Contains(s, "大海 2") || !strings.Contains(s, "未摘录.md") || !strings.Contains(s, "泉水 0") {
		t.Fatalf("render = %s", s)
	}
}
