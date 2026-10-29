package planview

import (
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

func delFixture(t *testing.T) (*Model, *Hooks, *[]roadmap.FileMove, *bool) {
	t.Helper()
	rm, err := roadmap.Parse([]byte("version = 2\n\n" +
		"[[branch]]\ntitle = \"并发\"\n\n" +
		"  [[branch.point]]\n  title = \"select\"\n" +
		"  river = [\"Study/river/01-并发/01-s.md\"]\n" +
		"  spring = \"Study/spring/select.md\"\n\n" +
		"[[branch]]\ntitle = \"语言\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	m := &Model{RM: rm, Root: t.TempDir(), View: viewGlobal, Expanded: map[int]bool{0: true}}
	var moves []roadmap.FileMove
	saved := false
	h := &Hooks{
		Confirm:    func(string) bool { return true },
		Save:       func(rm *roadmap.Roadmap) error { saved = true; return nil },
		ApplyMoves: func(ms []roadmap.FileMove) error { moves = ms; return nil },
		Unbound:    func(Tier) []string { return nil },
	}
	return m, h, &moves, &saved
}

// cursorTo 把光标移到首个匹配行
func cursorTo(t *testing.T, m *Model, kind rowKind, bi, pi int) {
	t.Helper()
	for i, r := range m.Rows() {
		if r.Kind == kind && r.Branch == bi && r.Point == pi {
			m.Cursor = i
			return
		}
	}
	t.Fatalf("找不到目标行 kind=%v branch=%d point=%d", kind, bi, pi)
}

func TestDoDeleteUnbindsAndCollects(t *testing.T) {
	m, h, moves, saved := delFixture(t)
	cursorTo(t, m, rowPoint, 0, 0)
	doDelete(m, *h)
	if !*saved {
		t.Fatal("应已落盘")
	}
	if len(m.RM.Branches[0].Points) != 0 {
		t.Fatal("知识点应被删除")
	}
	if len(*moves) != 1 {
		t.Fatalf("应有一条收容迁移，got %v", *moves)
	}
	want := roadmap.FileMove{From: "Study/river/01-并发/01-s.md", To: "Study/river/99-未挂载/01-s.md"}
	if (*moves)[0] != want {
		t.Fatalf("moves[0] = %v, want %v", (*moves)[0], want)
	}
}

func TestDoDeleteCancelled(t *testing.T) {
	m, h, _, saved := delFixture(t)
	h.Confirm = func(string) bool { return false }
	cursorTo(t, m, rowPoint, 0, 0)
	doDelete(m, *h)
	if *saved {
		t.Fatal("取消不应落盘")
	}
	if len(m.RM.Branches[0].Points) != 1 {
		t.Fatal("取消不应删点")
	}
}

func TestDoDeleteBranch(t *testing.T) {
	m, h, _, saved := delFixture(t)
	var trashed []string
	h.TrashRel = func(rel string) error { trashed = append(trashed, rel); return nil }
	h.SelectOne = func(_ string, opts []string) (int, bool) { return 0, true }
	cursorTo(t, m, rowBranchHead, 1, 0)
	doMenu(m, *h)
	if !*saved {
		t.Fatal("应已落盘")
	}
	if len(m.RM.Branches) != 1 {
		t.Fatal("主干应被删除")
	}
	if len(trashed) != 1 || trashed[0] != "Study/river/02-语言" {
		t.Fatalf("空板块目录应入回收站，got %v", trashed)
	}
}

func TestDoDeleteBranchNonEmptyRejected(t *testing.T) {
	m, h, _, saved := delFixture(t)
	h.SelectOne = func(_ string, opts []string) (int, bool) { return 0, true }
	cursorTo(t, m, rowBranchHead, 0, 0)
	doMenu(m, *h)
	if *saved {
		t.Fatal("非空主干不应落盘")
	}
	if m.Err == "" {
		t.Fatal("应提示仍有知识点")
	}
}
