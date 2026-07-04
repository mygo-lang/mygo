# Compiler HM 类型推导器计划

## Summary
在 `internal/mygo/type-inference` 中新增独立的 Hindley-Milner 推导 pass，采用 Haskell 98 级别能力：Algorithm W、let-polymorphism、occurs check、qualified types/typeclass 约束、实例解析与字典传递。顶层函数和 lambda 的参数/返回类型仍保持显式，推导覆盖函数体、`let`/`var` 初始化、调用、构造器、集合字面量、`None`/`Some`、`switch`/`if`/block 等表达式。

## Key Changes
- 新增 typed inference 子系统：
  - 定义内部类型表示：类型变量、构造类型、函数类型、Ref/Slice/Map/Set/Option/Result、Unit。
  - 定义 `Scheme`、`QualifiedType`、`Predicate`、`Subst`、`TypeEnv`、`InferState`。
  - 实现 instantiate/generalize/free type vars/unify/occurs check/apply substitution。
- 将推导改为 codegen 前置 pass：
  - 对每个函数、impl 方法、全局 let 先推导并保存 `TypedInfo`，包含表达式类型、绑定 scheme、函数调用类型实参、构造器类型实参、typeclass 字典选择。
  - `translateExpr` 后续优先消费 `TypedInfo`，减少当前基于 Go 类型字符串和 `expected` 的局部猜测。
- Typeclass 约束按 MIGRATE.md 收敛：
  - `using Show[A]` 进入 qualified type 环境。
  - 调用 `show(x)`、`equals(a,b)` 时生成 predicate，再解析为词法 using 字典或包级 impl helper。
  - 候选排序保持现有规则：显式/词法优先，再包级；具体类型覆盖优先，其次类型参数少，其次 `any` 少；并列报歧义。
- 表达式推导行为：
  - `let` 不带标注时自动泛化，后续每次使用 instantiate。
  - `var` 不泛化，按单态 mutable binding 处理。
  - `None` 可由期望类型、分支统一、赋值目标、构造器字段、返回类型推导为 `Option[A]`。
  - `Some(expr)`、`Ok(expr)`、`Err(expr)` 根据参数和上下文推导 enum 类型实参。
  - 空集合字面量必须有上下文；非空 Slice/Map/Set 从元素统一并与上下文校验。
  - `if`/`switch` 分支统一为同一类型；`while`/`return Unit` 保持无返回值 lowering。
- 兼容现有 Go lowering：
  - 保留 `goType`/Jennifer 生成接口，但让它从已解出的 MyGO 类型生成 Go 类型。
  - 移除或降级 `inferFuncTypeArgs`、字符串 `unifyType`、大量 `expected` 猜测逻辑。
  - 对推导发现的真实类型错误直接报错，不用 `any` 或反射兜底掩盖。

## Public/Internal Interfaces
- 新增内部入口，例如 `g.inferPackage() (*typedPackageInfo, error)`，由 `Generate` 在生成 Go 前调用。
- `exprCtx` 增加只读 typed info 引用，用于 lowering 查询表达式类型、调用 type args、字典参数。
- 不改变 MyGO 语法；顶层函数签名、lambda 参数/返回类型仍显式。
- 不实现 Rank-N、GADT、type family、associated types、kind polymorphism 等 Haskell 扩展。

## Test Plan
- 新增 compiler inference 单元测试：
  - identity/compose/const、let-polymorphism、多次 instantiate。
  - occurs check：`let f = func(x) x(x)` 类错误应失败。
  - `None`/`Some` 在返回、字段、分支、let 标注中的推导。
  - 泛型函数调用自动推导类型实参，包括返回类型反推。
  - Slice/Map/Set 非空与空字面量推导和错误诊断。
  - `using` 约束传播、局部字典优先、包级 impl fallback、歧义报错。
- 回归测试：
  - `go test ./internal/mygo/parser ./internal/mygo/compiler`。
  - 编译 `prelude`、`examples/main`、`examples/data-structure`。
  - 对当前 prelude 中真实类型不一致处期待明确错误，或先按现有语义修正示例后再作为通过用例。
- 生成代码校验：
  - 泛型函数、泛型 enum/struct/interface 保持 Go 泛型。
  - 不新增 runtime registry。
  - `reflect` 只保留真正动态 `any` fallback。

## Assumptions
- “至少和 Haskell 一样”按 Haskell 98 核心 HM + typeclass 约束理解，不包含高级 GHC 扩展。
- 顶层函数和 lambda 的参数/返回类型继续显式，不做语法扩展。
- 推导器应拒绝现有代码里的真实类型错误，而不是为了兼容旧生成结果自动吞掉错误。
