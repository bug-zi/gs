// Package seaport 海港:工作区根下 seaport/ 目录的自动导入——
// 扫描待导入文件,经 AI 按内容分类迁入 Study/sea/ 主题目录。
package seaport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bugzi/gs/internal/ai"
	"github.com/bugzi/gs/internal/workspace"
)

// importableExts 可导入的文本扩展名(小写)
var importableExts = map[string]bool{".md": true, ".txt": true}

// Fallback 未分类兜底目录名(sea 下,始终是合法分类目标)
const Fallback = "未分类"

// Dir 海港目录 <root>/seaport
func Dir(ws *workspace.Workspace) string {
	return filepath.Join(ws.Root, "seaport")
}

// Ensure seaport 目录不存在则创建
func Ensure(ws *workspace.Workspace) error {
	return os.MkdirAll(Dir(ws), 0o755)
}

// Scan 列出 seaport 顶层可导入文件与非文本文件;目录缺失返回空
func Scan(ws *workspace.Workspace) (files, skipped []string) {
	entries, err := os.ReadDir(Dir(ws))
	if err != nil {
		return nil, nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if importableExts[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, e.Name())
		} else {
			skipped = append(skipped, e.Name())
		}
	}
	return files, skipped
}

// Candidates sea 一级子目录名作为分类候选;兜底"未分类"始终在列
func Candidates(ws *workspace.Workspace) []string {
	entries, err := os.ReadDir(ws.SeaDir())
	var dirs []string
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
	}
	for _, d := range dirs {
		if d == Fallback {
			return dirs
		}
	}
	return append(dirs, Fallback)
}

// parseClassify 从回复中提取 {"分类":{文件名:目录名}};容忍围栏与前后杂文本
func parseClassify(reply string) (map[string]string, error) {
	s := reply
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		return nil, errors.New("回复中没有 JSON")
	}
	s = s[i : j+1]
	var wrapper struct {
		Fenlei map[string]string `json:"分类"`
	}
	if err := json.Unmarshal([]byte(s), &wrapper); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return wrapper.Fenlei, nil
}

// ChatClient 分类所需的最小 AI 能力(*ai.Client 满足;测试注入 fake)
type ChatClient interface {
	Chat(ctx context.Context, msgs []ai.Message) (string, error)
}

// Item 单篇导入结果
type Item struct {
	File string // seaport 中的原文件名
	Dest string // 工作区相对目标路径(仅成功时非空)
	Err  string // 迁移失败原因(空 = 成功;分类兜底进未分类不算失败)
}

// Result 一次后台导入的完整结果
type Result struct {
	Items []Item
	Note  string // 整体说明:AI 失败原因/取消原因(空 = 分类正常)
}

const classifyPrompt = `你是笔记分类器。根据每篇文件的文件名与内容,从候选目录中为它选择最合适的一个目录名。
只能从候选目录中选择,不得创造新目录。只输出一个 JSON 对象,不要输出任何其他文字,格式:
{"分类":{"<文件名>":"<候选目录名>"}}`

// Import 完整导入:扫描 → 批量 AI 分类 → 逐篇迁移。
// cc 为 nil 或分类失败时全部进未分类;ctx 取消则停止,剩余文件留在 seaport。
func Import(ctx context.Context, ws *workspace.Workspace, cc ChatClient) Result {
	files, _ := Scan(ws)
	var res Result
	if len(files) == 0 {
		return res
	}
	dests, note := classify(ctx, ws, cc, files)
	if note != "" {
		res.Note = note
	}
	for _, name := range files {
		if ctx.Err() != nil {
			res.Note = strings.TrimSpace(res.Note + " 已取消,剩余文件留在 seaport")
			return res
		}
		dir := dests[name]
		if dir == "" {
			dir = Fallback
		}
		dst, err := moveFile(ws, name, dir)
		if err != nil {
			res.Items = append(res.Items, Item{File: name, Err: err.Error()})
			continue
		}
		res.Items = append(res.Items, Item{File: name, Dest: ws.Rel(dst)})
	}
	return res
}

// classify 批量一次调用;返回 文件名→目录名 映射与失败说明(成功时说明为空)
func classify(ctx context.Context, ws *workspace.Workspace, cc ChatClient, files []string) (map[string]string, string) {
	if cc == nil {
		return nil, "AI 未配置,全部放入" + Fallback
	}
	cands := Candidates(ws)
	var b strings.Builder
	fmt.Fprintf(&b, "候选目录:%s\n", strings.Join(cands, "、"))
	for _, name := range files {
		content := ""
		if data, err := os.ReadFile(filepath.Join(Dir(ws), name)); err == nil {
			r := []rune(string(data))
			if len(r) > 2000 {
				r = r[:2000]
			}
			content = string(r)
		}
		fmt.Fprintf(&b, "\n【%s】\n%s\n", name, content)
	}
	reply, err := cc.Chat(ctx, []ai.Message{
		{Role: "system", Content: classifyPrompt},
		{Role: "user", Content: b.String()},
	})
	if err != nil {
		return nil, "AI 分类失败(" + err.Error() + "),全部放入" + Fallback
	}
	m, err := parseClassify(reply)
	if err != nil {
		return nil, "AI 响应无法解析(" + err.Error() + "),全部放入" + Fallback
	}
	valid := map[string]bool{}
	for _, c := range cands {
		valid[strings.ToLower(c)] = true
	}
	out := make(map[string]string, len(files))
	for _, name := range files {
		d := strings.TrimSpace(m[name])
		if d == "" || !valid[strings.ToLower(d)] {
			d = Fallback
		}
		out[name] = d
	}
	return out, ""
}

// moveFile 迁移单篇到 sea/<dir>/;重名自动加 -2、-3 后缀,绝不覆盖
func moveFile(ws *workspace.Workspace, name, dir string) (string, error) {
	dstDir := filepath.Join(ws.SeaDir(), dir)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	ext := filepath.Ext(name)
	dst := filepath.Join(dstDir, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(dst); err != nil {
			break // 不存在即可用;真正的错误由 rename 暴露
		}
		dst = filepath.Join(dstDir, fmt.Sprintf("%s-%d%s", base, i, ext))
	}
	return dst, os.Rename(filepath.Join(Dir(ws), name), dst)
}

// Print 渲染导入结果(由 REPL 在提示符前调用)
func Print(out io.Writer, res Result) {
	fmt.Fprintln(out, "seaport 导入完成:")
	if res.Note != "" {
		fmt.Fprintf(out, "  %s\n", res.Note)
	}
	for _, it := range res.Items {
		switch {
		case it.Err != "":
			fmt.Fprintf(out, "  %s 导入失败:%s\n", it.File, it.Err)
		default:
			fmt.Fprintf(out, "  %s → %s\n", it.File, it.Dest)
		}
	}
}
