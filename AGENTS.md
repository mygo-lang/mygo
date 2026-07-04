# AGENTS.md

## Project Shape

- `examples/main/main.mygo` is the canonical design sample for the language surface.
- `internal/mygo/parser/` owns syntax parsing.
- `internal/mygo/compiler/` owns lowering to generated Go entry points.
- `internal/mygo/ast/` owns the shared AST types.
- `internal/mygo/common/pos.go` owns shared position and error helpers.
- `prelude/prelude.mygo` is the built-in prelude source that is merged into every package before lowering.
- Generated Go lives next to the source example, e.g. `examples/main/zz_mygo.gen.go`, and should be treated as disposable.

## Type Model

- Keep type parameters explicit in the AST and preserve them in generated Go.
- The current design follows Lisette-style nominal concrete types and structural interfaces.
- Generic enums, structs, interfaces, and functions should remain generic in emitted Go rather than collapsing to `any`.
- Prefer top-level generic functions over generic methods whenever the same behavior can be expressed that way. Use receiver methods only when Go requires them for type identity or interface conformance.
- Named primitive spellings like `Int`, `String`, and `Bool` map to Go primitives in generation.
- `Unit` is a return-only marker in MyGO and should lower to a Go function with no return values, not to `struct{}`.

## Go FFI

- Use `import "go:pkg/name"` for Go packages.
- Allow an optional alias form like `import fmt "go:fmt"` when the Go package name should be explicit.
- Package-qualified selectors such as `fmt.Sprint(...)` should lower as Go selectors, not as struct field access.
- The built-in prelude provides common typeclasses such as `Show[A]` and `Eq[A]`; prefer using those protocols rather than ad hoc `any` formatting or conversion.
- The built-in prelude also owns foundational algebraic data types like `Option[A]` and `Result[A, E]`; use those rather than redeclaring them in example packages.
- Generated Go should only include helper imports when they are actually needed; `reflect` is now a fallback for truly dynamic `any` function calls, not a blanket import.
- Typeclass-style `impl` blocks should lower to standalone helper functions plus explicit function parameters at call sites, not to method dictionaries.
- `Ref[T]` is the non-nil reference form at the Go boundary and should lower to `*T` in generated Go.
- `Ref[T]` remains a compiler-recognized boundary type, not a prelude-declared enum or struct.
- `Ref.new(expr)` is the canonical MyGO expression for producing a `Ref[T]`; it lowers to Go address-taking (`&expr`) and should be preferred over exposing raw `&` syntax in MyGO source.
- `Option[Ref[T]]` is the preferred shape for possibly-nil pointer returns and should be preserved rather than collapsed to a bare pointer.
- `Option` continues to represent absence for nilable Go values and comma-ok style results.
- `Result` is the dedicated shape for Go `error`-bearing flows and should be used instead of encoding failures as `Option`.
- `List[A]` is a singly-linked list with `head: A` and `tail: Option[Ref[List[A]]]`; `None` terminates the list.
- `Slice[A]` is MyGO's canonical slice type spelling and lowers directly to Go's native slice `[]A`.
- `Map[K, V]` is Go's native map `map[K]V`.
- `Set[A]` is Go's native set `map[A]struct{}`.

## Workflow Notes

- Prefer small, focused changes that keep the example file in sync with compiler behavior.
- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work, not only return a value.
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment.
- The prelude should be authored in MyGO when possible; if a prelude fragment cannot yet be expressed in MyGO, it may be implemented in Go as the lowest-level fallback.

## Known Issues

- **prelude typeclasses fully generated**: The prelude's `enum`, `struct`, `interface`, and `impl` blocks are now compiled via the standard generator (no `skipTypeclasses`), so Show, Eq, and Enumerable impls are all registered at init time. The hand-written `prelude_go.go` still provides low-level Slice/Map/Set Enumerable helpers.
- **`where` removed from parser**: `where` is no longer a recognized keyword — the parser rejects it with a migration hint pointing to `using`. The lexer tokens `USING` is the only typeclass constraint keyword.
- **Prelude dispatch registry removed**: All runtime dispatch registry code (`show_showDispatchRegistry`, `Eq_equalsDispatchRegistry`, etc.) has been removed from the compiler in favor of explicit `using`-based dictionary passing. Hand-written `prelude_go.go` still provides low-level Slice/Map/Set helpers.
- **`prelude_go.go` does not compile**: `prelude/prelude_go.go:52` references `Eq[T]` which is undefined in the prelude package, causing `go vet` to fail. The prelude's `Eq` interface is generated via the standard generator during compilation, so the hand-written Go helpers cannot directly reference it.
- **`Nil` fully removed**: `translateIdent` no longer has hardcoded `Nil` support. New code should model absence with `Option`, as in `Option[Ref[List[T]]]`.
- **Generic `impl` parsing aligned**: `impl[T] List[T]: Enumerable[List[T], T]` now parses correctly (the `opt_impl_type_params` grammar bug is fixed). The prelude's Enumerable impls exercise this path.
- **`sumList` type ergonomics**: `examples/data-structure/data-structure.mygo` currently accepts `List[Int]`, creates a traversal ref with `Ref.new(lst)`, and walks `tail: Option[Ref[List[Int]]]`. This is runnable and keeps construction explicit, but it still takes the address of a local parameter copy; a future design may prefer accepting `Option[Ref[List[Int]]]` or `Ref[List[Int]]` directly.
- **AST `Col` vs `Column` inconsistency**: `MapLitPair` and `SetLitExpr` in `ast.go` use `Col int` instead of `Column int`. This causes `common.NodePos()` to silently return `(0, 0)` for these types via reflection, losing source position data for all map/set literal error messages.

## Current Semantics

- Files start with `package <package_name>` to set the generated Go package name. The old file-level `module` wrapper is removed, and declarations follow directly after the package header.
- Function bodies and other block forms are newline-separated statement lists; the last plain expression in a block is the return value.
- `if` now supports a single-line expression form like `if cond then a else b`, and that form does not require `end`.
- `let` introduces an immutable binding. Rebinding the same source name must use a later `let` and is treated as shadowing, not assignment.
- `var` introduces a mutable binding and may be assigned again later in the same scope.
- `let` may omit its type annotation when the initializer provides enough information for inference.
- `let _ = ...` is the supported discard form for return values that should not be bound.
- Pipe operators `<|` and `|>` are both supported in expression lowering.
- Struct literals support a constructor-like form such as `ABC { aaa: 123 }`.
- Generic struct literals can also carry explicit type arguments, such as `Box[Int64] { value: 123 }`.
- When a generic struct literal omits its type arguments, the compiler should infer them from the expected type or field values when possible.
- `Ref.new(expr)` constructs a reference value and is lowered as `&expr`; if the argument is already a ref/pointer, lowering leaves it unchanged rather than producing a pointer-to-pointer.
- `Slice[A]` is the only slice type spelling. The parser no longer accepts `A[]` or `Int[]` shorthand, which keeps type syntax aligned with ordinary generic instantiation.
- The parser test suite now covers package/function declarations, collection literals, chain postfix, `if`/`while`/`switch`, pipe precedence, struct/interface/impl declarations, `let`/`var`/assignment, func literals, `using` clauses, enum declarations, switch patterns, and nested/empty collection literals.
- Nested slice types are written explicitly as `Slice[Slice[Int]]`, and empty `[]` is treated as an empty slice literal in expression position.
- `using` clauses support multiple constraints and constraint type arguments in both function and interface method signatures.
- `where` has been removed from the main syntax; typeclass requirements now use `using` only. The parser rejects `where` with an explicit migration error.
- `impl` supports two forms: `impl Type : Interface[Args]` (named/generic) and `impl Interface[Args]` (anonymous default instance).
- `switch` pattern parsing currently accepts wildcard patterns and variant patterns with optional identifier arguments, such as `Some(x)`.
- Keep `examples/main/main.mygo` aligned with the compiler's current boundary behavior, especially for `Ref`, `Option`, and `Result`.
- Typeclass lookup should respect lexical scope first: local bindings and function-value bindings shadow typeclass names, `using`-bound methods are visible inside nested blocks, and package-level dispatch is the fallback.
- When multiple typeclass candidates are visible, prefer the more specific binding by comparing concrete type coverage first, then type-parameter usage, then `any` usage; report ambiguity when candidates remain tied.

## Collection Types

- `List[A]`: singly-linked list using `Option[Ref[List[A]]]` for the tail field — `None` is the terminator, `Some(ref)` points to the next node. This avoids a separate `Nil` helper and keeps the nil-termination semantics explicit through `Option`.
- `Slice[A]`: MyGO syntax `Slice[Int]` → Go `[]int`. Lowered directly to Go's native slice type via `goType` / `typeString`.
- `Map[K, V]`: lowered directly to Go's native `map[K]V` via `goType` / `typeString`.
- `Set[A]`: lowered directly to Go's native `map[A]struct{}` via `goType` / `typeString`.

### Design Rationale

- **Why `Option[Ref[List[T]]]` instead of just `Ref[List[T]]`?**
  `Option` provides an explicit `None` terminator for list ends, avoiding nil-pointer dereferences. `Ref` ensures the tail is a non-nil pointer (Go `*List[T]`), so when the tail is `Some`, we always have a valid pointer to dereference. This separates "no next node" (`None`) from "points to next node" (`Some(ref)`), which is both safer and more idiomatic in the type system.

- **Why `Slice[A]` / `Map[K,V]` instead of MyGO struct declarations?**
  These types wrap Go's native slices, maps, and sets directly. Declaring them as MyGO structs would add unnecessary runtime overhead (extra fields, allocation, indirection) and wouldn't provide additional type safety beyond the Go type system. Instead, the compiler recognizes the type names `Slice`, `Map`, `Set` and lowers them directly to Go builtins.

- **Why not `[]A` prefix syntax?**
  The parser uses ordinary generic syntax for slices, so `Slice[Int]` is the single canonical form. This keeps slice types visually consistent with `Map[K, V]`, `Set[A]`, and other generic nominal types.

- **Why not `Slice` / `Map` / `Set` as prelude struct types?**
  Previously, these were declared as structs in `prelude.mysrc` with placeholder fields (`entries: String`, `size: Int`). They were removed because: (1) they served no runtime purpose — the prelude struct declarations had no usable fields; (2) `genStruct` would emit them as Go structs, conflicting with the `goType` lowering to native Go types; (3) keeping them only in the compiler's type lowering is cleaner and zero-cost. `List[A]` remains as a prelude struct because it needs actual runtime data structure semantics.

## Collection Literals

- Slice: `[1, 2, 3]: Slice[Int]` → Go `[]int{1, 2, 3}`
- Map: `{"a": "1", "b": "2"}: Map[String, String]` → Go `map[string]string{"a": "1", "b": "2"}`
- Set: `{"x", "y"}: Set[String]` → Go `map[string]struct{}{"x": {}, "y": {}}`
- No list literal syntax (as designed).
- Type inference strategy: infer from element expressions first; fall back to type annotation if inference fails; error if neither can determine the type. Mismatched element types produce an error.
- Empty `{}` is treated as an empty map by default; if the expected type is `map[A]struct{}`, it becomes an empty set.
- Parser uses a heuristic in `{...}`: if every entry uses `:` separator → `MapLitExpr`, otherwise → `SetLitExpr`.

### AST Nodes (`internal/mygo/ast/ast.go`)

- `SliceLitExpr` — `{Line, Column, Elem TypeExpr, Elems []Expr}`
- `MapLitExpr` — `{Line, Column, Key TypeExpr, Val TypeExpr, Pairs []MapLitPair}`
- `MapLitPair` — `{Line, Col, Key Expr, Value Expr}`
- `SetLitExpr` — `{Line, Col, Elem TypeExpr, Elems []Expr}`

### Key Files

- **Parser**: `internal/mygo/parser/parser_expr.go` — `parseSliceLit()`, `parseCollectionLit()`, routed from `parsePrimary()`.
- **Compiler**: `internal/mygo/compiler/translate_expr.go` — `translateSliceLit()`, `translateMapLit()`, `translateSetLit()`, `translateEmptyMapLit()`.
- **Type parsing update** (`internal/mygo/parser/parser_core.go`): `parseType()` now treats `Slice[Int]` as the canonical slice type spelling and no longer recognizes `Int[]` shorthand.

## Recent Work

- **Typeclass refactoring (MIGRATE.md)**: Unified typeclass semantics onto a single `interface`/`impl`/`using` route, removing the old `where` syntax and runtime dispatch registry. Key changes:
  - **`examples/main/main.mygo`**: Migrated `where` → `using` constraints; replaced `Int[]` → `Slice[Int]` for collection type annotations.
  - **Parser anonymous impl support**: Both `parser.y` (yacc) and `parser_core.go` (recursive-descent) now accept `impl Interface[Args]` (anonymous) alongside the existing `impl Type : Interface[Args]` (named) form.
  - **`where` rejection**: The parser explicitly rejects `where` keywords with a migration hint pointing to `using`.
  - **`opt_impl_type_params` grammar fix**: Restored `maybe_name_list RBRACK` suffix on the `LBRACK` alternative so `impl[T] List[T]: ...` parses correctly.
  - **Added `currentImplLine`/`currentImplCol`**: Proper position tracking for impl declarations in the parser.
  - **Improved error reporting**: `error.go` `Error()` now includes line/column and the offending token.
  - **Compiler cleanup**: Removed dead dispatch registry code — `genTypeclassDispatchers`, `dispatchRegistryName`, `dispatchFuncName`, `sortedTypeclassNames`, `implDispatchKey`, `dispatchKeyForTypes`, `dispatchKeyExpr`, `dictVarName` all deleted.
  - The `using` constraint now directly generates explicit dictionary/function parameters at call sites (no more `reflect`-based fallback).
- **Complete Jennifer refactoring (Phase 2)**: Fixed all remaining jennifer API usages across the compiler. Changed all expression translation functions to return `jen.Code` instead of `string`. Key changes:
  - **translate_expr.go**: Fixed `translateSliceLit`, `translateMapLit`, `translateSetLit`, `translateEmptyMapLit` to return `jen.Code`. Used `jen.Dict` and `jen.DictFunc` for map/set/slice literal construction. Fixed error returns to use `nil` instead of `""` for jen.Code.
  - **translate_struct.go**: Changed `parts` from `[]jen.Code` to `jen.Dict`. Used `jen.DictFunc` for field initialization. Fixed type argument handling with proper `jen.Id()` calls.
  - **typeclass.go**: Fixed `callee.Call()` by using type assertion `(*jen.Statement).Call()`. Fixed `None` type parameter handling with proper iteration. Fixed `typeclassHelper` return to use `jen.Code` directly instead of wrapping in `jen.Id()`.
  - All function signatures now return `(jen.Code, string, error)` where first is generated code, second is type string, third is error.
  - Jennifer API patterns: `jen.Dict` is `map[Code]Code` (not a function), `jen.Lit()` takes `interface{}` for literals, `*jen.Statement` has `.Call()`, `.Dot()`, `.Op()`, `.Index()` methods requiring type assertion from `jen.Code` interface.
- Removed `where` from the parser/lexer path and switched the typeclass surface fully to `using`, with parser generation now going through `~/go/bin/golex` plus `goyacc`.
- **Complete Jennifer refactoring (Phase 1)**: Refactored `internal/mygo/compiler/` to use Jennifer for all code generation, eliminating string-based code generation. Deleted `section.go` and `unit_body_writer.go`. Converted `genGlobals()`, `genTypeclassDispatchers()`, `genImpl()`, `genFunc()`, `translateSwitch()`, and `translateWhile()` to use Jennifer's type-safe API. This improves type safety, maintainability, and eliminates string concatenation for generating Go code.
- Added `Ref.new(expr)` lowering for explicit `Ref[T]` construction, updated `examples/data-structure` to use it for `Option[Ref[List[A]]]` tails, and taught field lookup to resolve through generated Go pointer types like `*List[int]`.
- Introduced `Slice`, `Map`, and `Set` as compiler-handled collection types with no prelude struct declarations — lowered directly to Go natives (`[]A`, `map[K]V`, `map[A]struct{}`). `Slice[A]` is now the only slice type syntax.
- Further split `internal/mygo/compiler/` into focused files: `helpers.go`, `type_inference.go`, `typeclass.go`, `translate_struct.go`, and `go_package.go`, while keeping `generate.go`, `translate_expr.go`, `translate_call.go`, `translate_control.go`, `api.go`, and `types.go` as separate compiler concerns.
- Unified all position/error helpers onto `common.NodePos` and `common.ErrorAtPos`, removing the wrapper `pos.go` files from root, parser, and compiler packages.
- Unified shared line/error helpers into `internal/mygo/common/pos.go`.
- Split the compiler implementation into `internal/mygo/compiler/` with dedicated API, type, and implementation files.
- Split the monolithic AST, parser, and compiler implementation into dedicated subpackages.
- Moved the parser lexer/token machinery into `internal/mygo/parser/`.
- Added shared AST aliases and moved the canonical AST definitions into `internal/mygo/ast/`.
- Added `while` as an expression form with newline-delimited body parsing and Go `for`-loop lowering.
- Extended expression parsing and lowering to recognize `&&`, `||`, `-`, and `/`, while keeping comparison operators type-checked against `Eq` support.
- Improved numeric literal inference so expected integer and float types are preserved instead of defaulting too early.
- Added compiler coverage for `while` loops, arithmetic/logic operator precedence, and relation-operator rejection when `Eq` support is missing.
