# Codegen2 实现计划

## 概述

用 MyGO 语言实现 `internal/mygo/codegen2/`，功能与现有 `internal/mygo/codegen/`（Go 实现）对齐，并额外支持**尾递归优化**。codegen2 运行在 `ast2/parser2/typeinference2` 流水线上，使用 `ast2` 的精简 AST。

## 核心设计决策

### 1. 复用 ast2 AST

`codegen2` 直接使用 `internal/mygo/ast2/` 中的 AST 类型（`ast2.Expr`、`ast2.Decl`、`ast2.TypeExpr` 等），**不**使用现有的 `internal/mygo/ast/` 类型。这意味着 codegen2 是纯 MyGO 流水线的组成部分，不依赖旧的 Go AST。

### 2. 集成点

```mygo
-- import 路径
import ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"
import parser2 "github.com/mygo-lang/mygo/internal/mygo/parser2"
import typeinference2 "github.com/mygo-lang/mygo/internal/mygo/typeinference2"
```

### 3. 生成目标

codegen2 生成 Go 源代码（`go/ast` 包输出），与现有 codegen 语义一致但代码结构可以不同。

## 项目结构

```
internal/mygo/codegen2/
├── codegen2.mygo       # 主入口：Generate/GenerateFiles
├── types.mygo          # 类型定义（上下文、Generator 结构体）
├── translate_expr.mygo  # 表达式翻译
├── translate_stmt.mygo  # 语句翻译（block、let、return、assign）
├── translate_control.mygo # 控制流（if/switch/while）
├── translate_call.mygo  # 函数调用翻译
├── translate_literal.mygo # 字面量翻译（struct/slice/map/set/tuple）
├── gen_decl.mygo        # 声明生成（func/enum/struct/interface/impl）
├── translate_inline_go.mygo # 内联 Go 翻译
├── types_util.mygo      # 类型工具（goType、命名规则）
├── tailcall.mygo        # 尾递归优化
├── goast_helpers.mygo   # Go AST 辅助构造函数
└── zz_codegen2.gen.go   # 生成的 Go 代码
```

## 详细模块设计

### 1. types.mygo — 类型定义

```mygo
package codegen2

import ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"

-- 翻译上下文（对应 codegen.egCtx）
struct egCtx
  locals: Map[String, String]           -- 局部变量名 → Go 类型
  bindings: Map[String, String]          -- MyGO 名 → Go 名
  sourceTypes: Map[String, String]       -- 源类型信息
  mutable: Map[String, Bool]             -- 变量是否可变
  typeParams: Set[String]                -- 当前作用域的类型参数
  retType: String                        -- 当前函数的 Go 返回类型
  retTypes: Slice[String]               -- 多返回值类型
  -- 尾递归跟踪
  tailRecFuncName: Option[String]        -- 当前函数的尾递归候选名
  tailRecParamCount: Int                 -- 参数数量
end

-- Generator 结构体（对应 codegen.gen）
struct Generator2
  pkgName: String                        -- 包名
  importPaths: Slice[String]             -- import 路径列表
  currentFile: String                    -- 当前处理的文件名
  localSeq: Int                          -- 局部变量序号
  switchVarSeq: Int                      -- switch 变量序号
  needsCallAny: Bool                     -- 是否需要 callAny 辅助
end

-- 辅助：Go 类型映射
-- 使用 ast2.TypeExpr 直接映射到 Go 类型字符串
```

### 2. translate_expr.mygo — 表达式翻译

**入口**：`translateExpr(e: ast2.Expr, ctx: ref[egCtx], expected: String) -> GoExpr`

**ast2.Expr → Go 代码映射**：

| ast2.Expr | Go 代码 | 说明 |
|-----------|--------|------|
| IdentExpr(name) | `name` | 从 bindings 查找重命名 |
| NumberExpr(v) | `v` | 解析字面量后缀确定类型 |
| StringExpr(v) | `"v"` | Go 字符串字面量 |
| BoolExpr(v) | `true`/`false` | Go 布尔 |
| UnitExpr | `()` | Go 空结构体 |
| CallExpr(callee, args) | `fn(args...)` | 函数调用 |
| FieldExpr(base, field) | `base.field` | 字段访问/方法调用 |
| UnaryExpr(op, inner) | `op inner` | 一元操作 |
| BinaryExpr(op, l, r) | `l op r` | 二元操作（含管道） |
| IfExpr(cond, t, e) | `if cond { ... } else { ... }` IIFE 包装 | 条件表达式 |
| BlockExpr(items) | `{ stmts }` IIFE 包装 | 块表达式 |
| LetExpr(bind) | `var/:=` 声明 | let 绑定 |

**管道操作符**：
- `a |> f(b)` → `f(b, a)`（右结合管道）
- `f(x) <| a` → `f(x, a)`（左结合管道）

### 3. translate_stmt.mygo — 语句翻译

**入口**：`translateBlockStmts(n: ast2.BlockExpr, ctx: ref[egCtx], retType: String) -> Slice[GoStmt]`

处理 BlockExpr 中的多个表达式作为语句序列：
- 非最后一个表达式 → 语句形式
- 最后一个表达式 → return 语句（如果函数有返回值）

**Let 绑定**：
- `let x = value` → `x := value` 或 `var x T = value`
- 使用 `mutable` 标志决定 `:=` 还是 `=`

### 4. translate_control.mygo — 控制流

**If 表达式**：
- 语句位置 → `if cond { thenBlock } else { elseBlock }`
- 表达式位置 → IIFE 包装

**While 循环**：`for { cond; body }`

**Switch 模式匹配**（当前省略，parser2 AST 没有 switch 模式；可以后期扩展或只支持字面量模式）

### 5. translate_call.mygo — 函数调用

**尾递归检测**：
- 在 `translateExpr` 中，当处理 `CallExpr(IdentExpr(funcName), args)` 时：
  - 如果 `funcName == ctx.tailRecFuncName` → 标记为尾递归
  - 尾递归调用转换为 `param1, param2, ... = newArgs; continue`

**普通调用降级**：
- 普通函数调用 → Go 函数调用
- 无需处理 `using` 约束（typeinference2 已处理）

### 6. translate_literal.mygo — 字面量

ast2 没有专用的字面量类型（如 StructLitExpr），所有结构体字面量、集合字面量在 ast2 中被解析为 CallExpr。codegen2 需要识别这些模式：

- `StructName{field: value}` → 检测 CallExpr(Ident("FieldAssign"), ...) 模式
- `[1, 2, 3]` → 检测 CallExpr(Ident("SliceLit"), ...) 模式
- `{"key": value}` → 检测 CallExpr(Ident("MapLit"), ...) 模式

**注意**：由于 parser2/ast2 是精简 AST，codegen2 可能需要与 parser2 协商字面量的表示方式，或在 codegen2 中添加模式匹配来识别。

### 7. gen_decl.mygo — 声明生成

**函数声明** `FuncDecl(name, tps, params, ret, body)`：
```go
func name(params...) retType {
    body_stmts...
}
```

**结构体声明** `StructDecl(name, tps, fields)`：
```go
type name[tps] struct {
    fields...
}
```

**枚举声明** `EnumDecl(name, tps, variants)`：
```go
// 接口 + 变体结构体
type name interface { isName() }
type nameVariant struct { ... }
```

### 8. tailcall.mygo — 尾递归优化

**检测算法**：

```
isTailCall(body, funcName):
  如果 body 是 CallExpr(Ident(funcName), args):
    返回 true
  如果 body 是 IfExpr(cond, then, else):
    返回 isTailCall(then, funcName) || isTailCall(else, funcName)
  如果 body 是 BlockExpr([last]):
    返回 isTailCall(last, funcName)
  否则:
    返回 false
```

**转换**：

```mygo
-- 原始：
func f(x: Int) -> Int
  if x == 0 => x else f(x - 1)
end

-- 优化后：
func f(x: Int) -> Int
  for {
    if x == 0 then
      return x
    else
      x = x - 1
      continue
    end
  }
end
```

### 9. codegen2.mygo — 主入口

```mygo
-- 生成单个文件的 Go 源代码
func Generate(file: ast2.File, info: typeinference2.PackageInfo) -> Result[String, String]

-- 返回 文件名 → Go 代码 映射
func GenerateFiles(files: Slice[(String, ast2.File)], info: typeinference2.PackageInfo) -> Result[Map[String, String], String]
```

## 与现有 codegen 的主要差异

| 方面 | codegen (Go) | codegen2 (MyGO) |
|------|-------------|----------------|
| 语言 | Go | MyGO |
| AST | `internal/mygo/ast` | `internal/mygo/ast2` |
| 类型推断 | 完整 HM | 精简 HM |
| Go AST 构建 | `go/ast` 包 | 手动构建 Go 源代码字符串或简化 AST |
| Switch 模式 | 完整支持（变体模式 + 字面量模式） | 只支持简单的 if-else 链 |
| 尾递归优化 | ❌ 不支持 | ✅ 支持 |
| using 约束 | 复杂分发 | 简化为直接调用 |

## 实现建议

由于 MyGO 语言本身通过编译生成 Go 代码，codegen2 有两种实现策略：

### 策略 A：生成 Go 源代码字符串

codegen2 的每个翻译函数返回字符串（Go 源代码片段），最后拼接成完整 Go 文件。

**优点**：实现简单，不需要 Go AST 构造
**缺点**：字符串拼接容易出错，难以保证格式正确

### 策略 B：使用 go/ast 包（推荐）

codegen2 使用 MyGO 的 `go[T] {}` 内联 Go 特性调用 Go 的 `go/ast` 和 `go/token` 包构造 AST，最后用 `go/format` 输出。

**优点**：类型安全，与现有 codegen 架构一致
**缺点**：需要在 MyGO 中大量使用 `go[T]{}` 内联

**推荐选择策略 A**（首次实现）：codegen2 首先生成 Go 源代码字符串，利用 Go 的 `gofmt` 格式化。

## 测试策略

1. **单元测试**：对每个翻译函数编写测试
2. **集成测试**：parser2 → typeinference2 → codegen2 完整流水线
3. **对比测试**：对同一段 MyGO 代码，分别用 codegen 和 codegen2 生成 Go 代码，验证语义等价

## 实现步骤

1. 创建 `codegen2/` 目录和文件结构
2. 实现 `types.mygo`（上下文类型）
3. 实现 `types_util.mygo`（Go 类型映射）
4. 实现 `translate_expr.mygo`（基础表达式）
5. 实现 `translate_stmt.mygo`（语句）
6. 实现 `translate_control.mygo`（控制流）
7. 实现 `translate_call.mygo`（调用）
8. 实现 `translate_literal.mygo`（字面量）
9. 实现 `gen_decl.mygo`（声明生成）
10. 实现 `tailcall.mygo`（尾递归优化）
11. 实现 `codegen2.mygo`（主入口）
12. 测试验证