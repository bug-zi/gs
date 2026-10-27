package repl

import (
	"bytes"
	"strings"
	"testing"
)

func TestBackFromAIReturnsToMain(t *testing.T) {
	var out bytes.Buffer
	r := &REPL{out: &out, mode: "ai"}
	r.backFromAI()
	if r.mode != "" {
		t.Errorf("backFromAI 后 mode 应复位为主模式，got %q", r.mode)
	}
	if !strings.Contains(out.String(), "已返回主模式") {
		t.Errorf("应输出「已返回主模式」，got %q", out.String())
	}
}
