package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

func ts(min int) time.Time { return time.Date(2026, 10, 29, 14, min, 0, 0, workspace.CST) }

func TestLoadMissing(t *testing.T) {
	root := t.TempDir()
	got, err := Load(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("无文件应返回空切片: %v %v", got, err)
	}
}

func TestAddAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	if err := Add(root, "slice、map、channel 可以看作引用类型", ts(30)); err != nil {
		t.Fatal(err)
	}
	if err := Add(root, "defer 在 return 之后执行", ts(5)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "# 便签\n\n> 临时记录，待整理进 river/spring\n") {
		t.Fatalf("骨架头部缺失:\n%s", s)
	}
	if !strings.Contains(s, "- [1] 261029 14:30 slice、map、channel 可以看作引用类型") ||
		!strings.Contains(s, "- [2] 261029 14:05 defer 在 return 之后执行") {
		t.Fatalf("便签行缺失或编号/时间不对:\n%s", s)
	}
	got, err := Load(root)
	if err != nil || len(got) != 2 || got[0].Text != "slice、map、channel 可以看作引用类型" {
		t.Fatalf("往返不一致: %+v %v", got, err)
	}
}

func TestLoadSkipsUnknownLines(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "Study", "notes.md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("# 便签\n\n随手写的不是便签的行\n- [1] 261029 14:30 真便签\n- [x] 坏行\n- [2] 26102925:00 坏时间\n"), 0o644)
	got, err := Load(root)
	if err != nil || len(got) != 1 || got[0].Text != "真便签" {
		t.Fatalf("应只解析出 1 条合法便签: %+v %v", got, err)
	}
}

func TestSaveRenumbers(t *testing.T) {
	root := t.TempDir()
	// 手工造乱序编号文件，Save 后应重排
	os.MkdirAll(filepath.Dir(Path(root)), 0o755)
	os.WriteFile(Path(root), []byte("# 便签\n\n> 临时记录，待整理进 river/spring\n\n- [7] 261029 14:30 甲\n- [3] 261029 15:00 乙\n"), 0o644)
	ns, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(root, ns); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(Path(root))
	s := string(data)
	if strings.Contains(s, "[7]") || !strings.Contains(s, "- [1] 261029 14:30 甲") || !strings.Contains(s, "- [2] 261029 15:00 乙") {
		t.Fatalf("应重编号为 1/2:\n%s", s)
	}
}

func TestSaveEmptyKeepsSkeleton(t *testing.T) {
	root := t.TempDir()
	if err := Save(root, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(Path(root))
	if !strings.HasPrefix(string(data), "# 便签\n") {
		t.Fatalf("空保存应保留骨架:\n%s", data)
	}
}

func TestAppendNote(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "Study", "river", "01-基础", "03-切片.md")
	os.MkdirAll(filepath.Dir(target), 0o755)
	os.WriteFile(target, []byte("# 切片"), 0o644)

	if err := AppendNote(root, "Study/river/01-基础/03-切片.md", "扩容翻倍"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if got := string(data); got != "# 切片\n\n扩容翻倍\n" {
		t.Fatalf("追加格式不对: %q", got)
	}

	// 无尾换行的文件补换行后再追加；二次追加空行分隔
	if err := AppendNote(root, "Study/river/01-基础/03-切片.md", "三色标记"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(target)
	if got := string(data); got != "# 切片\n\n扩容翻倍\n\n三色标记\n" {
		t.Fatalf("二次追加格式不对: %q", got)
	}
}

func TestAppendNoteRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := AppendNote(root, "../outside.md", "x"); err == nil {
		t.Fatal("应拒绝 .. 穿越")
	}
}
