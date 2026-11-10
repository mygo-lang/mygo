# compiler2 实现说明

[`docs/spec.md`](../spec.md) 是语言行为的权威契约。本文档记录自托管
`parser2 -> ast2 -> typeinference2 -> codegen2` 管线的实现说明。

## 当前状态

共享一致性 fixture 与 bootstrap 工作流测试覆盖了管线的行为范围。

## 实现说明

- 生成的 Go 标识符、字典参数、蹦床位置等降低细节属于实现细节，不构成语言差异。
- 生成 Go 不保证在不同编译器版本或后端之间字节完全一致。
- 若观察到行为差异，以 `docs/spec.md` 的语言契约为准。

编译器验证命令见 [`core.md`](core.md)。
