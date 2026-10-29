# gs · Go 学习工作台

一个用 Go 编写的交互式 REPL 终端工具，运行在 Go 学习工作区中，围绕「知识存在哪、怎么流、怎么讲给人听」组织学习资料：

- **四站知识流转**：sea（外部知识）→ ship（加工中）→ river（沉淀）→ spring（手写总结），流转即文件真实迁移；seaport 海港启动时后台 AI 自动分类导入；`code` 层挂每知识点唯一演示代码
- **学习路线图**：`roadmap.toml` 树状知识地图，交互浏览器里 Tab 展开、Enter 深入、d/s/t 原位标记进度；知识点与五层笔记双向绑定，river 按树序编号，`/plan renumber` 一键全量重排
- **Agent 化 AI**（可选）：`/AI` 自主检索工作区作答（读自动、写审批），每轮答毕可确认存入大海并自动绑定知识点；`/review` 学习体检
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
├── seaport\                 # 海港：外部 md/txt 丢这里，启动后台 AI 分类导入大海
├── Study\
│   ├── logs\YYMMDD.md       # 每日日志（东八区六位日期）
│   ├── plan\
│   │   ├── 学习状态.md       # 断点与阶段清单
│   │   └── roadmap.toml     # 学习路线图正本（branch → point）
│   ├── sea\                 # 大海：外部知识原貌（ai\ clip\ interview\ inbox\ 未分类\ 与各主题子目录）
│   ├── ship\*.md            # 船坞：装船加工中的中间态（平铺不编号）
│   ├── river\               # 河流：沉淀正本，按树序编号
│   │   ├── 01-Go语言基础\03-slice底层结构与扩容.md
│   │   └── 99-未挂载\       # 入河未挂知识点的收容区
│   ├── spring\*.md          # 泉水：自己的总结，与路线图知识点一一对应
│   └── code\*.go            # 代码：每知识点唯一的演示代码
├── Project\<name>\          # 实践项目
└── Resource\                # 大资源（默认不扫描）
```

进入工作区目录后运行 `gs`，出现启动横幅与 `gs>` 提示符即成功；`seaport/` 有待导入文件时启动会提示并后台分类：

```
gs · Go 学习工作台 · D:\Code\Go
当前断点: 登录模块源码走读
目标: 2026-12-01 实习面试
上次学习: 10-24 (今天) · 连续 2 天
输入 /help 查看命令，/exit 退出
```

## 四站知识流转

知识按「离自己多近」分四站流动；`code` 是挂在知识点上的独立演示代码层，`seaport` 是大海的自动入口：

```
seaport（海港）─启动导入→ sea（大海）─/ship copy 装船→ ship（船坞）─/ship move 入河→ river（河流）─手搓→ spring（泉水）
```


| 站          | 目录            | 定位                                                      | 进入/流转方式                                                                      |
| ------------- | ----------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| sea 大海    | `Study/sea/`    | 外部知识原貌：剪藏、AI 问答存档、面试题正本，原件永久保留 | seaport 启动后台 AI 分类导入；`/sea import` 手动整理 inbox；`/AI` 会话存档         |
| ship 船坞   | `Study/ship/`   | 装船加工中的中间态（平铺不编号）                          | `/ship copy`：选 sea 条目复制装船（sea 原件与绑定保留），装船即开编辑器开工        |
| river 河流  | `Study/river/`  | 沉淀正本：加工完成的定稿，按树序编号                      | `/ship move`：选 ship 条目真实迁移入河，领号挂载，出处链同步改写                   |
| spring 泉水 | `Study/spring/` | 自己对该知识点的理解，**每个知识点有且仅有一篇**          | `/spring add <知识点>`：创建占位条目并自动绑定 → 编辑器手写（设计上只留手写入口） |
| code 代码   | `Study/code/`   | 每个知识点唯一的演示代码（.go）                           | `/code add <知识点>`：同 spring，创建占位并绑定后手写                              |

流转全部由用户显式命令触发，无自动判定：装船=宣告开始加工（复制语义），入河=宣告加工完成（真实迁移）；放弃加工则手动把文件移回 sea。绑定关系正本在 `roadmap.toml`：知识点带 `spring`/`code`（0/1 单值）、`river`/`ship`/`sea`（0..N）字段，流转时绑定随文件复制/迁移；文件不在任何知识点上时归「未挂载」，入河落 `99-未挂载/`，planview 浏览器内 `a` 键可补挂载。

**river 编号**：目录 `<两位主干序>-<主干名>/`、文件 `<点两位>-<标题>.md`，同点多篇 `.N` 递增，序号取自 `roadmap.toml` 数组顺序（两位补零保证编辑器字符串排序）。平时留空洞（新增领 max+1、删除留洞），`/plan renumber` 按树序全量重排：dry-run 预览改名与引用改写清单 → 确认执行 → roadmap 绑定、topics.toml、全库出处链同步改写。

**出处链**：river 条目头部自动带 `> 出处：Study/sea/<来源>.md`，迁移/重排时自动改写，是 planview `o` 键回溯与 `/review` 统计（未摘录/加工中/待改写）的数据基础。

**seaport 海港**：外部 md/txt 丢到工作区根 `seaport/`，启动时后台扫描并按 sea 现有主题目录让 AI 分类迁入（无法判定落 `未分类/`，未配置 `[ai]` 时全部进未分类），不阻塞进 REPL；非文本文件不处理仅提示。

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
/plan del branch|point <关键词>  删除空主干 / 删除知识点（解绑保留文件，river 收容 99-未挂载）
/plan renumber           按树序全量重排 river 编号（dry-run 预览，确认执行，引用同步改写）
/plan edit               编辑 roadmap.toml / 计划文件
/sea [关键词]            大海查阅（按一级子目录分组两级选择，关键词平铺直选）
/sea import              inbox → clip 整理：逐个预览、起标题、迁移（重名跳过）
/sea edit [关键词]       编辑大海条目
/ship [关键词]           船坞查阅（加工中条目）
/ship copy [关键词]      装船：选 sea 条目 → 复制入 ship → 自动开编辑器开工
/ship move [关键词]      入河：选 ship 条目 → 真实迁移入 river，领号挂载，出处链同步改写
/river [关键词]          河流查阅（按 roadmap 主干分组，未绑定归「未挂载」组）
/river edit [关键词]     编辑河流条目
/spring [关键词]         泉水查阅（按 roadmap 主干分组）
/spring add <知识点>     创建该知识点唯一的总结并自动绑定，直接进编辑器手写
/spring edit [关键词]    编辑泉水条目
/code [关键词]           演示代码查阅（按 roadmap 主干分组）
/code add <知识点>       创建该知识点唯一的演示代码（.go）并绑定
/code edit [关键词]      编辑演示代码
/sea|/ship|/river|/spring|/code del [关键词]  删除条目（预览影响，确认后移入回收站，可手动找回）
/search <关键词>         全局搜索 md 与 go 源码（空格分隔多词 AND）
/project [名称]          项目交互详情（循环菜单，见下）
/project edit <名称>     编辑项目文档
/show [主题]             验收演示：概览/知识/代码/日志/面试题 五节视图（n/p 翻节，o 打开文件，s 沉淀索引）
/show edit               维护沉淀索引 topics.toml
/review                  学习体检：知识流动概览（本地统计，无需 AI）+ 整理/合并/流动/规划四节建议（需配置 [ai]）
/AI [问题]               AI 对话：带参单发提问；无参进入 AI 模式（@路径 引用文件，答毕可存入 sea，/save 整段存档，/back 返回）
/help                    显示使用说明
/exit                    退出 gs（别名 /q /quit）
```

**/plan 浏览器**三层视图：全局树（主干 + 进度，Tab 原位展开）→ 主干页（知识点按分组排列）→ 知识点详情页（sea/ship/river/spring/code 五段清单 + 待归类区）。详情页 ↑↓ 选笔记，Enter/o 打开阅读，a 挂载 / x 卸载 / m 移动 / n 重命名，d/s/t 标记状态即时落盘；stdout 非 TTY 时自动降级为静态树。

**/project 详情页**五块循环菜单：README 全文渲染；框架树（AI 生成节点介绍并缓存到 `.gs/structures/`，二次秒开，树文件按命名约定联动笔记标 `·笔N`，可调起 IDE 打开源文件）；学习笔记（项目 `docs/study/` 与 `Study/sea/<项目名>/` 两处聚合）；项目日志（git 统计 + 日志列表）；升级计划（约定正本 `docs/project/升级计划.md`，缺失可 AI 生成初稿）。gs 自身未注册时列表尾部出现「＋ 将 gs（本工具）加入项目列表」，选中写入 `.gs/projects.toml`。

示例：

```
/logs context         在全部日志中搜含 context 的记录
/ship copy 中间件      把 sea 里含「中间件」的条目装船开工
/ship move            加工完成，选船坞条目入河领号
/plan renumber        预览并执行 river 编号全量重排
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

未配置 `[ai]` 时 `/AI`、`/review` 的 AI 部分与 seaport 智能分类不可用（seaport 文件全部进「未分类」，不会丢失）；三层统计等本地功能不受影响。

AI 写入边界：仅限工作区内 `.md` / `.go` / `.toml`，全部写操作（Agent 的 `write_file`、动作块）一律 diff 预览、确认后才落盘，拒绝则整体丢弃。

## 数据文件


| 位置                                  | 说明                                                                                                                   |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `<root>/Study/plan/roadmap.toml`      | 路线图正本（v2：`branch → point`，point 带 `group/status/tags/spring/code/river/ship/sea`），日常走命令维护，手编兜底 |
| `<root>/.gs/topics.toml`              | `/show` 沉淀的主题索引（`s` 键写入），`path` 为工作区相对路径                                                          |
| `<root>/.gs/projects.toml`            | 项目注册表（手动加入的项目，如 gs 自纳管）                                                                             |
| `<root>/.gs/structures/<项目名>.toml` | 项目框架树的 AI 介绍缓存                                                                                               |
| `<root>/.gs/trash/`                   | 删除机制回收站（软删除，`<时间戳>-<序号>/<原相对路径>`，手动 mv 找回）                                                 |
| `~/.gs/history`                       | REPL 命令历史                                                                                                          |

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
├── roadmap/          roadmap.toml v2：类型/解析/原子写、关键词三态匹配、绑定/流转操作、静态渲染（纯逻辑）
├── tier/             流转执行器：文件迁移、river 领号计算、出处链扫描改写（纯逻辑）
├── seaport/          海港：seaport/ 目录扫描、AI 分类、批量导入 sea（纯逻辑）
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

模块间只经构造函数注入依赖；`search` / `roadmap` / `tier` / `seaport` / `structure` / `agent` / `ai` / `diffx` / `index` / `complete` 为无 UI 纯逻辑包（颜色经回调注入），交互集中在 `command` 壳层与 `planview`。设计文档见 `docs/project/`（各模块 `design.md` 为需求正本，`全局/design.md` 为整体设计）。

## 开发

```bash
go test ./...                                  # 全部测试
go test ./internal/command -run TestLogsAdd    # 单个测试
```

- 纯逻辑包（`search` / `roadmap` / `tier` / `seaport` / `planview` 状态机 / `structure` / `agent` / `diffx` / `ai` 等）均为表驱动单元测试；AI 客户端用 `httptest.Server` 模拟端点，`structure` 的 git 统计打桩注入
- `command` 包测试用 `t.TempDir()` 搭建迷你工作区 fixture
- 依赖精选原则：REPL 输入与 `/plan` 浏览器手写（raw 模式 + 整帧 ANSI），不引 readline 框架；bubbletea 仅用于内置编辑器，交互组件用 `charmbracelet/huh`，渲染用 `glamour` / `lipgloss`，TOML 用 `BurntSushi/toml`；AI 调用与 diff 均手写，不引第三方
