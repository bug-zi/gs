package command

import (
	"fmt"
	"strings"
)

const helpText = `gs · Go 学习工作台

命令：
  /logs [关键词|日期]     查看日志：无参=今天(无则最近一天)；4位月日(1025)或6位年月日(261025)按日期查
  /logs edit [日期]       调起编辑器编辑日志（默认今天，不存在则创建；日期同上）
  /log add "<内容>"       向今日日志追加一条（超过40字会确认）
  /plan                   路线图浏览器（Tab 展开/收起，Enter 进入，d/s/t 标记，Esc 返回，q 退出）
  /plan <主干关键词>       打开浏览器并定位该主干；非交互环境输出静态树
  /plan status            学习状态速览
  /plan edit              编辑 roadmap.toml / 计划文件（DOCX 请用 Office 打开）
  /plan next "<断点>"     更新学习状态.md 的“下一步”
  /plan done|doing|todo <关键词>  快捷标记知识点（浏览器内 d/s/t 为主路径）
  /plan add branch|point  建树录入（point 全局唯一，可带分组）
  /plan renumber          按树序全量重排 river 编号（dry-run 预览，确认执行，引用同步改写）
  /plan del branch|point <关键词>  删除空主干 / 删除知识点（解绑保留文件，river 收容 99-未挂载）
  /sea [关键词]           大海条目查阅；/sea import 整理 inbox，/sea edit 编辑
  /ship [关键词]          船坞（加工中）查阅；/ship copy 装船（sea→ship 复制，装船即开编辑器），/ship move 入河（ship→river 真实迁移，领号挂载）
  /river [关键词]         河流条目查阅，/river edit 编辑
  /spring [关键词]        泉水查阅；/spring add <知识点> 创建该知识点唯一的总结并绑定
  /code [关键词]          代码查阅；/code add <知识点> 创建该知识点唯一的演示代码并绑定
  /notes [文本]           便签缓存：/notes <文本> 记一条，/notes 列出
  /notes edit|del         编辑便签文件 / 选择删除
  /notes move [关键词]    便签原文追加到 river/spring 条目并移出缓存
  /notes tidy             AI 整批归位到现有 river/spring（整批预览，一次确认）
  /sea|/ship|/river|/spring|/code del  删除条目（预览影响，确认后移入回收站，可手动找回）
  /search <关键词>        全局搜索 md 与 go 源码（支持空格分隔多词 AND）
  /project [名称]        查看项目聚合信息（介绍/学习日志/走读/设计/代码概况）
  /project edit <名称>   编辑项目文档
  /show [主题]           验收演示：概览/知识/代码/日志/面试题 五节视图
  /show edit             维护沉淀索引 topics.toml
  /review                AI 体检：给出知识整理/合并/学习规划建议（需配置 [ai]）
  /AI [问题]             AI 对话：带参单发提问，无参进入 AI 模式（/back 返回，@路径 引用文件）
  /help                   显示本说明
  /exit                   退出 gs

示例：
  /logs context        → 在全部日志中搜含 context 的记录
  /search context 超时  → 同时含两词的文件
  /plan next 登录模块源码走读`

func newHelp() Command {
	return Command{
		Name:  "help",
		Alias: []string{"h", "?"},
		Short: "显示使用说明",
		Run: func(ctx *Ctx) error {
			var b strings.Builder
			b.WriteString(helpText)
			b.WriteString("\n\n已启用命令：")
			for _, c := range ctx.Cmds {
				fmt.Fprintf(&b, "\n  /%s  %s", c.Name, c.Short)
			}
			fmt.Fprintln(ctx.Out, b.String())
			return nil
		},
	}
}

func newExit() Command {
	return Command{Name: "exit", Alias: []string{"q", "quit"}, Short: "退出 gs", Run: func(ctx *Ctx) error { return nil }}
}

// Build 全部 M1 命令；后续任务把新命令 append 进来
func Build() []Command {
	return []Command{newLogs(), newSea(), newShip(), newRiver(), newSpring(), newCode(), newNotes(), newPlan(), newSearch(), newShow(), newProject(), newReview(), newAI(), newHelp(), newExit()}
}
