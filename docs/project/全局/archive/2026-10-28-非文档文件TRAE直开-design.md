# 非文档文件 TRAE 直开 · 设计

- 日期：2026-10-28
- 状态：已确认（开发者已逐项拍板三问）
- 涉及模块：`internal/edit`、`internal/command`（打开行为为全局横切能力，归属全局）

## 背景与目标

gs 现在打开非文档文件（如 `.go` 代码）时，会先走 gs 内嵌查阅/编辑：短文件（≤20 行）进内嵌编辑，逾行文件弹「gs 查阅/编辑 vs TRAE/外部编辑器」菜单。开发者的习惯是代码文件一律在 TRAE 中打开。

目标：**凡非文档文件，在 gs 中一律直接调起 TRAE，不再进 gs 内嵌查阅/编辑、不再弹打开方式菜单；文档文件（.md 等）完全维持现状。**

## 已确认决策

1. **直接跳转（方案 A）**：非文档文件跳过 gs 内嵌查阅/编辑与逾行菜单，一律直接调起 TRAE。
2. **文档集合**：`.md` `.markdown` `.txt` `.doc` `.docx` `.pdf` 算文档（保持 gs 现有流程）；其余一切扩展名（含无扩展名）都走 TRAE。
3. **调起方式（方案 A）**：复用现有编辑器解析链（`[ui] editor` > `$EDITOR` > `$VISUAL` > trae > code > notepad，trae/code 自动 `--wait`）；「默认 TRAE」由兜底顺序保证，显式配置可覆盖，trae 失效时可降级 code/notepad。不新增配置项。

## 改动设计

### 1. 判定逻辑（internal/edit/editor.go）

- 新增 `IsDocFile(path string) bool`：取 `filepath.Ext` 转小写，命中文档集合返回 true；无扩展名返回 false（非文档）。
- `edit.Open` 顶部短路：`!IsDocFile(path)` → 跳过长短分流（不进内嵌编辑），直接 `openExternal(path, cfg)`。

### 2. 查阅分流（internal/command/view.go）

- `runViewPager` / `runViewPagerDel` 在现有逾行检查**之前**加非文档短路：`editPath()` 命中且 `!edit.IsDocFile(path)` → 打印「正在打开 <path> ...（编辑器关闭后回到 gs）」→ `edit.OpenExternal(ctx 配置)` 阻塞至编辑器关闭。
- 效果：非文档文件无论长短都不再进 gs 全屏查阅、不弹「逾行打开方式」菜单；文档文件路径逐字节保持现状（含逾行菜单、`[o]` TRAE 键）。

### 3. 各入口落点（自动覆盖，无需额外改动）

| 入口 | 文件类型 | 行为 |
|---|---|---|
| `/show o` 打开节的文件、索引 topics.toml | .go / .toml | TRAE 直开 |
| tier `code` add、ship code 文件 | .go | TRAE 直开 |
| `/plan edit` roadmap.toml | .toml | TRAE 直开 |
| `/plan edit` 计划文件 | .md / .docx | 现状不变 |
| 日志、笔记、spring、ship 笔记 | .md | 现状不变 |
| 项目「打开对应源文件 / 用 trae 打开」`openInIDE` | 任意 | 已走解析链，不动 |

## 错误处理

- 沿用 `openExternal` 现有错误：解析链全空 →「未找到可用编辑器，请在 .gs/config.toml 的 [ui] 段配置 editor = "..."」。
- 非交互环境：`OpenExternal` 不依赖 raw 模式，天然可用；`runViewPager` 短路发生在进入 raw 之前，无终端态问题。

## 测试

- `internal/edit`：
  - `IsDocFile` 表驱动：文档集合各扩展名、非文档扩展名（.go/.toml/.json）、无扩展名、大写扩展名（.MD）。
  - `Open` 分流：非文档短文件 + 配置指向不存在的编辑器 → 返回「未找到可用编辑器」错误（证明未进内嵌、走了外部链路）；文档短文件仍走内嵌（现状回归）。
- `internal/command`：
  - `runViewPager` 非文档短路：同法断言走外部；文档逾行文件仍弹菜单（现状回归）。

## 明确不改

- 解析链顺序、`--wait` 语义、`.cmd/.bat` 经 `cmd /c` 启动的包装逻辑。
- `openInIDE`（含 `[project] opener` 异步语义）。
- 逾行菜单本身与全屏查阅器（文档仍用）。
- 配置结构与默认值（零新增配置）。
