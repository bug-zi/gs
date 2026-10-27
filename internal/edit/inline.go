package edit

import (
	"bytes"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bugzi/gs/internal/workspace"
)

func normalizeCRLF(data []byte) string { return strings.ReplaceAll(string(data), "\r\n", "\n") }

// saveFile 按 detect 出的换行风格原子写回(项目纪律:一律原子写)
func saveFile(path, content string, crlf bool) error {
	out := []byte(content)
	if crlf {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	}
	return workspace.WriteFileAtomic(path, out)
}

// EditInline CLI 全屏编辑 path;tea 启动失败返回错误,由调用方降级外部编辑器。
// bracketed paste 为 bubbletea 默认启用,无需选项。
func EditInline(path string, data []byte) error {
	m := newEditor(path, data)
	p, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if fin, ok := p.(editor); ok {
		return fin.err // 从最终模型取保存错误(旧实现读初始副本,错误提示永不显示)
	}
	return nil
}
