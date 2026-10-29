package ui

import (
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

var (
	matchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // 黄：命中词
	pathStyle  = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // 红：错误
)

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func uintPtr(u uint) *uint    { return &u }

// gsStyle gs 专属深色 Markdown 样式：标题无 # 字面前缀，靠色彩层级区分；
// 低饱和暖色系（220 金 → 179 赭 → 180 麦 → 144 沙），与 ui 现有点缀色同族。
var gsStyle = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{Color: strPtr("252"), BlockPrefix: "\n", BlockSuffix: "\n"},
		Margin:         uintPtr(2),
	},
	BlockQuote:     ansi.StyleBlock{Indent: uintPtr(1), IndentToken: strPtr("│ ")},
	List:           ansi.StyleList{LevelIndent: 2},
	Heading:        ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Bold: boolPtr(true), BlockSuffix: "\n"}},
	H1:             ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("220"), Underline: boolPtr(true)}},
	H2:             ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("220")}},
	H3:             ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("179")}},
	H4:             ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("180")}},
	H5:             ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("144")}},
	H6:             ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("144"), Faint: boolPtr(true)}},
	Strikethrough:  ansi.StylePrimitive{CrossedOut: boolPtr(true)},
	Emph:           ansi.StylePrimitive{Italic: boolPtr(true)},
	Strong:         ansi.StylePrimitive{Bold: boolPtr(true)},
	HorizontalRule: ansi.StylePrimitive{Color: strPtr("240"), Format: "\n--------\n"},
	Item:           ansi.StylePrimitive{BlockPrefix: "• "},
	Enumeration:    ansi.StylePrimitive{BlockPrefix: ". "},
	Task:           ansi.StyleTask{Ticked: "[x] ", Unticked: "[ ] "},
	Link:           ansi.StylePrimitive{Color: strPtr("179"), Underline: boolPtr(true)},
	LinkText:       ansi.StylePrimitive{Color: strPtr("220"), Bold: boolPtr(true)},
	Image:          ansi.StylePrimitive{Color: strPtr("179"), Underline: boolPtr(true)},
	ImageText:      ansi.StylePrimitive{Color: strPtr("243"), Format: "图: {{.text}} →"},
	Code:           ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Prefix: " ", Suffix: " ", Color: strPtr("203"), BackgroundColor: strPtr("236")}},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: strPtr("244")}, Margin: uintPtr(2)},
		Chroma: &ansi.Chroma{
			Text:            ansi.StylePrimitive{Color: strPtr("#C4C4C4")},
			Error:           ansi.StylePrimitive{Color: strPtr("#F1F1F1"), BackgroundColor: strPtr("#F05B5B")},
			Comment:         ansi.StylePrimitive{Color: strPtr("#6A6A6A")},
			Keyword:         ansi.StylePrimitive{Color: strPtr("#D7BA7D")}, // 暖金，与标题同族
			KeywordType:     ansi.StylePrimitive{Color: strPtr("#CE9178")},
			Operator:        ansi.StylePrimitive{Color: strPtr("#C8C8C8")},
			Punctuation:     ansi.StylePrimitive{Color: strPtr("#A9A9A9")},
			Name:            ansi.StylePrimitive{Color: strPtr("#C4C4C4")},
			NameFunction:    ansi.StylePrimitive{Color: strPtr("#DCDCAA")},
			NameBuiltin:     ansi.StylePrimitive{Color: strPtr("#D7BA7D")},
			LiteralString:   ansi.StylePrimitive{Color: strPtr("#C69669")}, // 暖棕
			LiteralNumber:   ansi.StylePrimitive{Color: strPtr("#B5CEA8")},
			GenericDeleted:  ansi.StylePrimitive{Color: strPtr("#FD5B5B")},
			GenericInserted: ansi.StylePrimitive{Color: strPtr("#9CB98B")},
			GenericStrong:   ansi.StylePrimitive{Bold: boolPtr(true)},
			GenericEmph:     ansi.StylePrimitive{Italic: boolPtr(true)},
			Background:      ansi.StylePrimitive{BackgroundColor: strPtr("#373737")},
		},
	},
	DefinitionDescription: ansi.StylePrimitive{BlockPrefix: "\n» "},
}

var renderers sync.Map // 宽度 → *glamour.TermRenderer

// RenderMD 渲染 Markdown：按终端宽度取缓存渲染器（上限 100 列，非终端 90），
// 构造或渲染失败时原样返回
func RenderMD(src string) string {
	width := min(TermWidth(90), 100)
	v, ok := renderers.Load(width)
	if !ok {
		r, err := glamour.NewTermRenderer(glamour.WithStyles(gsStyle), glamour.WithWordWrap(width))
		if err != nil {
			return src
		}
		v, _ = renderers.LoadOrStore(width, r)
	}
	out, err := v.(*glamour.TermRenderer).Render(src)
	if err != nil {
		return src
	}
	return out
}

// RenderGo 渲染 Go 源码：包成 go 代码块走 RenderMD（gsStyle 低饱和 Chroma 配色）
func RenderGo(src string) string {
	return RenderMD("```go\n" + src + "\n```")
}

// HighlightWords 大小写不敏感地把 words 命中处上色，保留原大小写
func HighlightWords(line string, words []string) string {
	type span struct{ lo, hi int }
	var spans []span
	lower := strings.ToLower(line)
	for _, w := range words {
		w = strings.ToLower(w)
		if w == "" {
			continue
		}
		for i := 0; ; {
			j := strings.Index(lower[i:], w)
			if j < 0 {
				break
			}
			spans = append(spans, span{i + j, i + j + len(w)})
			i += j + len(w)
		}
	}
	if len(spans) == 0 {
		return line
	}
	// 合并重叠区间后从后往前替换，避免索引位移
	sort.Slice(spans, func(a, b int) bool { return spans[a].lo < spans[b].lo })
	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		if s.lo <= merged[len(merged)-1].hi {
			if s.hi > merged[len(merged)-1].hi {
				merged[len(merged)-1].hi = s.hi
			}
		} else {
			merged = append(merged, s)
		}
	}
	// 前向拼接：依序写出 命中前文段 + 上色命中段，最后补尾段
	var b strings.Builder
	last := 0
	for _, s := range merged {
		b.WriteString(line[last:s.lo])
		b.WriteString(matchStyle.Render(line[s.lo:s.hi]))
		last = s.hi
	}
	b.WriteString(line[last:])
	return b.String()
}

func Path(s string) string { return pathStyle.Render(s) }
func Dim(s string) string  { return dimStyle.Render(s) }
func Err(s string) string  { return errStyle.Render(s) }

var (
	goldStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true) // 品牌金粗
	labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))            // 赭：信息标签
	okStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("144"))            // 沙：成功语义
)

func Gold(s string) string  { return goldStyle.Render(s) }
func Label(s string) string { return labelStyle.Render(s) }
func OK(s string) string    { return okStyle.Render(s) }
