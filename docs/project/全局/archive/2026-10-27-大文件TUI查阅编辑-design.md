# 大文件 TUI 查阅/编辑 · 打开方式选择 design

> 创建于 2026-10-27 10:36（brainstorming 产出，AI 维护；开发者审核后生效）
> - 已归档：261027 随 大文件TUI查阅编辑 落地（go build + go test 全绿；TTY 手测清单见 plan Task 4）

## 背景与目标

gs 现行分流策略（`internal/edit/editor.go`）：≤20 行文件走 CLI 内编辑（EditInline），逾行（>20 行）文件一律转交外部编辑器（TRAE 优先，`--wait` 阻塞）。查阅侧（`internal/command/view.go` 的 `runViewPager`）对逾行文件同样直接转外部，gs 内没有任何逾行文件的查阅/编辑能力，导致百来行的笔记/日志必须离开 gs。

本设计为逾行文件补上**打开方式选择**：打开时用户可选

1. **gs 全屏 TUI 查阅/编辑**——markdown 渲染态浏览（可读性优先）+ 就地进入 gs 内编辑器；
2. **TRAE/外部编辑器打开**——维持现状链路。

设计约束（来自需求探讨）：排版好可读性强；编辑方便；不引入 vim 式复杂操作。

## 现状盘点（可复用资产）

| 资产 | 位置 | 结论 |
|---|---|---|
| `ui.RenderMD` | internal/ui/render.go | glamour 渲染 + 低饱和暖色 gsStyle（合规于配色纪律），宽度自适应（min(终端宽,100)）+ 按宽缓存渲染器。**直接可用** |
| `edit.EditInline` | internal/edit/inline.go | bubbletea 全屏编辑器，buffer/wrap 与行数无关（20 行只是策略阈值），自带滚动/软换行/CJK 列宽。**逾行编辑零开发** |
| `edit.OpenExternal` | internal/edit/editor.go | TRAE/code 家族自动注入 `--wait`，`.cmd` 包装走 `cmd /c`。**现状链路即所需** |
| `keySession` + termkey | command 包 / internal/termkey | 自绘全屏帧 + raw 单键基建，planview 浏览器与 selectFrom 已验证。**查阅器沿用此模式** |
| `selectFrom` | internal/command/prompt.go | 自绘单选菜单，Esc/Ctrl-C/D 返回 `ErrCancelled`（REPL 静默），非 TTY 降级编号列表。**打开方式选择直接复用** |
| termkey Kind 表 | internal/termkey/keyparse.go | **缺 PgUp/PgDn**（只有 ↑↓/Home/End），需小扩展 |

## 方案取舍

- **选定：termkey 自绘全屏查阅器 + 复用 EditInline/OpenExternal**。查阅是「渲染态」（`ui.RenderMD` 出成品，按行滚动窗口重绘），编辑是「原始态」（交给 EditInline 或 TRAE），两者天然分离，零编辑器自研。
- 否决：**gs 内自研全文编辑器**——百行文档下无语法高亮/多光标/AI，体验反而不如 TRAE，且 CJK 软换行光标映射是深坑，违背「编辑方便、避免复杂操作」。
- 否决：**把 pager（PagedPrint 分页）扩成渲染态视图**——渲染态全屏滚动与「打一页停一次」的行式分页模型冲突；全屏帧重绘（`drawSelectFrame`/planview 同款）更合适。
- bubbletea vs 自绘：查阅器用自绘（项目交互壳已从 bubbletea 迁出，raw 帧基建现成）；编辑态内部本就是 bubbletea（EditInline），被查阅器以「退 raw → 起编辑 → 回 raw」黑盒调用，不破坏迁出决策。

## 键位语义

| 键 | 查阅器内语义 |
|---|---|
| `↑`/`↓`/`j`/`k` | 滚动 1 行 |
| PgUp / PgDn / 空格 | 翻页（空格仅下翻；←/→ 保留光标移动语义，不绑翻页） |
| `g` / `G` / Home / End | 文首 / 文末 |
| `e` | 进入 gs 内编辑（EditInline；Ctrl+S 保存退出 / Esc 放弃，均回查阅并重渲染） |
| `o` | 退 raw 调 TRAE/外部编辑器（`--wait` 阻塞），关闭后回查阅并重渲染 |
| `r` | 重读文件重渲染（含终端缩放后的重排） |
| `q` / Esc / Ctrl-C / Ctrl-D | 退出查阅器回 `gs>` |

打开方式菜单（selectFrom）：`↑↓/jk` 选择、Enter 确认、**Esc 取消本次打开**（`ErrCancelled` 静默回 `gs>`，与全 gs 取消语义一致）。

## 详细设计

### 1. 打开方式选择（internal/command/view.go）

`runViewPager` 的逾行分支从「直接 `editOpen` 外部」改为 `runLargeFile(ctx, path)`：

- 非 TTY（`ui.PageSize()` false）：维持旧路，打印提示后 `edit.OpenExternal`，行为零变化；
- TTY：`selectFrom` 两项——「gs 查阅/编辑（全屏 TUI）」/「TRAE/外部编辑器打开」；
- 选 ext：打印旧提示文案 + `edit.OpenExternal`（关闭后直接回 `gs>`，旧语义）；
- 选 gs：进 `runMDViewer`。

`editOpen`/`edit.Open` 本身不动：短文件分流、logs 整篇直出路径、`/show o` 等其余直开点维持现状（边界见下）。

### 2. 全屏查阅器（internal/command/viewer.go，新建）

- **内容装配 `load()`**：读文件（失败显示错误占位）→ `ui.RenderMD` 渲染 → 按行拆分 → 每行 `ansi.Hardwrap(ln, width, true)`（charmbracelet/x/ansi，ANSI 感知强折，防超宽行撑破帧；width 与 RenderMD 同判 `min(ui.TermWidth(90), 100)`）。渲染结果缓存，滚动零重排。
- **帧绘制 `drawViewerFrame`**：状态行（路径 + `off+1..end/total` 行位置）+ 视口 h 行 + 键位提示行；`\r\x1b[J` 清屏区、`\r\n` 逐行（ConPTY 惯例）、提示行不带尾换行并以 `\x1b[{n}A\r` 归位帧首，整帧恰好占满屏不触发终端滚动。配色只经 `ui.Path`/`ui.Dim`/标题加粗，无新增彩色（纪律 2）。
- **主循环 `runMDViewer`**：`keySession` raw 取键，按键语义按上表分发；`e`/`o` 前 `sess.close()` 让出终端、结束后 `sess.resume()`（失败即退出查阅，与 pagerLoop 同模式）；`e` 路径 `edit.EditInline(path, data)`，tea 启动失败降级 `OpenExternal`；`o` 路径直接 `OpenExternal`。两路径返回后 `load()` 重读重渲染。
- **纯逻辑 `clampView(total, off, h)`**：视口顶行号夹回 `[0, max(0,total-h)]`，独立可测。

### 3. termkey 扩展（internal/termkey/keyparse.go）

新增 `KindPgUp` / `KindPgDown`（排在 KindEnd 之后），`String()` 补 `"pgup"`/`"pgdown"`；`parse()` 的 `ESC [` 分支补 `CSI 5~` / `CSI 6~` 解析：4 字节序列，残缺（只有 `ESC [ 5`）留 pending 拼包；`5`/`6` 后非 `~` 按未知序列降级为单 `KindEsc`（维持现有 default 语义）。

## 错误处理与边界

- 非交互终端：打开方式不询问，直接外部（旧路）；`runMDViewer` 内 `resume()` 失败兜底转外部编辑器。
- `selectFrom` 返回 `ErrCancelled` 时原样上抛，由 REPL 按既有语义静默——取消 = 放弃本次打开动作，无副作用（未发生任何写操作）。
- 渲染失败（理论上 glamour 失败时 RenderMD 返回原文）不新增错误路径。
- 查阅中途改终端宽高不自动重排，`r` 手动刷新（无 resize 监听）。
- 编辑态光标落在文档末（mdedit 既有「续写定位」，不改）。
- 代码块超宽行被 Hardwrap 强折，长表格可能折断对齐——可接受，后续需要再议横向滚动。

## 测试

- `keyparse_test.go` 追加：`ESC[5~`/`ESC[6~` → PgUp/PgDown；`5~` 分两块送达仍拼出一个键。
- `viewer_test.go`（新建）：`clampView` 表驱动（不足一页、范围内、越顶/越底、恰好到底、空文档）。
- 验收标准：`go build ./...` + `go test ./...` 全绿；TTY 手测清单见 plan。

## 涉及文件

| 文件 | 改动 |
|---|---|
| internal/termkey/keyparse.go | KindPgUp/KindPgDown 常量 + String + CSI 5~/6~ 解析 |
| internal/termkey/keyparse_test.go | 追加翻页键解析测试 |
| internal/command/viewer.go（新建） | clampView + drawViewerFrame + runMDViewer + runLargeFile |
| internal/command/viewer_test.go（新建） | clampView 测试 |
| internal/command/view.go | runViewPager 逾行分支改走 runLargeFile + 注释更新 |
| go.mod / go.sum | `go mod tidy`（x/ansi 转直接依赖） |
| docs/log/261027.md | 开发日志（实施时追加） |
