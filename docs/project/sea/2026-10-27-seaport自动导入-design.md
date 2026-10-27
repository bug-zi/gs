# seaport 自动导入 · 设计

> 创建于 2026-10-27(brainstorming 产出,状态:已确认待实施)

## 1. 背景与目标

sea 是学习工作区的外部知识库(`Study/sea/` 下按主题分目录)。目前导入靠手动:文件放进 `Study/sea/inbox/` 后执行 `/sea import` 逐篇整理(1.2.0 后 sea 主题目录重组,inbox/clip 已不在默认结构中)。

本设计引入 **seaport(海港)**:工作区根目录下的 `D:\Code\Go\seaport\` 作为 sea 的投递口。开发者把想导入的文件丢进去,gs 启动时自动完成「AI 内容分类 → 迁入 sea 对应主题目录」,原件随迁移消失(os.Rename 原子移动)。零交互、零打扰、不丢文件。

## 2. 需求决策记录(brainstorming 结论)

| 决策点 | 结论 |
|---|---|
| 交互方式 | 全自动静默导入,无逐篇确认 |
| 落点 | 按文件**内容**自动分类到 sea 一级主题目录 |
| 失败兜底 | AI 未配置/失败/目录不匹配 → 移入 `sea/未分类/` |
| 执行时序 | 启动后**后台异步**执行,不阻塞进 REPL |

## 3. 架构

### 3.1 新纯逻辑包 `internal/seaport`

无 UI 依赖,`out io.Writer` 由调用方注入,可独立测试(符合全局规则「命令层纯逻辑与交互壳分离」)。

- `Dir(ws *workspace.Workspace) string` — 返回 `<ws.Root>/seaport`
- `Ensure(ws) error` — 目录不存在则 `MkdirAll`(首次启动即建好)
- `Scan(ws) ([]string, error)` — 列出 seaport **顶层** `.md`/`.txt` 文件(仅文件,忽略子目录;扩展名不区分大小写)
- `Import(ctx context.Context, ws, cfg *config.Config, client *ai.Client, out io.Writer) Result` — 完整导入流程

### 3.2 分类引擎

复用 `internal/ai.Client`(与 `/AI` 同一套 `~/.gs/config.toml` 的 `[ai]` 配置),零新增配置项。

分类调用约定:

- 候选目录 = `Study/sea/` 下的一级子目录名(动态读取,目录增减自动适配;含 `未分类`)
- 输入:每篇 = 文件名 + 内容前 2000 字符(截断仅用于分类,迁移是全文原样)
- 一次调用批量分类全部文件,temperature 0
- 要求只返回 JSON:`{"分类": {"<文件名>": "<候选目录名>", ...}}`

### 3.3 集成点 `internal/repl`

`Run()` 启动时序:

1. `seaport.Ensure` + `seaport.Scan`
2. 无文件 → 直接进 REPL(零开销,不读 AI 配置不建连接)
3. 有文件 → banner 追加一行 `seaport 有 N 个文件,后台导入中…`,起 goroutine 执行 `Import`
4. REPL 主循环每次 `reader.Read()` 返回后,非阻塞收割结果 channel,有则打印导入明细
5. 退出(/exit、Ctrl-D)不等待后台任务,直接结束

## 4. 数据流

```
启动 → Scan → 无文件:进 REPL
            → 有文件:banner 提示 → goroutine:
    读候选目录 → 批量 LLM 分类 → 解析校验 → 逐篇 os.Rename → 结果入 channel
REPL 首次回车后 → 打印「已导入 N 篇:xxx.md → sea/<目录>/」
```

- 迁移即导入:原样搬运,不加导入头、不改内容
- 重名:目标已存在同名文件时,文件名加序号后缀(`闭包.md` → `闭包-2.md`)
- 同盘 rename 原子,天然满足「原件直接删除」与全局原子写纪律的精神(不产生半截文件)

## 5. 错误处理与边界

| 情形 | 处理 |
|---|---|
| AI 未配置(`[ai]` 段缺失) | 全部移 `sea/未分类/`,结果行说明原因 |
| 网络失败 / 响应不可解析 | 同上 |
| 返回目录名不在候选中(精确匹配,不分大小写) | 该篇移 `sea/未分类/` |
| 返回 JSON 缺某文件条目 | 该篇移 `sea/未分类/` |
| rename 失败 | 留在 seaport,结果行报错 |
| 退出时后台未完成 | 不等待直接退出;未迁移文件原封在 seaport,下次启动自愈重试 |
| 非 `.md`/`.txt` 文件(pdf、图片等) | 不处理,留在 seaport,banner 提示一次 |
| seaport 中的子目录 | 忽略,仅处理顶层文件 |
| ctx 取消(REPL 已退出) | 停止迁移,不再写输出 |

## 6. 测试

`internal/seaport` 单测(fake AI client 注入):

- Scan:扩展名过滤、子目录忽略、空目录
- 候选目录读取:sea 子目录动态列出
- LLM 响应解析:合法 JSON、非法 JSON、目录名不在候选、缺条目
- 兜底:AI 未配置/调用失败 → 未分类
- 重名后缀、rename 失败留存
- ctx 取消停止迁移
- REPL 集成:结果收割打印(经 out 断言)
