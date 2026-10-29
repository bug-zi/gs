# REPL 候选面板交互升级：频率排序 / 循环选择 / 延迟参数提示

日期：2026-10-28
状态：**已落地（2026-10-28，`go build`+`go vet`+`go test ./...` 全绿；真机冒烟待开发者）**
- 已归档：261028 随 REPL 候选面板交互升级落地
所属：`全局/`（输入行交互；前案：自绘输入行 2026-10-26）

- 需求来源：开发者口述三条——① 输入 `/` 后候选按使用频率从高到低排序；② 面板选中第一行按 ↑ 跳最后一行（循环）；③ 输入 `/sea` 这类唯一命中后约 1 秒即显示参数用法提示，不必先敲空格。
- 已确认决策：频率数据**从历史文件推导**（`~/.gs/history` 已记录每条提交行，启动时统计，零新增持久化状态，老历史即刻生效）。

## 一、命令按使用频率排序

- 启动时一次性重排，`complete` 包匹配逻辑零改动：
  - `repl` 新增 `freqFromHistory(hist *History, cmds []command.Command) map[string]int`：遍历历史行，取 `/` 开头行的首词，经 `command.Find` 归一别名到正式命令名后计数。
  - `repl` 新增 `sortCmdsByFreq(cmds []command.Command, freq map[string]int)`：按次数**稳定降序**（`sort.SliceStable`）；并列与零次保持 `Build()` 默认序，自然垫底。
  - `repl.Run` 中排完序再 `toCompleteCmds` → `BuildTree`。候选匹配按树序遍历，`/` 全量列表与前缀过滤候选即自动按频率排列。
- 边界：
  - 二级子命令（`cmd.Subs`）**不参与**排序，维持原序（集合小且语义分组）。
  - `/help` 输出不变（仍按默认序，作稳定参考）；频率只影响候选面板。
  - 排序冻结在启动时刻：会话内新敲命令不改变本次面板顺序，下次启动生效——避免打字时面板自行跳动。

## 二、上下键循环选择

- 提取纯函数 `wrapSel(sel, delta, n int) int`（可单测）：
  - `delta<0`（↑）：`sel<0` → 0（就近选第一行）；`sel==0` → `n-1`（跳最后一行）；否则 `sel-1`。
  - `delta>0`（↓）：`sel==n-1` → 0（回第一行）；否则 `sel+1`（含 `-1` → 0）。
- `rawReader.Read` 的 `KindUp`/`KindDown` 面板分支改调 `wrapSel`；面板未弹时 ↑↓ 仍走历史翻阅（`histMove`），行为不变。

## 三、唯一命中 1 秒延迟参数提示

- `complete.Result` 增字段 `Single string`（首词唯一命中某命令时为其正式名，含精确命中 ghost 态，如 `/sea`、`/SE`；≥2 候选或零候选为空）与 `SingleHint string`（对应命令的参数提示，供延迟提示渲染，别名候选归一到正式名）。`For` 内在单命中分支填充。
- `rawReader` 新增状态：`hintTimer *time.Timer`、`hintOn bool`、`hintLine string`（触发时的行文本快照）。
- 触发与失效：
  - 每轮按键处理完后：停掉 pending 定时器、`hintOn=false`；重绘后若「`res.Single != ""` 且面板未弹」→ 以当前行文本为快照，起 1 秒 `time.AfterFunc`。
  - 定时器回调：加锁复核「行文本仍等于快照、仍唯一命中、面板未弹」→ `hintOn=true` 并重绘；否则放弃。
  - `Read` 返回（Enter/Esc/Ctrl-D）时停表，协程不外泄。
- 渲染：`hintOn` 时在输入行下方画一行暗色用法提示 `  sea <关键词>`（两空格缩进 + 命令名 + `cmd.Hint`；无 Hint 的命令不触发显示）。面板与延迟提示互斥——面板弹出（敲空格后）走原 `Hint` 顶部灰字路径。
- 并发安全：重绘闭包与定时器回调共用一把 `sync.Mutex`；定时器协程只在持锁期间写 stdout。主循环阻塞在 `os.Stdin.Read` 时定时器可独立完成重绘（句柄不同，raw 模式下安全）。
- `plainReader` 降级路径不受影响。

## 范围外（后续另议）

配色美化（提示符上色、选中行反白改主题金、面板右侧一句话命令描述、banner 调整等候选方向）不在本轮；三件落地验收后单独出美化提案再议。

## 测试

- `complete`：`Single` 字段用例（精确命中 / 大小写唯一命中 / 多候选 / 零命中 / 二词阶段为空）。
- `repl`：`wrapSel` 循环边界用例；`freqFromHistory` 别名归一与非 `/` 行忽略用例；`sortCmdsByFreq` 稳定性与零次垫底用例。
- 落地判定：`go build ./...` + `go test ./...` 全绿（项目标准）；真机冒烟（1 秒提示、循环键感）由开发者验收。
