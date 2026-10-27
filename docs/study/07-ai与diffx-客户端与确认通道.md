# 07 ai 与 diffx：客户端与确认通道

> 走读要点：裸 http 调 OpenAI 兼容端点、动作 JSON 提取、diff 确认写入。

**关键文件**：`internal/ai/client.go`、`internal/ai/actions.go`、`internal/diffx/diffx.go`、`internal/command/ai.go`、`internal/command/review.go`

**待填**：

- chat/completions 请求构造与 120s 超时（无 SDK、非流式）
- 对话上下文组装：系统提示 + 滚动窗口历史 + @文件
- fenced json 动作块的提取与路径白名单校验（仅 md/go/toml）
- 行级 LCS diff 与 y/n/a 逐条确认流程
- /review 的输入聚合与三类建议输出

笔记：

（待填）
