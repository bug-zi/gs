# AI 模式 Agent 化设计（工具调用 + 检索式 RAG）

> 创建于 2026-10-26（brainstorming 产出，spec 正本；designs-specs 按 README.md 机制另行生成）

## 背景与目标

当前 `/AI` 是单轮流式 chat：系统提示注入断点/阶段清单、`@路径` 展开文件、回复末尾 JSON 动作块写文件。模型无法自主查资料、多步完成任务。

目标：把 AI 模式升级为 Agent 轮——模型可自主调用工具（检索工作区资料、读写文件），多步推理后给出最终回答。检索采用 **Agent 工具式 RAG**（模型自主搜索+读文件），不建向量库、不依赖 embedding 端点。

### 已确认的关键决策

| 决策点 | 结论 |
|---|---|
| RAG 形态 | Agent 工具式检索，复用现有 `internal/search`，零新增外部依赖 |
| 工具边界 | 只读检索工具 + 受确认的写文件工具（逐次 y/n 审批） |
| 交互形态 | 统一 Agent 轮：AI 子模式与单发 `/AI` 全部走 Agent，不调工具时体验与现在一致 |
| 架构 | 方案 A：新建 `internal/agent` 纯逻辑包；`internal/ai` 扩展 tools 协议；command 层只留 UI 壳 |

### 非目标

- 不引入向量库 / embedding / 第三方 agent 框架
- 不提供 shell 命令执行工具
- 不改 `/review`、会话存档（aisave）流程
- 不做 Hist 自动摘要压缩（本轮只做「工具中间消息不进 Hist」）

## 架构总览

```
用户输入（AI 子模式 / 单发 /AI）
  → expandAtFile（@路径 展开，保持不变）
  → messages = [system] + Hist + [user]
  → agent.Run ──循环（≤ MaxSteps=12）──
  │    1. ai.ChatWithTools(msgs, tools)
  │    2. 返回含 tool_calls → 逐个执行工具（write_file 先经审批钩子）
  │       → append assistant + role:"tool" 消息 → 继续循环
  │    3. 返回纯 content → 作为最终回答结束
  │    4. 达到步数上限 → 追加一条 user 消息强制总结
  ← 最终回答 → command 层展示 → Hist 只追加 [user, 最终 assistant]
```

## 一、`internal/ai` 扩展（协议层）

```go
type Tool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    Parameters  json.RawMessage `json:"parameters"` // JSON Schema 原样透传
}

type ToolCall struct {
    ID        string `json:"id"`
    Name      string `json:"name"` // 实际来自 function.name
    Arguments string `json:"arguments"`
}

type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`  // assistant 发起调用时
    ToolCallID string     `json:"tool_call_id,omitempty"` // role:"tool" 回填用
}
```

- `chatReq` 增加 `Tools []Tool \`json:"tools,omitempty"\``
- 新方法：

```go
// onDelta 可为 nil；content 增量实时回调（供 UI 流式展示），
// tool_calls 增量不回调、按 index 静默拼装（首片带 id/name，arguments 逐段追加）
func (c *Client) ChatWithTools(ctx context.Context, msgs []Message, tools []Tool,
    onDelta func(delta string)) (Message, error)
```

- 沿用现有流式 SSE + 空闲看门狗 + 整体兜底超时；`delta.tool_calls` 按 OpenAI 标准协议拼装（GLM 兼容）
- 现有 `Chat(ctx, msgs)` 保留为 `ChatWithTools(msgs, nil, nil)` 的便捷委托，`/review` 等调用方零改动
- 空内容判定调整：有 tool_calls 即视为有效响应；仅当既无 content 又无 tool_calls 才报「响应缺少内容」

## 二、`internal/agent` 包（纯逻辑，可独立测试）

```go
type Tool struct {
    Def  ai.Tool // name / description / JSON Schema
    Exec func(ctx context.Context, args json.RawMessage) (string, error)
}

type Hooks struct {
    OnToolCall func(name, argsJSON string)       // 每次工具调用前回调（步骤行展示）
    OnDelta    func(delta string)                 // 透传 ChatWithTools 的 content 增量
}

type Agent struct {
    Client   ChatClient // 接口：ChatWithTools(ctx, msgs, tools, onDelta)；*ai.Client 天然满足，便于脚本化 fake 测试
    Tools    []Tool
    MaxSteps int // ≤0 时取默认 12
}

// Run 执行一轮 Agent 循环。返回最终 assistant 消息与本轮产生的全部消息
// （含 assistant/tool 中间消息），Hist 取舍由调用方决定。
func (a *Agent) Run(ctx context.Context, msgs []ai.Message, hooks Hooks) (ai.Message, []ai.Message, error)
```

### 循环语义

1. 调 `ChatWithTools`，把返回的 assistant 消息 append 进消息序列
2. 若含 `tool_calls`：逐个执行（`OnToolCall` → 写工具先 `Approve` → `Exec`），每个调用 append 一条 `role:"tool"`、`tool_call_id` 对应、content 为工具结果文本的消息；单工具失败不终止循环，错误文本作为结果回给模型自纠；然后回到 1
3. 若为纯 content：结束，返回该消息
4. 步数达到 `MaxSteps`：append 一条 user 消息「已达步数上限，请基于已有信息直接给出最终回答」，再调一次模型，返回其回答（此步不再执行工具）

### 工具集（4 个）

工具实例由 `agent` 包提供各工具独立构造函数（`SearchTool(root, exclude, maxFileKB, …)`、`ReadFileTool(root, defaultLines)`、`ListDirTool(root)`、`WriteFileTool(root, approve)`），在 command 层用 `ctx.WS` 与 `ctx.Cfg` 构造后注入 `Agent.Tools`。agent 包自身不 import config/ui；可 import workspace 仅复用 `WriteFileAtomic` 原子写，import search 复用检索（search 侧新增无 config 依赖的 `SearchFiltered` 入口）。

| 工具 | 参数（JSON Schema） | 行为 |
|---|---|---|
| `search_workspace` | `query` (必填) | 包装 `search.Search`：多词 AND 检索 .md/.go；返回截断结果（上限 20 文件 × 每文件 5 条命中行，附相对路径与行号） |
| `read_file` | `path` 必填；`offset`、`limit` 可选 | 读工作区相对路径文件，行号标注；默认前 400 行，`offset`/`limit` 翻页；单次读取总量上限（复用 `cfg.Scan.MaxFileKB` 防御超大文件） |
| `list_dir` | `path` 可选（默认根） | 列一层目录：名称、类型（文件/目录）、大小；不递归 |
| `write_file` | `path`、`content` 均必填 | 覆盖或新建文件（内容即完整新文件）；执行前 `Approve(path, "write")`；落盘走 `workspace.WriteFileAtomic` |

学习状态（Study/plan/学习状态.md）、日志、sea/river/spring 笔记均可由 `read_file`/`search_workspace` 覆盖，不单设专用工具。

### 路径安全（所有含 path 参数的工具共用 helper）

- 仅接受工作区相对路径（正斜杠）；拒绝绝对路径
- `filepath.Join(root, path)` 后用 `filepath.Rel(root, …)` 校验，结果以 `..` 开头即拒绝（防穿越）
- 拒绝时返回明确错误文本（「路径必须为工作区相对路径且不得逃逸根目录」），让模型自行修正

### 写文件审批

- 审批不进 Hooks，而是作为 `WriteFileTool(root, approve)` 的构造参数：`approve func(absPath, content string) bool`，拿到绝对路径与完整新内容，供 command 层渲染 diff 预览后再询问
- approve 返回 false 时工具不落盘，返回「用户拒绝写入 <path>…」作为 tool 结果；模型据此转为文字建议
- approve 由 command 层实现（diff 预览 + 应用/跳过选择菜单），agent 包不持有任何 UI

## 三、`command/ai.go` 改造（只做壳）

- `AITurn` 构建 `agent.Agent`（注入 4 工具 + hooks）并调 `Run`：
  - `OnToolCall` → 打印暗色步骤行，如 `· search_workspace {"query":"channel 死锁"}`（无 emoji，主题色系暗色）
  - `OnDelta` → 最终回答真流式打印（替换现有「AI 思考中…」整段等待；`/AI` 单发同样生效）
  - `Approve` → 复用现有 y/n 确认 UI 风格，逐次确认
- **Hist 只追加 `[user, 最终 assistant content]`**：工具中间消息丢弃，防止上下文逐轮膨胀；连续轮次涉及同一资料时模型重新检索
- 系统提示（`aiSystemTpl`）更新：
  - 4 个工具的名称、用途、使用建议
  - 工作区目录结构说明（Study/sea、river、spring、logs、plan）
  - 行为准则：涉及学习资料/笔记/进度的问题先检索再回答；不确定的内容不臆造；写文件前说明意图（用户会看到确认请求）
- 原 JSON 动作块协议从系统提示中移除；`aisave` 会话存档、`/review` 流程零改动（`/review` 仍走 `Chat`）

## 四、错误处理

| 场景 | 行为 |
|---|---|
| 工具执行失败 | 错误文本作为 tool 结果回给模型，循环继续，模型自纠 |
| 写入被拒绝 | 「用户拒绝写入」回给模型，模型转文字建议 |
| 步数上限 | 强制总结消息，返回最终回答 |
| 空闲/整体超时、网络错误 | 沿用现有 `errIdleTimeout`/`mapCause` 语义，中断整轮并报错 |
| Hist 为空/`@文件` 不存在 | 保持现状（expandAtFile 已有提示） |

## 五、测试

| 层 | 用例 |
|---|---|
| `internal/ai` | `httptest` 模拟 SSE：tool_calls 分片乱序/逐段到达的拼装正确性；content 与 tool_calls 混合流；无 tools 时行为与旧 `Chat` 一致 |
| `internal/agent` | 脚本化 fake Client（按调用次序返回预设响应）：单轮直答；多步工具循环终止；审批 false 时工具被拦截且模型收到拒绝文本；路径逃逸/绝对路径被拒；步数上限触发强制总结；工具报错不中断循环 |
| `internal/command` | 步骤行展示与「Hist 只存两条」的 glue 测试（httptest SSE 脚本直跑 `AITurn`）；确认流不自动化，走手动验收 |

## 验收标准

1. `go build ./...` 与 `go test ./...` 全部通过
2. `/AI` 提问「我最近学了什么」时，能看到步骤行（模型调用 search/read 工具）后给出基于日志/笔记的回答
3. 要求 AI 改写某笔记时，逐次出现 y/n 确认，拒绝后模型不再写盘
4. 简单问答（不涉资料）体验与改造前一致：单轮流式回答、无工具行噪音
5. Hist 不因 Agent 轮膨胀：多轮对话后请求体积只随 user/最终回答增长
