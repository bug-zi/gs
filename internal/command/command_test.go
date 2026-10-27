package command

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpListsCommands(t *testing.T) {
	var out bytes.Buffer
	help := newHelp()
	ctx := &Ctx{Out: &out, Cmds: []Command{help, newExit()}}
	if err := help.Run(ctx); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/logs", "/sea import", "/plan next", "/search", "/exit"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("帮助缺 %s", want)
		}
	}
}

func TestFindWithAlias(t *testing.T) {
	cmds := []Command{newExit()}
	if Find(cmds, "q") == nil {
		t.Fatal("应通过别名 q 找到 exit")
	}
	if Find(cmds, "nope") != nil {
		t.Fatal("未知命令应返回 nil")
	}
}
