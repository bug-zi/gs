package planview

import (
	"errors"
	"testing"

	"github.com/bugzi/gs/internal/roadmap"
)

// flowCapture 记录 hook 调用与落盘次数
type flowCapture struct {
	copies []string // "src->dst"
	moves  []string
	chains []string
	saved  int
}

func shipFlowFixture(t *testing.T) (*Model, *Hooks, *flowCapture) {
	t.Helper()
	rm, err := roadmap.Parse([]byte("version = 2\n\n" +
		"[[branch]]\ntitle = \"并发\"\n\n" +
		"  [[branch.point]]\n  title = \"select\"\n" +
		"  ship = [\"Study/ship/1.3.0 函数.md\"]\n" +
		"  sea = [\"Study/sea/1 a.md\", \"Study/sea/2 b.md\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	fc := &flowCapture{}
	h := &Hooks{
		Save:             func(*roadmap.Roadmap) error { fc.saved++; return nil },
		CopyFileRel:      func(src, dst string) error { fc.copies = append(fc.copies, src+"->"+dst); return nil },
		MoveFileRel:      func(src, dst string) error { fc.moves = append(fc.moves, src+"->"+dst); return nil },
		RewriteChainsRel: func(src, dst string) error { fc.chains = append(fc.chains, src+"->"+dst); return nil },
	}
	m := &Model{RM: rm, Root: t.TempDir(), View: viewPoint, Branch: 0, Point: 0,
		Expanded: map[int]bool{}, Exists: func(string) bool { return true }}
	return m, h, fc
}

// cursorToNote 把光标移到指定层的指定笔记行
func cursorToNote(t *testing.T, m *Model, tier Tier, rel string) {
	t.Helper()
	for i, r := range m.Rows() {
		if r.Kind == rowNote && r.Tier == tier && r.Note == rel {
			m.Cursor = i
			return
		}
	}
	t.Fatalf("找不到笔记行 tier=%v rel=%s", tier, rel)
}

func cursorNote(t *testing.T, m *Model) row {
	t.Helper()
	r := m.rowAt(m.Cursor)
	if r == nil {
		t.Fatal("光标越界")
	}
	return *r
}

func TestDoShipCopy(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	cursorToNote(t, m, TierSea, "Study/sea/1 a.md")
	doShipCopy(m, *h)
	if len(fc.copies) != 1 || fc.copies[0] != "Study/sea/1 a.md->Study/ship/1 a.md" {
		t.Fatalf("copies = %v", fc.copies)
	}
	p := m.RM.PointAt(m.curRef())
	if len(p.Sea) != 2 {
		t.Fatalf("sea 绑定应保留，got %v", p.Sea)
	}
	if len(p.Ship) != 2 || p.Ship[1] != "Study/ship/1 a.md" {
		t.Fatalf("Ship = %v", p.Ship)
	}
	if fc.saved != 1 {
		t.Fatal("应已落盘")
	}
	if r := cursorNote(t, m); r.Note != "Study/sea/2 b.md" {
		t.Fatalf("装船后应定位到下一未消费 sea 行，got %s", r.Note)
	}
}

func TestDoShipCopyCursorStaysWhenAllConsumed(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	cursorToNote(t, m, TierSea, "Study/sea/1 a.md")
	doShipCopy(m, *h)
	doShipCopy(m, *h) // 最后一篇：装完无未消费行
	if len(fc.copies) != 2 {
		t.Fatalf("copies = %v", fc.copies)
	}
	if r := cursorNote(t, m); r.Note != "Study/sea/2 b.md" {
		t.Fatalf("全部消费后应留在原地，got %s", r.Note)
	}
}

func TestDoShipCopyNotOnSeaRow(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	cursorToNote(t, m, TierShip, "Study/ship/1.3.0 函数.md")
	doShipCopy(m, *h)
	if m.Err == "" || len(fc.copies) != 0 || fc.saved != 0 {
		t.Fatalf("非 sea 行应拒绝：err=%q copies=%v", m.Err, fc.copies)
	}
}

func TestDoShipCopyNilHook(t *testing.T) {
	m, h, _ := shipFlowFixture(t)
	h.CopyFileRel = nil
	cursorToNote(t, m, TierSea, "Study/sea/1 a.md")
	doShipCopy(m, *h)
	if m.Err != "该操作不可用" {
		t.Fatalf("err = %q", m.Err)
	}
}

func TestDoShipCopyTargetExists(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	h.CopyFileRel = func(string, string) error { return errors.New("目标已存在: Study/ship/1 a.md") }
	cursorToNote(t, m, TierSea, "Study/sea/1 a.md")
	doShipCopy(m, *h)
	if m.Err == "" || fc.saved != 0 {
		t.Fatalf("目标已存在应报错不落盘：err=%q", m.Err)
	}
	if p := m.RM.PointAt(m.curRef()); len(p.Ship) != 1 {
		t.Fatalf("绑定不应变化，Ship = %v", p.Ship)
	}
}

func TestDoShipCopyMissingFile(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	m.Exists = func(string) bool { return false }
	cursorToNote(t, m, TierSea, "Study/sea/1 a.md")
	doShipCopy(m, *h)
	if m.Err == "" || len(fc.copies) != 0 {
		t.Fatalf("失效文件应拒绝：err=%q", m.Err)
	}
}

func TestDoShipRiver(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	cursorToNote(t, m, TierShip, "Study/ship/1.3.0 函数.md")
	doShipRiver(m, *h)
	want := "Study/river/01-并发/01-函数.md"
	if len(fc.moves) != 1 || fc.moves[0] != "Study/ship/1.3.0 函数.md->"+want {
		t.Fatalf("moves = %v", fc.moves)
	}
	if len(fc.chains) != 1 || fc.chains[0] != "Study/ship/1.3.0 函数.md->"+want {
		t.Fatalf("chains = %v", fc.chains)
	}
	p := m.RM.PointAt(m.curRef())
	if len(p.Ship) != 0 || len(p.River) != 1 || p.River[0] != want {
		t.Fatalf("Ship=%v River=%v", p.Ship, p.River)
	}
	if fc.saved != 1 {
		t.Fatal("应已落盘")
	}
}

func TestDoShipRiverDupSeq(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	m.RM.PointAt(m.curRef()).River = []string{"Study/river/01-并发/01-函数.md"}
	cursorToNote(t, m, TierShip, "Study/ship/1.3.0 函数.md")
	doShipRiver(m, *h)
	want := "Study/river/01-并发/01.1-函数.md"
	if fc.moves[len(fc.moves)-1] != "Study/ship/1.3.0 函数.md->"+want {
		t.Fatalf("moves = %v", fc.moves)
	}
}

func TestDoShipRiverDupAdvanceOnConflict(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	m.RM.PointAt(m.curRef()).River = []string{"Study/river/01-并发/01-函数.md"}
	calls := 0
	h.MoveFileRel = func(src, dst string) error {
		calls++
		fc.moves = append(fc.moves, src+"->"+dst)
		if calls == 1 {
			return errors.New("目标已存在: " + dst)
		}
		return nil
	}
	cursorToNote(t, m, TierShip, "Study/ship/1.3.0 函数.md")
	doShipRiver(m, *h)
	want := "Study/river/01-并发/01.2-函数.md"
	if fc.moves[len(fc.moves)-1] != "Study/ship/1.3.0 函数.md->"+want {
		t.Fatalf("moves = %v", fc.moves)
	}
	if m.Err != "" {
		t.Fatalf("顺延重试后不应报错：%q", m.Err)
	}
}

func TestDoShipRiverChainFailKeepsMove(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	h.RewriteChainsRel = func(string, string) error { return errors.New("boom") }
	cursorToNote(t, m, TierShip, "Study/ship/1.3.0 函数.md")
	doShipRiver(m, *h)
	if m.Err == "" {
		t.Fatal("出处链失败应提示")
	}
	p := m.RM.PointAt(m.curRef())
	if len(p.Ship) != 0 || len(p.River) != 1 || fc.saved != 1 {
		t.Fatalf("迁移与落盘不应回滚：Ship=%v River=%v saved=%d", p.Ship, p.River, fc.saved)
	}
}

func TestDoShipRiverNotOnShipRow(t *testing.T) {
	m, h, fc := shipFlowFixture(t)
	cursorToNote(t, m, TierSea, "Study/sea/1 a.md")
	doShipRiver(m, *h)
	if m.Err == "" || len(fc.moves) != 0 {
		t.Fatalf("非 ship 行应拒绝：err=%q", m.Err)
	}
}

func TestDoShipRiverNilHook(t *testing.T) {
	m, h, _ := shipFlowFixture(t)
	h.MoveFileRel = nil
	cursorToNote(t, m, TierShip, "Study/ship/1.3.0 函数.md")
	doShipRiver(m, *h)
	if m.Err != "该操作不可用" {
		t.Fatalf("err = %q", m.Err)
	}
}
