# plan 模块重构：交互式树状浏览器设计

日期：2026-10-26
状态：**已落地（2026-10-26）**——roadmap v2 / planview / command 装配全量实施；v2 初稿 8 主干 102 知识点生成（算法与面试主干经开发者编辑内容清单时删去）
所属模块：plan（`docs/project/plan/` 本目录；取代「知识绑定」设计的数据模型部分）
前置说明：建立在路线图升级轮已落地的 `internal/roadmap` 包之上，对其类型与 schema 做 v2 重构

---

## 1. 背景与动机（开发者原话整理）

- 现在的 `/plan` 只是一个外壳：平铺展示全部叶子，**无法选中、无法点进去进一步查看**。
- 底层是「一个整个的 toml 文件」，不容易编辑也不容易读取。
- 期望：`/plan` 首屏只见一级主干（`Go 语言基础 ··· 0/22`、`并发编程 ··· 0/16` …）；主干行 **Tab 展开/再 Tab 收起**二级；**Enter 进入**该层级；知识点点进去显示其关联的 **sea/river/spring 笔记**。
- **spring 深度绑定**：sea、river 笔记可多份，但每个知识点的 spring **有且仅有一份**——spring 是自己对该知识点的总结，天然唯一。
- 流程：先确认设计 → 商讨确认一级/二级标题内容 → 重建 /plan 模块。

## 2. 已确认的关键决策（brainstorming 问答记录）

| # | 问题 | 决策 |
|---|---|---|
| 1 | 层级结构 | **知识点升为二级**：一级=主干、二级=知识点（如 变量与常量、select）、三级=笔记。原枝干层（语法核心等）降为知识点上的 `group` 分组标签，不再单独成层 |
| 2 | 主干行 Enter 语义 | **进入独立主干页**（只显示该主干的知识点列表，分组小标题排列）；Tab 留在全局树原位展开/收起，两套并存 |
| 3 | 知识点详情页操作 | **打开笔记（↑↓ 选、Enter/o 阅读）+ d/s/t 原位状态标记**。流转入口 r/w、挂载 a **不进浏览器**，留在命令层（`/river add`、`/spring add`） |
| 4 | 存储形态 | **重排后的单 TOML**（version 2）：`branch → point`，point 带 `group/status/tags/spring/river/sea`；日常维护全走命令与树内操作，手编兜底 |
| 5 | 浏览器实现 | **方案甲：新建 `internal/planview` 自研轻量 raw 浏览器**（MakeRaw + 最小按键解析 + 整帧 ANSI）；抽共享按键包（动刚稳定的 repl v2）与回 bubbletea（已证伪）均否 |
| 6 | 返回键 | **Esc = 返回上一层**（全局树上 Esc = 退出浏览器）；q = 任意层直接退出浏览器；笔记阅读内部沿用现有 pager 键位（q 退出阅读回详情页） |
| 7 | spring 绑定机制 | TOML 单值字段 + **文件名约定双保险**：`Study/spring/<知识点标题>.md`；知识点标题全局唯一保证不冲突 |

## 3. 数据模型 v2

```toml
version = 2

[[branch]]
title = "并发编程"

  [[branch.point]]
  title = "select"              # 知识点（二级），全局唯一
  group = "基础原语"             # 分组标签（原枝干），可空
  status = "doing"              # todo(缺省,可省略) | doing | done
  tags = ["高频"]
  spring = "Study/spring/select.md"          # 0 或 1（单值 = 天然 1:1）
  river = ["Study/river/select语义.md"]      # 0..N
  sea = ["Study/sea/ai/2026-10-26-1430.md"]  # 0..N
```

约束：

- **知识点标题全局唯一**（Add 查重从「同枝干」改为「全树」），支撑 spring 文件名约定与浏览定位。
- `status` 缺省视为 `todo`；非法值解析报错（沿 v1 校验）。
- 绑定路径一律**工作区相对路径**（含 `Study/` 前缀，与出处链同规范）；文件不存在时视图标 `[失效]`，不自动清理。
- v1 的 `link`/`links` 字段与 `topic` 层废止；`note` 可选字段保留。
- 文件写入一律原子写；重编码丢手写注释属预期（命令维护为主）。

**迁移**：不写迁移工具。v1 的 80 叶内容本就待重议，v2 初稿由「内容商讨」（见第 9 节）重新生成；当前 v1 无任何绑定数据可迁（YAGNI）。

## 4. 交互设计（三层视图）

| 视图 | 内容 | 按键 |
|---|---|---|
| **全局树**（首屏） | 一级主干列表 + 进度；Tab 展开的主干下平铺知识点（分组小标题 + 状态行） | ↑↓ 移光标（自动跳过分组标题行）；主干行：Tab 展开/收起、Enter 进主干页；知识点行：Enter 进详情页；Esc=退出浏览器；q=退出浏览器 |
| **主干页** | 单主干全部知识点，按分组小标题排列 | ↑↓；Enter→详情页；Esc/q=返回全局树；d/s/t 原位标记 |
| **知识点详情页** | 标题行（知识点 · 状态 · 主干/分组）+ 三段清单：spring ×0/1（未建提示 `/spring add <知识点>`）、river ×N、sea ×N | ↑↓ 选笔记行；Enter/o 打开 pager 阅读；d/s/t 标记状态；Esc/q=返回主干页 |

- 状态标记**即时落盘**（原子写）+ 进度行即时刷新；落盘失败底部错误行提示，不退出浏览器。
- **模式切换**：planview 为 raw 模式整帧渲染；打开笔记前退 raw → 行式 pager（现有 `runViewPager`）→ 返回后重进 raw（进退路径沿用 repl v2 验证过的 MakeRaw/Restore 模式，defer 兜底绝不残留 raw 状态）。
- **降级**：stdout 非 TTY 或 MakeRaw 失败 → 静态输出（首屏=一级树；`/plan <主干>` = 该主干静态树，即现有 render 保留为降级通道）。
- 渲染配色沿用 ui 主题系（低饱和暖色），状态符号 `[x]/[~]/[ ]`、「高频」星标、dot 引导线惯例不变；无 emoji。

## 5. 命令面调整

| 命令 | 行为 |
|---|---|
| `/plan` | 默认 = 交互浏览器 |
| `/plan <主干关键词>` | 打开浏览器并预展开、光标定位该主干；非 TTY 输出静态树（零匹配报错列全部主干名，沿现状） |
| `/plan done/doing/todo <关键词>` | 保留（参数快捷通道）；匹配改为知识点标题三态 |
| `/plan status / next / edit` | 保留不动 |
| `/plan add` | 保留；参数改 `branch\|point`，point 全局查重，`group` 可选参数 |
| `/spring add <知识点关键词>` | **重构**：定位知识点 → 创建 `Study/spring/<知识点>.md`（占位提示行）→ 自动绑定；已存在则提示并直接打开编辑。spring 1:1 的唯一创建入口 |
| `/river add` | 流程尾部追加一步「挂到知识点？（关键词过滤，回车跳过）」→ 命中则写入 point.river |
| `/sea` 侧 | 不变；sea 来源挂载走 `/river add` 追问之外的手编/后续挂载命令（v1 不做专用命令，浏览器只读展示） |

范围外（v1 不做）：浏览器内流转（r/w）与挂载（a）；跨主干多选展开记忆（会话内展开态不持久化）；知识点 rename/move 命令（手编 TOML 兜底）。

## 6. 代码架构

```
internal/roadmap/          # v2 重构（纯逻辑，不 import ui）
├── roadmap.go  # Roadmap/Branch/Point 类型（v2 schema）；Load/Parse/Save/Validate（全局查重）
├── ops.go      # MatchPoints 标题三态、SetStatus、进度统计（按 point）、
│               # AddBranch/AddPoint（全局查重）、BindSpring/BindRiver/BindSea（幂等写入）
└── render.go   # 静态树渲染保留（降级通道）；Glyph/ColorFn 惯例不变

internal/planview/         # 新包：交互树浏览器
├── state.go    # 纯状态机：光标、展开集合、视图栈、d/s/t 迁移——表驱动可测
├── keys.go     # 最小按键解析：ESC 序列 ↑↓、Tab(\t)、Enter(\r)、Esc(\x1b)、d/s/t/q，其余忽略
├── view.go     # 整帧 ANSI 渲染（反白光标行、\x1b[J 清帧、runewidth 列宽）+ MakeRaw 进退
└── browser.go  # 三层视图循环；进 pager 前退 raw、返回重进 raw；标记经回调落盘
```

- `command/plan.go` 只做装配：加载 roadmap + 注入回调（OpenNote=pager、OnStatusChange=Save）调 `planview.Run`；`plan <主干>` 传初始定位。
- `roadmap` 包保持不 import ui（ColorFn 注入惯例）；`planview` 是交互壳可 import ui；不 import repl，不改动刚稳定的 repl v2 输入层。
- workspace 层无新增（RoadmapFile 已有）。

## 7. 错误处理

| 场景 | 行为 |
|---|---|
| roadmap.toml 缺失 | 提示「尚未生成」+ `/plan status` 替代入口（沿现状），不自动创建 |
| TOML 解析失败 | 行列号报错 + 提示 `/plan edit` 修复，REPL 不崩 |
| 非 TTY / MakeRaw 失败 | 静态降级输出 |
| 绑定路径文件不存在 | 详情页标 `[失效]` |
| `/spring add` 已存在 | 提示已有并直接打开，不重复创建 |
| 浏览器内落盘失败 | 底部错误行提示，不退出 |
| raw 异常退出 | defer Restore，绝不残留 raw 状态 |

## 8. 测试与验收

- `internal/roadmap` v2 单测：v2 解析/序列化回读、Validate（全局查重、status 校验）、MatchPoints 三态、进度统计、Bind 系列幂等写入、静态渲染快照。
- `internal/planview` 单测：状态机表驱动（光标跳过分组行、Tab 展开收起、视图栈 push/pop、d/s/t 迁移、未知键忽略）、按键解析（ESC 序列整块/分块）、整帧渲染快照（stripANSI 比对）。
- command 层：`/spring add` 创建+绑定（含已存在分支）、`/river add` 挂载追问（可跳过）、`/plan` 装配降级。
- 验收：`go build ./...` + `go test ./...` 全绿；真机冒烟——Tab 展开/收起、Enter 三层进入、Esc 逐层返回、d/s/t 即时刷新进度、Enter 打开笔记（TRAE/pager）、`[失效]` 标注、非 TTY 降级、`/spring add` 全流程。

## 9. 后续步骤（开发者指定顺序）

1. **内容商讨**：重议一级主干与二级知识点清单（现 v1 的 9 主干/80 叶/枝干分组全部可调），产出 v2 初稿 `roadmap.toml`。
2. 实施计划（writing-plans）→ 重建 /plan 模块。

## 10. 取代关系

- `2026-10-26-知识绑定-design.md`：**数据模型部分被本设计取代**（`links[]` 数组、`plan link/unlink`、分层分组视图方案废止）；其「/review 未挂载统计」思路在新模型下（sea/river 不被任何 point 引用）保留待重议。该 spec 状态改「已被取代」。
- `2026-10-26-路线图升级-design.md`：实施结论有效；其 `links` 字段修订进一步演化为 `spring/river/sea` 三字段（见本设计第 3 节）。

## 商讨记录

- 2026-10-26 建立商讨稿：开发者真机试用后提出「只见图谱外壳、无法点选深入、TOML 不易读」三层痛点，要求按多层树状浏览器重构，并给出 Tab 展开/Enter 进入/spring 唯一绑定等原始设计。
- 2026-10-26 层级定案：知识点升为二级（枝干降为分组标签），三级=笔记（sea/river 多份、spring 1:1）。
- 2026-10-26 交互定案：Enter 进入独立主干页（与 Tab 原位展开并存）；详情页操作=打开笔记+d/s/t 原位标记（r/w/a 不进浏览器）。
- 2026-10-26 存储定案：重排后单 TOML v2；spring 单值字段+文件名约定双保险；知识点全局唯一。
- 2026-10-26 实现定案：方案甲自研轻量 raw 浏览器（internal/planview）；Esc 统一返回（开发者修正），q 任意层退出。
- 2026-10-26 设计五节逐节确认，落盘本 spec；同步标注知识绑定 spec 被取代、路线图升级 spec 修订记录。
