package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newWS(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"Study/logs", "Study/plan"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &Workspace{Root: root}
}

func TestDetect(t *testing.T) {
	ws := newWS(t)
	deep := filepath.Join(ws.Root, "Study", "Project", "demo")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Detect(deep)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Root != ws.Root {
		t.Fatalf("got %s want %s", got.Root, ws.Root)
	}
	if _, err := Detect(t.TempDir()); err == nil {
		t.Fatal("非工作区目录应报错")
	}
}

func TestLogFileName(t *testing.T) {
	tm := time.Date(2026, 10, 24, 20, 0, 0, 0, time.UTC) // 东八区为 10-25 凌晨
	if got := LogFileName(tm); got != "261025.md" {
		t.Fatalf("got %s", got)
	}
}

func TestParseLogDate(t *testing.T) {
	if _, ok := ParseLogDate("261024.md"); !ok {
		t.Fatal("261024.md 应可解析")
	}
	if _, ok := ParseLogDate("notes.md"); ok {
		t.Fatal("notes.md 不应可解析")
	}
}

func TestListLogsOrder(t *testing.T) {
	ws := newWS(t)
	names := []string{"261015.md", "261024.md", "261021.md", "readme.md"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(ws.LogsDir(), n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dates, err := ws.ListLogs()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"261024", "261021", "261015"}
	for i, w := range want {
		if got := dates[i].Format("060102"); got != w {
			t.Fatalf("第 %d 个日志 got %s want %s", i, got, w)
		}
	}
}

func TestStreak(t *testing.T) {
	ws := newWS(t)
	now := time.Now().In(CST)
	for _, d := range []time.Time{now, now.AddDate(0, 0, -1), now.AddDate(0, 0, -3)} {
		p := filepath.Join(ws.LogsDir(), LogFileName(d))
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := ws.Streak(); got != 2 {
		t.Fatalf("streak got %d want 2", got)
	}
}

func TestTierDirs(t *testing.T) {
	ws := newWS(t)
	cases := map[string]string{
		ws.SeaDir():    filepath.Join(ws.Root, "Study", "sea"),
		ws.RiverDir():  filepath.Join(ws.Root, "Study", "river"),
		ws.SpringDir(): filepath.Join(ws.Root, "Study", "spring"),
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}
}
