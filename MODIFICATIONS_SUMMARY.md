# 修改内容总结

本次改动主要围绕 prelude 迁移、类型类/接口生成，以及 parser 对 `using` 语法的支持完善。

## 1. Prelude 迁移到 `lib/prelude`

- 将内置 prelude 从 `prelude/` 迁移到 `lib/prelude/`。
- 代码生成阶段自动注入的 dot-import 目标同步改为 `github.com/mygo-lang/mygo/lib/prelude`。
- 相关生成逻辑也更新为跳过新的 prelude 路径，避免重复导入。
- 原来的 `prelude/prelude.mygo` 和 `prelude/zz_prelude.gen.go` 已移除。

## 2. 接口与类型类生成调整

- 接口在生成到 Go 时，从 `interface` 形式调整为带函数字段的 `struct` 形式。
- 单方法接口做了更直接的特化处理，unit 返回值会被正确映射为无返回值函数。
- 类型类方法调用现在会更稳定地解析约束函数名，支持方法名大小写的兼容查找。
- `using` 约束在函数与 impl 生成阶段都支持根据实现名或绑定名解析具体实现。

## 3. `using` 语法与 parser 更新

- parser 现在支持 `using FastEq: Eq[Int]` 这类“绑定实现名 + 接口名”的写法。
- 相关测试补充了 named using implementation 的解析场景。
- parser 生成代码和测试文件都随 grammar 变动做了同步更新。

## 4. 其他同步改动

- `lsp/` 下的生成文件已与最新语法和生成规则同步。
- `changelog.md` 中的 prelude 路径说明已更新。

## 影响

- 现有依赖 prelude 的包需要通过新的 `lib/prelude` 路径工作。
- 类型类与接口相关的生成结果发生了结构性变化，相关调用与测试需要一起保持一致。
- 新的 `using` 绑定写法可以明确指定实现名，便于在同名接口实现中做更精细的选择。
