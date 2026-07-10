# AGENTS.semantics.md — Current Syntax Rules

## Current Semantics

- Files start with `package <package_name>` to set the generated Go package name. The old file-level `module` wrapper is removed, and declarations follow directly after the package header.
- Function bodies and other block forms are newline-separated statement lists; the last plain expression in a block is the return value.
- `if` now supports a single-line expression form like `if cond then a else b`, and that form does not require `end`.
- `let` introduces an immutable binding. Rebinding the same source name must use a later `let` and is treated as shadowing, not assignment.
- `var` introduces a mutable binding and may be assigned again later in the same scope.
- `let` may omit its type annotation when the initializer provides enough information for inference.
- `let _ = ...` is the supported discard form for return values that should not be bound.
- Tuple values use anonymous structs in lowering, while `let (a, b) = expr` destructures a tuple return directly and `let c = expr` keeps the tuple as a single anonymous struct value.
- Tuple destructuring supports nested patterns and `_` ignore slots, so `let (_, b) = expr` and `let (a, (_, c)) = expr` bind only the named leaves.
- Pipe operators `<|` and `|>` are both supported in expression lowering.
- Struct literals support a constructor-like form such as `ABC { aaa: 123 }`.
- Generic struct literals can also carry explicit type arguments, such as `Box[Int64] { value: 123 }`.
- When a generic struct literal omits its type arguments, the compiler should infer them from the expected type or field values when possible.
- Struct field declarations may carry an optional Go struct tag as a trailing string literal, using either normal double quotes or triple-double-quoted multiline strings. The parser stores the literal text on `Field.Tag`, and `genStruct` should emit it as a Go struct tag.
- `Ref.new(expr)` constructs a reference value and is lowered as `&expr`; if the argument is already a ref/pointer, lowering leaves it unchanged rather than producing a pointer-to-pointer.
- `Slice[A]` is the only slice type spelling. The parser no longer accepts `A[]` or `Int[]` shorthand, which keeps type syntax aligned with ordinary generic instantiation.
- The parser test suite now covers package/function declarations, collection literals, chain postfix, `if`/`while`/`switch`, pipe precedence, struct/interface/impl declarations, `let`/`var`/assignment, func literals, `using` clauses, enum declarations, switch patterns, and nested/empty collection literals.
- Integer literals support hex (`0xff` / `0XFF`), octal (`0o777` / `0O777`), and binary (`0b1010` / `0B1010`) prefix syntax. These are parsed as `NUMBER` tokens in the lexer — the raw literal string (e.g. `"0xff"`) is stored as `LiteralExpr.Value` with `Kind: "number"`.
- Supported numeric types: `Int`, `Int8`, `UInt8`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Int64`, `UInt`, `UInt64`, `Float32`, `Float64`. All are represented as `*NamedType` in the AST and lowered to corresponding Go primitives via `goType`, `hmTypeString`, `jenTypeExpr`, and `typeString`. The prelude provides `Show` and `Eq` impls for all of them.
- Nested slice types are written explicitly as `Slice[Slice[Int]]`, and empty `[]` is treated as an empty slice literal in expression position.
- `using` clauses support multiple constraints and constraint type arguments in both function and interface method signatures.
- `where` has been removed from the main syntax; typeclass requirements now use `using` only. The parser rejects `where` with an explicit migration error.
- `impl` supports two forms: `impl Type : Interface[Args]` (named/generic) and `impl Interface[Args]` (anonymous default instance).
- `switch` pattern parsing currently accepts wildcard patterns and variant patterns with optional identifier arguments, such as `Some(x)`.
- `switch` pattern parsing also accepts tuple patterns such as `(Some(_), None)` and recursively nests them, with `_` treated as an ignore slot instead of a binding.
- Tuple return lowering now supports multi-return Go signatures when the declared function return type is a tuple, and tuple destructuring in `let` only activates when the binding uses parenthesized names.
- Keep `examples/main/main.mygo` aligned with the compiler's current boundary behavior, especially for `Ref`, `Option`, and `Result`.
- Typeclass lookup should respect lexical scope first: local bindings and function-value bindings shadow typeclass names, `using`-bound methods are visible inside nested blocks, and package-level dispatch is the fallback.
- When multiple typeclass candidates are visible, prefer the more specific binding by comparing concrete type coverage first, then type-parameter usage, then `any` usage; report ambiguity when candidates remain tied.

## Pattern Matching (`switch`/`case`)

### Syntax
```mygo
switch target_expr
  case Variant1(arg1, arg2) => body1,
  case Variant2 => body2,
  case _ => defaultBody
end
```
Commas between cases are optional (Rust/Scala style).

### Parser (`parser.y`)
- `switch_case_separator` consumes an optional comma after each case body before the next `case` or `end`.

### Go Backend (`compiler/translate_control.go`)
- `translateSwitch()` emits if-else chains with type assertions instead of Go `switch x.(type)`:
  ```go
  if v, ok := target.(OptionSome[A]); ok {
      return body_with_v_F0
  } else if _, ok := target.(OptionNone); ok {
      return body
  } else {
      panic("unreachable")
  }
  ```
- Pattern bindings use `v.F0`, `v.F1`, etc., scoped per if-branch.
- Wildcard `_` patterns become plain `else` branches.
- Expression form is wrapped in an immediately-invoked function literal `func() T { ... }()`.

### HM Type Inference (`internal/mygo/typeinference/`)
- `InferState` gains `PkgInfo` field for enum variant lookup during pattern inference.
- `inferSwitch()` extends each case body's environment with pattern bindings from the matched variant's fields.
- Field types are resolved by substituting the target enum's type parameters with the concrete type arguments.
- Helper functions: `resolveEnumType()`, `lookupEnum()`, `findEnumVariant()`, `substituteTypeParams()`.

### Tests
- `TestTranslateSwitchUsesIfElse` (3 subtests): expression form with variant patterns, wildcard pattern, statement form (no expected type).
- `TestE2ESwitchGeneratedCodeIsValidGo`: full compiler pipeline produces valid Go syntax verified by `go/parser`.

## New Block Syntax (`if =>` / `case then...end`)

Per MIGRATE.md "新语句块方案", the yacc parser supports:

- **`if cond => a else b`** — inline if with `=>` instead of `then`, added as `IF expr ARROW expr ELSE expr`.
- **`case pattern then ... end`** — switch case block form, added as `CASE pattern THEN block_expr ... END`.
- Both forms coexist with the existing `if cond then a else b` and `case pattern => expr` syntax.

### Parser changes
- `parser.y`: two new grammar alternatives (one in `if_expr`, one in `switch_case`) — conflicts reduced from 33 to 29 shift/reduce.
- `parser.go`: regenerated via `goyacc`.
- `parser_test.go`: three new tests (`TestParseFileSupportsIfArrowForm`, `TestParseFileSupportsSwitchCaseThenEndBlock`, `TestParseFileSupportsMixedSwitchCaseForms`).
