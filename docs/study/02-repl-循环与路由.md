# 02 repl 循环与路由 ★核心

> 走读要点：主循环、命令分发、ai 子模式状态机。

**关键文件**：`internal/repl/repl.go`、`internal/repl/input.go`、`internal/repl/suggest.go`

**待填**：

- 主循环一次迭代的步骤（读行 → 路由 → 执行 → 提示符恢复）
- readline / plain 双实现的接口抽象与降级预案
- ai 子模式如何切换提示符与路由（NextMode）
- 编辑距离 ≤2 的未知命令建议

笔记：

（待填）
