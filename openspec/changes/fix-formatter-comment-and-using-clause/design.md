## Context

动机见 proposal.md。以下事实由一次临时 CST 探针确认，是设计与取舍的依据：

- `func f(x: Int) -> String using ToString[Int]` 的 CST 是 `[func, name, FunctionParameters, FunctionReturnType, Constraints, Block, end]`，`Constraints` 位于 `Block` 之前，头部边界 `memberHeaderEnd` 也把它包含进来（`headerEnd` 指向 `Block` 之前的 trivia）。也就是说「子句跑到 body 里」不是头部边界算错。
- 单次 `FormatSyntaxBlocks` 对同一输入产出的头部是 `func f(x: Int) -> String usingToString[Int]`：结构正确但丢了 `using` 与约束之间的空格。
- `FormatSource` 的 fixpoint 第二轮把该文本重新词法分析成单一标识符 `usingToString`，于是 `using` 关键字消失，`Constraints` 节点不再生成，子句落进 `Block`，并且此后每轮渲染都稳定在这个已被破坏的形状。
- 空格丢失的机制：parser2 的 `functionUsingChildren` 把分隔 trivia 留在 `Constraint` 子节点内部；子节点渲染时该 trivia 成为首部 trivia，而 `syntaxCanonicalWhitespace` 在「同一元素列表内没有前置 token」时返回 `""`，空格被丢弃。
- 分隔结构的分隔符一律追加在 item 文本之后（`renderSyntaxDelimitedItems`）。item 区域末尾是行注释时，逗号落在注释后并被注释吞掉，下一轮再加一个；`formatSyntaxFixpoint` 最多 6 轮，所以单次 `mygo fmt` 增加 6–7 个逗号。注释位于两个 item 之间时不会累积，但后一个 item 会丢掉缩进。

## Goals / Non-Goals

**Goals：** 在 formatter 内部修掉这两个缺陷；让单次渲染的输出保持解析结构；把该性质纳入仓库级校验。

**Non-Goals：** 不改 parser2 的无损 CST 形状或词法（`Constraints`/`Constraint` 的 trivia 归属保持现状）；不动 pipe/长表达式的排版规则；不改降级、AST、CLI 接口；不重写 legacy 渲染器。

## Decisions

### 1. 空格缺陷在 formatter 修（推荐）

在渲染节点时把「父级前一个有效 token」传入 canonical 空白判定，使首部 trivia 的分隔按父级上下文计算：`using` 后接约束节点时得到空格。

替代方案一：把分隔 trivia 从 `Constraint` 节点移回父级列表。否决——无损 CST 现在就是对的（`TestFunctionDeclarationOwnsUsingConstraints` 断言 kinds 与字节），移动 trivia 会改变 CST 形状并波及降级与既有 CST 测试，收益不抵风险。

替代方案二：对渲染结果做文本后处理插入空格。否决——脆弱且会引入第二套词法认知，容易再次破坏结构。

### 2. 分隔符与注释的放置规则

item 区域按「首部注释行 + 有效内容 + 尾部注释」解读：注释独立成行并按 item 缩进渲染，分隔符只追加在有效内容之后；区域只有 trivia 时不追加分隔符，原样输出注释。这样尾部注释不再吞掉分隔符（不累积），注释后的 item 也不再丢缩进。

### 3. 结构保持作为一等约束

单次 pass 的输出必须重新解析为等价结构（忽略位置与注释）。因为本例中破坏发生在 *单次* 渲染（`usingToString`），只在 fixpoint 之后校验是不够的。实现上把「格式化前后 AST 等价」扩展到仓库范围（忽略位置字段的反射比较），并新增 `using` 子句与注释分隔的 fixtures；发现其它既有缺陷时显式记录，不再静默排除。

### 4. 验证与回归

沿用既有 formatter 语料与 golden；新增的注释分隔与 `using` 用例断言逐字节输出与二次格式化不变。修复后删除当前仅为 `internal/mygo/typeinference2/env.mygo` 保留的非幂等例外。

## Risks / Trade-offs

- [父级上下文传入可能改变其它位置的空白] → 先跑既有 formatter 语料与 golden，再跑仓库级等价校验；出现差异逐一确认而不是批量接受。
- [注释独立成行会改变既有注释 golden] → 有意更新 golden，并同时断言二次格式化字节相等。
- [仓库级等价校验可能暴露其它既有缺陷] → 在 tasks 中显式记录清单，作为后续变更而不是本变更悄悄扩大范围。
- [`using` 子句输出变化] → 现状是破坏性输出，修复只会让这些文件可被安全格式化（仓库内 14 个文件受影响）。

## Migration Plan

先修 formatter 渲染（空格与分隔符规则），按 bootstrap 顺序同步 `internal/mygo/formatter` 生成 Go；运行 `go test ./internal/mygo/parser2 ./internal/mygo/formatter`，再跑全量 `go test ./...`；最后跑仓库级幂等与等价扫描并补文档。回滚时一并还原手写源码与生成产物。

