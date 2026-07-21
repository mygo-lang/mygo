# 自举实现分析报告

> 分析日期：2026-07-21
> 分析对象：`parser2` + `typeinference2` + `codegen2` + `ast2`

## 1. 整体架构

自举编译器由三个组件组成，形成一个流水线：

```
MyGO 源码
   │
   ▼
┌──────────┐     ┌─────────────────┐     ┌──────────┐
│ parser2  │────▶│ typeinference2   │────▶│ codegen2 │
│ (parsec) │     │ (HM 类型推断)     │     │ (字符串  │
│          │     │                  │     │  代码生成)│
└──────────┘     └─────────────────┘     └──────────┘
      │                  │                       │
      ▼                  ▼                       ▼
  ast2.File        typeinference2.         Go 源码字符串
                    PackageInfo
```

所有组件均用 MyGO 语言编写，通过自举编译器编译为 Go 代码（`zz_*.gen.go`）。

## 2. 组件明细

### 2.1 `ast2` — 自举 AST（`internal/mygo/ast2/ast2.mygo`）

- **声明类型**: `ImportDecl`, `FuncDecl`, `StructDecl`, `EnumDecl`, `InterfaceDecl`, `ImplDecl`
- **表达式类型**: `IdentExpr`, `NumberExpr`, `StringExpr`, `BoolExpr`, `UnitExpr`, `CallExpr`, `FieldExpr`, `UnaryExpr`, `BinaryExpr`, `IfExpr`, `BlockExpr`, `LetExpr`
- **类型表达式**: `NamedType`, `FuncType`, `TupleType`, `UnitType`
- **位置信息**: ❌ 没有任何位置信息（行/列），注释里写着"positions out until parser2 starts carrying source spans"

### 2.2 `parser2` — 自举解析器（`internal/mygo/parser2/parser.mygo`）

- **风格**: 基于 `lib/text/parsec` 的 parser combinator 风格
- **词法分析**: 内联在 parser 中，无单独的 lex/yacc 步骤
- **运算符优先级**: 用 `chainLeft` 处理（`|>`, `||`, `&&`, 比较, `+-`, `*/`）
- **支持的功能**:
  - ✅ `package` 声明
  - ✅ `import` 语句（含别名）
  - ✅ `struct` / `enum` 定义
  - ✅ `interface` / `impl` 定义
  - ✅ 函数声明
  - ✅ `if-then-elsif-else-end` 块
  - ✅ `if => else` 单行表达式
  - ✅ `let` 绑定
  - ✅ `!` 和 `-` 一元运算符
  - ✅ 字符串字面量（含转义 `\n`, `\t`, `\"`, `\\`）
  - ✅ 数字字面量
  - ✅ 布尔字面量 `true`/`false`
  - ✅ `#` 行注释
- **不支持的功能**:
  - ❌ `var` 可变变量
  - ❌ `while` 循环
  - ❌ `return` 语句
  - ❌ `switch/case` 模式匹配
  - ❌ `go[T]{...}` 内联 Go 嵌入
  - ❌ `using` 类型类约束
  - ❌ `type` 别名
  - ❌ `enum` 变体类型参数
  - ❌ 十六进制/八进制/二进制字面量
  - ❌ 三重引号字符串 `"""`
  - ❌ 位置/跨度追踪
  - ❌ `func_lit` 字面函数

### 2.3 `typeinference2` — 自举类型推断（`internal/mygo/typeinference2/`）

- **风格**: 简化的 Hindley-Milner 风格
- **支持的类型**: `MonoType` 枚举（`TVar`, `TCon`, `TFunc`, `TTuple`, `TUnit`）
- **入口**: `InferFile(file: ast2.File) -> Result[PackageInfo, String]`
- **环境**: 仅有 `true`/`false` 内建，无任何内建类型（`Int`, `String`, `Bool` 等）
- **支持的推断**:
  - ✅ 标识符查找
  - ✅ 字面量（数字 → `Int`, 字符串 → `String`, 布尔 → `Bool`, 单元 → `TUnit`）
  - ✅ 一元运算符（`!` → `Bool`, `-` → `Int`）
  - ✅ 二元运算符（算术 → `Int`, 比较 → `Bool`）
  - ✅ 函数调用
  - ✅ 条件表达式（`if`）
  - ✅ 块表达式
  - ✅ `let` 绑定
  - ✅ 结构体字段访问
- **不支持的**:
  - ❌ 泛型/多态（`Scheme.Bound` 总是空的）
  - ❌ 类型类（`using` 约束）
  - ❌ `interface`/`impl` 签名推断（`InterfaceDecl`/`ImplDecl` 直接透传）
  - ❌ 用户自定义运算符

### 2.4 `codegen2` — 自举代码生成器（`internal/mygo/codegen2/`）

- **输出**: Go 源码字符串
- **关键文件**:
  - `codegen2.mygo` — 入口：`Generate`, `GenerateFiles`, `GenerateSource`
  - `gofile.mygo` — Go 文件渲染（通过 `go/ast` + `go/format` 生成格式化代码）、parser2→ast2 类型转换
  - `decls.mygo` — 声明翻译（struct/enum/interface/impl/func）
  - `translate_expr.mygo` — 表达式翻译（包含 if 表达式临时变量展开、返回语句翻译）
  - `types.mygo` — `egCtx` 上下文和 `Generator2` 结构体
  - `types_util.mygo` — Go 类型映射工具、名字修饰、符号处理
  - `tailcall.mygo` — 尾递归优化
- **支持的功能**:
  - ✅ `struct` → Go struct
  - ✅ `enum` → 接口 + 结构体类型（Go 的 sum type 模式）
  - ✅ `interface` → Go interface
  - ✅ `impl` → 方法实现
  - ✅ 函数定义 → Go 函数
  - ✅ 运算符翻译（`+`, `-`, `*`, `/`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `&&`, `||`, `|>`, `<|`）
  - ✅ `if` 表达式的临时变量展开
  - ✅ `let` 变量声明
  - ✅ 尾递归优化
  - ✅ 类型参数（泛型）
  - ✅ 内联 Go 嵌入（`go[T]{...}` 语法）
  - ✅ 特殊集合类型（`Ref` → `*`, `Slice` → `[]`, `Map` → `map[K]V`, `Set` → `map[K]struct{}`）
  - ✅ 基本类型到 Go 的映射
  - ✅ 名字修饰（名字修饰/消除冲突）
  - ✅ 导入路径处理（`go:` 前缀）
- **潜在的 bug**:
  - `ctxSetTailRecParamNames` 中 `go[()]{code: "ctx.tailRecParamNames = names"}` 这行有类型不匹配（应该用 `=` 而不是 `:=`/`=`），但因为在 `go[T]` 嵌入中写的是字符串，实际会被原样插入 Go 代码
  
## 3. 自举与非自举组件的功能差距

### 3.1 Parser vs Parser2

| 特性 | parser (yacc) | parser2 (parsec) |
|------|:---:|:---:|
| 通用方法 | ✅ | ❌ |
| `var` 可变声明 | ✅ | ❌ |
| `while` 循环 | ✅ | ❌ |
| `return` 语句 | ✅ | ❌ |
| `switch/case` 匹配 | ✅ | ❌ |
| 内联 Go `go[T]{...}` | ✅ | ❌ |
| `using` 类型类 | ✅ | ❌ |
| `type` 别名 | ✅ | ❌ |
| `func` 字面量 | ✅ | ❌ |
| 位置/跨度追踪 | ✅ | ❌ |
| 十六进制/八进制/二进制字面量 | ✅ | ❌ |
| 三重引号字符串 `"""` | ✅ | ❌ |
| 运算符优先级（`|>`, `||`, `&&`, 比较, `+-`, `*/`） | ✅ | ✅ |
| `if-then-elsif-else-end` | ✅ | ✅ |
| `if => else` | ✅ | ✅ |
| 泛型 struct/enum/interface/impl | ✅ | ✅ |
| `let` 绑定 | ✅ | ✅ |
| 字符串/数字/布尔字面量 | ✅ | ✅ |

### 3.2 Type Inference vs Type Inference2

| 特性 | typeinference | typeinference2 |
|------|:---:|:---:|
| Hindley-Milner 算法 | ✅ | ✅ (简化版) |
| 泛型/多态 | ✅ | ❌ (`Bound` 始终为空) |
| 类型类/`using` | ✅ | ❌ |
| `interface`/`impl` 推断 | ✅ | ❌ (直接透传) |
| 代码量 | ~2800 行 | ~312 行 |

### 3.3 Codegen vs Codegen2

| 特性 | codegen | codegen2 |
|------|:---:|:---:|
| 输出格式 | Jennifer AST | 纯字符串 |
| `switch/case` | ✅ | ❌ |
| `while` 循环 | ✅ | ❌ |
| `return` | ✅ | ❌ |
| 内联 Go | ✅ | ❌ |
| 泛型 | ✅ | ✅ |
| 尾递归优化 | ❌ | ✅ |
| 单元测试 | ~20 个 | 5 个 |

## 4. 当前自举实现的关键问题

### 4.1 自举解析器无法解析自身

最大的问题：`parser2` **无法解析 `parser2` 自身的代码**。因为 `parser2.mygo` 中使用了以下 `parser2` 本身不支持的语法：

- 没有 `var`/`let` 区分（但 parser2 自身代码可能用到）
- 内联 Go 嵌入 `go[T]{...}`（`parser2.mygo` 中没有直接使用，但 `codegen2` 中使用了）

例如在 `codegen2/types_util.mygo` 中大量使用了 `go[String]{code: "..."}` 语法，如果自举编译 `codegen2` 自身，parser2 无法解析这些表达式。

### 4.2 类型推断过于简化

`typeinference2` 的 `initialEnv()` 只包含了 `true`/`false`：

```mygo
func initialEnv() -> Slice[EnvEntry]
  [
    EnvEntry { Name: "true", Scheme: Scheme { Bound: [], Body: MonoType.TCon("Bool", emptyMonoTypes()) } },
    EnvEntry { Name: "false", Scheme: Scheme { Bound: [], Body: MonoType.TCon("Bool", emptyMonoTypes()) } },
  ]
end
```

没有内置类型、没有运算符重载支持、没有泛型多态。这意味着它在当前 form 下基本无法用于生产级的 MyGO 代码的完整推断。

### 4.3 AST 桥接问题

`codegen2/gofile.mygo` 中的 `convertFile`/`convertDecl`/`convertExpr` 等函数负责将 `parser2` 的 AST 转换为 `ast2` 的 AST。但 `parser2.File` 和 `ast2.File` 的定义**完全一致**——它们是在不同 package 中定义并复制的：

- `parser2.File` = `{ PackageName: String, Decls: Slice[Decl] }`
- `ast2.File` = `{ PackageName: String, Decls: Slice[Decl] }

这种冗余转换增加了复杂性但几乎没有价值。理想情况下 parser2 应该直接产出 `ast2` 类型，但 MyGO 编译器在编译时对包级别的循环依赖检查可能会导致问题。

### 4.4 缺少元数据

- **无位置信息**: 错误报告难以定位源代码位置
- **无测试覆盖率**: 缺乏自举组件的端到端编译测试
- **无基准测试**: 无法评估性能

## 5. 自举路径状态总结

```
自举状态: 🟡 部分实现
```

- **Parser2**: 可解析 MyGO 的一个子集（约 60% 语法特性）
- **TypeInference2**: HM 类型推断的最小实现（约 30% 功能）
- **Codegen2**: 代码生成的最小实现（约 40% 功能），内联 Go 嵌入 `go[T]{...}` 用于底层能力
- **集成测试**: 5 个测试用例验证基本流程

## 6. 建议

### 短期（1-2 步）
1. **为 parser2 添加位置追踪** — 不要求精确的位置，即使只是简单的行号也能大幅改进调试
2. **补全 `var` 声明和 `while` 循环解析** — 这两个是解析器自举的最小功能依赖
3. **添加 `return` 语句支持**

### 中期（3-5 步）
4. **实现在 parser2 中的内联 Go 嵌入解析 `go[T]{...}`** — 这对自举编译器编译自身生成的代码（如 `codegen2` 的代码）至关重要
5. **增强 typeinference2 支持泛型**（至少基本的 `Scheme.Bound` 使用）
6. **端到端自举测试** — 验证 `parser2` 能否正确解析自身源码

### 长期
7. **代码生成器整合** — 消除非自举代码生成器和自举代码生成器的重复
8. **移除 ast2 桥接层** — 让 parser2 直接产出 `ast2` 类型
9. **完整的类型推断** — 支持类型类、泛型、用户自定义类型