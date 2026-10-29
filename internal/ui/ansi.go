package ui

// 交互壳手绘 ANSI 共用色板：与 render.go 的 lipgloss 主题同族（220 金 → 179 赭 → 144 沙），低饱和、禁彩亮
const (
	ANSIGold      = "\x1b[38;5;220m\x1b[1m" // 金粗：品牌字/选中行箭头
	ANSIOchre     = "\x1b[38;5;179m"        // 赭：banner 标签
	ANSIOchreBold = "\x1b[38;5;179m\x1b[1m" // 赭粗：ai 提示符
	ANSIBright    = "\x1b[38;5;255m"        // 亮白：选中项文字
	ANSIMid       = "\x1b[38;5;245m"        // 中灰：未选项
	ANSIIdle      = "\x1b[38;5;240m"        // 暗灰：箭头/提示行
	ANSIFaint     = "\x1b[2m"               // 弱化字重：与 lipgloss Dim 同效（已消费行选中时保持灰字用）
	ANSISelBg     = "\x1b[48;5;236m"        // 选中行微底
	ANSIItalic    = "\x1b[3m"               // 斜体
	ANSIReset     = "\x1b[0m"
)

// 五层站点点缀色：与主题金赭沙同亮度带的低饱和色（禁彩亮），/plan 各层小节标题区分用
const (
	ANSISea    = "\x1b[38;5;110m" // 雾蓝：sea 来源
	ANSIShip   = "\x1b[38;5;179m" // 赭：ship 加工中（主题中段色）
	ANSIRiver  = "\x1b[38;5;72m"  // 黛青：river 沉淀
	ANSISpring = "\x1b[38;5;108m" // 苔绿：spring 我的总结
	ANSICode   = "\x1b[38;5;139m" // 藕紫：code 演示代码
)
