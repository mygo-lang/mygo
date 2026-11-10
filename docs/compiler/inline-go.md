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
- The AST node is `InlineGoExpr(Type, Code, Operands, TypeOperands, RawType)` in `internal/mygo/ast2/ast2.mygo`. `GoOperand` and `GoTypeOperand` carry the value and type bindings.
- Parser ownership lives in `internal/mygo/parser2/syntax_parse_expressions.mygo`; `parseInlineGoExpr` builds the CST and `parseInlineGoBodyItems` groups the `code:`, `in`, and `type` entries.
- Type inference lives in `internal/mygo/typeinference2/infer.mygo`. It infers every operand expression normally, then assigns the explicit result type from `go[T]`.
- Codegen2 lowering lives in `internal/mygo/codegen2/translate_ast.mygo`. It renders each operand to Go (value operands as expressions, type operands via the Go type mapping), substitutes `{name}` placeholders in the raw snippet, and treats `go[()]` as a statement.
- Inline Go type operands automatically translate MyGO types (like `Int`, `String`, `Slice[Int]`, `Map[String, Bool]`) to their corresponding Go type representations (like `int`, `string`, `[]int`, `map[string]bool`).
- Keep inline Go examples small and boundary-focused. Prefer ordinary MyGO, Go FFI imports, `Ref.new`, `Option`, and `Result` when those can express the behavior without raw Go.

### Explicit raw result shape (`raw:`)

The self-hosted pipeline accepts an optional `raw:` field in a go block. The
type in `go[T]` remains the MyGO result type; `raw:` declares the Go-visible
shape the snippet evaluates to. Omitting `raw:` keeps the previous behavior
exactly: the snippet returns `T` itself, or one of the existing tuple forms.

```mygo
func AsRSAKey(key: Any) -> Option[Ref[rsa.PublicKey]]
  go[Option[Ref[rsa.PublicKey]]] {
    raw: Ref[rsa.PublicKey]
    code: """
      func() *rsa.PublicKey {
        typed, ok := {key}.(*rsa.PublicKey)
        if !ok {
          return nil
        }
        return typed
      }()
    """
    in key = key
  }
end
```

Supported `(target, raw)` pairs:

| Target `T` | Raw shape | Result |
| --- | --- | --- |
| `Option[Ref[U]]` | `Ref[U]` | nil becomes `None`, non-nil becomes `Some` |
| `Option[U]` | `(U, Bool)` | comma-ok becomes `Some` / `None` |
| `Result[U, Error]` | `(U, Error)` | `Ok` / `Err` |
| `Result[U, String]` | `(U, Error)` | `Err` uses `err.Error()` |
| `Result[(), Error]` | `Error` | `Ok(())` / `Err` |
| `Result[(), String]` | `Error` | `Err` uses `err.Error()` |
| any `T` | `T` | identity; emit the snippet directly |

- `raw:` requires an expression-form snippet. Statement lists are rejected
  with a diagnostic; statement-position use is also rejected.
- Unsupported pairs fail generation rather than guessing a conversion. A Go
  `error` cannot reconstruct a rich payload such as `ast2.Diagnostic`; keep
  rich-error `Result` values in direct-construction form and omit `raw:`.
- codegen2 emits the legacy interface representation. Codegen3 follows its
  selected ABI: canonical `Option[Ref[U]]` is the pointer itself, while
  `Option[U]` / `Result` use `Option__Some` / `Result__Ok` helpers;
  a legacy prelude uses `Some` / `None` / `Ok` / `Err`.
### Key Files

- **AST**: `internal/mygo/ast2/ast2.mygo` — `InlineGoExpr`, `GoOperand`, `GoTypeOperand`.
- **Parser**: `internal/mygo/parser2/syntax_parse_expressions.mygo` — `parseInlineGoExpr()`, `parseInlineGoBodyItems()`.
- **Inference**: `internal/mygo/typeinference2/infer.mygo` — the `InlineGoExpr` case.
- **Codegen2**: `internal/mygo/codegen2/translate_ast.mygo` — inline Go expression and statement lowering.
