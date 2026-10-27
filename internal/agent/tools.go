package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/search"
	"github.com/bugzi/gs/internal/workspace"
)

// resolveIn 校验 rel 为 root 内相对路径（拒绝绝对路径、Windows 盘符根路径与 .. 穿越），返回绝对路径
func resolveIn(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	// Windows 下 "\x" 是 rooted 但非绝对（IsAbs=false），须按分隔符前缀一并拒绝
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") || strings.HasPrefix(clean, "..") || clean == "." {
		return "", fmt.Errorf("路径必须是工作区相对路径: %s", rel)
	}
	return filepath.Join(root, clean), nil
}

const searchParams = `{"type":"object","properties":{"query":{"type":"string","description":"检索词，多个词为 AND 关系"}},"required":["query"]}`

// SearchTool 检索工具：包装 search.SearchFiltered，输出截断为 maxFiles 个文件、每文件 maxHits 条命中
func SearchTool(root string, exclude []string, maxFileKB int64, maxFiles, maxHits int) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "search_workspace",
			Description: "在整个学习工作区内检索 .md 与 .go 文件（多个词为 AND 关系），返回命中文件、小节与命中行。",
			Parameters:  json.RawMessage(searchParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			results, _, err := search.SearchFiltered(root, exclude, maxFileKB, in.Query)
			if err != nil {
				return "", err
			}
			if len(results) == 0 {
				return "无命中", nil
			}
			total := len(results)
			if total > maxFiles {
				results = results[:maxFiles]
			}
			var b strings.Builder
			for _, r := range results {
				fmt.Fprintf(&b, "【%s】\n", r.Path)
				hits := r.Hits
				if len(hits) > maxHits {
					hits = hits[:maxHits]
				}
				for _, h := range hits {
					fmt.Fprintf(&b, "  L%d（%s）\n", h.Line, h.Unit)
					for _, c := range h.Context {
						fmt.Fprintf(&b, "    %s\n", strings.TrimSpace(c))
					}
				}
			}
			fmt.Fprintf(&b, "\n（命中 %d 个文件）", total)
			return strings.TrimSpace(b.String()), nil
		},
	}
}

const readFileParams = `{"type":"object","properties":{"path":{"type":"string","description":"工作区相对路径（正斜杠）"},"offset":{"type":"integer","description":"起始行号（1 起），默认 1"},"limit":{"type":"integer","description":"读取行数"}},"required":["path"]}`

// ReadFileTool 读文件工具：行号标注，默认前 defaultLines 行，offset/limit 分段续读
func ReadFileTool(root string, defaultLines int) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "read_file",
			Description: "读取工作区内一个文本文件，带行号；大文件用 offset/limit 分段读取。",
			Parameters:  json.RawMessage(readFileParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			p, err := resolveIn(root, in.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			if in.Offset < 1 {
				in.Offset = 1
			}
			if in.Limit <= 0 {
				in.Limit = defaultLines
			}
			if in.Offset > len(lines) {
				return fmt.Sprintf("%s 共 %d 行，offset=%d 越界", in.Path, len(lines), in.Offset), nil
			}
			hi := in.Offset - 1 + in.Limit
			if hi > len(lines) {
				hi = len(lines)
			}
			var b strings.Builder
			for i := in.Offset - 1; i < hi; i++ {
				fmt.Fprintf(&b, "%d\t%s\n", i+1, lines[i])
			}
			if hi < len(lines) {
				fmt.Fprintf(&b, "…（共 %d 行，已显示 %d-%d，可用 offset=%d 续读）", len(lines), in.Offset, hi, hi+1)
			}
			return b.String(), nil
		},
	}
}

const listDirParams = `{"type":"object","properties":{"path":{"type":"string","description":"工作区相对路径，默认根目录"}}}`

// ListDirTool 列目录工具：单层，目录带 / 后缀，文件带大小
func ListDirTool(root string) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "list_dir",
			Description: "列出工作区内某个目录的一层内容（不递归）。",
			Parameters:  json.RawMessage(listDirParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			p := root
			if strings.TrimSpace(in.Path) != "" {
				var err error
				if p, err = resolveIn(root, in.Path); err != nil {
					return "", err
				}
			}
			entries, err := os.ReadDir(p)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			for i, e := range entries {
				if i == 200 {
					fmt.Fprintf(&b, "…（还有 %d 项未列出）\n", len(entries)-200)
					break
				}
				if e.IsDir() {
					fmt.Fprintf(&b, "%s/\n", e.Name())
					continue
				}
				size := ""
				if info, err := e.Info(); err == nil {
					size = fmt.Sprintf("（%d B）", info.Size())
				}
				fmt.Fprintf(&b, "%s%s\n", e.Name(), size)
			}
			return strings.TrimRight(b.String(), "\n"), nil
		},
	}
}

const writeFileParams = `{"type":"object","properties":{"path":{"type":"string","description":"工作区相对路径（正斜杠），仅允许 .md/.go/.toml"},"content":{"type":"string","description":"完整的新文件内容"}},"required":["path","content"]}`

// WriteFileTool 写文件工具：经 approve 审批后原子落盘；路径与扩展名校验复用 ai.SafePath
func WriteFileTool(root string, approve func(absPath, content string) bool) Tool {
	return Tool{
		Def: ai.Tool{
			Name:        "write_file",
			Description: "写入工作区文件（覆盖或新建）。content 必须是完整新内容；调用后用户会看到确认请求，拒绝后不要再尝试写该文件。",
			Parameters:  json.RawMessage(writeFileParams),
		},
		Exec: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			p, err := ai.SafePath(root, in.Path)
			if err != nil {
				return "", err
			}
			if approve != nil && !approve(p, in.Content) {
				return "用户拒绝写入 " + in.Path + "，不要再尝试写入该文件，改为文字建议。", nil
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			if err := workspace.WriteFileAtomic(p, []byte(strings.ReplaceAll(in.Content, "\r\n", "\n"))); err != nil {
				return "", err
			}
			return "已写入 " + in.Path, nil
		},
	}
}
