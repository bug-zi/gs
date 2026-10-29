package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateEntry(t *testing.T) {
	ws := wsForTest(t)
	p, err := CreateEntry(ws, "river", "Go并发要点", "Study/sea/clip/a.md", "摘录正文\n")
	if err != nil {
		t.Fatal(err)
	}
	if p != "Study/river/Go并发要点.md" {
		t.Fatalf("p = %s", p)
	}
	data, err := os.ReadFile(filepath.Join(ws.RiverDir(), "Go并发要点.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Go并发要点\n\n> 出处：Study/sea/clip/a.md\n\n摘录正文\n"
	if string(data) != want {
		t.Fatalf("got %q want %q", data, want)
	}
	// 同名冲突
	if _, err := CreateEntry(ws, "river", "Go并发要点", "", "x"); err == nil {
		t.Fatal("同名应报错")
	}
	// 非法主题
	if _, err := CreateEntry(ws, "river", "a/b", "", "x"); err == nil {
		t.Fatal("含分隔符应报错")
	}
	// spring 走同一模板
	if _, err := CreateEntry(ws, "spring", "我的理解", "Study/river/Go并发要点.md", "正文"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(filepath.Join(ws.SpringDir(), "我的理解.md"))
	if !strings.Contains(string(data), "> 出处：Study/river/Go并发要点.md") {
		t.Fatalf("spring 出处行缺失: %s", data)
	}
}

func TestCreateCodeEntry(t *testing.T) {
	ws := wsForTest(t) // Study/code 目录不存在，应自动建
	p, err := CreateCodeEntry(ws, "slice 底层结构与扩容")
	if err != nil {
		t.Fatal(err)
	}
	if p != "Study/code/slice 底层结构与扩容.go" {
		t.Fatalf("p = %s", p)
	}
	data, err := os.ReadFile(filepath.Join(ws.CodeDir(), "slice 底层结构与扩容.go"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "package main") || !strings.Contains(s, "func main()") {
		t.Fatalf("模板内容不符:\n%s", s)
	}
	if strings.Contains(s, "fmt.") || strings.Contains(s, "TODO") || strings.Contains(s, "go run") {
		t.Fatalf("模板不应含占位演示代码:\n%s", s)
	}
	// 同名冲突
	if _, err := CreateCodeEntry(ws, "slice 底层结构与扩容"); err == nil {
		t.Fatal("同名应报错")
	}
	// 非法主题
	if _, err := CreateCodeEntry(ws, "a/b"); err == nil {
		t.Fatal("含分隔符应报错")
	}
}
