# 自举编译器推进计划

> 基于最新代码分析更新（2026-07-21）
> 覆盖: `parser2` + `typeinference2` + `codegen2` + `ast2`

## 主要发现：分析报告已过时

`plans/self-host-analysis.md` 中标记为 ❌ 的多个特性已在代码中**已实现**：

### ✅ 已实现但报告中标记为缺失

| 特性 | 报告中状态 | 实际状态 | 位置 |
|------|:---------:|:--------:|:----:|
| `var` 可变声明 | ❌ | ✅ 已解析 | parser2 L78, L382-394 |
| `while` 循环 | ❌ | ✅ 已解析 | parser2 L80, L396-404 |
| `return` 语句 | ❌ | ✅ 已解析 | parser2 L81-87, L406-415 |
| 内联 Go `go[T]{...}` | ❌ | ✅ 已解析 | parser2 L88-89, L417-443 |
| enum 泛型参数 | ❌ | ✅ 已解析 | parser2 L149 (typeParamList) |
| typeinference2 中 VarExpr | ❌ | ✅ 已推断 | typeinference2/infer L78 |
| typeinference2 中 WhileExpr | ❌ | ✅ 已推断 | typeinference2/infer L79, L86-102 |
| typeinference2 中 ReturnExpr | ❌ | ✅ 已推断 | typeinference2/infer L80-81 |
| typeinference2 中 InlineGoExpr | ❌ | ✅ 已推断 | typeinference2/infer L82 |

### ❌ 真正缺失的功能（codegen2 代码生成缺口）

`translate_expr.mygo` 中**缺少**对以下 parser2 已支持的 AST 节点的翻译：

| 特性 | parser2 | typeinference2 | codegen2 | 影响 |
|------|:-------:|:--------------:|:--------:|:----:|
| `var` 声明代码生成 | ✅ | ✅ | ❌ | 变量声明转 Go `var` |
| `while` 循环代码生成 | ✅ | ✅ | ❌ | 循环转 Go `for` |
| `return` 代码生成 | ✅ | ✅ | ⚠️ 部分 | 只有 `BlockExpr` 尾部和 `if` 可用 |
| 内联 Go 代码生成 | ✅ | ✅ | ❌ | 无法生成 `go[T]{...}` |
| `VarExpr` 在 translate_expr 中 | ✅ | ✅ | ❌ | 直接导致 `var` 语句崩溃 |

### 其他注意到的差距

- `FieldExpr`/`CallExpr` 等表达式虽然已经支持，但 `TypeInference2` 中的 `initialEnv` 只有 `true`/`false`
- 自举测试覆盖不足 — 仅有 5 个 e2e 测试，没有 `parser2` 解析自身源码的测试
- 没有位置/跨度追踪 — 所有节点都是无位置的

## 执行计划

### 第一阶段：填补 codegen2 代码生成缺口

#### 1.1 VarExpr 生成器支持（`translate_expr.mygo`）
- **文件**: `internal/mygo/codegen2/translate_expr.mygo`
- **改动**: 在 `translateStmt` 中添加 `VarExpr` 分支
- **逻辑**: 与 `LetExpr` 类似，但生成 `var name T = value` 而不是 `name := value`
- **测试**: 添加 `var` 声明的生成测试，验证 Go 输出

#### 1.2 WhileExpr 生成器支持（`translate_expr.mygo`）
- **文件**: `internal/mygo/codegen2/translate_expr.mygo`
- **改动**: 在 `translateStmt` 和 `translateReturnExpr` 中添加 `WhileExpr` 分支
- **逻辑**: 生成 `for { cond; if !cond { break }; body; continue }` 或简化为 `for cond { body }`
- **测试**: 添加 `while` 循环的生成测试

#### 1.3 InlineGoExpr 生成器支持（`translate_expr.mygo`）
- **文件**: `internal/mygo/codegen2/translate_expr.mygo`
- **改动**: 在 `translateExpr` 中添加 `InlineGoExpr` 分支
- **逻辑**: 直接输出内联 Go 代码字符串
- **测试**: 添加 `go[T]{...}` 的生成测试

#### 1.4 全面处理 ReturnExpr/ReturnWithExpr
- **文件**: `internal/mygo/codegen2/translate_expr.mygo`
- **改动**: 在 `translateStmt` 中添加对 return 表达式的处理
- **测试**: 确保 `return expr` 和 `return` 都能生成正确代码

### 第二阶段：完善类型推断环境

#### 2.1 为 typeinference2 添加内置类型环境
- **文件**: `internal/mygo/typeinference2/env.mygo`
- **改动**: 在 `initialEnv()` 中添加 `String`、`Int`、`Bool`、`Float` 等基础类型
- **理由**: 即使不做完全泛型，至少需要让字面量类型能被识别

#### 2.2 为 codegen2 提供更准确的类型信息
- **文件**: `internal/mygo/codegen2/codegen2.mygo`
- **改动**: 将 `typeinference2.PackageInfo` 传递给代码生成阶段
- **理由**: 目前 `GenerateFiles` 接受 `info` 但不使用

### 第三阶段：端到端自举测试

#### 3.1 parser2 解析自身源码测试
- **文件**: `internal/mygo/parser2/parser_test.go`
- **测试**: 用 parser2 解析 `parser.mygo` 自身，验证能成功输出 AST
- **不变量**: 输出的 AST 可以遍历不 panic

#### 3.2 自举管道端到端测试
- **文件**: `internal/mygo/codegen2/codegen2_test.go`
- **测试**: `parser2 → ast2 → typeinference2 → codegen2` 完整管道测试
- **输入**: 一段自举组件自身的有效 MyGO 源码
- **验证**: 生成的 Go 代码可通过 `go/parser` 解析

#### 3.3 codegen2 生成自身代码（自举测试）
- **目标**: 验证 `codegen2` 能处理 `typeinference2` 和 `ast2` 的源码
- **方法**: 收集 `typeinference2` 和 `ast2` 的所有 `.mygo` 文件，通过 `GenerateSource` 运行
- **预期**: 生成的 Go 代码可被 Go 编译

### 第四阶段（可选）：其他增强

#### 4.1 `func_lit`（匿名函数）解析支持
- **状态**: parser2 不支持 `func_lit`，这是一个已知限制
- **影响**: 阻碍使用高阶函数写法

#### 4.2 为 parser2 添加位置/跨度信息（部分）
- **状态**: `ast2` 的所有节点都没有 `Pos` 字段
- **建议**: 至少在 parser2 级别添加行号追踪，用于错误报告

### 实现顺序建议

```
第一期（最高优先级）
  ├── 1.1 VarExpr 生成器
  ├── 1.2 WhileExpr 生成器
  ├── 1.3 InlineGoExpr 生成器
  └── 1.4 ReturnExpr 全面处理

第二期（测试覆盖）
  ├── 2.1 typeinference2 内置类型环境
  └── 3.1-3.3 端到端自举测试

第三期（增强）
  └── 4.1-4.2 func_lit 和位置信息