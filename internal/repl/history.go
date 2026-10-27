package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// History REPL 行历史：内存 + 追加持久化（每行一条，与原 readline 历史文件同格式）
type History struct {
	mu    sync.Mutex
	items []string
	file  string
}

// LoadHistory 读取历史文件；缺失/损坏从空开始
func LoadHistory(path string) *History {
	h := &History{file: path}
	f, err := os.Open(path)
	if err != nil {
		return h
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimRight(sc.Text(), "\r"); line != "" {
			h.items = append(h.items, line)
		}
	}
	return h
}

// Items 返回历史快照（旧→新）
func (h *History) Items() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.items...)
}

// Add 追加一条并持久化；空白与相邻重复忽略
func (h *History) Add(s string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.TrimSpace(s) == "" {
		return
	}
	if n := len(h.items); n > 0 && h.items[n-1] == s {
		return
	}
	h.items = append(h.items, s)
	if h.file == "" {
		return
	}
	f, err := os.OpenFile(h.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err == nil {
		fmt.Fprintf(f, "%s\n", s)
		f.Close()
	}
}
