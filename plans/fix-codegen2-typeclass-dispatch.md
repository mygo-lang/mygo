# 修复 codegen2 的 typeclass 实现分发

## 背景

`codegen2`（AST 直生路径）中的 typeclass 方法调用分发通过 `translateImplMethodCall`（`internal/mygo/codegen2/translate_ast.mygo:800`）实现。分发有两个路径：

1. **Lexical dictionary**（来自 `using` 约束）：`dictFuncForMethod` — 通过 `constraintFuncs`、`constraintFuncNames`、`constraintFuncArgTypes` 解析
2. **Package-level impl helper**：`matchingReceiverCandidate` — 通过 `packageCandidates` 解析

## Bug 分析

### 根因

问题出在 `dictFuncForMethod` 函数（`translate_ast.mygo:843-858`）：

```mygo
func dictFuncForMethod(method: String, receiverType: String, ctx: Ref[egCtx]) -> Option[String]
  let names = ctx.constraintFuncNames.Get(method)
  let types = ctx.constraintFuncArgTypes.Get(method)
  let exactMatch = switch types
    case Some(typeList) =>
      switch names
        case Some(nameList) => if receiverType == "" then None else matchDictFuncByType(nameList, typeList, receiverType, 0) end
        case None => None
      end
    case None => None
  end
  switch exactMatch
    case Some(f) => Some(f)
    case None => ctx.constraintFuncs.Get(method)  # ← BUG: 只有一个条目
  end
end
```

当 `receiverType == ""`（无法解析接收者静态类型）时，`dictFuncForMethod` 回退到 `ctx.constraintFuncs.Get(method)`。但 `constraintFuncs` 是一个 `Map[String, String]`，对于同一个方法名只存储**一个**条目（最后一个注册的）。

而在 `addDictionaryMethods`（`decls.mygo:473-499`）中：
```mygo
ctx.constraintFuncs.Set(method.Name, paramName)       # ← 覆盖！只保留最后一个
ctx.constraintFuncNames.Set(method.Name, existingNames.Append(paramName))  # ← 保留完整列表
```

所以 `constraintFuncs` 只保留最后一个值（例如 `EqualsFn_1`），而 `constraintFuncNames` 保留了完整列表（例如 `[EqualsFn, EqualsFn_1]`）。

**触发条件**：当 `receiverStaticType` 无法解析接收者类型时（返回 `""`），分发会退化为使用 `constraintFuncs` 中最后一个值，这可能不是正确的函数。

但更关键的问题是：**`dictFuncForMethod` 中当 `receiverType == ""` 时不应该使用 `constraintFuncs`，而应该选择 `constraintFuncNames` 中的第一个函数**（即在 `using` 约束列表中声明的第一个约束对应的函数），因为如果代码生成器不知道接收者的具体类型，使用第一个约束的函数是最合理的默认值。

## 修复方案

### 修改 1：修复 `dictFuncForMethod` 中的回退逻辑

在 `internal/mygo/codegen2/translate_ast.mygo` 中，将 `dictFuncForMethod` 函数修改为：

```mygo
func dictFuncForMethod(method: String, receiverType: String, ctx: Ref[egCtx]) -> Option[String]
  let names = ctx.constraintFuncNames.Get(method)
  let types = ctx.constraintFuncArgTypes.Get(method)
  let exactMatch = switch types
    case Some(typeList) =>
      switch names
        # Try exact type match first
        case Some(nameList) => if receiverType == "" then None else matchDictFuncByType(nameList, typeList, receiverType, 0) end
        case None => None
      end
    case None => None
  end
  switch exactMatch
    case Some(f) => Some(f)
    case None =>
      # Fall back to the first registered function when the receiver type
      # cannot be resolved.  The first entry corresponds to the first
      # constraint in the `using` clause, which is the most general match.
      switch names
        case Some(list) => list.Get(0)
        case None => None
      end
  end
end
```

### 修改 2：确保 `receiverStaticType` 尽可能返回非空类型

在 `receiverStaticType`中，添加对参数类型（`paramTypes`）的兜底检查。当 `locals` 和 `sourceTypes` 都没有时，检查参数的**模式绑定类型**（`patternTypes`）和**函数参数类型**是否有更精确的信息。

实际上，`receiverStaticType` 已经在做这个工作了（先查 `patternTypes`，再查 `sourceTypes`，再查 `locals`）。问题不在于它返回空，而在于**调用方（`dictFuncForMethod`）在接收到空类型时的行为不对**。

### 无需修改的部分

- `addDictionaryMethods` 中的 `constraintFuncs.Set` — 保留，因为它是旧代码的兼容接口
- `matchingReceiverCandidate` — 与当前 bug 无关

## 测试计划

1. 现有测试全部通过
2. 写一个包含多个 `using` 约束的测试用例，确保在接收者类型可解析和不可解析的情况下都能正确分派

## 影响范围

- 影响：只有 `codegen2` 路径（AST 直生），不影响传统的 Go `ast` 代码生成路径
- 风险：低。修改范围小，只改变了回退逻辑，且回退逻辑的语义现在是确定性的（总是选择第一个约束）