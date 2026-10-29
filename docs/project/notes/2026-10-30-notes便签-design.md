# notes 便签：临时记录缓存与 AI 整理归位 · 设计文档

> 创建于 2026-10-30，brainstorming 产出。需求正本暂以本对话为准（模块正本 `notes.md` 由开发者酌情另立）。

## 背景与目标

学习时常有一两句临时想法（如「slice、map、channel 可以看作引用类型」），当下不知道该放进 river 哪篇或哪个 spring，直接记会丢。`/notes` 提供一个轻量缓存区：随手记录，之后手动归位（`move`），或交给 AI 整批整理归位（`tidy`）；没有合适归宿的便签留在缓存区。

## 已确认的关键决策

| 决策点 | 结论 |
|---|---|
| 存储形态 | 单文件 `Study/notes.md`（否决：每便签一文件的 notes/ 目录） |
| 手动归位 | 提供 `/notes move` 命令（选便签 → 选目标 → 追加 + 移除），另有 `del` 丢弃 |
| tidy 交互 | 整批方案 + 一次确认（否决：agent 逐笔 write_file 确认、只建议不动手） |
| tidy 实现 | 独立一次性流程：agent 循环只挂只读工具，最终输出结构化 JSON 方案（方案 A） |
| 合并粒度 | AI 返回目标文件完整新内容（预览即 diff），否决插入指令锚点方案 |

## 存储格式

`Study/notes.md`，UTF-8，工具全权管理、人也可读：

```markdown
# 便签

> 临时记录，待整理进 river/spring

- [1] 261029 14:30 slice、map、channel 可以看作引用类型
- [2] 261029 15:12 defer 在 return 之后执行
```

- 每条一行 `- [序号] YYMMDD HH:MM 文本`；时间戳东八区（`workspace.CST`），取录入时刻
- 序号仅展示用，每次保存按顺序重排（1 起连续）
- 文件不存在视为空表；不识别的行解析时跳过、不保存（自由手改走 `/notes edit`，工具不保未知行）
- 路径知识收敛在 `notes.Path(root)` 唯一来源，不另加 `workspace.NotesFile()`（计划期收敛）

## 纯逻辑包 internal/notes（不 import ui）

```go
type Note struct {
    Time time.Time // 东八区
    Text string
}
```

- `Load(root) ([]Note, error)`：解析子弹行（锚定 `- [N] YYMMDD HH:MM ` 完整格式），文件不存在返回空切片
- `Save(root, notes) error`：重编号 + 原子写（`workspace.WriteFileAtomic`）；文件不存在则先建骨架（固定头部）
- `Add(root, text, now) error`：Load → 追加 → Save
- tidy 方案类型与执行：
  ```go
  type Placement struct {
      NoteIdx    int    // 0 起，对应 Load 顺序
      Target     string // 工作区相对路径
      NewContent string // 目标文件完整新内容
  }
  type Plan struct{ Placements []Placement }

  func ParsePlanJSON(s string, ns []Note) (Plan, error) // 校验：JSON 合法、NoteIdx 在界、Target 非..穿越/非绝对、NewContent 非空（需便签清单做越界校验）
  func Apply(root string, plan Plan, notes []Note) error
  ```
- `Apply` 语义：按 plan 顺序逐目标原子写；任一目标写失败即中止且**不动 notes.md**（便签仍在，幂等可重跑）；全部成功后重写 notes.md 移除已归位便签。同一目标出现多个 placement 时按序应用，AI 侧保证 content 累计包含前序内容

## 命令层 /notes（internal/command/notes.go）

注册名 `notes`，`Short` 如「便签：临时记录缓存/手动归位/AI 整理」，`Subs: ["edit", "del", "move", "tidy"]`（Tab 补全自动生效，`help.go` 无需特判）。首词路由：命中子命令走子命令，否则整行拼接视为录入。

| 用法 | 行为 |
|---|---|
| `/notes` | 列出全部便签（编号 + 时间 + 文本，超长截断显示）；空则「暂无便签，用 /notes <文本> 记录」 |
| `/notes <文本>` | Args 拼接为一条便签，`Add` 后回显确认 |
| `/notes edit` | 编辑器打开 notes.md（`editOpen`，不存在先建骨架） |
| `/notes del [kw]` | 便签选择器（关键词过滤文本）→ confirm → 移除保存 |
| `/notes move [kw]` | 便签选择器 → 选站（river/spring，二级选择）→ `pickTierEntryGrouped` 选条目 → 预览 confirm → 便签**原文原样追加**到目标文件末尾（确保原文件以换行结尾后，追加空行 + 文本；不改写原文），并从 notes.md 移除 |
| `/notes tidy` | AI 整批整理，见下节 |

便签选择器为平铺 menuOption（`[N] MMDD HH:MM 文本`），不走分组。

## /notes tidy AI 流程

1. `Load` 便签，空 → 「没有便签可整理」；`ensureAIState` 失败（未配置）→ 现有友好错误
2. 候选清单：`ListTierFiles` 取 `river` + `spring` 全部条目相对路径，注入提示词
3. `agent.Agent.Run` 一轮循环，只挂只读工具（复用 `agent.SearchTool` / `ReadFileTool` / `ListDirTool`），步数默认
4. 提示词规则：便签按序号列出；只能归位到候选清单中的现有文件；没有合适的就不出现在方案里（留在 notes）；`content` 必须保留目标原文、把便签融入合适位置；同一目标多条便签时各 placement 的 content 累计包含前序内容
5. 约定最终回答为 JSON：`{"placements":[{"note":1,"target":"Study/river/...","content":"...完整新内容..."}]}`，`note` 为提示词中便签序号（1 起）
6. `ParsePlanJSON` 校验（下标映射 0 起）→ 逐条渲染预览：便签文本 → 目标路径，`diffx` 计算目标旧内容 vs NewContent 只显新增行 → 一次 confirm 应用全部
7. `Apply` 落盘 → 汇报「归位 N 条、保留 M 条」

解析失败（AI 未给合法 JSON）→ 报错退出，便签原样不动，可重跑。

## 边界处理

- 预览确认到落盘之间目标文件被手改 / 便签被手改：单用户 REPL，窗口秒级，接受直接覆盖（本节即声明）
- 便签文本本身以 `- [` 开头：解析锚定完整格式不误判；录入不做限制
- notes.md 损坏/乱改：解析只取合法子弹行，其余忽略；下次保存即归一
- 单文件两步写（目标 + notes.md）非跨文件原子：以「先目标后 notes.md、失败中止」保证便签不丢

## 测试策略

- `internal/notes`：Load/Save 往返、Add 首建骨架、保存重编号、不识别行跳过、`- [` 伪装行不误判、ParsePlanJSON 非法矩阵（坏 JSON / 下标越界 / 路径穿越 / 空 content）、Apply 成功路径与目标写失败不动 notes.md、同目标多 placement 顺序应用
- command 层：首词路由（子命令 vs 录入文本）、move/del 交互流（沿用现有输入注入测试风格）、tidy 以 fake ChatClient 返回固定 JSON 验证预览、confirm 与落盘、解析失败不动文件

## 涉及文件

- 新增 `internal/notes/`（notes.go / plan.go + 测试）
- 新增 `internal/command/notes.go`（+ 测试）
- `internal/workspace/workspace.go`：加 `NotesFile()`
- 命令注册处（repl 命令表）挂载 `/notes`

## 非目标

- 不接入 roadmap/tier 绑定体系（便签是路线图无关的缓存区）
- tidy 不新建任何文件；没有合适目标就留在 notes
- 不做标签、搜索、定时自动 tidy（列表短，关键词过滤选择器够用）
