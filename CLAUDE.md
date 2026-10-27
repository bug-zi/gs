# gs · Go 学习工作台 CLI

运行在 Go 学习工作区（`D:\Code\Go`）的交互式 REPL 终端工具：日志/笔记/计划检索、`/show` 验收演示、`/AI` 与 `/review` 辅助。源码在 `cmd/gs/` 与 `internal/`。

## 文档体系与维护权责（最重要）

`docs/` 目录是开发者与 AI 共同维护的项目文档库，权责划分如下：

| 内容 | 维护方 | AI 的角色 |
|---|---|---|
| docs/project/全局/design.md（全局需求正本） | 开发者主导 | 可提建议；经开发者审核通过才可修改，**不得擅自改写** |
| docs/project/<模块>/design.md（模块需求正本） | 开发者主导 | 可提建议；经审核通过才可协助，不得擅自改写 |
| docs/project/<模块>/designs-specs.md | **AI 生成并维护** | 基于 design.md 严格生成，是开发的直接依据；开发者仅简单审核 |
| docs/project/project.md | 开发者主导 | 可建议；协助完善需审核通过 |
| docs/log/<YYMMDD>.md | AI | 每天一个开发日志；**每个小节标题下必须附创建时间行**（`> 创建于 YYYY-MM-DD HH:MM`，用 date 取实际时间，不估不编） |
| docs/project/idea/ | **开发者** | AI 不需要关注、不得修改（例外：AI 产出的 spec/plan 按机制存放于此，只动自己产出的文件） |
| docs/project/README.md | **AI** | spec/plan 存放与归档机制正本 |
| docs/project/ 内各模块 spec/plan 文档 | **AI** | brainstorming/写计划产出，命名 `YYYY-MM-DD-<主题>-design.md` / `-plan.md`，按模块就地存放；落地（`go build ./...` + `go test ./...` 通过即算）或废弃后移入所属目录 archive/（机制见 README.md） |
| docs/project/新功能开发区.md | **开发者** | AI 据此 brainstorming 立项并实施；完成后条目勾选并按轮归档 |
| docs/project/优化建议区.md | **开发者** | AI 据此实施优化；实施后按轮归档进归档区 |
| docs/project/问题疑惑区.md | **开发者** | **AI 不要主动查阅**，开发者指明时才阅读并答疑 |
| docs/project/临时草稿（待写入）.md | **开发者** | **AI 不需要读取**；建后不再读取/修改 |
| docs/study/ | 开发者 | 代码走读学习笔记，AI 可受托撰写或补全，归属开发者 |
| 各子目录同名说明 md（log.md / 全局.md 等） | 随目录权责 | 目录功能与用法说明；新建子目录时随建随补 |

工作流程：开发者写 design.md → AI 据此生成 designs-specs.md → AI 按 designs-specs.md 开发 → AI 记开发日志、按机制归档 spec/plan。

归档纪律：只是 `mv` 位置变化、文件永不删除；废弃仅开发者明说才判定；历史日志不改。

`doc/`（旧文档库快照）只读保留，永不改动其内文件。

## 全局硬性规则

1. 所有 git 操作（add / commit / push / pull / checkout / reset 等）由开发者自己完成；**AI 禁止自主执行任何 git 命令**，最多在开发者明确要求时给出命令建议。
2. 终端 UI（lipgloss/huh 样式）禁止使用 emoji 作图标与彩亮饱和配色；须使用与主题色相近的色系，保持整体色彩风格一致。
3. 源码与文档统一 UTF-8；代码注释用中文、仅写在非直观处。
4. 命令层纯逻辑与交互壳分离；`search`/`index`/`workspace`/`ai`/`diffx` 保持无 UI 纯逻辑包，可独立测试。
5. 文件写入一律走原子写（临时文件 + rename），不产生半截文件。

## Git 操作纪律（开发者指令）

- **禁止 AI 自主执行任何 git 操作**（add / commit / push / pull / checkout / reset 等全部命令）。
- 所有 git 操作由开发者自主完成；AI 最多在开发者明确要求时给出命令建议，不得代为执行。
