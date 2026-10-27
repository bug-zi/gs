package structure

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

// GitRun git 执行器（函数变量，测试可打桩）；dir 为仓库目录，全部只读命令
var GitRun = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

// GitInfo 项目 git 活跃信息；Available=false 表示无 git 或非仓库
type GitInfo struct {
	Available  bool
	Created    string // YYYY-MM-DD 首次提交
	LastCommit string // YYYY-MM-DD 最近提交
	Total      int
	Recent30   int
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// GitStats 汇总项目 git 信息；任一命令失败整体降级为不可用
func GitStats(projectPath string) GitInfo {
	created, err := GitRun(projectPath, "log", "--reverse", "--format=%as")
	if err != nil {
		return GitInfo{}
	}
	last, err := GitRun(projectPath, "log", "-1", "--format=%as")
	if err != nil {
		return GitInfo{}
	}
	totalS, err := GitRun(projectPath, "rev-list", "--count", "HEAD")
	if err != nil {
		return GitInfo{}
	}
	recentS, err := GitRun(projectPath, "rev-list", "--count", "--since=30 days ago", "HEAD")
	if err != nil {
		return GitInfo{}
	}
	total, _ := strconv.Atoi(strings.TrimSpace(totalS))
	recent, _ := strconv.Atoi(strings.TrimSpace(recentS))
	return GitInfo{
		Available:  true,
		Created:    firstLine(created),
		LastCommit: firstLine(last),
		Total:      total,
		Recent30:   recent,
	}
}

// LogDate 项目学习日志条目
type LogDate struct {
	Raw     string // 260101，拼文件名用
	Display string // 2026-01-01，展示与比较用
}

// ProjectLogDates 项目 docs/log 日期，新在前；目录缺失返回空
func ProjectLogDates(projectPath string) []LogDate {
	entries, err := os.ReadDir(filepath.Join(projectPath, "docs", "log"))
	if err != nil {
		return nil
	}
	var out []LogDate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		if len(base) != 6 {
			continue
		}
		t, err := time.ParseInLocation("060102", base, workspace.CST)
		if err != nil {
			continue
		}
		out = append(out, LogDate{Raw: base, Display: t.Format("2006-01-02")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Display > out[j].Display })
	return out
}

// GoStats 统计 .go 文件数与总行数（自 command 包迁入）
func GoStats(projectPath string) (files, lines int) {
	filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".go") {
			files++
			if data, err := os.ReadFile(p); err == nil {
				lines += strings.Count(string(data), "\n")
			}
		}
		return nil
	})
	return files, lines
}
