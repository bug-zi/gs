package command

import (
	"testing"

	"github.com/bugzi/gs/internal/termkey"
)

func TestPagerKeyString(t *testing.T) {
	cases := []struct {
		name string
		key  termkey.Key
		want string
		ok   bool
	}{
		{"字母小写化", termkey.Key{Kind: termkey.KindRune, R: 'Q'}, "q", true},
		{"空格翻页", termkey.Key{Kind: termkey.KindRune, R: ' '}, " ", true},
		{"回车", termkey.Key{Kind: termkey.KindEnter}, "", true},
		{"esc", termkey.Key{Kind: termkey.KindEsc}, "esc", true},
		{"ctrl+c", termkey.Key{Kind: termkey.KindCtrlC}, "ctrl+c", true},
		{"ctrl+d", termkey.Key{Kind: termkey.KindCtrlD}, "ctrl+d", true},
		{"方向键忽略", termkey.Key{Kind: termkey.KindUp}, "", false},
		{"退格忽略", termkey.Key{Kind: termkey.KindBackspace}, "", false},
	}
	for _, c := range cases {
		got, ok := pagerKeyString(c.key)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: pagerKeyString(%v) = (%q, %v)，want (%q, %v)", c.name, c.key, got, ok, c.want, c.ok)
		}
	}
}
