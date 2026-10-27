package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bugzi/gs/internal/workspace"
)

// TestGatherReviewMaterialsTruncatesLogs 体检材料的日志部分超长时应截断，防止 prompt 无限膨胀
func TestGatherReviewMaterialsTruncatesLogs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{filepath.Join(root, "Study", "logs"), filepath.Join(root, "Study", "plan")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ws := &workspace.Workspace{Root: root}
	var b strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&b, "行%d内容\n", i)
	}
	logPath := filepath.Join(ws.LogsDir(), workspace.LogFileName(time.Now()))
	if err := os.WriteFile(logPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, logs := gatherReviewMaterials(ws)
	if !strings.Contains(logs, "截断") {
		t.Fatal("超长日志应被截断并提示")
	}
	if strings.Contains(logs, "行200内容") {
		t.Fatal("截断后不应保留末尾行")
	}
}
