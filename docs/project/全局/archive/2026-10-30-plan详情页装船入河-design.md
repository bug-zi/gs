# /plan 详情页装船与入河（sea→ship 复制、ship→river 迁移）设计

日期：2026-10-30
状态：**已落地（2026-10-30 00:35）**——详情页 c 装船 / r 入河全量实施，全仓 build+vet+test 全绿（真机冒烟待开发者）
所属模块：全局（跨 sea/ship/river 流转与 planview 交互；是 `archive/2026-10-27-四站流转与river编号-design.md` 第 11 节「范围外：planview 浏览器内流转按键——后续可议」的落地）

---

## 1. 背景与动机（开发者原话整理）

- 在 `/plan` 知识点详情页（spring/code/river/ship/sea 五节清单）浏览时，经常想顺手把 sea 中某篇复制到 ship（装船），或把 ship 中某篇移入 river（入河）。
- 现状必须退出浏览器回 gs 首页敲 `/ship copy` / `/ship move`，用选择器重新定位文件，路径绕。
- 典型场景：详情页 sea 节 5 篇，想一次把其中 3 篇装船——希望在详情页内连续完成。

## 2. 已确认的关键决策（brainstorming 问答记录）

| # | 问题 | 决策 |
| --- | --- | --- |
| 1 | 操作粒度 | **逐条单键**：光标在 sea 行按 `c` 装船、ship 行按 `r` 入河；装船后光标定位到下一 sea 行，连按 `c c c` 即可连装多篇。批量多选（空格/v 标记）不做，后续可议 |
| 2 | 装船后行为 | **只装船不打开编辑器**：落盘更新清单即可；想开始加工时对 ship 行按 Enter/o 打开。与 `/ship copy`（装船即开编辑器）口径不同——浏览流不被打断 |
| 3 | 键位形态 | **详情页专用单键**（方案 A），不复用 `o` 菜单（方案 B，多一步被否）；与现有 `x` 卸载 / `m` 移动 / `a` 挂载单键风格一致 |

## 3. 交互设计

### 3.1 键位

| 键 | 光标位置 | 动作 |
| --- | --- | --- |
| `c` | 详情页 sea 笔记行 | 装船（sea→ship 复制） |
| `r` | 详情页 ship 笔记行 | 入河（ship→river 迁移） |

- 两键仅详情页生效；光标不在对应层的笔记行时按 `x`/`m` 现行口径报错误行（如「光标不在 sea 条目上」），不改状态。
- 详情页提示行（`view.go` viewHint）追加：`c 装船　r 入河`。

### 3.2 装船流程（`c`，sea 行）

1. 目标路径 = `Study/ship/<sea 原文件名>`（保留原名，含源编号——与 `/ship copy` 同口径）。
2. `CopyFileRel` hook 复制文件（sea 原件与 sea 绑定**原样保留**）；目标已存在则报错误行，不产生任何变更。
3. 当前知识点 ship 清单追加绑定（去重）；sea 绑定不动。
4. 落盘（复用现有 `save`）；sea 行随即自动灰显（现有 `seaShipped` 渲染逻辑生效）。
5. 光标重新定位到同一条 sea 行后下移一行（落点优先下一个未消费 sea 行），便于连按。

### 3.3 入河流程（`r`，ship 行）

与 `/ship move` 同口径（省去选择器与挂载追问——ship 行必然属于当前知识点）：

1. 领号改名：剥 ship 文件名编号（`StripNumPrefix`），入 `Study/river/<NN>-<主干>/<点两位>-<标题>.md`；同点重名篇序自动顺延（`NextDupSeq` 起步 + 目标已存在冲突顺延重试）。
2. `MoveFileRel` hook 迁移文件（同盘 rename + 目录自动创建）。
3. 绑定迁移 ship[]→river[]（复用 `MoveTierBinding`，全树扫描幂等）。
4. 落盘；sea 行灰显联动同上。
5. `RewriteChainsRel` hook 全库出处链改写；**失败仅提示错误行，不回滚**（与 `/ship move` 一致）。

### 3.4 错误处理一览

| 场景 | 行为 |
| --- | --- |
| 光标不在 sea/ship 笔记行按 `c`/`r` | 错误行提示，无变更 |
| 装船目标 `Study/ship/<名>` 已存在 | 报「目标已存在」，无变更（与 `/ship copy` 幂等口径一致） |
| hook 为 nil（降级/测试环境） | 报「该操作不可用」（现行 Hooks 口径） |
| 入河领号冲突 | 篇序顺延重试；穷尽后报错误行 |
| 出处链改写失败 | 提示错误行；迁移与绑定已完成，不回滚 |
| 操作对象文件已 `[失效]`（磁盘不存在） | 报错拒绝（文件都不在，复制/迁移无从谈起） |

## 4. 技术方案

### 4.1 改动点

```
internal/planview/
├── flows.go     # 新增 doShipCopy / doShipRiver（纯状态机；绑定用现成 RM 方法，落盘走现有 save）
├── browser.go   # 键位分发加 'c'/'r' 分支（pauseAndAct 现成模式）
└── view.go      # viewHint 详情页提示行追加 c/r

internal/command/
└── plan.go      # Hooks 接线新增 3 个回调
```

### 4.2 Hooks 增量（planview.Hooks）

| Hook | 壳层实现 | 用途 |
| --- | --- | --- |
| `CopyFileRel(srcRel, dstRel string) error` | `tier.CopyFile(ctx.WS.Root, …)`（现有） | 装船复制（原子写） |
| `MoveFileRel(srcRel, dstRel string) error` | `tier.MoveFile(ctx.WS.Root, …)`（现有） | 入河迁移 |
| `RewriteChainsRel(srcRel, dstRel string) error` | `tier.RewriteSourceChains(ctx.WS.Root, …)`（现有） | 入河出处链改写 |

- **不复用 `ApplyMoves`**：那是 renumber 批量语义（附带 topics.toml 改写与空目录清理），与 `/ship move` 单文件口径不符。
- 入河顺序 move→绑定→落盘→chains，chains 失败不回滚，与 `/ship move` 完全一致。

### 4.3 入河目标名计算

- planview 内用现成 `roadmap.StripNumPrefix / NextDupSeq / RiverFileName` 计算 dstRel，与 command 层 `riverLanding`（tier_cmd.go）同构；冲突顺延重试循环留在 planview 侧。
- 不抽公共函数：两处各约十几行，按「不过度抽象」原则保持各自独立，注释互相指认。

## 5. 测试

`internal/planview/flows_test.go` 补用例（沿现有表驱动风格，注入 fake hooks）：

1. 光标在 sea 行按装船 → ship 清单追加、sea 清单不变、Save 被调、CopyFileRel 收到正确 src/dst。
2. 光标在 ship 行按入河 → river 追加领号名、ship 移除、MoveFileRel/RewriteChainsRel 收到正确路径。
3. 入河重名 → 篇序顺延（NextDupSeq 起步 + 冲突再顺延）。
4. 光标不在对应层（river 行按 c、sea 行按 r、非笔记行）→ 报错无变更。
5. hook 为 nil → 报「该操作不可用」。
6. 装船目标已存在（CopyFileRel 返回错）→ 错误行提示、绑定未变。
7. 装船后光标定位：连续装船可顺序推进。

壳层接线（plan.go Hooks）与 `/ship copy`/`/ship move` 复用同一批 tier 函数，不重复测试；验收：`go build ./...` + `go vet ./...` + `go test ./...` 全绿 + 真机冒烟（详情页 c 装船 3 篇 → r 入河 1 篇 → 清单/灰显/出处链跳转核对）。

## 6. 范围外（本轮不做）

- 批量多选（空格/v 标记 + 一键批量装船/入河）——逐条单键已覆盖连装诉求，批量模式后续可议。
- 详情页 river→其他站、ship→sea 回流等逆向流转——沿用 2026-10-27 设计的手动兜底口径。
- `/ship copy`、`/ship move` 命令行入口不变（命令层与浏览器层并存，口径一致）。

## 商讨记录

- 2026-10-30 立项：开发者提出在 /plan 详情页直接装船/入河的诉求（典型场景：sea 5 篇连装 3 篇）；AI 探场确认 planview 状态机 + Hooks 架构可平滑扩展，/ship copy、/ship move 核心逻辑均可复用。
- 2026-10-30 问答定案：逐条单键（连按推进）而非批量多选；只装船不打开编辑器。
- 2026-10-30 设计（键位/装船流/入河流/错误处理/技术方案/测试/范围外）整篇呈现，开发者确认，落盘本 spec。
