# ship

ship（船坞）模块文档目录：四站流转（2026-10-27 立项）新增的中间态「加工中」层的需求记录与 spec/plan 归属。

**使用方式**：涉及 `/ship` 命令、`Study/ship/` 目录（平铺不编号，加工中暂存）、sea→ship 装船与 ship→river 入河流转的功能，其 design.md / spec / plan 放本目录；落地或废弃的 spec/plan 移入本目录 `archive/`。

**流转定位**：sea（原始笔记永久库，装船复制不迁移、sea 原件与其绑定永远保留）──`/ship copy` 装船（复制一份文本入 ship 开始加工，自动开编辑器；不改写出处链，指向 sea 原件的「> 出处：」依然有效）──▶ ship（加工中副本暂存）──`/ship move` 入河（宣告加工完成，真实迁移出 ship，领号挂载）──▶ river。装船是 copy、入河是 move：ship→river 是全流程唯一真实迁移；两个流转动作均由开发者手动判定手动触发，无自动机制。编号与重排（`/plan renumber`）规则见 river 侧。

**需求正本**：ship 层无独立 design.md——其需求定义于跨层设计 `全局/archive/2026-10-27-四站流转与river编号-design.md`（已落地归档）；正本级修订（`全局/三层学习框架-design.md` 增补 ship 层）待开发者审核。

**边界**：跨层框架决策见 `全局/三层学习框架-design.md`；该正本修改权在开发者，AI 只可提建议或经审核协助。
