package repl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bugzi/gs/internal/seaport"
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

func TestDrainPort(t *testing.T) {
	var out bytes.Buffer
	r := &REPL{out: &out}
	r.drainPort() // 无任务:不应有任何输出
	if out.Len() != 0 {
		t.Fatalf("无任务不应输出: %s", out.String())
	}
	r.portRes = make(chan seaport.Result, 1)
	r.portRes <- seaport.Result{Items: []seaport.Item{
		{File: "闭包.md", Dest: "Study/sea/Go语言基础/闭包.md"},
	}}
	r.drainPort()
	if !strings.Contains(out.String(), "seaport 导入完成") {
		t.Fatalf("应打印导入结果: %s", out.String())
	}
	out.Reset()
	r.drainPort() // 已收割:不应重复打印
	if out.Len() != 0 {
		t.Fatalf("不应重复打印: %s", out.String())
	}
}
