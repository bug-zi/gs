# code · 代码层设计

日期：2026-10-27
状态：**AI 起草，待开发者审核定稿**——依开发者指示立项并已落地（首份 spec 见 `plan/archive/2026-10-27-code层知识点演示代码-design.md`）；本文档对照代码现状（`internal/command/tier_cmd.go`、`internal/planview/state.go` 等）整理既有定案，未新增需求。

## 1. 定位

code 是四层学习框架的第四层：**动手演示**。每个 plan 知识点配一份可运行的 .go 演示代码，相关代码就写在这一个文件里。四层闭环：sea 来源 → river 沉淀 → spring 总结 → code 动手（框架级修订权在 `全局/三层学习框架-design.md`，正本由开发者维护）。

## 2. 存储结构

`Study/code/`，平铺；一个知识点一个 .go 文件，文件名 = 知识点标题（标题全树唯一保证不撞，与 spring 同约定）。不引入 go.mod：工作区根无 go.mod，编辑器按单文件 package 解析无重名冲突；单文件 `go run "Study/code/<标题>.go"` 直接可跑。

新建文件初始模板（可运行、gofmt 合规）：

```go
// 知识点「<标题>」演示代码
// 运行：go run "Study/code/<标题>.go"
package main

import "fmt"

func main() {
	// TODO: 写下与该知识点相关的演示代码
	fmt.Println("<标题>")
}
```

## 3. 命令行为（已落地核对）

| 命令 | 行为 |
|---|---|
| `/code add <知识点关键词>` | 定位知识点（唯一直达/多匹配弹选/空参全量弹选）→ 已绑定则直接打开；否则创建模板文件 → 绑定落盘 → 调起编辑器 |
| `/code [关键词]` | 查阅：按 roadmap 主干分组两级选择，未绑定 .go 归尾部「未挂载」组；有关键词平铺直选；.go 走语法高亮渲染 |
| `/code edit [关键词]` | 选择条目调起编辑器 |

渲染分流：`.md` 走 `ui.RenderMD`，`.go` 走 `ui.RenderGo`（chroma 低饱和配色，复用 gsStyle）。

## 4. 与 roadmap 的绑定

知识点通过 `code` 字段**唯一绑定**（0/1，重绑覆盖语义，镜像 `spring` 字段）。`/plan` 浏览器知识点详情页在 spring 小节后渲染「code · 演示代码 ×0/1」小节：未建给提示行、已绑可 Enter/o 打开、失效标 `[失效]`。

## 5. 与其他模块的关系

- **plan**：绑定关系正本在 roadmap.toml；详情页可见可打开。
- **spring**：同构的 0/1 绑定与创建套路（`/code add` 镜像 `/spring add`）。
- **大文件 TUI**（2026-10-27 并行落地）：逾行 .go 经 runLargeFile 打开方式选择器；其中暂无代码高亮，待统一。

## 6. 边界（不做）

- gs 内 go run / go vet 集成；
- `/search` 索引纳入 .go；
- 一个知识点多份代码文件。
