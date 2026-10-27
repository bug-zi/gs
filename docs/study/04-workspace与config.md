# 04 workspace 与 config

> 走读要点：工作区判定、原子写、三层配置合并。

**关键文件**：`internal/workspace/workspace.go`、`internal/config/config.go`

**待填**：

- 工作区判定标准（Study/logs + Study/plan 存在）
- WriteFileAtomic 的临时文件 + rename 流程
- 配置合并顺序：全局 ~/.gs → 工作区 .gs → 环境变量 GS_API_KEY
- 日志日期推算（东八区六位日期）与项目发现规则

笔记：

（待填）
