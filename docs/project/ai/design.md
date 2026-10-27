# ai · AI 模块设计

日期：2026-10-27
状态：**AI 起草，待开发者审核定稿**——依开发者指示补充模块正本；需求源自 `全局/design.md` §5.4/§9 与已定稿 spec（`archive/2026-10-26-AI模式Agent化-design.md`），并对照代码现状（`internal/ai`、`internal/agent`、`internal/command/ai.go / review.go / aisave.go`）核实整理。

## 1. 定位

AI 能力模块，覆盖三块：`/AI` 交互问答（Agent 化）、`/review` 学习体检、底层 OpenAI 兼容客户端与 diff 写入通道。核心边界（1.0 已定）：**读自动、写审批**——AI 可自主检索工作区，但一切文件写入以 diff 预览、用户确认后才落盘。

## 2. 已确认的核心决策

| 决策点 | 结论 |
|---|---|
| 协议 | OpenAI 兼容（base_url + api_key + model 三件套），`net/http` 直连不用 SDK |
| 写入边界 | 全部写操作走 diff 预览 + 确认；拒绝则丢弃，不部分应用 |
| 交互形态 | `/AI <问题>` 单发；`/AI` 进入子模式（提示符 `ai>`，/back 返回） |
| 未配置时 | `/AI`、`/review` 提示配置方法后返回，其余命令完全不受影响 |

## 3. /AI 子模式（Agent 化已落地）

- **上下文**：系统提示（教学角色设定 + 工作区概览：断点摘要、阶段清单）+ 对话历史 + 用户 `@文件` 引用展开的文件内容。
- **Agent 循环**（`internal/agent`）：调模型 → 执行工具 → 回填结果，直到给出最终回答或达到步数上限（默认 12 步）；达上限后不带工具强制总结。工具执行前打印步骤行，回答流式输出。
- **工具集**：`search_workspace`（关键词检索工作区 .md/.go，多词 AND）、`read_file`（按行读取）、`write_file`（写入，必经审批）。
- **写审批**：已存在文件展示行级 diff（增绿删红），新建文件展示内容预览 → 选择「应用/跳过」；取消（Esc）视为拒绝。
- **历史取舍**：每轮只把 user 提问与最终回答存入历史，中间工具消息不进历史。

## 4. 会话存档（三层框架 Q3/Q7 定案）

- `/save`：当前会话整理为 md 存入 `sea/ai/`；标题取首轮提问前 30 字（放引用行），文件名用存档时刻（YYYY-MM-DD-HHMM）。
- 离开模式（/back、/exit）时主动询问「本次 AI 会话存入大海？」；空会话或已存档静默跳过（防重复询问）。
- 存档产物属 sea 模块，行为细节见 `sea/design.md`。

## 5. /review 体检

- **材料收集**（各部分限量防超长）：学习状态断点、阶段清单、三层统计、知识库全文（限量截断）、近 30 天日志。
- **三层统计本地展示**：确定性统计（出处行反查未摘录/待改写），无 AI 配置也可见。
- **单次调用**：系统提示「严格的 Go 学习教练，只给可执行建议」，输出四节建议——知识整理、合并建议、三层流动建议、学习规划。
- 回复中的文字建议以 Markdown 渲染展示；若含 gs-actions 动作块（如合并条目），同样走 diff 确认通道。

## 6. 配置

```toml
[ai]
base_url = "https://api.example.com/v1"
api_key  = ""            # 建议留空走环境变量 GS_API_KEY（优先级最高）
model    = "deepseek-chat"
temperature = 0.3
```

合并顺序：全局 `~/.gs/config.toml` 为基底，工作区 `.gs/config.toml` 覆盖非空字段。AI 请求失败（网络/鉴权/超时）显示原始错误与配置检查指引，不影响 REPL 存活。

## 7. 代码结构

| 包 | 职责 |
|---|---|
| `internal/ai` | OpenAI 兼容客户端、消息与工具定义、gs-actions 动作解析 |
| `internal/agent` | 工具调用循环（无 UI；展示与审批经 Hooks/构造参数注入） |
| `command/ai.go` | /AI 命令、子模式轮次、写审批 |
| `command/aisave.go` | 会话存档、离开询问、inbox 计数 |
| `command/review.go` | /review 材料收集与体检调用 |
| `command/review_stats.go` | 三层统计（GatherTierStats，确定性、无 AI 依赖） |

## 8. 归档

- `archive/2026-10-26-AI模式Agent化-design.md` + `-plan.md` — Agent 化改造（已落地，随本模块立项迁入）。
