# code 层：知识点级 Go 演示代码 设计

日期：2026-10-27
状态：**已落地（2026-10-27）**——按本设计实施完成：`go build ./...` + `go vet` + `go test ./...` 全绿，真实 roadmap 副本冒烟核心链路（创建+绑定+模板可运行）通过；逾行长 .go 的查看高亮待与大文件 TUI 会话的 viewer 统一
- 已归档：261027 随 code 层知识点演示代码落地
所属模块：plan（`code` 绑定字段正本在 roadmap.toml；跨 spring/视图层改动，`docs/project/全局/三层学习框架-design.md` 框架总纲下新增第四层）
前置依赖：路线图 v2 与 plan 浏览器重构均已落地（point 级 `spring`/`river`/`sea` 字段模型）

---

## 1. 背景与动机（开发者原话整理）

- plan 中的每个知识点，除了配有一个 spring 级别的 md 总结笔记以外，还需要有一个 .go 文件的代码演示文件，该知识点相关的代码存在这个 go 文件中。
- 在 gs 中建立新模块 code，和 spring 一样参考 plan 中的架构初始化：每一个 plan 中的知识点在 spring 都有一个与之对应的笔记文档，code 也同之——每一个 plan 中的知识点在 code 中都有一个与之对应的 go 文件。
- 在 plan 中的知识点下也要能找到对应的这个 go 文件（plan 浏览器详情页可见、可打开）。

## 2. 定位与现状盘点

学习闭环由三层补成四层：**sea 来源 → river 摘录 → spring 总结 → code 动手**。spring 承载「用自己的话总结」，code 承载「亲手写出可运行的演示代码」。

现状：

- `roadmap.Point` 已有 `spring`（0/1）/`river`/`sea` 绑定字段；`/spring add` 套路成熟（定位 → 已存在则打开 / 创建占位并绑定落盘再打开）；
- `/plan` 浏览器知识点详情页已渲染 spring/river/sea 三小节，笔记行 Enter/o 可打开；
- `Study/code/` 目录与 `code` 字段均不存在；chroma 已在依赖树（glamour 间接依赖），可直接用于 Go 语法高亮。

## 3. 已确认的关键决策（brainstorming 问答记录）

| # | 问题 | 决策 |
|---|---|---|
| 1 | 存放位置与组织 | **gs 新增与 spring 平级的 code 层**：`Study/code/<标题>.go` 平铺，命名与 `Study/spring/<标题>.md` 完全对称（标题全树唯一 → 文件名天然不撞）。独立于 `Study/code_learing` 模块（其按学习模块分域的章节/菜单体系不与 roadmap 知识点混编）。不引入 go.mod：工作区根无 go.mod，编辑器按单文件 package 解析，无重名冲突 |
| 2 | 绑定模型 | point 级新字段 `code`（0/1 覆盖式，与 `spring` 完全对称），正本在 roadmap.toml |
| 3 | plan 联动 | 知识点详情页新增 code 小节（spring 之后），未创建给提示行，已绑定可打开 |
| 4 | 命令面 | 新增 `/code` 命令：`/code add` 镜像 `/spring add`；默认与 `edit` 走板块化浏览/编辑（主干分组 + 未挂载组） |
| 5 | 查阅渲染 | .go 文件不走 markdown 渲染，用 chroma 语法高亮（低饱和样式，遵守「禁彩亮饱和色」硬性规则） |

## 4. 数据模型

```toml
[[branch.point]]
title = "slice 底层结构与扩容"
status = "doing"
spring = "Study/spring/slice 底层结构与扩容.md"
code = "Study/code/slice 底层结构与扩容.go"   # 新增，0/1 覆盖式
sea = ["Study/sea/Go语言基础/1.2.1 Go语言基础之切片.md"]
```

- `Point.Code string`（toml `code,omitempty`）+ `BindCode(ref, rel)`（覆盖语义，镜像 `BindSpring`）；
- 绑定路径一律工作区相对路径（含 `Study/` 前缀），与既有规范一致；
- `Study/code/` 目录缺失时首次创建自动建（MkdirAll）；
- 落盘走既有 `roadmap.Save`（原子写；重编码丢手写注释为既有预期）。

新建文件初始模板（可运行、gofmt 合规、UTF-8）：

```go
// 知识点「slice 底层结构与扩容」演示代码
// 运行：go run "Study/code/slice 底层结构与扩容.go"
package main

import "fmt"

func main() {
	// TODO: 写下与该知识点相关的演示代码
	fmt.Println("slice 底层结构与扩容")
}
```

## 5. 命令面（新增 `/code`，改 2 处渲染分流）

| 命令/改动 | 行为 |
|---|---|
| `/code add <关键词>` | 镜像 `/spring add`：定位知识点（唯一直达/多匹配弹选/空参全量弹选）→ `p.Code` 非空直接打开；否则创建 `Study/code/<标题>.go`（模板内容）+ `BindCode` 落盘 + 打开编辑器 |
| `/code [关键词]` / `/code edit [关键词]` | 板块化浏览/编辑：按 roadmap 主干分组（复用 `pickTierEntryGrouped`），未绑定 .go 进「未挂载」组；查阅态走代码高亮渲染 |
| `openPlanNote`（plan.go）与 `runTierItemView`（tier_view.go） | 按扩展名分流渲染：`.md` → `ui.RenderMD`（现状），`.go` → chroma 高亮（样式与 ui 主题近色系，收敛为一个 ui 层小函数，如 `ui.RenderGo`） |

## 6. /plan 浏览器

知识点详情页 spring 小节之后新增：

```
code · 演示代码 ×0/1
  （尚未创建，/code add slice 底层结构与扩容 可创建并绑定）
```

- 已绑定时显示路径行（复用 noteRow，含 `[失效]` 标注），Enter/o 打开；
- `browser.go` 按键循环与 Hooks 零改动（行机制通用）。

## 7. 代码架构

延续「命令层纯逻辑与交互壳分离」：

```
internal/workspace/workspace.go  # 加 CodeDir()
internal/roadmap/roadmap.go      # Point 加 Code 字段
internal/roadmap/ops.go          # 加 BindCode
internal/command/knowledge.go    # tierDirs 加 code；ListTierFiles 按层选扩展名
                                 #（spring/river/sea 仍 .md，code 取 .go）
internal/command/entry.go        # 新增 CreateCodeEntry（写 .go + 模板，原子写）；
                                 # CreateEntryIn 签名不动
internal/command/tier_cmd.go     # 新增 runCodeAdd 与 newCode()；runSpringAdd 不动
internal/command/help.go         # 注册 /code 命令
internal/command/tier_view.go    # codeBinds；松绑条目标题按层取 .md/.go 后缀；
                                 # runTierItemView 渲染分流
internal/command/plan.go         # openPlanNote 渲染分流
internal/planview/state.go       # pointRows 加 code 小节（纯状态机，无新依赖）
internal/ui/…                    # 新增 RenderGo（chroma 终端高亮，低饱和样式）
```

边界划分：`roadmap`/`planview` 纯逻辑包不 import ui、不碰文件存在性；渲染、IO、交互全在 command 壳层。

## 8. 错误处理

| 场景 | 行为 |
|---|---|
| `Study/code/` 目录缺失 | 创建时 MkdirAll 自动建 |
| `p.Code` 为空但同名 .go 已存在（手工放过文件） | 报「条目已存在」，与 spring 同规则；手工在 roadmap.toml 补 `code = ...` 即接管 |
| 绑定指向的文件不存在 | 详情页/列表标 `[失效]`，不自动清理（同既有约定） |
| 关键词零/多匹配 | 与 `/spring add` 同规则：零匹配报错，多匹配弹 select |
| roadmap.toml 缺失 | `/code add` 报错（同 spring）；`/code` 浏览退化为按目录平铺（rm 为 nil 归单组，复用 buildRoadmapGroups 既有行为） |

## 9. 测试与验收

- `internal/roadmap`：含 `code` 字段的 TOML 往返解析；`BindCode` 覆盖语义；
- `internal/planview`：详情页 code 小节三态（未建 hint / 已绑定 / 失效标注）行数与文本；
- `internal/command`：`ListTierFiles("code")` 只收 .go；`CreateCodeEntry` 模板内容与原子写；`runCodeAdd` 创建+绑定+落盘、已绑定直接打开（交互壳按惯例不自动测）；
- 验收：`go build ./...` + `go vet ./...` + `go test ./...` 全绿；真实工作区冒烟——`/code add 切片` 建文件并绑定 → 编辑器打开 → `/plan` 详情页见 code 小节 → Enter 打开 → `/code` 列表按主干分组可见。

## 10. v1 边界（不做）

- gs 内 go run / go vet 集成（跑代码仍靠终端，后续走优化建议区）；
- /ai、/show 消费 code 文件（两者当前不消费任何绑定）；
- `/search` 索引纳入 .go（避免代码噪音混入知识库全文检索，需要时再议）；
- `/spring add` 侧「同名文件已存在则直接接管绑定」的对齐改造（保持现状对称）；
- 一个知识点多份代码文件（0/1 已满足「每个知识点一个演示文件」诉求）。

## 商讨记录

- 2026-10-27 建立商讨稿：开发者提出 plan 每个知识点在 spring 笔记之外还要有对应 .go 演示文件并在 plan 下可见；AI 盘点现状（roadmap v2 绑定模型、/spring add 套路、plan 浏览器三小节、Study/code_learing 既有模块），给出三种存放方案。
- 2026-10-27 存放定案：开发者选「在 gs 中建立新模块 code，和 spring 一样参考 plan 架构初始化并绑定」，即 `Study/code/<标题>.go` 平铺对称方案，plan 详情页可见可打开。
- 2026-10-27 设计六节（定位与数据模型、命令层、plan 浏览器、纯逻辑包适配、测试、非目标）逐节呈现并获确认（「继续」），落盘本 spec。
