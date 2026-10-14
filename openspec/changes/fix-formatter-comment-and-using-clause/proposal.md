## Why

`mygo fmt` 目前会破坏源码：块形式声明里的 `using` 约束子句会被渲染成 `usingToString`，重新解析后 `using` 关键字消失、约束丢失并产生伪语句；被换行的分隔结构里，尾随行注释会被反复追加逗号，永不收敛（单次 `fmt` 因内部 fixpoint 最多 6 轮而增加 6–7 个逗号）。两者都在 HEAD 上复现，仓库内 14 个 `.mygo` 文件含此形态的 `using` 签名。

## What Changes

- 分隔结构换行时，分隔符只追加到确有内容的 item 文本之后、注释之前：行尾注释保持原样，不吸收分隔符，也不再被误当作 item。
- item 区域内的注释独立成行并保持其缩进层级，使后续 item 不再丢缩进；换行分隔结构的渲染在单个 pass 内收敛。
- 修复声明头部渲染丢失的关键分隔空白（`using` 与其约束之间的空格），使 `using ToString[Int]` 保持原样，不再产出会被重新词法分析为单一标识符的 `usingToString[Int]`。
- 增加输出结构保持约束：单次渲染的结果重新解析后必须与输入具有等价结构，fixpoint 不得固化解构性输出。
- 把跨模块 AST 等价校验扩展到仓库范围，并把 `using` 子句与注释分隔纳入 fixtures；修复后移除现有的非幂等例外。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `formatter-cli`: 换行分隔结构中分隔符与注释的放置、声明 `using` 约束子句的渲染，以及「格式化输出必须保持解析结构」的约束。

## Impact

影响 `internal/mygo/formatter` 的分隔 item 渲染、声明头部/canonical 空白处理，以及 formatter 测试与仓库级等价校验。不修改 parser2 的无损 CST 形状（`Constraints`/`Constraint` 节点结构保持），不改 AST、降级或 CLI 接口；无新增依赖。

