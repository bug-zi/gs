package complete

import (
	"reflect"
	"testing"
)

var testTree = Tree{
	{Name: "logs", Alias: []string{"log"}, Subs: []string{"add", "edit"}, Hint: "<关键词|日期>"},
	{Name: "sea", Subs: []string{"edit", "import"}, Hint: "<关键词>"},
	{Name: "help", Alias: []string{"h"}},
}

func TestFor(t *testing.T) {
	cases := []struct {
		name string
		line string
		want Result
	}{
		{"空输入", "", Result{}},
		{"无斜杠", "logs", Result{}},
		{"仅斜杠弹全部", "/", Result{Candidates: []string{"logs", "sea", "help"}}},
		{"命令前缀唯一", "/lo", Result{Ghost: "gs"}},
		{"别名让位于命令名", "/log", Result{Ghost: "s"}},
		{"完整命令无提示", "/logs", Result{}},
		{"尾随空格列子命令", "/logs ", Result{Candidates: []string{"add", "edit"}, Hint: "<关键词|日期>"}},
		{"sea尾随空格", "/sea ", Result{Candidates: []string{"edit", "import"}, Hint: "<关键词>"}},
		{"子命令前缀唯一", "/sea e", Result{Ghost: "dit", Hint: "<关键词>"}},
		{"子命令前缀唯一之二", "/sea i", Result{Ghost: "mport", Hint: "<关键词>"}},
		{"无子命令的尾随空格", "/help ", Result{}},
		{"未知命令", "/xyz ", Result{}},
		{"大小写不敏感", "/LO", Result{Ghost: "gs"}},
		{"深度参数位仅提示", "/logs add ", Result{Hint: "<关键词|日期>"}},
	}
	for _, tc := range cases {
		got := For(testTree, tc.line)
		if got.Ghost != tc.want.Ghost || !reflect.DeepEqual(got.Candidates, tc.want.Candidates) || got.Hint != tc.want.Hint {
			t.Errorf("%s: For(%q) = %+v, want %+v", tc.name, tc.line, got, tc.want)
		}
	}
}
