package command

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/bugzi/gs/internal/termkey"
	"github.com/bugzi/gs/internal/structure"
	"github.com/bugzi/gs/internal/ui"
	"github.com/bugzi/gs/internal/workspace"
)

// treeRow 交互树视图的一行（渲染前的纯数据）
type treeRow struct {
	rel      string // 相对项目根路径
	isDir    bool
	expanded bool
	head     string // 树形前缀 + 折叠符 + 名字（素文本）
	desc     string // AI 介绍（可空）
	notes    int    // 关联笔记数
}

func (r treeRow) noteMark() string {
	if r.notes <= 0 {
		return ""
	}
	return fmt.Sprintf("　·笔%d", r.notes)
}

func (r treeRow) plain() string { return r.head + r.desc + r.noteMark() }

// foldMark 目录折叠指示符；文件占两格对齐
func foldMark(isDir, open bool) string {
	switch {
	case !isDir:
		return "  "
	case open:
		return "▾ "
	default:
		return "▸ "
	}
}

// flattenTree 从 root 展开当前视图行：目录默认折叠，expanded 中的目录递归展开
func flattenTree(root, projectRoot string, exclude []string, expanded map[string]bool,
	cache *structure.StructureCache, matched map[string][]structure.Note) []treeRow {
	var rows []treeRow
	var walk func(prefix, indent string)
	walk = func(prefix, indent string) {
		entries := structure.Scan(filepath.Join(projectRoot, filepath.FromSlash(prefix)), prefix, exclude)
		for i, e := range entries {
			branch, child := "├── ", "│   "
			if i == len(entries)-1 {
				branch, child = "└── ", "    "
			}
			open := expanded[e.Rel]
			name := e.Name
			if e.IsDir {
				name += "/"
			}
			row := treeRow{
				rel:      e.Rel,
				isDir:    e.IsDir,
				expanded: open,
				head:     indent + branch + foldMark(e.IsDir, open) + name,
				desc:     cache.Find(e.Rel),
				notes:    len(matched[e.Rel]),
			}
			rows = append(rows, row)
			if e.IsDir && open {
				walk(e.Rel, indent+child)
			}
		}
	}
	walk(root, "")
	return rows
}

// treeWindow 视口窗口：不超过 avail 全显；超出则以光标为中心滑动，贴边取齐
func treeWindow(n, cur, avail int) (lo, hi int) {
	if avail <= 0 || n <= avail {
		return 0, n
	}
	lo = cur - avail/2
	if lo < 0 {
		lo = 0
	}
	if lo+avail > n {
		lo = n - avail
	}
	return lo, lo + avail
}

// renderRow 单行渲染：超宽降级为素文本截断；光标行整行反白；普通行介绍/笔记灰色
func renderRow(r treeRow, selected bool, width int) string {
	plain := r.plain()
	if runewidth.StringWidth(plain) > width {
		return runewidth.Truncate(plain, width, "…")
	}
	if selected {
		return "\x1b[7m" + plain + "\x1b[27m"
	}
	line := r.head
	if r.desc != "" {
		line += ui.Dim(r.desc)
	}
	line += ui.Dim(r.noteMark())
	return line
}

// drawTreeFrame 重绘整帧：标题 + 键位提示 + 视口行（光标反白）+ 越界提示；
// 写完光标移回帧首，供下次整体重绘覆盖
func drawTreeFrame(ctx *Ctx, p workspace.Project, st *treeView, rows []treeRow) {
	title := "\x1b[1m框架树 · " + p.Name + "\x1b[22m"
	if st.root != "" {
		title += "　" + st.root + "/"
	}
	hint := ui.Dim("↑↓/jk 移动　Tab 展开/收起　Enter 进入/打开　Esc 返回上级（根目录退出）　r 重生成介绍　q 退出")
	avail := 0
	if h, ok := ui.PageSize(); ok {
		avail = h - 4
	}
	lo, hi := treeWindow(len(rows), st.cur, avail)
	width := ui.TermWidth(90)
	if width > 100 {
		width = 100
	}
	var lines []string
	if lo > 0 {
		lines = append(lines, ui.Dim(fmt.Sprintf("　…（上方还有 %d 行）", lo)))
	}
	for i := lo; i < hi; i++ {
		lines = append(lines, renderRow(rows[i], i == st.cur, width))
	}
	if hi < len(rows) {
		lines = append(lines, ui.Dim(fmt.Sprintf("　…（下方还有 %d 行）", len(rows)-hi)))
	}
	var b strings.Builder
	b.WriteString("\r\x1b[J")
	b.WriteString(title + "\r\n")
	b.WriteString(hint + "\r\n")
	for _, l := range lines {
		b.WriteString(l + "\r\n")
	}
	fmt.Fprintf(&b, "\x1b[%dA\r", 2+len(lines)) // 上移行数 = 标题+提示+选项，差一会使旧帧残留堆叠
	fmt.Fprint(ctx.Out, b.String())
}

// keySession 交互视图的 raw 按键会话：暂停（让给 huh/编辑器）后可重开，退出保证还原终端
type keySession struct{ kr *termkey.KeyReader }

func (s *keySession) resume() bool {
	if s.kr != nil {
		return true
	}
	kr, err := termkey.NewKeyReader()
	if err != nil {
		return false
	}
	s.kr = kr
	return true
}

func (s *keySession) close() {
	if s.kr != nil {
		s.kr.Close()
		s.kr = nil
	}
}

// treeView 交互态：当前根目录、展开集合、光标行
type treeView struct {
	root     string
	expanded map[string]bool
	cur      int
}

// parentDir 上一级目录；根返回空串
func parentDir(dir string) string {
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		return dir[:i]
	}
	return ""
}

// projectNoteMap 项目 go 文件 → 关联笔记映射（树标记与文件操作共用）
func projectNoteMap(ctx *Ctx, p workspace.Project) map[string][]structure.Note {
	return structure.MatchNotes(structure.WalkGoFiles(p.Path, ctx.Cfg.Scan.Exclude), allProjectNotes(ctx, p))
}

// runTreeBrowser 交互树循环：↑↓移动 Tab 展开/收起 Enter 进入/打开 Esc 返回 q 退出 r 重生成介绍。
// 非 TTY 无法进 raw 模式时降级为整棵展开树一次性打印。
func runTreeBrowser(ctx *Ctx, p workspace.Project, cache *structure.StructureCache) error {
	sess := &keySession{}
	defer sess.close()
	if !sess.resume() {
		return printStaticTree(ctx, p, cache)
	}
	st := &treeView{expanded: map[string]bool{}}
	matched := projectNoteMap(ctx, p)
	for {
		rows := flattenTree(st.root, p.Path, ctx.Cfg.Scan.Exclude, st.expanded, cache, matched)
		if st.cur >= len(rows) {
			st.cur = len(rows) - 1
		}
		if st.cur < 0 {
			st.cur = 0
		}
		drawTreeFrame(ctx, p, st, rows)
		k, ok := sess.kr.Read()
		if !ok {
			return nil
		}
		switch k.Kind {
		case termkey.KindUp, termkey.KindDown:
			delta := -1
			if k.Kind == termkey.KindDown {
				delta = 1
			}
			if next := st.cur + delta; next >= 0 && next < len(rows) {
				st.cur = next
			}
		case termkey.KindTab:
			if r := rowAt(rows, st.cur); r != nil && r.isDir {
				st.expanded[r.rel] = !st.expanded[r.rel]
			}
		case termkey.KindEnter:
			r := rowAt(rows, st.cur)
			if r == nil {
				continue
			}
			if r.isDir {
				st.root = r.rel
				st.cur = 0
				continue
			}
			// 文件操作（huh 菜单/外部编辑器）需要普通终端，先退出 raw
			sess.close()
			fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
			if err := fileActions(ctx, p, r.rel); err != nil {
				return err
			}
			matched = projectNoteMap(ctx, p)
			if !sess.resume() {
				return printStaticTree(ctx, p, cache)
			}
		case termkey.KindEsc:
			sess.close()
			fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
			if st.root == "" {
				return nil
			}
			st.root = parentDir(st.root)
			st.cur = 0
			if !sess.resume() {
				return printStaticTree(ctx, p, cache)
			}
		case termkey.KindCtrlC, termkey.KindCtrlD:
			sess.close()
			fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
			return nil
		case termkey.KindRune:
			switch k.R {
			case 'q':
				sess.close()
				fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
				return nil
			case 'j':
				if st.cur < len(rows)-1 {
					st.cur++
				}
			case 'k':
				if st.cur > 0 {
					st.cur--
				}
			case 'r':
				sess.close()
				fmt.Fprint(ctx.Out, "\r\x1b[J\r\n")
				if !aiConfigured(ctx) {
					fmt.Fprintln(ctx.Out, ui.Err("未配置 AI，无法生成介绍"))
				} else if err := genTreeDescriptions(ctx, p, cache, st.root); err != nil {
					fmt.Fprintln(ctx.Out, ui.Err("AI 生成介绍失败："+err.Error()))
				} else {
					fmt.Fprintln(ctx.Out, ui.Dim("介绍已更新"))
				}
				if !sess.resume() {
					return printStaticTree(ctx, p, cache)
				}
			}
		}
	}
}

// rowAt 光标越界安全取行
func rowAt(rows []treeRow, i int) *treeRow {
	if i < 0 || i >= len(rows) {
		return nil
	}
	return &rows[i]
}

// printStaticTree 非交互降级：整棵展开树一次性打印
func printStaticTree(ctx *Ctx, p workspace.Project, cache *structure.StructureCache) error {
	fmt.Fprint(ctx.Out, ui.RenderMD(renderTree(p.Name, "", p.Path, ctx.Cfg.Scan.Exclude,
		cache, projectNoteMap(ctx, p))))
	return nil
}
