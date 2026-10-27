# project 模块升级：项目交互详情与 gs 自纳管设计

日期：2026-10-26
状态：**待开发者审阅**
所属模块：project（本文件为该模块目录首份文档，随本设计产生 `docs/project/project/`；本文件为该轮 brainstorming 产出的设计 spec）

---

## 1. 背景与动机（开发者原话整理）

- gs 定位是整个 Go 学习工作区的「平台大管家」，但 `/project` 的项目列表里缺少 gs 自身（当前 `internal/workspace/project.go:29` 对目录名 `gs` 硬编码排除）。希望 gs 能提示开发者将自身纳入项目列表，而不是靠手动改代码。
- 选中项目后只渲染一次性静态文本（README 摘录 + 三段文档列表 + 代码统计），是个「死胡同」页面。希望顺着项目入口继续深入，呈现该项目的学习情况。
- 详情页暂定为 5 块内容：README、框架树、学习笔记、项目日志、升级计划；**框架树和学习笔记是重点**。

## 2. 已确认的关键决策（brainstorming 问答记录）

| # | 问题 | 决策 |
|---|---|---|
| 1 | 项目列表管理模型 | **自动发现 + `.gs/projects.toml` 注册表**：gs 检测到自己未注册时，在选择列表尾部显示「＋ 将 gs（本工具）加入项目列表」，选中即写入注册表；之后 gs 正常出现在列表并带「本工具」标记 |
| 2 | 框架树介绍来源 | **AI 生成 + 缓存**：首次查看时 AI 逐节点生成介绍，存 `.gs/structures/<项目名>.toml`；之后读缓存秒开，提供「重新生成介绍」入口 |
| 3 | 学习笔记来源 | **两处聚合**：项目内 `docs/study/` + 工作区 `Study/sea/<项目名>/`，不迁移旧笔记 |
| 4 | 树与笔记联动 | **命名约定联动**：笔记文件名含源文件名即关联；树中文件带笔记计数标记，双向跳转 |
| 5 | 升级计划来源 | **约定正本文件** `<项目>/docs/project/升级计划.md`；不存在时可 AI 生成初稿 / 手动创建 / 返回 |
| 6 | 详情页结构 | 5 块内容循环菜单：README / 框架树 / 学习笔记 / 项目日志 / 升级计划 / 返回 |

开发者已确认的拍板项：① `/project <名>` 带参直达同样进交互菜单（不再是静态渲染）；② 介绍缓存生成后直接保存，不走 diff 确认（.toml 数据非代码）。

## 3. 项目列表与 gs 自纳管

### 3.1 注册表 `.gs/projects.toml`

```toml
[[projects]]
rel = "Project/gs"        # 相对工作区根的正斜杠路径
added = "2026-10-26"      # 加入日期
# 结构预留 status / note 字段（本版不启用，不写入）
```

- 注册表只存**手动加入**的项目；有效列表 = 自动发现 ∪ 注册表（按去重）。
- gs 自身识别沿用现有「目录名 == gs」约定；排除逻辑从硬排除改为**条件排除**：`name == "gs" && 未注册` 才隐藏。
- 注册表中的项目即使不满足 `isProjectDir` 标记（如刚建目录）也纳入列表，展示带「(手动)」后缀。

### 3.2 交互

- `/project` 无参 → huh 选择列表。若 gs 未注册，列表尾部追加特殊项「＋ 将 gs（本工具）加入项目列表」；选中即写注册表并提示成功，本次不进入详情（下次选择即见）。
- gs 已在列表时，选项文案为 `gs · 本工具（Project/gs）`。
- 列表为空且 gs 未注册：不再直接报错，仍显示提示项。

## 4. 详情页：循环菜单

选中项目后进入循环菜单（huh select），每块内容看完回到菜单，选「返回」退出到主 REPL：

```
项目 · HDU-KillCourse        Study/Project/HDU-KillCourse
> README
  框架树
  学习笔记
  项目日志
  升级计划
  返回
```

- 原 `renderProject` 静态渲染整体废弃（其文档列表职责由各块吸收）。
- `/project edit` 子命令保留不动。

## 5. 五块内容设计

### 5.1 README

- 读 `<项目>/README.md` 全文，markdown 渲染进 pager，不再截断 40 行。
- 无 README → 显示「（无 README.md）」并回菜单。

### 5.2 框架树（重点）

**扫描**：文件系统扫项目顶层文件与目录；选目录递归进入（列表末尾恒有「返回上级」）；忽略 `.git` 及 `config.Scan.Exclude` 中的段名。只列目录与源码/文档类文件（`.go` `.md` `.mod` `.sum` `.yaml` `.yml` `.json` `.toml` `.html` `.css` `.js`），其余忽略（降噪）。

**介绍**：每个节点一行「名字 + 一两句话介绍」，介绍来自缓存 `.gs/structures/<项目名>.toml`：

```toml
generated = "2026-10-26T15:04:05+08:00"

[[nodes]]
path = "internal/config"          # 相对项目根的正斜杠路径（目录不带尾斜杠）
desc = "配置加载与校验"

[[nodes]]
path = "main.go"
desc = "入口：加载配置、启动 Web 服务"
```

- 缓存命中 → 直接渲染；未命中且 AI 已配置 → 提示「AI 分析中…」，生成后**直接写缓存**（不走 diff 确认）再渲染。
- AI 输入：相对路径清单 + 文本类文件头部若干行（每文件 ≤ 40 行、总预算 ≤ 12000 字符，超出截断）；要求输出 JSON 数组 `[{path, desc}]`，`desc` ≤ 30 字。解析失败报错不写缓存。
- AI 未配置 → 纯树 + 介绍列显示「（未配置 AI，见 ~/.gs/config.toml）」。
- 树视图菜单底部提供「重新生成介绍（AI）」，覆盖缓存。

**节点操作**：选文件 → 弹操作菜单：

```
main.go   入口：加载配置、启动 Web 服务
> 在 TRAE 中打开
  查看相关笔记（2 篇）     # 无关联笔记时隐藏此项
  返回
```

选目录 → 进入下层；介绍缓存只按精确 path 匹配，子目录未生成介绍时显示名字即可（生成以顶层一轮为主，进入子目录时可对当层再触发生成，同规则）。

**笔记标记**：有关联笔记的文件行尾附主题色文本标记 `·笔N`（N 为篇数；遵守硬性规则 2：不用 emoji、不用彩亮饱和色）。

### 5.3 学习笔记（重点）

**来源聚合**，分组列出：

- 项目内：`<项目>/docs/study/**/*.md`，显示项目内相对路径；
- 笔记海：`Study/sea/<项目名>/**/*.md`，显示 `sea/<项目名>/...` 相对工作区路径；目录不存在则该组显示「（无）」。

**交互**：选中任一篇 → pager 阅读（渲染 markdown）；阅读中可：编辑（复用 `editOpen`）/ 在 TRAE 中打开对应源文件（见 5.6）/ 返回。

### 5.4 项目日志

汇总视图（一次性渲染进 pager）：

| 项 | 口径 |
|---|---|
| 创建时间 | `git log --reverse --format=%as` 首行；无 git 仓库 → 目录 ModTime 并标注「（目录时间）」 |
| 最近学习 | max(项目 `docs/log/*.md` 最新日期, git 最近提交日期)；两者皆无 → 「（无记录）」 |
| 学习日志 | 项目 `docs/log/` 篇数 |
| 提交 | 总数 `git rev-list --count HEAD`；近 30 天 `git rev-list --count --since="30 days ago" HEAD` |
| 代码规模 | 复用现有 `goStats` |

下方附「项目学习日志」日期列表（项目 `docs/log/*.md`，新在前），选中进入阅读；无则显示「（无）」。git 命令全部只读；git 不存在或非仓库时对应项降级显示，不报错。

### 5.5 升级计划

- 读 `<项目>/docs/project/升级计划.md` 全文，markdown 渲染进 pager；文件存在时另提供「编辑」入口。
- 不存在 → 弹三选项：
  1. **AI 生成初稿**（未配 AI 时隐藏）：输入 = README + 介绍缓存 + `docs/project/` 现有文档标题清单；产出走现有 AI 动作确认机制（create + 预览确认后原子写入正本路径）；
  2. **手动创建空文件**：写入仅含一级标题的空骨架（`# <项目名> 升级计划`），原子写；
  3. 返回。

### 5.6 树 ↔ 笔记联动（命名约定）

- **关联规则**：笔记文件名去 `.md` 后，包含源文件完整名（含扩展名，如 `main.go`，大小写不敏感）即关联；或包含源文件去扩展名基名且基名长度 ≥ 4（如 `main函数分析.md` ↔ `main.go`；避免 `go` 这类短名误匹配）。
- **正向**：框架树文件节点显示 `·笔N`；节点操作菜单多「查看相关笔记（N 篇）」，选中直接进入该笔记阅读（多篇时先选择）。
- **反向**：阅读笔记时，从笔记名反解源文件名（提取其中出现的 `.go` 文件名），在项目中按 basename 唯一命中 → 显示「在 TRAE 中打开对应源文件（main.go）」；解不出或不唯一则不显示。
- 匹配范围为该笔记所属来源的项目内 `**/*.go`（项目 docs/study 与 sea/<项目名> 均参与）。

## 6. TRAE 打开与配置

- `~/.gs/config.toml` 新增段：

```toml
[project]
opener = "trae"   # 打开源文件用的 IDE 命令，默认 trae
```

- 打开动作：`exec.Command(opener, <文件绝对路径>)`，`Start` 后不等待进程退出（IDE 异步接管）。
- `exec.LookPath` 失败 → 报错「未找到 <opener> 命令，请安装或在 ~/.gs/config.toml [project] opener 配置」。
- 说明：gs **程序**执行 `trae` / 只读 `git log` 属工具功能，与「AI 会话不执行 git 操作」的开发者纪律分属两回事，本设计不含任何会话级 git 写操作。

## 7. 代码架构

延续全局硬性规则 4「命令层纯逻辑与交互壳分离」：

```
internal/workspace/
└── registry.go   # ProjectsRegistry 类型；Load/Save（原子写）；Workspace.RegistryPath()
                 # 有效项目列表合并逻辑 Discover ∪ Registry、gs 条件排除

internal/structure/            # 新纯逻辑包
├── scan.go     # Scan(dir, exclude) []Entry（名字/是否目录/相对路径）；忽略规则在此
├── cache.go    # StructureCache Load/Save（.gs/structures/<项目名>.toml，原子写）
├── notes.go    # 聚合笔记来源、命名约定匹配 MatchNotes(goFiles, notes) map[string][]Note
│               # 、从笔记名反解源文件名 ReverseResolve
└── gitinfo.go  # GitStats(path)（创建/最近提交/总数/近30天）；exec 封装为可注入函数变量，测试打桩
                # log dates、goStats 由本包收口或复用 command 层现有函数后迁移

internal/command/project.go    # 壳：列表/提示项、菜单循环、huh 交互、pager、editOpen、
                               # AI 生成调用（复用 ensureAIState / applyActions 机制）、TRAE 执行
```

- `config` 包新增 `Project{ Opener string }` 段，默认 `trae`。
- `goStats` 从 command 层迁入 `internal/structure`（stats 归口）；`truncateLines` 仍被 ai.go 使用，保留在 command 包；`listDocs` 随 `/project edit` 保留。

## 8. 错误处理

- 未配 AI：框架树显示纯树 + 提示语；升级计划「AI 生成初稿」项隐藏；其余功能不受影响。
- 项目无 .git / git 命令不可用：项目日志对应项降级显示，不报错。
- 无 docs/study、无 sea/<项目名>、无 docs/log：对应组显示「（无）」。
- 介绍缓存 TOML 解析失败：视为无缓存，提示可「重新生成介绍」。
- AI 生成输出解析失败：报错不写缓存，可重试。
- TRAE 命令不存在：见第 6 节报错文案。
- 全程任何单块出错只影响该块，回菜单继续，REPL 不崩。

## 9. 测试与验收

- `internal/structure` 单测：扫描（含排除目录、非源码/文档文件过滤）、缓存往返、命名约定匹配边界（完整名 / 短基名 <4 不匹配 / 大小写 / 多笔记）、ReverseResolve 唯一性、GitStats 打桩。
- `internal/workspace` 单测：注册表 Load（不存在→空）/ Save 原子写、列表合并去重、gs 条件排除三态（未注册隐藏 / 注册后显示 / 非注册表普通项目不受影响）。
- command 层测试：菜单分发、提示项注入、五块内容组装的降级路径（扩展现有 `project_test.go`）。
- 验收标准：`go build ./...` + `go test ./...` 通过；真实工作区走通——gs 提示项出现并一键加入、KillCourse 菜单五块可达、框架树 AI 生成缓存后二次进入秒开、笔记双向跳转命中、升级计划空项目可生成初稿。
