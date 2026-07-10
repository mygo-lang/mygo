# MyGO Typeclass 新语法方案

## Summary
把 typeclass 语义收敛成“具名/匿名实例 + `using` 显式约束”的单一路线，保留 MyGO 命名风格，目标是让解析、编译、示例代码三者一致，不再混用旧的 runtime registry 和局部降级逻辑。

## Key Changes
- 语法层保留三件事：
  - `interface` 定义 typeclass
  - `impl` 定义实例
  - `using` 定义上下文需求
- 增加两类实例形态：
  - 具名实例：`impl intShow: Show[Int]`
  - 匿名实例：`impl Show[String]`
- `using` 统一写成显式依赖列表，支持带实例名的绑定：
  - `func demo(x: Int) -> String using intShow: Show[Int]`
  - `func demo[A](x: A) -> String using showA: Show[A]`
- 解析规则上移除 `where` 作为主入口，只保留迁移兼容或直接报错。
- 编译语义改成显式字典传递：
  - `using` 约束在调用点自动补齐为参数
  - `show(x)` 这类方法调用绑定到已解析的 dictionary，不再走全局注册表
- 实例选择顺序固定为：
  - 显式传入实例
  - 词法作用域可见实例
  - 包级默认实例
  - 多候选并列则报歧义

## 示例语法
```mygo
import strconv "go:strconv"

interface Show[A]
  func show(value: A) -> String
end

impl intShow: Show[Int]
  func show(value: Int) -> String
    strconv.Itoa(value)
  end
end

impl Show[String]
  func show(value: String) -> String
    value
  end
end

func demo(x: Int) -> String using intShow: Show[Int]
  x.show()
end

func demo2[A](x: A) -> String using showA: Show[A]
  x.show()
end
```

## Test Plan
- Parser
  - `interface / impl / using` 都能解析
  - 具名实例与匿名实例都能区分
  - `where` 在新语法下明确失败或给出迁移提示
  - 接口方法签名里的 `using` 能保留
- Compiler
  - 单方法和多方法 typeclass 都能生成显式参数传递
  - 同名方法在不同 interface 下不串台
  - 泛型实例保持泛型，不压扁成 `any`
  - 歧义实例能正确报错
- Regression
  - `prelude/prelude.mygo` 能迁移到新语法
  - `examples/main/main.mygo` 和 `examples/data-structure/data-structure.mygo` 保持可运行
  - 生成 Go 中不再出现 registry / reflect fallback 作为 typeclass 主路径

## Assumptions
- 主语法最终以 MyGO 命名为准，继续使用 `interface / impl / using`。
- `impl Show[String]` 代表匿名默认实例，`impl intShow: Show[Int]` 代表命名实例。
- `using` 的目标是静态字典传递，不再引入新的运行期查找中心。
- 这次重构会一并迁移 prelude，避免主语言和 prelude 双轨并存。
---

# 按源文件拆分生成

## Summary
将编译器从“一个包生成一个 `zz_mygo.gen.go` 文件”改为“每个 `.mygo` 源文件独立生成一个 `.gen.go` 文件”。支持 `_test.mygo` 自动映射为 Go `_test.go` 测试文件。

## 关键改动

### API 变更
- `CompileDir(dir string) (string, error)` → `CompileDir(dir string) ([]string, error)`，返回所有生成的文件路径。
- `compileDir(dir, workspaceRoot, noPrelude) (string, error)` 同理返回 `[]string`。
- `(p *Package) Generate() (string, error)` → `(p *Package) GenerateFiles() (map[string]string, error)`，返回 `文件名 → Go源码`。
- CLI `sync`/`build` 命令的 `written` 变量保持 `[]string`，但现在是每包多文件。

### AST 变更
- 6 种 `Decl` 类型新增 `SourceFile string` 字段：`ImportDecl`, `EnumDecl`, `StructDecl`, `InterfaceDecl`, `ImplDecl`, `FuncDecl`。
- `Package` 新增 `Files map[string][]Decl`，按源文件名分组声明。
- 非源码声明（prelude 注入）的 `SourceFile == ""`。

### 生成行为
- 每个 `.mygo` 源文件生成一个 `zz_<basename>.gen.go` 文件。
- 命名规则：
  - `main.mygo` → `zz_main.gen.go`
  - `example_test.mygo` → `zz_example_test.gen.go`（Go 编译时自动识别为 `_test.go`）
- 同目录下多个 `.mygo` 文件各自独立生成，但都属于同一个 Go package。
- prelude 的 import 由每个引用它的文件独立携带。

### 测试支持
- `_test.mygo` 中的 `func TestXxx()` 自动生成到 `zz_xxx_test.gen.go`，`go test` 自动识别。
- 不需要特殊的 package 名或 import 处理。

## Test Plan
- `CompileDir` 返回路径列表，验证数量和命名。
- 单文件包：1 个 .mygo → 1 个 .gen.go。
- 多文件包：N 个 .mygo → N 个 .gen.go。
- `_test.mygo` → `*_test.go` 文件存在且内容合法。
- prelude 编译通过。
- `go build` / `go test` 正常。

---

# 多行字面量 NEWLINE 支持

## 问题
`{ "key": value }` 和 `Struct { field: value }` 在一行上能正确解析，但多行写法会报语法错误（`NEWLINE` 打断了 `COLON expr` 或 `COMMA entry` 的匹配）。

## 解决方案
在 lexer 层面（`parser_lexer.go`），添加 `braceDepth` 计数器：
- 遇到 `{` 时 `braceDepth++`
- 遇到 `}` 时 `braceDepth--`
- 当 `braceDepth > 0` 时，`NEWLINE` token 被跳过（不返回，继续读取下一个 token）

这样 LBRACE...RBRACE 块内的所有 NEWLINE 都不会出现在 yacc 状态机中，多行 `{ "a": 1, "b": 2 }` 和 `Struct { field: 1, field2: 2 }` 都能正确解析。

## 注意
- 此修改只影响 LBRACE...RBRACE 块内的 NEWLINE，不影响其他语法（如 `let x = 1\n` 中的 NEWLINE 仍然正常）。
- 尾部逗号（trailing comma）如 `{ "a": 1, \n }` 仍然不被支持——这是因为原始 yacc 规则 `COMMA collection_entry` 期望 COMMA 后面紧跟一个 entry。
- 原始的 `Map[KeyType, ValueType] { "key": value }` 语法已被删除，恢复为 bare `{ "key": value }` 和 `Struct { field: value }`。

## 改动文件
- `internal/mygo/parser/parser_lexer.go` — 添加 `braceDepth` 和跳过逻辑
- `internal/mygo/parser/parser.y` — 添加 `//go:build ignore`（go 1.26 兼容）
