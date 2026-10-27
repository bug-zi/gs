# gs · Go 学习工作台 CLI 设计文档

日期：2026-10-24
状态：设计已确认，待制定实现计划

## 1. 背景与目标

`D:\Code\Go` 是 Go 语言学习总目录，包含学习目标、日志、笔记、计划、项目等，内容分散在多个目录且组织方式不一，人工维护困难。

`gs` 是一个用 Go 编写的交互式 REPL 终端工具，运行在学习工作区中，提供：

- **查找与展示**：日志、笔记、计划、项目的快速检索与渲染。
- **验收演示**：按主题聚合笔记、代码、日志、面试题，生成面向老师验收的演示视图。
- **读写编辑**：所有内容既可查看也可编辑（结构化追加、编辑器调起、AI 辅助改写）。
- **AI 辅助**：接入 LLM API，提供 `/AI` 交互模式与 `/review` 内容体检建议。

## 2. 已确认的核心决策

| 决策点 | 结论 |
|---|---|
| 交互形态 | 交互式 REPL（启动 `gs` 进入，`/exit` 退出） |
| `/show` 内容来源 | 实时自动搜索 + 可沉淀为 `topics.toml` 索引 |
| 写操作边界 | 全命令可读写：结构化追加 + 编辑器调起 + AI diff 确认写入 |
| 技术栈 | Go 标准库为主 + 少量精选库 |
| AI 能力边界 | 问答 + 文件修改以 diff 预览、用户确认后写入；不做全自动 agent |
| AI 协议 | OpenAI 兼容协议（base_url + api_key + model 三件套） |
| 工具名与位置 | `gs`，源码位于 `Project/gs/` |

## 3. 工作区布局假设

gs 运行于学习工作区（默认当前目录，`--root` 可覆盖）。判定标准：目录下存在 `Study/logs/` 与 `Study/plan/`。

```
D:\Code\Go\
├── Study\
│   ├── logs\YYMMDD.md          # 每日日志，东八区六位日期
│   ├── notes\GPT\*.md          # 复习笔记，按主题一文件
│   ├── plan\                   # Go实习面试学习计划.md / 学习状态.md / DOCX
│   └── Project\<name>\         # 学习项目：README、docs/log、docs/study、docs/project
├── Project\<name>\             # 实践项目（如 fileforge）
└── Resource\                   # 大资源，永不扫描
```

## 4. 命令总表

| 命令 | 功能 |
|---|---|
| `/help` | 显示 gs 使用说明（命令列表、参数、示例） |
| `/logs` | 今日日志（无今日日志则提示并落到最近一天） |
| `/logs <关键词>` | 按关键词过滤全部日志 |
| `/logs <MMDD>` | 按日期查看 |
| `/log add "<内容>"` | 向今日日志追加一条（超 40 汉字时警告并要求确认） |
| `/logs edit [MMDD]` | 调起编辑器编辑指定日（默认今日）日志 |
| `/notes` | 列出 `Study/notes/` 全部主题供选择 |
| `/notes <关键词>` | 模糊匹配主题并渲染笔记全文 |
| `/notes new <主题>` | 新建笔记文件 |
| `/notes edit <主题>` | 调起编辑器编辑笔记 |
| `/plan` | 显示 `学习状态.md` 当前断点摘要 + 计划阶段清单 |
| `/plan edit` | 调起编辑器编辑计划文件（先选：执行版/状态文件） |
| `/plan next "<断点描述>"` | 更新 `学习状态.md` 的断点字段 |
| `/project` | 列出所有发现的项目供选择 |
| `/project <名称>` | 聚合展示项目详情 |
| `/project edit <名称>` | 选择并编辑该项目的某个文档 |
| `/show` | 进入主题选择（沉淀主题 + 扫描热门关键词） |
| `/show <主题>` | 生成五节演示视图 |
| `/search <关键词>` | 全局搜索（/show 的底座，独立可用） |
| `/review` | AI 体检：笔记整理/合并/学习规划建议 |
| `/AI [问题]` | 进入 AI 子模式（提示符 `ai>`），`@文件` 引用上下文 |
| `/exit` | 退出 gs |

## 5. 核心交互设计

### 5.1 启动横幅

```
gs · Go 学习工作台 · D:\Code\Go
当前断点: KillCourse 总览核对 → 登录模块 | 目标: 2026-12-01 实习面试
上次学习: 10-24 (今天) · 连续 2 天
```

断点摘要解析自 `学习状态.md`；上次学习与连续天数由日志目录文件名推算。

### 5.2 `/show` 演示视图

五节结构，逐节浏览：`① 概览`（出现模块与时间线）`② 笔记` `③ 代码`（文件+行号）`④ 日志`（按时间）`⑤ 面试题`（路径或内容含 interview/面试 的 md）。

操作键：`n` 下一节、`p` 上一节、`o` 打开原文件、`s` 沉淀为索引、`q` 退出。

- 无索引时实时扫描生成；按 `s` 后写入 `topics.toml`，之后 `/show <主题>` 优先读索引。
- 沉淀条目中每个文件可写一句演示提示（如"重点讲 3 个 goroutine 泄漏的例子"），`/show edit <主题>` 可维护。
- 某节无内容时显示"（无相关内容）"并跳过。

### 5.3 编辑的实现方式

不内嵌编辑器，三层配合：

1. **结构化追加**：`/log add`、`/plan next` 走交互输入，带格式校验。
2. **编辑器调起**：`/logs edit`、`/notes edit` 等直接用系统编辑器打开目标文件。探测顺序：`$EDITOR` → `$VISUAL` → `code` → `notepad`。
3. **AI 辅助改写**：`/AI` 模式或 `/review` 中产生，一律以 diff 预览、确认后写入。

### 5.4 `/AI` 子模式

- 提示符变为 `ai>`，`/back` 返回主模式。
- 上下文 = 系统提示（角色设定 + 工作区概览：目录树、计划摘要、当前断点）+ 对话历史（滚动窗口，超长时裁剪最早轮次）+ 用户 `@文件` 引用的文件内容。
- AI 响应中的文件修改以 JSON 动作返回，gs 解析后转 diff 展示，用户逐条确认：`y` 应用 / `n` 跳过 / `a` 全部应用。
- `/AI <问题>` 单发模式：带上下文问一次即返回主模式，不进入子模式。

## 6. 架构

```
Project/gs/
├── cmd/gs/main.go          # 入口：解析 --root，装配依赖，进 REPL
├── internal/
│   ├── repl/               # REPL 循环、行编辑、命令路由、/AI 子模式状态机
│   ├── command/            # 每个命令一个文件（logs.go / notes.go / plan.go /
│   │                       #   project.go / show.go / search.go / review.go / ai.go）
│   ├── workspace/          # 工作区校验、路径定位、项目自动发现、日志日期推算
│   ├── search/             # 关键词扫描引擎：.md 按标题切分、.go 按函数、命中行上下文
│   ├── index/              # topics.toml 读写（沉淀索引）
│   ├── ai/                 # OpenAI 兼容客户端、对话组装、动作 JSON 解析、提示词模板
│   ├── diffx/              # 行级 LCS diff 生成
│   ├── edit/               # 编辑器探测与调起
│   ├── config/             # 配置加载与合并
│   └── ui/                 # Markdown 渲染、选择器/确认框封装、配色主题
└── docs/design.md
```

模块间只经构造函数注入依赖，`command/` 不互相调用；`search`、`index`、`workspace`、`ai` 均为无 UI 纯逻辑包，可独立测试。

### 6.1 精选依赖

| 库 | 职责 |
|---|---|
| `ergochat/readline` | REPL 行编辑、历史、上下键 |
| `charmbracelet/huh` | 选择器、确认框、输入表单 |
| `charmbracelet/glamour` | Markdown 终端渲染 |
| `charmbracelet/lipgloss` | 配色与布局（huh/glamour 已带入） |
| `BurntSushi/toml` | topics.toml 与 config.toml 解析 |

AI 调用不用 SDK：`net/http` + `encoding/json` 直接 POST。行级 diff 手写。

**风险预案**：若 `ergochat/readline` 在 Windows 中文输入下光标错位，`repl` 包降级为 `bufio.Scanner` + 内存历史数组，接口不变，其余模块不受影响。M1 开工前先做 30 分钟 spike 验证。

## 7. 数据设计

### 7.1 沉淀索引 `<root>/.gs/topics.toml`

```toml
[[topics]]
name = "context"
created = "2026-10-24"
files = [
  { path = "Study/notes/GPT/Go并发与互斥锁.md", hint = "" },
  { path = "Study/Project/feedsystem_video_go/internal/feed/context.go", hint = "重点讲超时控制" },
]
logs = ["261021", "261023"]
interview = "Study/Project/feedsystem_video_go/doc/interview-questions.md"
```

`path` 一律为工作区相对路径（正斜杠）。文件被移动/删除时展示前校验存在性，失效条目标灰并提示清理。

### 7.2 配置（合并顺序：全局为基底，工作区覆盖非空字段）

- 全局：`~/.gs/config.toml`
- 工作区：`<root>/.gs/config.toml`
- API key 另可走环境变量 `GS_API_KEY`（优先级最高）

```toml
[ai]
base_url = "https://api.example.com/v1"
api_key  = ""            # 建议留空走环境变量
model    = "deepseek-chat"
temperature = 0.3

[scan]
exclude = [".git", ".codegraph", ".agents", "Resource", "node_modules", "_docx_render"]
max_file_kb = 512        # 超过则跳过
```

未配置 ai 段时：`/AI`、`/review` 提示配置方法后返回，其余命令完全不受影响。

### 7.3 `学习状态.md` 结构化更新规则

解析「## 当前断点」小节内的 `- 键：值` 行。`/plan next "<描述>"` 更新 `下一步` 键：存在则替换该行，不存在则在「## 当前断点」小节末尾追加。其余键不改不删。同时把 `更新时间` 行刷新为当日。

## 8. 搜索引擎设计

- **范围**：`Study/` 与 `Project/` 全目录；排除 `scan.exclude` 与超过 `max_file_kb` 的文件。
- **文件类型**：`.md`（解析标题层级，按小节切分为检索单元）、`.go`（按包+函数粗切分）。其他扩展名跳过。
- **匹配**：关键词大小写不敏感，支持中英文与多词 AND（空格分隔，全部命中才收录）。
- **输出单元**：`{文件路径, 小节/函数名, 行号, 命中行及前后各 1 行上下文}`。
- **性能**：当前工作区 md 约 250 个，全量扫描为毫秒级，不做持久化索引。
- **`/show` 热门关键词**：无参数进入主题选择时，除沉淀主题外，从日志与笔记标题中提取高频词（去停用词）供快速选择。

## 9. AI 集成设计

### 9.1 协议

单文件 `chat/completions` 调用：`POST {base_url}/chat/completions`，Bearer 鉴权，流式关闭（非流式简化解析）。超时 120 秒。

### 9.2 修改动作协议

系统提示要求 AI 在需要改文件时，于回复末尾输出 fenced json 块：

```json
{"actions": [
  {"file": "Study/notes/GPT/xxx.md", "action": "edit", "content": "<完整新内容>"},
  {"file": "Study/notes/GPT/新主题.md", "action": "create", "content": "<初始内容>"}
]}
```

gs 提取后：`create` 直接显示将创建的文件与内容摘要；`edit` 将 `content` 与磁盘现状做行级 diff 展示（增绿删红）。逐条确认后才写盘；拒绝则丢弃，不部分应用同一条。只允许写工作区内 `.md`/`.go`/`.toml` 文件，越界路径直接拒绝。

### 9.3 `/review` 体检

单次调用。输入：全部笔记文件 + 近 30 天日志 + `学习状态.md` + 计划阶段清单（超长则各取摘要）。系统提示要求输出三类建议：**笔记整理**（结构优化、过期内容）、**合并建议**（主题重叠的笔记）、**学习规划**（结合断点与目标的下一步）。建议以 Markdown 分节展示；若 JSON 动作块存在，同样走 diff 确认通道。

## 10. `/project` 项目发现与聚合

- **发现规则**：扫描 `Study/Project/*/`、`Project/*/`、`Study/*/`（一层）中含 `go.mod`、`README.md` 或 `.git` 的目录。
- **聚合视图**：① README 介绍（渲染）② 学习日志（`docs/log/`）③ 走读笔记（`docs/study/`）④ 设计文档（`docs/project/`）⑤ 代码概况（`.go` 文件数与总行数）。缺失的小节显示"（无）"。

## 11. 错误处理

- 工作区校验失败：提示缺少 `Study/logs|plan`，建议 `gs --root <路径>`。
- 所有文件写入统一 UTF-8；写前读一次确认存在性，笔记/日志追加失败不产生半截文件（先写临时文件再 rename）。
- 编辑器调起失败：回退链走完后，提示将 `editor = "..."` 写入 `<root>/.gs/config.toml`（`[ui]` 段），优先级置于 `$EDITOR` 之前。
- AI 请求失败（网络/鉴权/超时）：显示原始错误与配置检查指引，不影响 REPL 存活。
- 命令输入的未知命令：提示最近似命令（编辑距离 ≤2 建议）。

## 12. 测试策略

| 层 | 方式 |
|---|---|
| `search` `index` `workspace` `diffx` `ai`(JSON 提取) | 表驱动单元测试 |
| `command` 各命令 | `t.TempDir()` 搭建迷你工作区 fixture 做集成测试 |
| AI 客户端 | `httptest.Server` 模拟 OpenAI 兼容端点 |
| `repl`/`ui` | 保持薄，不写自动化测试，靠真实使用验收 |

## 13. 实施分期

- **M1 日常维护**：工程骨架 + REPL + `/help` `/logs` `/log add` `/logs edit` `/notes`(new/edit) `/plan`(edit/next) `/search` + 配置与编辑器调起。readline 中文 spike 在此期最前端。
- **M2 验收演示**：`/show`（扫描、五节视图、沉淀索引）+ `/project`（发现、聚合）。
- **M3 AI 增强**：OpenAI 兼容客户端 + diff 通道 + `/AI` 子模式 + `/review` 体检。

每期结束时独立可用、可日常启用，下一期不依赖上一期的未完成部分。
