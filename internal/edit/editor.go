package edit

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/config"
)

// fallbackEditors 无显式配置时的兜底顺序：TRAE 优先（开发者主力），code/notepad 兜底
var fallbackEditors = []string{"trae", "code", "notepad"}

// docExts 文档类扩展名（小写比较）：维持 gs 现有查阅/编辑流程；
// 其余文件（代码/配置等）一律外部编辑器直开（默认解析链命中 TRAE）
var docExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true,
	".doc": true, ".docx": true, ".pdf": true,
}

// IsDocFile 判断 path 是否文档类文件；无扩展名视为非文档
func IsDocFile(path string) bool {
	return docExts[strings.ToLower(filepath.Ext(path))]
}

// Resolve 编辑器解析顺序：config UI.Editor > $EDITOR > $VISUAL > trae > code > notepad。
// 都找不到时返回空串。
func Resolve(cfg *config.Config) string {
	if cfg != nil && cfg.UI.Editor != "" {
		return cfg.UI.Editor
	}
	for _, k := range []string{"EDITOR", "VISUAL"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	for _, c := range fallbackEditors {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// Open 编辑入口统一分流：非文档文件一律外部编辑器直开；
// 文档文件 ≤ maxCliLines 行走 CLI 全屏编辑，否则外部编辑器。
// tea 启动失败自动降级外部。
func Open(path string, cfg *config.Config) error {
	if !IsDocFile(path) { // 非文档文件（代码/配置等）不进内嵌，直开外部
		return openExternal(path, cfg)
	}
	if data, err := os.ReadFile(path); err == nil && ShouldCliEdit(data) {
		if err := EditInline(path, data); err == nil {
			return nil
		}
	}
	return openExternal(path, cfg)
}

// OpenExternal 调起外部编辑器打开 path（跳过长短分流），阻塞直到编辑器退出。
// 供「用 IDE 打开」类动作复用编辑器解析链与 --wait 语义。
func OpenExternal(path string, cfg *config.Config) error {
	return openExternal(path, cfg)
}

// openExternal 调起外部编辑器打开 path，阻塞直到编辑器退出；
// trae/code 家族自动带 --wait（但若经由 start 分离式启动的 .cmd 包装，包装层仍会秒退，
// 此时需在配置里直指主程序 exe 才能真正阻塞）
func openExternal(path string, cfg *config.Config) error {
	ed := Resolve(cfg)
	if ed == "" {
		return errors.New("未找到可用编辑器，请在 .gs/config.toml 的 [ui] 段配置 editor = \"...\"")
	}
	lp, pre, err := splitEditor(ed)
	if err != nil {
		return err
	}
	args := waitArgs(lp, append(pre, path))
	// Windows 下 .cmd/.bat（如 code.cmd）必须经 cmd /c 启动
	if ext := strings.ToLower(filepath.Ext(lp)); ext == ".cmd" || ext == ".bat" {
		argv := append([]string{"cmd", "/c", lp}, args...)
		return exec.Command(argv[0], argv[1:]...).Run()
	}
	cmd := exec.Command(lp, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// waitArgs trae/code 家族是秒退型 GUI 编辑器，注入 --wait 才有「编辑完才返回」语义；
// 非 VS Code 系（notepad/vim 等天然阻塞）不动，已带 -w/--wait 不重复
func waitArgs(lp string, args []string) []string {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(lp), filepath.Ext(lp)))
	if !strings.HasPrefix(base, "trae") && !strings.HasPrefix(base, "code") {
		return args
	}
	for _, a := range args {
		if a == "-w" || a == "--wait" {
			return args
		}
	}
	return append([]string{"--wait"}, args...)
}

// splitEditor 先把整串当可执行路径解析（支持含空格的安装路径，如 VS Code 默认目录），
// 失败再按空白拆成「命令 + 参数」（支持 "code -w" 形式）。
func splitEditor(ed string) (lp string, args []string, err error) {
	if p, err := exec.LookPath(ed); err == nil {
		return p, nil, nil
	}
	parts := strings.Fields(ed)
	if len(parts) == 0 {
		return "", nil, errors.New("编辑器不可用: " + ed)
	}
	lp, err = exec.LookPath(parts[0])
	if err != nil {
		return "", nil, errors.New("编辑器不可用: " + parts[0])
	}
	return lp, parts[1:], nil
}

// maxCliLines 短文件阈值：不超过此行数进 CLI 内编辑，否则外部编辑器
const maxCliLines = 20

// ShouldCliEdit 行数 ≤ maxCliLines 视为短文件（走 CLI 内编辑/查阅）；
// 逾行文件由调用方转交外部编辑器
func ShouldCliEdit(data []byte) bool { return countLines(data) <= maxCliLines }

// countLines 行数：\n 计数，末行无换行符补 1；空文件为 0
func countLines(data []byte) int {
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}
