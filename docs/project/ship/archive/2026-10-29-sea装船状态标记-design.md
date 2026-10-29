# sea 选择器「已装船」置灰标记 — 设计

- 日期：2026-10-29
- 状态：已落地（2026-10-29，口径经开发者确认）
- 模块：ship（sea→ship 流转的展示侧）

## 目标

`/sea` 浏览与 `/ship copy` 装船共用同一 sea 条目选择器。已装船过的 sea 文件在列表中用注释式阴影灰（`ui.Dim`）标示，未装船的正常色；排序上未装船的排前面，已装船的排后面，让"待装船原材料"一眼可辨。

## 判定规则（动态计算，不落任何持久状态）

sea 文件基名为 `X`（去 `.md` 后缀）时，满足任一即视为已装船：

1. `Study/ship/X.md` 存在（加工中）；
2. `Study/river/` 下存在文件名经 `roadmap.StripNumPrefix` 处理后等于 `X` 的条目（已入河；入河改名 `NN-标题.md` / `NN.dup-标题.md`，去编号后与装船时基名一致）。

- 每次打开选择器时用 `ListTierFiles` 现算 ship/river 两级文件清单，构建 `map[基名]bool`。
- ship/river 目录列举出错时静默不标灰：宁漏标、不阻塞浏览。
- 不依赖路线图绑定，未挂载散件同样生效。

## 展示实现

- `TierItem` 增加 `Shipped bool` 字段；`buildDirGroups`（仅 sea 使用）构建条目时打标。
- `menuOption` 增加 `Dim bool` 字段；`drawSelectFrame` 非光标行套 `ui.Dim`（与键位提示同色系的阴影灰）；光标行保持反白高亮不变。
- 非 TTY 降级的 `plainSelect` 编号列表同样套 `ui.Dim`。
- 生效范围：`/sea` 浏览选择器 + `/ship copy` 装船选择器（同一 `pickTierEntryGrouped`，一处改动两处生效）。

## 排序

组内条目按「未装船优先，组内各自按名升序」排列：先未装船（按名升序），后已装船（按名升序）。板块（组）间排序不变，仍按组名排序；「未挂载/未归类」散件组同样适用此排序。

## 已知边界

- sea 不同子目录的同名文件会一起标灰——与 `/ship copy` 按基名复制的既有行为一致，不额外处理。
- 入河后被 `/plan renumber` 重命名的 river 条目，去编号前缀后基名不变，仍能匹配。

## 测试

- 判定函数三分支单测：ship 命中 / river 编号名命中 / 都不命中。
- `buildDirGroups` 打标与组内排序（未装船在前）单测。
- `menuOption` Dim 渲染单测（非光标行含阴影色、光标行不受影响）。
