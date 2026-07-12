# inline-go.md — Inline Go Embedding

## Inline Go Embedding

- Syntax:
  ```mygo
  let y: Int = go[Int] {
    code: "{x} + 1"
    in x = n
  }
  ```
  Type operands are also supported:
  ```mygo
  let y: String = go[String] {
    code: "{T}({v})"
    in v = n
    type T = String
  }
  ```
  Multiple type operands are allowed, and they can be mixed freely with value operands:
  ```mygo
  let m: Bool = go[Bool] {
    code: "map[{K}]{V}{{v}}"
    in v = 1
    type K = String
    type V = Int
  }
  ```
- The AST node is `GoExpr{Result TypeExpr, Code string, Operands []GoOperand, TypeOperands []GoTypeOperand}` in `internal/mygo/ast/ast.go`. `GoTypeOperand{Name string, Type TypeExpr}` carries type bindings.
- Parser ownership lives in `internal/mygo/parser/parser.y`; `go`, `in`, and `type` (within go blocks) are lexer keywords.
- The `Lex` function in `parser.y` maps `"type"` to the `TYPE` token so it's recognized as a keyword inside go blocks.
- HM inference (`internal/mygo/typeinference/infer.go`) infers every operand expression normally, then assigns the explicit result type from `go[T]`.
- Compiler lowering lives in `internal/mygo/compiler/translate_go.go`. It renders each operand to Go (value operands as expressions, type operands via `goType`), substitutes `{name}` placeholders in the raw snippet, and returns an empty type for `go[()]` so statement lowering treats it as a statement.
- Inline Go type operands automatically translate MyGO types (like `Int`, `String`, `Slice[Int]`, `Map[String, Bool]`) to their corresponding Go type representations (like `int`, `string`, `[]int`, `map[string]bool`).
- Keep inline Go examples small and boundary-focused. Prefer ordinary MyGO, Go FFI imports, `Ref.new`, `Option`, and `Result` when those can express the behavior without raw Go.

### Key Files

- **AST**: `internal/mygo/ast/ast.go` — `GoExpr`, `GoOperand`, `GoTypeOperand`.
- **Lexer**: `internal/mygo/parser/parser_lex.l` — `type` keyword → `TYPE` token.
- **Lexer runtime**: `internal/mygo/parser/parser_lexer.go` — `nextToken()` maps `TYPE` to `tokKeyword`.
- **Parser grammar**: `internal/mygo/parser/parser.y` — `go_expr`, `go_field_list`, `go_operand`, `go_type_operand` rules.
- **Parser state**: `internal/mygo/parser/parser_state.go` — `currentGoTypeOperands` accumulator.
- **Compiler**: `internal/mygo/compiler/translate_go.go` — `translateGoExpr()` resolves both `GoOperand` and `GoTypeOperand` via `g.goType()`.
