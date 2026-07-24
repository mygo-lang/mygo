# 修复 ResultEq.Equals 编译问题

## 问题分析

`go build ./...` 在 `prelude/zz_prelude.gen.go:106` 报错：
```
cannot use __mygo_match___mygo_expr_2.F0 (variable of type A) as E value in argument to EqualsFn_1
```

根源：`ResultEq.Equals` 使用 `using Eq[A], Eq[E]` 两个约束，codegen2 在生成 `l.Equals(r)` 调用时，没有正确选择 `EqualsFn`（对应 `Eq[A]`），而是错误地用 `EqualsFn_1`（对应 `Eq[E]`）。

## 根因

### 问题 1：typeinference2 缺少内置 Eq 实例

旧 typeinference 在 [`internal/mygo/typeinference/solver.go:136`](internal/mygo/typeinference/solver.go:136) 的 `RegisterBuiltInInstances` 中为所有原生类型注册了 Eq 实例：
```go
instances := []*Instance{
    {ClassName: "Eq", Type: TCon{Name: "Int"}},
    {ClassName: "Eq", Type: TCon{Name: "Int8"}},
    // ...所有原生类型 + Ref
}
```

typeinference2 的 [`solver.mygo`](internal/mygo/typeinference2/solver.mygo) 完全缺少这个机制。

### 问题 2：codegen2 patternTypes 传播不工作

虽然已经添加了 `patternTypes` 字段到 `egCtx`，并在 `receiverStaticType` 中优先检查它，但调试显示 `receiverType=""`，说明 `patternTypes["l"]` 没有被正确设置。

## 修复方案

### 步骤 1：typeinference2 添加内置 Eq 实例

在 [`internal/mygo/typeinference2/solver.mygo`](internal/mygo/typeinference2/solver.mygo) 中添加 `registerBuiltInEqInstances` 函数，注册所有原生类型的 Eq 实例（与旧 typeinference 保持一致）。

### 步骤 2：调试 codegen2 patternTypes

添加临时 `panic` 在 `receiverStaticType` 中，当 `IdentExpr` 的 `patternTypes.Get()` 返回 `None` 但 `locals.Get()` 也有值时，打印所有上下文中 `patternTypes` 的键值对，以确认 map 共享是否正常。

### 步骤 3：根据调试结果修复

根据步骤 2 的结果，确定 `patternTypes` 未设置的原因，可能是：
- `seedEnumVariantFieldTypes` 没有正确调用
- 上下文引用链中断
- `bindPatternArgumentsWithTypes` 中的 `case None => ()` 分支被触发

## 实施顺序

1. 先添加 typeinference2 内置 Eq 实例
2. 重新生成 .gen.go 文件
3. 测试是否修复
4. 如果未修复，使用 panic 调试 patternTypes
5. 修复 patternTypes 传播问题
6. 重新测试
