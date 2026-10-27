package termkey

import (
	"reflect"
	"testing"
)

func acts(kinds ...Kind) []Key {
	out := make([]Key, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, Key{Kind: k})
	}
	return out
}

func runes(str string) []Key {
	var out []Key
	for _, r := range str {
		out = append(out, Key{Kind: KindRune, R: r})
	}
	return out
}

func TestKeyParserBasics(t *testing.T) {
	var p Parser
	cases := []struct {
		name string
		in   []byte
		want []Key
	}{
		{"普通字符", []byte("/logs"), runes("/logs")},
		{"CR回车", []byte("\r"), acts(KindEnter)},
		{"LF回车", []byte("\n"), acts(KindEnter)},
		{"CRLF算一次回车", []byte("\r\n"), acts(KindEnter)},
		{"退格", []byte("\x7f"), acts(KindBackspace)},
		{"Tab", []byte("\t"), acts(KindTab)},
		{"CtrlC", []byte("\x03"), acts(KindCtrlC)},
		{"CtrlD", []byte("\x04"), acts(KindCtrlD)},
		{"杂散NUL忽略", []byte{'a', 0, 'b'}, runes("ab")},
		{"上箭头", []byte("\x1b[A"), acts(KindUp)},
		{"下箭头", []byte("\x1b[B"), acts(KindDown)},
		{"右箭头", []byte("\x1b[C"), acts(KindRight)},
		{"左箭头", []byte("\x1b[D"), acts(KindLeft)},
		{"孤立ESC", []byte("\x1b"), acts(KindEsc)},
		{"中文", []byte("并发"), runes("并发")},
	}
	for _, tc := range cases {
		p = Parser{}
		got := p.Feed(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestKeyParserSplitArrival(t *testing.T) {
	// ESC 序列分两块到达
	var p Parser
	if got := p.Feed([]byte("\x1b[")); len(got) != 0 {
		t.Fatalf("残包应无动作: %v", got)
	}
	if got := p.Feed([]byte("A")); !reflect.DeepEqual(got, acts(KindUp)) {
		t.Fatalf("补齐后应得 up: %v", got)
	}
	// 残缺 UTF-8 分两块到达：「发」= E5 8F 91
	p = Parser{}
	if got := p.Feed([]byte{0xe5}); len(got) != 0 {
		t.Fatalf("残缺首字节应无动作: %v", got)
	}
	if got := p.Feed([]byte{0x8f, 0x91}); !reflect.DeepEqual(got, runes("发")) {
		t.Fatalf("补齐后: %v", got)
	}
	// 「并」= E5 B9 B6
	p = Parser{}
	if got := p.Feed([]byte{0xe5, 0xb9}); len(got) != 0 {
		t.Fatalf("残缺两字节应无动作: %v", got)
	}
	if got := p.Feed([]byte{0xb6}); !reflect.DeepEqual(got, runes("并")) {
		t.Fatalf("补齐后: %v", got)
	}
}

func TestKeyParserCRLFRun(t *testing.T) {
	// 连续多次回车（CRLF 混 LF）：每个 \r 吞掉紧随的 \n，\n 单独也算回车
	var p Parser
	got := p.Feed([]byte("\r\n\r\n\n"))
	if !reflect.DeepEqual(got, acts(KindEnter, KindEnter, KindEnter)) {
		t.Fatalf("got %v", got)
	}
}

func TestKeyParserPageKeys(t *testing.T) {
	var p Parser
	if got := p.Feed([]byte("\x1b[5~\x1b[6~")); !reflect.DeepEqual(got, acts(KindPgUp, KindPgDown)) {
		t.Fatalf("翻页键: got %v", got)
	}
	// 残包：5~ 分两块送达仍应拼出 PgUp
	p = Parser{}
	if got := p.Feed([]byte("\x1b[5")); len(got) != 0 {
		t.Fatalf("残包不应出键: %v", got)
	}
	if got := p.Feed([]byte("~")); !reflect.DeepEqual(got, acts(KindPgUp)) {
		t.Fatalf("补齐后应得 pgup: %v", got)
	}
}
