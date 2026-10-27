# gs · Go 学习工作台

一个用 Go 编写的交互式 REPL 终端工具，运行在 Go 学习工作区中，围绕「知识存在哪、怎么流、怎么讲给人听」组织学习资料：

- **三层知识体系**：sea（外部知识）→ river（摘录沉淀）→ spring（原创输出），出处链自动溯源，`/review` 反查未摘录与待改写
- **学习路线图**：`roadmap.toml` 树状知识地图，交互浏览器里 Tab 展开、Enter 深入、d/s/t 原位标记进度；知识点与三层笔记双向绑定
- **Agent 化 AI**（可选）：`/AI` 自主检索工作区作答（读自动、写审批），会话一键存入大海；`/review` 学习体检
- **项目大管家**：`/project` 循环菜单聚合 README、框架树（AI 生成节点介绍）、学习笔记、项目日志与升级计划

## 构建与运行

需要 Go 1.27+：

```bash
go build ./cmd/gs        # 构建出 gs 可执行文件
go run ./cmd/gs          # 或直接运行
```

## 快速开始

gs 运行于「学习工作区」：启动时从当前目录向上探测（`--root` 可显式指定），判定标准是目录下存在 `Study/logs/` 与 `Study/plan/`：

```
D:\Code\Go\                  ← 工作区根目录
├── Study\
│   ├── logs\YYMMDD.md       # 每日日志（东八区六位日期）
│   ├── plan\
│   │   ├── 学习状态.md       # 断点与阶段清单
│   │   └── roadmap.toml     # 学习路线图正本（branch → point）
│   ├── sea\                 # 大海：外部知识（剪藏、AI 问答存档、面试题正本）
│   │   ├── ai\              #   /AI 会话存档落点
│   │   ├── clip\            #   剪藏归位处
│   │   ├── interview\       #   面试题正本
│   │   └── inbox\           #   导入中转：外部 md 丢这里
│   ├── river\*.md           # 河流：从大海摘录的沉淀，条目自动带出处行
│   └── spring\*.md          # 泉水：自己的理解，与路线图知识点一一对应
├── Project\<name>\          # 实践项目
└── Resource\                # 大资源（默认不扫描）
```

进入工作区目录后运行 `gs`，出现启动横幅与 `gs>` 提示符即成功；`Study/sea/inbox/` 有待导入文件时启动会提示数量：

```
gs · Go 学习工作台 · D:\Code\Go
当前断点: 登录模块源码走读
目标: 2026-12-01 实习面试
上次学习: 10-24 (今天) · 连续 2 天
输入 /help 查看命令，/exit 退出
```

## 三层知识体系

知识按「离自己多近」分三层流动，每层一主题一文件：


| 层          | 定位                                                       | 录入通道                                                                                |
| ------------- | ------------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| sea 大海    | 外部知识：剪藏文章、AI 问答，原样粘贴，不完全属于自己      | `/sea import`（inbox → clip 逐个起标题迁入）、`/AI` 会话存档（`/save` 或离开时确认）   |
| river 河流  | 摘录沉淀：从大海筛出值得反复看的内容                       | `/river add`：选 sea 条目 → 预填「出处 + 全文」→ 编辑器删减保存 → 尾部追问挂到知识点 |
| spring 泉水 | 原创输出：自己对该知识点的理解，**每个知识点有且仅有一篇** | `/spring add <知识点>`：创建占位条目并自动绑定 → 编辑器手写（设计上只留手写入口）      |

 river 条目头部自动带出处行 `> 出处：Study/sea/<来源>.md`，是 `/review` 三层统计（未摘录 / 待改写）与溯源的数据基础。绑定关系正本在 `roadmap.toml`：知识点带 `spring`（0/1 单值）、`river`（0..N）、`sea`（0..N）三字段；`/river` `/spring` 查阅按主干分组，未绑定的归「未挂载」组。

## 命令一览

```
/logs [关键词|日期]       查看日志：无参=今天(无则最近一天)；4位月日(1025)或6位年月日(261025)按日期查
/logs edit [日期]        调起编辑器编辑日志（默认今天，不存在则创建）
/log add "<内容>"        向今日日志追加一条（超过 40 字会确认）
/plan                    交互式路线图浏览器（Tab 展开/收起，Enter 进入，d/s/t 标记，Esc 返回，q 退出）
/plan <主干关键词>        打开浏览器并定位该主干；非交互环境输出静态树
/plan status             学习状态速览
/plan next "<断点>"      更新 学习状态.md 的「下一步」
/plan done|doing|todo <关键词>   快捷标记知识点（浏览器内 d/s/t 为主路径）
/plan add branch|point   建树录入（point 标题全局唯一，可带分组）
/plan edit               编辑 roadmap.toml / 计划文件
/sea [关键词]            大海查阅（按一级子目录分组两级选择，关键词平铺直选）
/sea import              inbox → clip 整理：逐个预览、起标题、迁移（重名跳过）
/sea edit [关键词]       编辑大海条目
/river [关键词]          河流查阅（按 roadmap 主干分组，未绑定归「未挂载」组）
/river add [关键词]      摘录：选 sea 条目 → 预填出处+全文 → 编辑器 → 追问挂到知识点
/river edit [关键词]     编辑河流条目
/spring [关键词]         泉水查阅（按 roadmap 主干分组）
/spring add <知识点>     为知识点创建唯一总结并自动绑定，直接进编辑器手写
/spring edit [关键词]    编辑泉水条目
/search <关键词>         全局搜索 md 与 go 源码（空格分隔多词 AND）
/project [名称]          项目交互详情（循环菜单，见下）
/project edit <名称>     编辑项目文档
/show [主题]             验收演示：概览/知识/代码/日志/面试题 五节视图（n/p 翻节，o 打开文件，s 沉淀索引）
/show edit               维护沉淀索引 topics.toml
/review                  AI 体检：三层流动统计（本地，无需 AI）+ 四节整理/规划建议（需配置 [ai]）
/AI [问题]               AI 对话：带参单发提问；无参进入 AI 模式（@路径 引用文件，/save 存档，/back 返回）
/help                    显示使用说明
/exit                    退出 gs（别名 /q /quit）
```

**/plan 浏览器**三层视图：全局树（主干 + 进度，Tab 原位展开）→ 主干页（知识点按分组排列）→ 知识点详情页（spring/river/sea 三段笔记清单）。详情页 ↑↓ 选笔记，Enter/o 打开阅读，d/s/t 标记状态即时落盘；stdout 非 TTY 时自动降级为静态树。

**/project 详情页**五块循环菜单：README 全文渲染；框架树（AI 生成节点介绍并缓存到 `.gs/structures/`，二次秒开，树文件按命名约定联动笔记标 `·笔N`，可调起 IDE 打开源文件）；学习笔记（项目 `docs/study/` 与 `Study/sea/<项目名>/` 两处聚合）；项目日志（git 统计 + 日志列表）；升级计划（约定正本 `docs/project/升级计划.md`，缺失可 AI 生成初稿）。gs 自身未注册时列表尾部出现「＋ 将 gs（本工具）加入项目列表」，选中写入 `.gs/projects.toml`。

示例：

```
/logs context         在全部日志中搜含 context 的记录
/sea import           把 inbox 里的外部文章整理归位
/river add GMP        从大海找 GMP 相关条目摘录
/spring add select    为 select 知识点写自己的总结
/plan done GMP        标记知识点为已学（唯一直达，多匹配弹选择）
/search context 超时  同时含两词的文件
```

## 配置

配置文件为 TOML，合并优先级从高到低：

1. 环境变量 `GS_API_KEY`
2. `<工作区根>/.gs/config.toml`
3. `~/.gs/config.toml`（全局，建议放这里）

```toml
[ai]
base_url = "https://api.example.com/v1"   # OpenAI 兼容端点
api_key  = ""                             # 建议留空，走 GS_API_KEY 环境变量
model    = "deepseek-chat"
temperature = 0.3

[scan]
exclude = [".git", ".codegraph", ".agents", "Resource", "node_modules", "_docx_render"]
max_file_kb = 512                         # 超过则搜索时跳过

[ui]
editor = ""   # 编辑器解析链：此项 → $EDITOR → $VISUAL → trae → code → notepad；
              # 短文件（≤ 内置编辑器行数阈值）优先走 CLI 全屏编辑，失败自动回落外部编辑器

[project]
opener = ""   # /project 内打开源文件用的 IDE 命令；空 = 跟随编辑器解析链
```

未配置 `[ai]` 时 `/AI` 与 `/review` 的 AI 部分会提示配置方法；三层统计等本地功能不受影响。

AI 写入边界：仅限工作区内 `.md` / `.go` / `.toml`，全部写操作（Agent 的 `write_file`、动作块）一律 diff 预览、确认后才落盘，拒绝则整体丢弃。

## 数据文件


| 位置                                  | 说明                                                                                                         |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `<root>/Study/plan/roadmap.toml`      | 路线图正本（v2：`branch → point`，point 带 `group/status/tags/spring/river/sea`），日常走命令维护，手编兜底 |
| `<root>/.gs/topics.toml`              | `/show` 沉淀的主题索引（`s` 键写入），`path` 为工作区相对路径                                                |
| `<root>/.gs/projects.toml`            | 项目注册表（手动加入的项目，如 gs 自纳管）                                                                   |
| `<root>/.gs/structures/<项目名>.toml` | 项目框架树的 AI 介绍缓存                                                                                     |
| `~/.gs/history`                       | REPL 命令历史                                                                                                |

## 架构概览

```
cmd/gs/main.go        入口：--root 解析、工作区探测、配置装配、进 REPL
internal/
├── repl/             REPL 主循环：自研 raw 输入（行状态/渲染/历史/补全建议），不依赖 readline
├── complete/         补全引擎：Ghost 续写、候选面板、参数提示（纯逻辑）
├── termkey/          跨平台原始按键解析
├── command/          一命令一文件，Build() 集中注册；Ctx 依赖注入；菜单/pager 等交互壳
├── workspace/        工作区检测、路径定位、日志日期推算、项目发现与注册表
├── search/           检索引擎：md 按标题切分、go 按函数切分，无持久化索引；热门关键词提取
├── roadmap/          roadmap.toml v2：类型/解析/原子写、关键词三态匹配、绑定操作、静态渲染（纯逻辑）
├── planview/         /plan 交互式树浏览器：状态机 + 整帧 ANSI 渲染 + raw 模式进退
├── structure/        项目结构扫描、AI 介绍缓存、笔记命名约定关联、git 统计（纯逻辑）
├── agent/            AI Agent 工具循环：调模型 → 执行工具 → 回填，直至最终回答或步数上限
├── ai/               OpenAI 兼容客户端、工具定义、gs-actions 动作解析、路径校验
├── diffx/            行级 LCS diff
├── edit/             编辑器解析与调起；短文件走内置 CLI 编辑器（bubbletea，列表续行）
├── index/            topics.toml 读写
├── config/           配置加载与合并
└── ui/               Markdown 渲染（glamour）、配色（lipgloss）、交互组件（huh）
```

模块间只经构造函数注入依赖；`search` / `roadmap` / `structure` / `agent` / `ai` / `diffx` / `index` / `complete` 为无 UI 纯逻辑包（颜色经回调注入），交互集中在 `command` 壳层与 `planview`。设计文档见 `docs/project/`（各模块 `design.md` 为需求正本，`全局/design.md` 为整体设计）。

## 开发

```bash
go test ./...                                  # 全部测试
go test ./internal/command -run TestLogsAdd    # 单个测试
```

- 纯逻辑包（`search` / `roadmap` / `planview` 状态机 / `structure` / `agent` / `diffx` / `ai` 等）均为表驱动单元测试；AI 客户端用 `httptest.Server` 模拟端点，`structure` 的 git 统计打桩注入
- `command` 包测试用 `t.TempDir()` 搭建迷你工作区 fixture
- 依赖精选原则：REPL 输入与 `/plan` 浏览器手写（raw 模式 + 整帧 ANSI），不引 readline 框架；bubbletea 仅用于内置编辑器，交互组件用 `charmbracelet/huh`，渲染用 `glamour` / `lipgloss`，TOML 用 `BurntSushi/toml`；AI 调用与 diff 均手写，不引第三方
