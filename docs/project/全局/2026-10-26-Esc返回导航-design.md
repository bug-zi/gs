# Esc 返回导航 · 全层补齐 design

> 创建于 2026-10-26（brainstorming 产出，AI 维护；开发者审核后生效）
> 2026-10-26 修订：§3（选择菜单 Esc 取消）已由并行会话实现（见 docs/log/261026.md），本 design 据实修订；剩余范围为 §1 pager 单键化与 §2 AI 子模式 Esc。

## 背景与目标

gs 现行交互模型是「Enter 进入 / Tab 补全与展开收起」，但缺少与之相对的「返回」键：查阅 pager 要输 `q` + 回车，AI 子模式要输 `/back`，huh 选择框按 Esc 会冒出英文报错。本设计统一补齐 **Esc = 返回上一级**，形成「Enter 进、Esc 退」的完整闭环。

## 按键语义总表

| 层 | Esc | Enter |
|---|---|---|
| 项目树浏览器（project_tree_view.go） | 返回上级目录，根目录退出（**已有，不动**） | 进入目录 / 打开文件 |
| 查阅 pager（`/plan` `/sea` `/river` `/spring` `/logs` `/show` 及 project 子视图） | 退出视图回 `gs>` 提示符 | 翻下一页（同空格） |
| AI 子模式输入行 `ai>` | 返回主模式（等价 `/back`，存档逻辑不变） | 提交问题 |
| 主模式输入行 `gs>` | 关闭候选面板（现状保留） | 提交命令 |
| 选择菜单 selectFrom（prompt.go，已自绘 raw） | 安静取消整条命令，不输出错误（**已实现**） | 确认 |
| 确认/输入框 confirm / inputLine（暂留 huh） | 本次不处理（并行会话判定单字段无渲染 bug；Esc 中止仍冒英文报错，如需处理另行立项） | 确认 / 提交 |

## 方案取舍

查阅 pager 单键化只有一条可行路径：行缓冲模式下裸 Esc 读不出来，必须进 raw 模式。故采用：

- **双模式按键适配**（已选定）：TTY 下用现成 `keySession` + `termkey.KeyReader` 进 raw，逐键分发；非 TTY（管道/测试）降级为现有 `pagerKeys` 行读取。`ui` 包零改动。
- 否决：只改退出键（技术上不可行）、整体重写为 bubbletea 全屏组件（项目已从 bubbletea 迁出，不走回头路）。

## 详细设计

### 1. 查阅 pager 单键化（internal/command/view.go、show.go）

- 新增纯函数 `pagerKeyString(k termkey.Key) (string, bool)`：
  - `KindRune` → 小写字符串（如 `"e"` `"r"` `"q"` `"b"` `" "`）；
  - `KindEnter` → `""`（PagedPrint 语义：下一页）；
  - `KindEsc` → `"esc"`；`KindCtrlC` → `"ctrl+c"`；`KindCtrlD` → `"ctrl+d"`；
  - 其余（方向键、Home/End 等）→ 忽略，返回 false。
- `runPager` 改双模式：
  - TTY：`keySession`（复用 project_tree_view.go 同包基建）`resume()` 成功后，用 `termkey.KeyReader.Read()` 逐键经 `pagerKeyString` 转 string，进与降级路径同一个 switch；
  - 非 TTY：保持现有 `pagerKeys` 行读取不变。
- switch 新增 case：`"esc"`、`"ctrl+c"`、`"ctrl+d"` → `return nil`（退出视图）。附带收益：raw 模式下 Ctrl-C 不再强杀整个 gs 进程，而是优雅退出当前视图。
- `e` 编辑分支：先 `sess.close()` 让出终端，`editOpen` 结束后 `sess.resume()`，失败则走降级（同树浏览器 `r` 重生成介绍的处理模式）。
- `runShowPager`（show.go）同样改造：`n` `p` `o` `s` `q` 单键化，Esc/Ctrl-C/Ctrl-D 退出；其中 `o`（huh 选文件）、`s`（沉淀）、`e`（如有编辑）等任何子交互发生前 `sess.close()` 让出终端，结束后 `sess.resume()`，失败则走降级（同树浏览器处理模式）。
- `ui.PagedPrint` **不动**：翻页中途按 Esc 落入 default（跳完剩余页），随后键位提示行处 Esc 生效退出，行为自然衔接。
- 提示文案：`[e]编辑 [r]刷新 [q]退出` → `[e]编辑 [r]刷新 [q/Esc]退出`；show 的 `[q]退出` → `[q/Esc]退出`。降级行模式与 raw 共用同一 switch：输入文本 `esc` + 回车同样退出视图（可接受，无副作用）。

### 2. AI 子模式 Esc 返回（internal/repl/rawreader.go、repl.go）

- repl 包新增 sentinel：`var ErrEscBack = errors.New("esc back")`。
- `rawReader.Read` 的 `case termkey.KindEsc:`：当前 prompt 为 ai 提示符（`strings.HasPrefix(r.prompt, "ai")`）时 `return "", ErrEscBack`；否则维持现状（关闭候选面板）。主模式 `gs>` 的 Esc 关面板功能保留。
- `repl.Run` 循环：`errors.Is(err, ErrEscBack)` 时，若 `r.mode == "ai"` 则执行返回主模式逻辑后 continue；否则忽略重读。
- `repl.exec` 中 ai 模式 `case "back"` 的存档 + 切模式逻辑抽成方法 `backFromAI()`，Esc 与 `/back` 共用，行为零变化。
- `plainReader` 降级路径无 raw 按键，`/back` 命令保留兜底。
- AI 模式提示行 `AI 模式：直接输入问题，/save 存档，/back 返回主模式` 补 Esc 字样：`…，/back 或 Esc 返回主模式`。

### 3. 选择菜单 Esc 安静取消 —— 已由并行会话实现（2026-10-26 修订）

> 实施状态：并行会话已将 `selectFrom` 重写为 termkey raw 自绘菜单（↑↓/jk 移动、❯ 光标行、Esc/Ctrl-C/D 取消），
> 返回新增 sentinel `ErrCancelled`（prompt.go），`repl.exec` 已对其静默回提示符。效果与本节原设计一致，
> 实现方式由「huh 中止翻译」升级为「自绘菜单原生取消」，本节目标达成，不再需要实施。
> `confirm` / `inputLine` 并行会话判定暂留 huh；其 Esc 中止会冒 `huh.ErrUserAborted` 英文报错，
> 本次不处理，如后续需要另行立项。

原设计要点（存档备查）：command 包新增 `ErrCancelled` 哨兵；三个 huh 封装把 `huh.ErrUserAborted`
翻译为 `ErrCancelled`，中间调用方透传；`repl.exec` 对 `ErrCancelled` 静默。

## 错误处理与边界

- raw 模式不可用（非 TTY / MakeRaw 失败）一律降级行读取，不新增错误路径；`keySession` 的 defer close 保证终端状态必被还原（已有实现）。
- huh 的 Esc 取消语义 = 放弃本次整条命令操作（如「选到一半不选了」），安全无副作用：取消点都在任何写操作发生之前。
- pager 内嵌 PagedPrint 的 `b` 上一页、空格下一页行为不变。

## 测试

- `pagerKeyString` 表驱动单测：rune 大小写、Enter、Esc、Ctrl-C/D、方向键忽略。
- `backFromAI`：输出断言（空会话静默存档、mode 复位、提示文案）。
- 验收标准：`go build ./...` + `go test ./...` 全绿；TTY 手测各层 Esc 语义。

## 涉及文件

| 文件 | 改动 |
|---|---|
| internal/command/view.go | runPager 双模式 + switch 扩展 + pagerKeyString/pagerKeySource + 提示文案 |
| internal/command/show.go | runShowPager 单键化 + 提示文案 |
| internal/repl/rawreader.go | ErrEscBack + ai 提示符 Esc 分支 |
| internal/repl/repl.go | Run 处理 ErrEscBack + backFromAI 抽取 + 提示文案 |
| internal/command/prompt.go | **本次不动**（Esc 取消已由并行会话实现，ErrCancelled 已存在） |
| 测试文件 | view_test.go（pagerKeyString）、repl_test.go（backFromAI） |
