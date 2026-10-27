package search

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bugzi/gs/internal/config"
)

var engWordRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]+`)

var hotStop = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "err": true, "nil": true,
	"func": true, "return": true, "package": true, "import": true, "test": true,
	"testing": true, "md": true, "go": true,
}

// HotKeywords 英文词取自日志正文与三层知识库文件名，中文主题取自三层文件名按「与/和/的」切分
func HotKeywords(root string, cfg *config.Config, limit int) []string {
	count := map[string]int{}
	add := func(w string, n int) {
		w = strings.TrimSpace(w)
		if len([]rune(w)) < 2 {
			return
		}
		count[w] += n
	}
	entries, _ := os.ReadDir(filepath.Join(root, "Study", "logs"))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "Study", "logs", e.Name()))
		if err != nil {
			continue
		}
		for _, w := range engWordRe.FindAllString(strings.ToLower(string(data)), -1) {
			if !hotStop[w] {
				add(w, 1)
			}
		}
	}
	var names []string
	for _, tier := range []string{"sea", "river", "spring"} {
		_ = filepath.WalkDir(filepath.Join(root, "Study", tier), func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(d.Name()), ".md") {
				names = append(names, strings.TrimSuffix(d.Name(), ".md"))
			}
			return nil
		})
	}
	for _, n := range names {
		for _, w := range engWordRe.FindAllString(strings.ToLower(n), -1) {
			if !hotStop[w] {
				add(w, 2)
			}
		}
		for _, seg := range splitCJK(strings.TrimPrefix(n, "Go")) {
			add(seg, 2)
		}
	}
	type kv struct {
		k string
		v int
	}
	var list []kv
	for k, v := range count {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	if len(list) > limit {
		list = list[:limit]
	}
	out := make([]string, 0, len(list))
	for _, it := range list {
		out = append(out, it.k)
	}
	return out
}

// splitCJK 按「与/和/的」等分隔符切词组，丢弃纯 ASCII 段
func splitCJK(s string) []string {
	var out []string
	for _, seg := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '与' || r == '和' || r == '的' || r == '/' || r == '_' || r == '-'
	}) {
		hasCJK := false
		for _, r := range seg {
			if r >= 0x4e00 && r <= 0x9fff {
				hasCJK = true
				break
			}
		}
		if hasCJK {
			out = append(out, seg)
		}
	}
	return out
}
