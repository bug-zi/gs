# ai

ai 模块文档目录：AI 能力（/AI 交互模式、/review 体检、底层客户端与 Agent 循环）的需求正本与 spec/plan 归属。

**使用方式**：涉及 `/AI`、`/review`、AI 客户端协议、Agent 工具循环、写审批 diff 通道、会话存档的功能，其 design.md / spec / plan 放本目录；落地或废弃的 spec/plan 移入本目录 `archive/`。

**本目录内容**：

- `design.md` — AI 模块需求正本（AI 起草，待开发者审核定稿）
- `archive/` — 已落地/废弃的 spec/plan 归档处（AI模式Agent化 spec/plan 已随模块立项迁入）

**边界**：`design.md` 的修改权在开发者；AI 只可提建议或经审核协助。跨命令模块的全局性交互调整放 `全局/`。
