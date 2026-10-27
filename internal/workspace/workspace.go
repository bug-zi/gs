package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CST 东八区，日志文件名与“今天”的判定一律用它
var CST = time.FixedZone("CST", 8*3600)

type Workspace struct{ Root string }

func Detect(start string) (*Workspace, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		if isWorkspace(dir) {
			return &Workspace{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("在 %s 及其上级目录中未找到学习工作区（需含 Study/logs 与 Study/plan），可用 gs --root <路径> 指定", start)
		}
		dir = parent
	}
}

func isWorkspace(dir string) bool {
	return dirExists(filepath.Join(dir, "Study", "logs")) && dirExists(filepath.Join(dir, "Study", "plan"))
}

func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func (w *Workspace) LogsDir() string   { return filepath.Join(w.Root, "Study", "logs") }
func (w *Workspace) SeaDir() string    { return filepath.Join(w.Root, "Study", "sea") }
func (w *Workspace) RiverDir() string  { return filepath.Join(w.Root, "Study", "river") }
func (w *Workspace) SpringDir() string { return filepath.Join(w.Root, "Study", "spring") }
func (w *Workspace) CodeDir() string   { return filepath.Join(w.Root, "Study", "code") }
func (w *Workspace) PlanDir() string   { return filepath.Join(w.Root, "Study", "plan") }
func (w *Workspace) StateFile() string {
	return filepath.Join(w.PlanDir(), "学习状态.md")
}

// RoadmapFile 路线图正本路径 <root>/Study/plan/roadmap.toml
func (w *Workspace) RoadmapFile() string {
	return filepath.Join(w.PlanDir(), "roadmap.toml")
}

// Rel 返回相对 Root 的正斜杠路径，用于展示与索引
func (w *Workspace) Rel(path string) string {
	rel, err := filepath.Rel(w.Root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func LogFileName(t time.Time) string { return t.In(CST).Format("060102") + ".md" }

func ParseLogDate(name string) (time.Time, bool) {
	base := strings.TrimSuffix(name, ".md")
	if len(base) != 6 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("060102", base, CST)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// ListLogs 全部日志日期，新在前
func (w *Workspace) ListLogs() ([]time.Time, error) {
	entries, err := os.ReadDir(w.LogsDir())
	if err != nil {
		return nil, err
	}
	var dates []time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if t, ok := ParseLogDate(e.Name()); ok {
			dates = append(dates, t)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].After(dates[j]) })
	return dates, nil
}

// Streak 以东八区今天为起点向回数连续日志天数；今天无日志则从昨天起算
func (w *Workspace) Streak() int {
	dates, err := w.ListLogs()
	if err != nil || len(dates) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, d := range dates {
		seen[d.Format("20060102")] = true
	}
	day := time.Now().In(CST)
	if !seen[day.Format("20060102")] {
		day = day.AddDate(0, 0, -1)
	}
	n := 0
	for ; seen[day.Format("20060102")]; day = day.AddDate(0, 0, -1) {
		n++
	}
	return n
}

// WriteFileAtomic 先写临时文件再 rename，避免半截文件
func WriteFileAtomic(path string, data []byte) error {
	tmp := path + ".gs-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// IndexPath 沉淀索引文件路径 <root>/.gs/topics.toml
func (w *Workspace) IndexPath() string {
	return filepath.Join(w.Root, ".gs", "topics.toml")
}
