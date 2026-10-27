# 自绘输入行 v2：无框架 raw 读取器（重写 bubbletea 方案）

日期：2026-10-26
状态：**商讨中即实施**（开发者明确要求重写；v1 bubbletea 方案在 trae 终端四轮修复无效）
所属：`全局/`（前案：2026-10-26-自绘输入行-design.md，交互规格沿用）

## 重写原因

GS_KEYLOG 实证：按键送达完全正常（`/logs` + 两次 `enter` + `ctrl+d` 全部正确记录），但界面无响应。四轮修复（控制台模式还原 / KeyCtrlJ / 输入缓冲冲刷）均无效，证明故障在 bubbletea 的控制台生命周期与「普通写入 REPL」的交错层——框架内部行为，外部无法根治。

## 方案：框架-free raw 读取器

- **输入**：`x/term.MakeRaw(stdin)` 进 raw 模式，自己读字节、自己解析（Enter/退格/方向键/Tab/Esc/Ctrl-C/D/UTF-8 中文）；`Restore` 退出。终端状态进出完全由我们自己的代码负责，框架零参与。
- **输出**：全部普通写入 os.Stdout + 基础 ANSI（清行、光标移动），宽字符列宽用 runewidth（已在依赖）。无渲染器 goroutine、无帧管理、无状态残留。
- **保留**：`internal/complete`（补全计算）、`repl/history.go`（历史）原样复用；`GS_PLAIN_INPUT` / 非 TTY / MakeRaw 失败 → plainReader；GS_KEYLOG 诊断保留（记原始字节 + 解析结果）。
- **删除**：teaReader / textinput / bubbletea 在 repl 包的全部引用（huh 与内部编辑器属其他包，不动）。

## 模块划分

```
internal/repl/
  linestate.go    # 纯状态机：LineState{runes,pos} 插入/删除/移动（可测试）
  keyparse.go     # 纯解析：字节流 → 按键动作（含 ESC 序列、UTF-8 组装、跨块残包）（可测试）
  linerender.go   # 纯渲染：输入行/面板 → ANSI 字符串（宽字符列宽，可测试）
  rawreader.go    # 装配：MakeRaw + 读循环 + 状态机 + 渲染 + 历史/补全接线（薄，不自动测）
  input.go        # NewReader 默认 rawReader；降级条件不变
  line.go         # 删除（tea 版）
  drain_windows.go# 保留（raw 退出后清控制台输入缓冲兜底）
```

## 交互语义

与 v1 spec 一致（ghost Tab/→ 接受、≥2 候选弹面板 ↑↓ 高亮 Enter 采纳、↑↓ 历史、Ctrl-C 清行、Ctrl-D 退出、Esc 关面板）。

## 测试

- linestate：插入/删除/移动/边界
- keyparse：普通字符、Enter(\r,\n)、ESC 序列整块与分块到达、UTF-8 中文跨块、Ctrl 系
- linerender：ghost、面板高亮、宽字符列宽
- 回归：全仓 11 包

## 实施记录

- 2026-10-26 立项即实施，见当日日志。
- 2026-10-26 **落地**：`linestate.go`（状态机，3 用例）、`keyparse.go`（字节解析器：ESC 序列/UTF-8 跨块组装/CRLF 合并/杂散控制字节忽略，19 断言）、`linerender.go`（纯 ANSI 渲染，runewidth 宽字符列宽，4 用例）、`rawreader.go`（MakeRaw 读循环装配）；`line.go` tea 版删除；`input.go` 切换 rawReader；`repl` 包不再引用 bubbletea（huh/内部编辑器不受影响）。`go build` + `go vet` + 全量 11 包全绿。真实终端验收待开发者。
