package roadmap

import "testing"

func shipFixture() *Roadmap {
	return &Roadmap{Version: Version, Branches: []Branch{{
		Title:  "并发编程",
		Points: []Point{{Title: "select", Status: StatusDoing, Sea: []string{"Study/sea/a.md", "Study/sea/b.md"}}},
	}}}
}

func TestMoveTierBindingSeaToShip(t *testing.T) {
	rm := shipFixture()
	hits := rm.MoveTierBinding("Study/sea/a.md", "Study/ship/a.md", SeaList, ShipList)
	if len(hits) != 1 || hits[0] != (PointRef{0, 0}) {
		t.Fatalf("hits = %v, want [{0 0}]", hits)
	}
	p := rm.PointAt(PointRef{0, 0})
	if len(p.Sea) != 1 || p.Sea[0] != "Study/sea/b.md" {
		t.Fatalf("Sea = %v, want [Study/sea/b.md]", p.Sea)
	}
	if len(p.Ship) != 1 || p.Ship[0] != "Study/ship/a.md" {
		t.Fatalf("Ship = %v, want [Study/ship/a.md]", p.Ship)
	}
}

func TestMoveTierBindingSameListRename(t *testing.T) {
	rm := shipFixture()
	rm.PointAt(PointRef{0, 0}).River = []string{"Study/river/old.md"}
	rm.MoveTierBinding("Study/river/old.md", "Study/river/new.md", RiverList, RiverList)
	if got := rm.PointAt(PointRef{0, 0}).River; len(got) != 1 || got[0] != "Study/river/new.md" {
		t.Fatalf("River = %v, want [Study/river/new.md]", got)
	}
}

func TestMoveTierBindingMissingAndIdempotent(t *testing.T) {
	rm := shipFixture()
	if hits := rm.MoveTierBinding("Study/sea/none.md", "Study/ship/x.md", SeaList, ShipList); len(hits) != 0 {
		t.Fatalf("未命中应返回空，hits = %v", hits)
	}
	rm.MoveTierBinding("Study/sea/a.md", "Study/ship/a.md", SeaList, ShipList)
	if hits := rm.MoveTierBinding("Study/sea/a.md", "Study/ship/a.md", SeaList, ShipList); len(hits) != 0 {
		t.Fatalf("二次迁移应无命中，hits = %v", hits)
	}
}

func TestParseShipField(t *testing.T) {
	data := "version = 2\n\n[[branch]]\ntitle = \"A\"\n\n  [[branch.point]]\n  title = \"x\"\n  ship = [\"Study/ship/x.md\"]\n"
	rm, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := rm.Branches[0].Points[0].Ship; len(got) != 1 || got[0] != "Study/ship/x.md" {
		t.Fatalf("Ship = %v", got)
	}
}
