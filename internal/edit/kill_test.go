package edit

import "testing"

func TestKillRingYank(t *testing.T) {
	k := &killRing{}
	if _, ok := k.yank(0); ok {
		t.Fatal("空环 yank 应失败")
	}
	k.push("a")
	k.push("b")
	if s, _ := k.yank(0); s != "b" {
		t.Fatalf("最近条目: %q", s)
	}
	if s, _ := k.yank(1); s != "a" {
		t.Fatalf("次新条目: %q", s)
	}
	if s, _ := k.yank(2); s != "b" {
		t.Fatalf("轮换回绕: %q", s)
	}
	k.push("") // 空串不入环
	if s, _ := k.yank(0); s != "b" {
		t.Fatalf("空串应忽略: %q", s)
	}
}

func TestKillRingLimit(t *testing.T) {
	k := &killRing{}
	for i := 0; i < killLimit+5; i++ {
		k.push(string(rune('a' + i)))
	}
	if len(k.items) != killLimit {
		t.Fatalf("容量应封顶 %d, got %d", killLimit, len(k.items))
	}
	if s, _ := k.yank(0); s != string(rune('a'+killLimit+4)) {
		t.Fatalf("最旧应被挤出: %q", s)
	}
}
