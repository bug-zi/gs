# gs 学习笔记（导读）

> 写给项目作者本人（Go 学习者）的代码走读笔记：复盘 1.0.0 的架构决策与实现手法。
> 配套代码：`cmd/gs/`、`internal/`。笔记中的 `文件:行号` 可直接对照源码。

## 这个项目是什么

gs 是一个运行在 Go 学习工作区（`D:\Code\Go`）中的交互式 REPL 终端工具，提供日志/笔记/计划的检索展示、面向验收的 `/show` 演示视图、以及接入 LLM 的 `/AI` 与 `/review` 辅助。单二进制 Go 程序，标准库为主 + 少量精选库（readline / huh / glamour / lipgloss / toml）。

| 包 | 目录 | 一句话 |
|---|---|---|
| main | `cmd/gs/main.go` | 入口：解析 --root，装配依赖，进 REPL |
| repl | `internal/repl/` | REPL 循环、行编辑双实现、命令路由、ai 子模式状态机 |
| command | `internal/command/` | 每个命令一个文件，Ctx 构造函数注入依赖 |
| workspace | `internal/workspace/` | 工作区校验、路径定位、项目发现、日志日期推算、原子写 |
| config | `internal/config/` | 全局/工作区/环境变量三层配置合并 |
| search | `internal/search/` | 关键词扫描引擎（md 按小节、go 按函数切分）+ 热门关键词 |
| index | `internal/index/` | topics.toml 沉淀索引读写 |
| ai | `internal/ai/` | OpenAI 兼容客户端（裸 http）、动作 JSON 提取与路径校验 |
| diffx | `internal/diffx/` | 行级 LCS diff 生成 |
| edit | `internal/edit/` | 编辑器探测链（$EDITOR → $VISUAL → code → notepad）与调起 |
| ui | `internal/ui/` | glamour Markdown 渲染、lipgloss 配色封装 |

## 学习路线（建议顺序）

1. `01-总体架构与入口.md` ★核心 —— main.go 装配顺序就是全项目的依赖图
2. `02-repl-循环与路由.md` ★核心 —— 主循环、路由、ai 子模式状态机
3. `03-command-命令层.md` ★核心 —— Ctx 注入模式、命令注册、纯逻辑+薄交互壳
4. `04-workspace与config.md` —— 工作区判定、原子写、配置合并顺序
5. `05-search-搜索引擎.md` —— 扫描切分策略、多词 AND 匹配
6. `06-show与index-演示与沉淀.md` —— 五节视图构建、pager 交互、topics.toml
7. `07-ai与diffx-客户端与确认通道.md` —— 裸 http 调用、动作 JSON、diff 确认写入
8. `08-ui与edit-渲染与编辑器.md` —— glamour/lipgloss 封装、编辑器回退链

01–03 是地基，读通后其余按需查阅。

## 快速上手命令

```bash
go build ./...        # 构建
go test ./...         # 全部测试
go test ./internal/command/ -v   # 单包测试
go run ./cmd/gs       # 在学习工作区中启动 REPL
```

## 高频名词表

| 名词 | 含义 |
|---|---|
| 工作区 | 含 `Study/logs/` 与 `Study/plan/` 的目录（默认当前目录，--root 覆盖） |
| Ctx | `command.Ctx`：注入 WS/Cfg/Out/Args 等依赖的命令上下文 |
| 沉淀索引 | `.gs/topics.toml`：/show 按 s 后落盘的主题 → 文件映射 |
| 五节视图 | /show 的 ①概览②笔记③代码④日志⑤面试题 逐节浏览结构 |
| diff 通道 | AI 修改动作 → 行级 diff 预览 → 用户 y/n/a 确认后才写盘 |
| ai 子模式 | /AI 进入，提示符 `ai>`，/back 返回主模式 |
| 原子写 | 先写临时文件再 rename，避免半截文件 |

## 阅读源码时的几条心法

1. `command/` 不互相调用，全部依赖经构造函数注入 —— 想追调用链先看 `main.go` 的装配。
2. `search`/`index`/`workspace`/`ai` 是无 UI 纯逻辑包，可独立测试 —— 读逻辑直接看包内函数，不用关心终端交互。
3. 交互壳集中在 `command/prompt.go`（huh 封装）与 `repl/input.go`（readline 双实现），核心逻辑不掺 I/O。
4. AI 调用不用 SDK：`net/http` + `encoding/json` 直接 POST，读 `ai/client.go` 就能看到完整协议。
5. 测试用 `t.TempDir()` 搭迷你工作区 fixture（`wsForTest`），读测试文件是理解命令行为的最快路径。

（章节文件：`01-总体架构与入口.md` … `08-ui与edit-渲染与编辑器.md`；走读中的疑问记入 `docs/project/问题疑惑区.md`）
