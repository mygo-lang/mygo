# bootstrap 与语言规范的差异

[`docs/spec.md`](../spec.md) 是语言行为的权威契约。本文档记录默认编译链与
自举编译链 `parser2 -> ast2 -> typeinference2 -> codegen2` 之间有意保留的差异。

## 当前状态

在共享一致性 fixture 与 bootstrap 工作流测试所覆盖的对齐范围内，不再存在
已确认的源码语言或编译工作流差异。

## 有意保留的差异

- 两条编译链是刻意独立实现；生成的 Go 标识符、字典参数、蹦床位置等降低细节
  可能不同。这些是实现细节，不构成语言差异。
- bootstrap 不保证生成与默认链字节完全相同的 Go。
- 若观察到行为差异，以 `docs/spec.md` 所述默认编译链为准；不应把 bootstrap
  行为解释为新的语言承诺。

bootstrap 的验证命令见 [`core.md`](core.md)。
