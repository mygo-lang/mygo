# semantics.md — Current Syntax Rules

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
- Struct field declarations may carry an optional Go struct tag as a trailing string literal, using normal double quotes, triple-double-quoted multiline strings, or backtick-quoted raw strings. The parser stores the literal text on `Field.Tag`, and `genStruct` should emit it as a Go struct tag.
- String literals come in three forms: (1) double-quoted `"..."` with escape processing (`\n`, `\t`, `\\`, `"`); (2) triple-double-quoted `"""..."""` for multiline strings with escape processing; (3) backtick-quoted `` `...` `` (raw string literals) with no escape processing — all content between the backticks is preserved verbatim, including newlines and backslashes. The closing backtick must appear on the same line. The parser stores the processed content in `LiteralExpr.Value` with `Kind: "string"` for all three forms.
- `Ref.new(expr)` constructs a reference value and is lowered as `&expr`; if the argument is already a ref/pointer, lowering leaves it unchanged rather than producing a pointer-to-pointer.
- `Slice[A]` is the only slice type spelling. The parser no longer accepts `A[]` or `Int[]` shorthand, which keeps type syntax aligned with ordinary generic instantiation.
- The parser test suite now covers package/function declarations, collection literals, chain postfix, `if`/`while`/`switch`, pipe precedence, struct/interface/impl declarations, `let`/`var`/assignment, func literals, `using` clauses, enum declarations, switch patterns, and nested/empty collection literals.
- Integer literals support hex (`0xff` / `0XFF`), octal (`0o777` / `0O777`), and binary (`0b1010` / `0B1010`) prefix syntax. These are parsed as `NUMBER` tokens in the lexer — the raw literal string (e.g. `"0xff"`) is stored as `LiteralExpr.Value` with `Kind: "number"`.
- Supported numeric types: `Int`, `Int8`, `UInt8`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Int64`, `UInt`, `UInt64`, `Float32`, `Float64`. All are represented as `*NamedType` in the AST and lowered to corresponding Go primitives via `goType`, `hmTypeString`, `jenTypeExpr`, and `typeString`. The prelude provides `Show` and `Eq` impls for all of them.
- Nested slice types are written explicitly as `Slice[Slice[Int]]`, and empty `[]` is treated as an empty slice literal in expression position.
- `using` clauses support multiple constraints and constraint type arguments in both function and interface method signatures.
- `where` has been removed from the main syntax; typeclass requirements now use `using` only. The parser rejects `where` with an explicit migration error.
- `impl` supports three forms: `impl Type` (inherent methods), `impl Type : Interface[Args]` (named/generic typeclass implementation), and `impl Interface[Args]` (anonymous default instance).
- `switch` pattern parsing currently accepts wildcard patterns and variant patterns with optional identifier arguments, such as `Some(x)`.
- `switch` pattern parsing also accepts tuple patterns such as `(Some(_), None)` and recursively nests them, with `_` treated as an ignore slot instead of a binding.
- Tuple return lowering now supports multi-return Go signatures when the declared function return type is a tuple, and tuple destructuring in `let` only activates when the binding uses parenthesized names.
- Keep `examples/main/main.mygo` aligned with the compiler's current boundary behavior, especially for `Ref`, `Option`, and `Result`.
- Typeclass lookup should respect lexical scope first: local bindings and function-value bindings shadow typeclass names, `using`-bound methods are visible inside nested blocks, and package-level dispatch is the fallback.
- When multiple typeclass candidates are visible, prefer the more specific binding by comparing concrete type coverage first, then type-parameter usage, then `any` usage; report ambiguity when candidates remain tied.

## Inherent Struct Impl

Structs may define methods in a standalone `impl Type` block without declaring or satisfying an interface.

```mygo
struct Rectangle
  width: Float64
  height: Float64
end

impl Rectangle
  func area(self: Rectangle) -> Float64
    self.width * self.height
  end

  func scale(self: Rectangle, factor: Float64) -> Rectangle
    Rectangle {
      width: self.width * factor,
      height: self.height * factor,
    }
  end
end
```

Generic receiver types use the same impl type-parameter syntax as typeclass impls:

```mygo
struct Box[A]
  value: A
end

impl[A] Box[A]
  func get(self: Box[A]) -> A
    self.value
  end

  func map[B](self: Box[A], f: func(A) -> B) -> Box[B]
    Box[B] { value: f(self.value) }
  end
end
```

Methods in an inherent impl must declare the receiver explicitly as the first parameter. The conventional receiver name is `self`, but it is not syntactically special.

```mygo
func area(self: Rectangle) -> Float64
```

Method call syntax is sugar over a receiver-first function call:

```mygo
let r = Rectangle { width: 10.0, height: 5.0 }
let a = r.area()
let bigger = r.scale(2.0)
```

The calls above resolve as if written:

```mygo
let a = Rectangle_area(r)
let bigger = Rectangle_scale(r, 2.0)
```

### Name Mangling

Inherent methods are emitted as top-level Go functions with a stable receiver-name prefix. This keeps MyGO source free to reuse method names across receiver types while avoiding Go symbol collisions.

- `impl Rectangle` method `area` lowers to `Rectangle_area`.
- `impl[A] Box[A]` method `get` lowers to `Box_get`.
- `impl[K, V] MapEntry[K, V]` method `key` lowers to `MapEntry_key`.
- The receiver's base named type participates in the mangled name; type arguments and impl type parameters remain in the generated Go signature, not in the symbol name.
- If two inherent impl methods have the same receiver base type and method name in the same package, report a duplicate method error.
- Different receiver base types may use the same method name because their mangled Go symbols differ.

For example:

```mygo
impl Rectangle
  func area(self: Rectangle) -> Float64
    self.width * self.height
  end
end

impl Circle
  func area(self: Circle) -> Float64
    self.radius * self.radius * 3.14159
  end
end
```

lowers to distinct Go functions:

```go
func Rectangle_area(self Rectangle) float64
func Circle_area(self Circle) float64
```

Selectors without a call keep their existing field-access meaning. Method lookup only applies to call expressions such as `value.method(args...)`, and field access takes precedence when resolving non-call selectors.

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

## Function Declaration Multiline Support

Per AGENTS.genfiles.md, the compiler supports per-`.mygo` file generation to `.gen.go`. Function declarations now support spanning across multiple lines at key positions.

### Syntax

Function signatures can break across lines at the following positions:

```mygo
// Parameters spanning multiple lines
func add(
  x: Int,
  y: Int
) -> Int
  x + y
end

// Return type on a new line
func add(x: Int, y: Int)
  -> Int
  x + y
end

// `using` clause on a new line (must come after `-> type`)
func foo(x: Int) -> String
  using Show
  show(x)
end

// Trailing comma supported
func handleCompletion(
  store: Ref[DocumentStore],
  uri: String,
  line: Int,
  char: Int,
) -> CompletionList
  ...
end
```

The same multiline support applies to interface method signatures (`func_sig`) and function literals (`func_lit`).

### Parser changes (`parser.y`)

- **`%type <token> opt_newlines`** — added type declaration so `opt_newlines` can appear between action blocks.
- **`opt_newlines`** — added `$$` assignments for both alternatives (`/* empty */` and `opt_newlines NEWLINE`).
- **`func_decl` / `func_sig`** — added `opt_newlines` between `opt_type_params`/`LPAREN`, `LPAREN`/`maybe_param_list`, `RPAREN`/`ARROW`, and `type`/`opt_using_clause`.
- **`func_decl` body** — added `opt_newlines` before `opt_using_clause`.
- **`maybe_param_list`** — changed `param_list` branch to `opt_newlines param_list opt_newlines`, allowing leading/trailing blank lines around the parameter list.
- **`param_list`** — added `opt_newlines` around `COMMA`, and added a `param_list opt_newlines COMMA` branch to support trailing commas.
- **`func_lit`** — added `opt_newlines` between `LPAREN`/`maybe_param_list`, `maybe_param_list`/`RPAREN`, `RPAREN`/`ARROW`, and `type`/`block_expr`.
- Regenerated `parser.go` via `goyacc`.
- New test: `TestParseFileMultilineFuncDecl` covering all multiline scenarios.
