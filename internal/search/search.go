package search

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bugzi/gs/internal/config"
)

type Hit struct {
	Unit    string   // 命中所在小节标题或函数名
	Line    int      // 1 起始行号
	Context []string // 命中行及前后各 1 行
}

type FileResult struct {
	Path string // 相对 root 的正斜杠路径
	Hits []Hit
}

var funcRe = regexp.MustCompile(`func\s+(?:\([^)]*\)\s*)?(\w+)\s*\(`)

// Search 多词 AND：文件内全部词命中才收录；行级任一词命中即为 Hit
func Search(root string, cfg *config.Config, query string) ([]FileResult, []string, error) {
	return SearchFiltered(root, cfg.Scan.Exclude, cfg.Scan.MaxFileKB, query)
}

// SearchFiltered 与 Search 相同，但直接接收排除目录与大小上限（供无 config 依赖的调用方使用）
func SearchFiltered(root string, exclude []string, maxFileKB int64, query string) ([]FileResult, []string, error) {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil, nil, nil
	}
	var results []FileResult
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excluded(exclude, root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".md" && ext != ".go" {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > maxFileKB*1024 {
			return nil
		}
		if fr := searchFile(path, root, words); fr != nil {
			results = append(results, *fr)
		}
		return nil
	})
	return results, words, err
}

func excluded(exclude []string, root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		for _, ex := range exclude {
			if seg == ex {
				return true
			}
		}
	}
	return false
}

func searchFile(path, root string, words []string) *FileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lower := make([]string, len(lines))
	for i, l := range lines {
		lower[i] = strings.ToLower(l)
	}
	for _, w := range words {
		found := false
		for _, l := range lower {
			if strings.Contains(l, w) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	isMd := strings.EqualFold(filepath.Ext(path), ".md")
	unit := ""
	var hits []Hit
	for i := range lines {
		trimmed := strings.TrimSpace(lines[i])
		if isMd && strings.HasPrefix(trimmed, "#") {
			unit = strings.TrimLeft(trimmed, "# ")
		} else if !isMd && strings.HasPrefix(trimmed, "func ") {
			if m := funcRe.FindStringSubmatch(trimmed); m != nil {
				unit = m[1]
			}
		}
		for _, w := range words {
			if strings.Contains(lower[i], w) {
				hits = append(hits, Hit{Unit: unit, Line: i + 1, Context: contextOf(lines, i)})
				break
			}
		}
	}
	if len(hits) == 0 {
		return nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	return &FileResult{Path: filepath.ToSlash(rel), Hits: hits}
}

func contextOf(lines []string, i int) []string {
	lo := i - 1
	if lo < 0 {
		lo = 0
	}
	hi := i + 2
	if hi > len(lines) {
		hi = len(lines)
	}
	return lines[lo:hi]
}
