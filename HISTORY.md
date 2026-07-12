## Recent Work

- **Prelude moved to project root**: Moved `lib/prelude/` to project root `prelude/`. Updated all Go import paths from `github.com/mygo-lang/mygo/lib/prelude` to `github.com/mygo-lang/mygo/prelude`, and updated all test relative paths and documentation references accordingly.
- **Option.UnwrapOr**: Added standalone `UnwrapOr[A](opt: Option[A], defaultVal: A) -> A` function in `prelude/option.mygo`.

- **If expression `elsif` syntax**: Added `elsif` keyword for block `if cond then ... elsif cond then ... else ... end`. Removed old inline `if cond then a else b` (use `if cond => a else b`) and bare block `if cond NEWLINE ... end` (no `then`) forms. Parser: `if_block_tail` grammar rule, `ifParts` struct, `currentIfPartsStack`. `elsif` is a separate yacc token (not IDENT) in both `parser.y` and `parser_lex.l`. EBNF, examples, README, and all tests updated to use `=>` form.

- **Prelude refactoring**: Split `lib/prelude/prelude.mygo` into separate files by concern: `conversion.mygo` (`From`/`Into` impls), `default.mygo` (`Default` impls), `numeric.mygo` (`Ord`/`Show`/`Eq` numeric impls), `slice.mygo` (Slice helpers). Removed `import fmt "go:fmt"` and `import strconv "go:strconv"` from `prelude.mygo`. Moved `IEnumerable` interface and `Option`/`List`/`Slice`/`Map`/`Set` implementations out.

- **Inline Go `goExprCode` improvements** (`translate_go.go`): Added `goExprCode()` that parses simple Go expressions into proper `jen.Code` — function calls (`fn(arg)` → `jen.Id("fn").Call(jen.Id("arg"))`), slice operations (`arr[x:]` → `jen.Id("arr").Index(...)`), len comparisons (`arr[:len(arr)] == x`). Added `goCalleeCode()` for Go built-in types (`string`, `int`, `bool`). Uses three regex patterns: `goSimpleCallRE`, `goSliceFromRE`, `goSliceToLenEqRE`.

- **Go multi-return tuple destructuring**: `emitMultiReturnBindPattern()` generates `result, err := goFunc()` directly (no intermediate struct). `isGoMultiReturnTypeString()` detects Go-style multi-return signatures. Supports nested patterns, `_` slots. Applied in both `translateBlock` and `translateFunctionBlock`.

- **Inherent impl static methods**: Added `HasReceiver` field to `inherentMethod` struct. `isInherentReceiverParam()` checks if first param matches impl type. `translateInherentTypeCall()` handles `TypeName.method(args)` for receiverless methods. `paramJen()`/`goTypeJen()` helpers for generating typed Go params.

- **FuncLit param isolation**: Added `currentParamsStack` (`[][]ast.Param`) to parser for nested func lit parameter isolation. The `func_lit` rule pushes/pops the stack, preventing parameter leakage between nested function literals.

- **Lexer multiline string fix** (`parser_lex.l`): Replaced `scanMultilineString()` (which relied on unreliable regex-based `scan()` detection) with `scanMultilineStringFromLookahead()` using `Lookahead()`/`Next()` API for reliable `"""..."""` delimiter detection.

- **Type compatibility improvements**: `goTypeCompatible()` now handles `C[rune]`/`C[int32]` unifying with `String` (`isStringRuneSequenceType()`), and `rune`/`int32` alias pair (`isRuneGoAliasPair()`). `matchTypeclassHelper` fix: uses first type arg's type string for receiver matching when impl has type args.

- **`translateTypeclassCall` resolution order fixed**: `constraintFuncForMethod` (using-param) → `typeclassMethods` (lexical bindings) → `matchTypeclassHelper` (package-level dispatch) → receiver-type matching.

- **Docs**: Updated `docs/compiler/semantics.md` (elsif, static methods, multi-return, func lit isolation), `docs/compiler/inline-go.md` (goExprCode), `docs/compiler/genfiles.md` (type compatibility extensions), `docs/compiler/core.md` (elsif keyword, multiline string scanner).

- **Phase 2: String utility functions**: Added 17 string manipulation functions to `lib/prelude/string.mygo` — `HasPrefix`, `HasSuffix`, `Trim`, `TrimSpace`, `TrimPrefix`, `TrimSuffix`, `Split`, `SplitN`, `Join`, `Replace`, `ReplaceAll`, `ToUpper`, `ToLower`, `Repeat`, `Index`, `LastIndex`, `Fields`. All wrap Go's `strings` package via `go[T]{}` inline embeddings. Generated Go code in `lib/prelude/zz_string.gen.go`.
- **Phase 3: Sorting and Searching (`lib/sort/`)**: New library package with generic `Sort`, `IsSorted`, `BinarySearch`, `Min`, `Max` functions. All implemented via inline Go embeddings (quicksort algorithm, no Go `sort` dependency). `BinarySearch` returns `Option[Int]`.
- **Compiler: MyGo-to-Go type normalization** (`compiler/type_inference.go`): Added `normalizeMygoTypeToGo()` that converts MyGo collection types (`Ref[T]` → `*T`, `Slice[T]` → `[]T`, `Set[T]` → `map[T]struct{}`, `Map[K,V]` → `map[K]V`) and primitives (`String` → `string`, etc.) to their Go equivalents for improved type compatibility checking in `goTypeCompatible()`.
- **Docs**: Added `docs/compiler/stdlib-phase2.md` (string utilities), `docs/compiler/stdlib-phase3.md` (sort package), and updated `docs/compiler/genfiles.md` (type normalization).

- **`-> Unit` → `-> ()` migration**: All source files migrated from `-> Unit` / `go[Unit]` to `-> ()` / `go[()]`. The old `Unit` name is no longer recognized as a return-type keyword.
  - **Parser** (`parser.y`): Added `| LPAREN RPAREN` alternative to the `type` rule so `()` is recognized as a type expression (empty tuple → unit type). Regenerated `parser.go` via `goyacc`. Conflicts: 39/5 (was 29/4).
  - **HM type inference** (`typeinference/types.go`): `typeFromAST` returns `TUnit{}` for empty `TupleType` (i.e. `()`).
  - **HM return handling** (`typeinference/infer.go`): When declared return type is `TUnit{}` or corresponds to `()`, skip unification — the body value is discarded in void context. Forces function return type to `TUnit{}`.
  - **Codegen** (`compiler/type_inference.go`): `isUnitType()` now checks for `*TupleType` with 0 elements alongside `*NamedType{Name:"Unit"}`.
  - **All source files**: `prelude.mygo`, `examples/*`, tests — every `-> Unit` → `-> ()` and `go[Unit]` → `go[()]`.
  - **Test fix** (`compiler/zz_test.go`): `TestCompilePrelude` no longer looks for the removed `optionMap` function; tests the Option Enumerable impl and `optionFlatMap` instead.

- **New numeric types and hex/octal/binary integer literals**: Added full support for `Int8`, `UInt8`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Float32` as first-class numeric types alongside existing `Int`, `Int64`, `UInt`, `UInt64`, `Float64`. Also added lexer support for hex (`0xff` / `0XFF`), octal (`0o777` / `0O777`), and binary (`0b1010` / `0B1010`) integer literal prefixes. Key changes:
  - **Lexer** (`parser_lex.l`): Added `hexnumber`, `octnumber`, `binnumber` lex rules with prefix patterns, placed before the general `number` rule so they match first. Regenerated `lex.yy.go` via `golex`.
  - **Type inference** (`typeinference/infer.go`): Added all new types to `initialTypeEnv` builtins list so they participate in HM inference.
  - **Compiler type lowering** (`type_inference.go`): Extended `primitiveGoName()` with all new types (`Int8`, `Int16`, `Int32`, `UInt`, `UInt8`, `UInt16`, `UInt32`, `UInt64`, `Float32`). `goType()`, `hmTypeString()`, and `typeString()` already had full coverage.
  - **Jennifer code gen** (`jennifer_gen.go`): Added `jen.Int8()`, `jen.Int16()`, `jen.Int32()`, `jen.Uint()`, `jen.Uint8()`, `jen.Uint16()`, `jen.Uint32()`, `jen.Uint64()`, `jen.Float32()` cases in both `jenTypeExpr` and `jenHKTTypeExpr`.
  - **Expression translation** (`translate_expr.go`): Extended accepted `expected` types list in literal lowering and `hasEqSupport()` to cover all new Go native type names.
  - **Go FFI helpers** (`helpers.go`): Added all new `types.Kind` entries to `goMyGoTypeString()`.
  - **Prelude** (`prelude.mygo`): Added `Show` and `Eq` interface impls for `Int8`, `UInt8`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Float32`.
  - **Tests** (`parser_test.go`): Added `TestParseFileSupportsNewNumericTypes` (verifies let bindings with new type annotations) and `TestParseFileSupportsHexOctalBinaryLiterals` (verifies `0xff`, `0o777`, `0b1010` parse as NUMBER literals with correct value strings).

- **Impl method call self-resolution fix**: The compiler can now resolve `FieldExpr` method calls on the same impl type (e.g., `self.isSome()` inside `IOption.isNone`). Three key changes:
  - `types.go`: Added `implTypeKey` and `implTypeParams` to `exprCtx` for carrying impl context.
  - `compiler_impl.go` (`genImpl`): Sets `implTypeKey`/`implTypeParams` on the exprCtx when generating impl method helpers.
  - `translate_call.go` (`translateCall`): New case in `FieldExpr` branch detects method calls matching the current impl's interface methods, translating them to helper function calls (e.g., `isSome_a[A](self)`) with explicit type arguments. This fixes `!self.isSome()` producing `!unknown()` in generated Go.

- **Inline Go embedding syntax**: Added Rust-`asm!`-style, expression-first raw Go embedding via `go[T] { code: "..."; in name = expr }`. The parser now has `GO`/`IN` tokens and a `GoExpr` AST node; HM infers operands and uses the explicit result type; compiler lowering substitutes named `{operand}` placeholders and supports `go[Unit]` in statement position. Parser/typeinference/compiler tests cover parsing, operand checking, placeholder errors, and generated Go validity. `examples/main/main.mygo` includes a small `raw_total` sample.
- **Prelude compilation fix with `--no-prelude` flag**: Added a `NoPrelude` flag to `Package` and `--no-prelude` CLI switch (`cmd/mygo/main.go`) to disable prelude auto-import during compilation. This breaks the circular dependency when compiling the prelude itself. Key changes:
  - `internal/mygo/compiler/types.go`: Added `NoPrelude bool` field to `Package` struct.
  - `internal/mygo/compiler/api.go`: Added public `CompileDirNoPrelude()` and `SyncNoPrelude()` functions; refactored `loadPackage()` to accept `noPrelude bool` and use `pkg.NoPrelude` instead of `pkg.Name != "prelude"` to gate prelude merging.
  - `cmd/mygo/main.go`: Parses `--no-prelude` before the subcommand; dispatches to `SyncNoPrelude`/`CompileDirNoPrelude` when set. Usage: `mygo --no-prelude sync prelude/`.
  - `internal/mygo/compiler/zz_test.go`: `TestCompilePrelude` passes `noPrelude=true` to `loadPackage`.

- **HM inference is now the default codegen type path**: `compiler.Generate()` runs `typeinference.InferPackage()` as a required pre-pass and returns inference errors instead of silently continuing. `translateExpr` consults `TypedInfo.ExprTypes` for expected/result types before falling back to local lowering, so generated calls and bindings prefer HM-derived types.
- **Go import type queries participate in HM**: `typeinference` now loads `go:` imports through Go's importer, registers aliases as `TGoPackage`, and resolves selectors such as `fmt.Sprint` to HM function types. Variadic Go functions are represented with `TFunc.Variadic`, and `any` unifies at the Go boundary. Local bindings like `let show = fmt.Sprint` infer as function values and lower to direct calls instead of `callAny`.
- **Yacc parser state isolation fixes**: The yacc parser keeps explicit stacks for nested call callees/arguments, block bodies, and function-type parameters, fixing cases like `Some(fn(v))`, switch case body leakage, and `func(A) -> B` parameter loss. Yacc wildcard pattern parsing also treats `_` as `WildcardPattern` when it comes through the IDENT path.
- **Generic enum constructor inference fixed**: Enum variant field types now substitute enum type parameters with the constructor scheme's type variables, so constructors like `Some` can instantiate independently per call site, e.g. `Some(fn(v))` in `optionMap[A, B]` infers `Option[B]`.
- **Scala3-style named `using` bindings**: Added `BindName` field to `ast.Constraint` for named bindings like `using intShow: Show[Int]`. The yacc constraint grammar handles `COLON IDENT constraint_suffix` (named) versus bare `constraint_suffix` (simple). The compiler's `genFunc` uses `BindName` as the Go parameter name when available. The call-site auto-injection of `using` parameters was improved with a three-tier resolution strategy: lexical scope → caller's typeclass bindings → package-level helper functions. The example `main.mygo` demonstrates the new syntax with `using myEq: Eq[A]`.

- **Typeclass refactoring (MIGRATE.md)**: Unified typeclass semantics onto a single `interface`/`impl`/`using` route, removing the old `where` syntax and runtime dispatch registry. Key changes:
  - **`examples/main/main.mygo`**: Migrated `where` → `using` constraints; replaced `Int[]` → `Slice[Int]` for collection type annotations.
  - **Parser anonymous impl support**: `parser.y` now accepts `impl Interface[Args]` (anonymous) alongside the existing `impl Type : Interface[Args]` (named) form.
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
- **Parser refactor**: Simplified declaration parsing by extracting names before type parameters for `ENUM`, `STRUCT`, `INTERFACE`, and `impl`. Fixed generic impl parsing by using `p.currentType` to hold the interface reference. Refactored `type` production to use explicit `case` statements for better code generation. Added nested type argument tracking via `savedTypeNameStack` to correctly handle `Map[Map[String, Int], Int]` and similar nested generics.
- **Prelude migration**: Migrated `prelude/prelude.mygo` to new typeclass syntax:
  - Generic impls with type parameters: `impl[T] List[T]: Enumerable[List[T], T]`
  - Named impls: `impl Int: Show[Int]`
  - Anonymous impls: `impl Show[String]`
  - All collection types (`List`, `Slice`, `Map`, `Set`) now have `Enumerable` implementations using `using` constraints.
- **Code generation cleanup**: Removed dead code from `compiler/code.go` and `compiler/compiler_impl.go`. Improved `compiler/generate.go` and control flow translation.
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
- **Jennifer upgrade to v1.7.1**: Upgraded `github.com/dave/jennifer` from v1.0.0 to v1.7.1 to get better `Custom()` API support. This required migrating all `.Index()` calls for type parameter rendering to use `Custom()` or `bracketArgs()` to avoid colon-separated output (`[A any:E any]` → `[A any, E any]`).
- **Bracket rendering via Custom**: Added `bracketArgs()` and `addTypeParams()` helpers that use `jen.Options{Open, Close, Separator}` with `Custom()` to render Go type parameter brackets correctly (e.g., `[A any, B any]`, `HKT[C, A]`). This replaces `Index()` for all type parameter lists and generic instantiation.
- **HKT interface generation fixed**: `genInterface` now uses `jenHKTTypeExpr` with HKT set detection, emitting `HKT[C, A]` instead of the old broken `C[A]` for higher-kinded type parameters in `Enumerable`-style interfaces.
- **Enum constructor generation**: `genEnum` now emits constructor functions (e.g., `func Some[A any](a0 A) Option[A]`) alongside the type definitions. Previously constructors were only in the now-deleted `zz_mygo.gen.go` files.
- **Type params on generated types**: `genEnum`, `genStruct`, `genInterface` now emit type parameters on the Go type definitions (e.g., `type Option[A any]`, `type List[A any]`).
- **`prelude_go.go` fixed**: Changed `Eq_equals(v, item)` → `eq.equals(v, item)` in `containsSlice` to match the dictionary-passing architecture. Added `containsMap` and `containsSet` helpers that were stubbed as `panic("unimplemented")`.
- **`prelude.mygo` indent fix**: Fixed extra leading whitespace on `resultMap` function declaration.

