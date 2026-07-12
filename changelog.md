# Changelog — Large Refactor: Type Inference, Go FFI Auto-Wrap, Prelude Uppercase APIs & LSP Extraction

## 1. Parser — Nesting & Correctness Fixes
- **Type annotation fix**: Separated `currentType` / `currentAnnotType` in parser state; `let`/`var` decls now correctly use the annotated type.
- **Struct literal nesting**: Added `currentStructBaseStack` / `currentStructFieldsStack` to support nested struct literals (e.g., `Foo{bar: Baz{x: 1}}`).
- **If-expr nesting**: Added `currentIfCondStack` for nested `if...then...end` expressions.
- **Switch/pattern nesting**: Added `currentSwitchTargetStack`, `currentSwitchCasesStack`, `currentPatternStack` for nested switch statements.
- **Empty map literal disambiguation**: When `currentSetElems` is empty, produces `MapLitExpr` instead of `SetLitExpr` (empty `{}` is now a map by default).
- **`LiteralPattern` AST node**: New pattern type for direct literal matching in switch cases (`case 0 then ...`, `case "hello" => ...`).

## 2. HM Type Inference — Forward References & Prelude Enum Awareness
- **Forward reference support**: Top-level function signatures are pre-registered in the type environment before processing bodies, enabling cross-function forward references.
- **Prelude enum constructors**: `Some`, `Ok`, `Err` are registered as proper function type constructors (not just opaque type names).
- **Interface method resolution**: When a name is not in env, searches package interfaces for matching methods.
- **`Ref[T].value` inference**: Field access `expr.value` on `Ref[T]` resolves to inner type `T`.
- **Struct field type resolution**: Concrete struct field access resolves to the declared field type; inferred field values are unified with declared types for type error detection.
- **Empty map lit in struct**: When `{}` initializes a `Map[K,V]` field, key/value types are filled from the field declaration.
- **Option/Result pattern bindings**: `switch x case Some(v) then ...` extracts the inner type for `v`.
- **Numeric type unification**: Numeric types (`Int`, `Int64`, `Float32`, etc.) are mutually compatible during unification.
- **`UnitLitExpr`**: The `()` expression returns `TUnit` type.

## 3. Go FFI Auto-Wrapping (`go[T]` blocks)
- **`*T` → `Option[Ref[T]]`**: If raw Go code returns `*T`, and the expected MyGO type is `Option[Ref[T]]`, auto-wraps: `nil` → `None[T]()`, non-nil → `Some[T](ref)`.
- **`T` → `Option[T]`**: If raw Go code returns `T` and expected is `Option[T]`, wraps in `Some[T]()`.
- **`(T, error)` → `Result[T, error]`**: If Go returns `(T, error)` tuple, destructs to `Ok[T,E](val)` / `Err[T,E](err)`.
- **Generic `Result[Ok, Err]` wrapping**: Any `Result[Ok, Err]` target wraps Go return in `Ok()`.
- **Tests**: `TestInlineGoOptionAutoWraps`, `TestInlineGoRefAutoWrapsAsOption`, `TestInlineGoResultAutoWraps`.

## 4. Compiler Translation Layer — Major Rework
### `translate_call.go`
- **`Ref[T].value` calls**: Short-circuits to the inner type.
- **`translatePreludeCall`**: First-class dispatch for prelude function calls.
- **Bogus fallback removed**: `jen.Id("unknown").Call()` replaced with hard error via `common.ErrorAtPos`.
- **`Ref.new(non-addressable)`**: Uses temp variable for CallExpr arguments.
- **`translateContainerMethod`**: Maps `Map.Set/Get/Delete`, `Slice.Len/Get/Set` to Go native operations.
- **Multi-return Go functions**: Proper return type string for functions returning multiple values.

### `translate_control.go`
- **`translateFunctionBlock`**: Direct block expression translation for function bodies — avoids unnecessary IIFE wrapping.
- **Tuple return**: Proper `return v1, v2` for multi-return functions.
- **`isTupleLikeTypeString`**: Recognizes `(type1, type2)` syntax in addition to `struct { ... }`.
- **If-expr branch incompatibility**: Gracefully handles incompatible branch types with `_ =` binding.
- **`LiteralPattern` switch**: `case literal then ...` support.
- **`translatePreludeVariantCase`**: Handles `Option.Some/None` and `Result.Ok/Err` patterns on prelude enum values (non-enum switches).
- **Type assertion fix**: `.Assert(...)` instead of `Op(".").Parens(...)` for correct Go syntax.
- **Unique switch var names**: `v_1`, `v_2` via `switchVarSeq` for nested switch scoping.
- **Consistent `jen.Return().Add(code)`**: Fixes spacing in generated Go code.

### `translate_expr.go`
- **`ensureRelationAllowed`**: Resolves `any`-typed identifiers to their source type; normalizes MyGO primitive names for comparison.
- **`translateEqRelation`**: Normalizes type before equality support check.
- **`translateSliceLit`**: Accepts `Slice[T]` expected type in addition to `[]T`.
- **`translateMapLit`**: Empty map literal support; `parseTypeName` for MyGO type string parsing.
- **`splitMyGoSliceExpected` / `splitMyGoMapExpected`**: Parse `Slice[...]` / `Map[..., ...]` from expected type strings.
- **`translateEmptyMapLit`**: Now handles `Map[K, V]` expected type in addition to `map[K]V`.

### `translate_struct.go`
- **Struct literal field accumulation**: Collects all field values first, then emits in order.
- **`TypeName{key: val}` syntax**: Uses `jen.Id(typeName).Values(parts)` instead of `jen.Lit(jen.DictFunc(...)).Index(...)`.
- **`substituteTypeExpr`**: New function for type parameter substitution in field types.

## 5. Compiler Type System & Code Generation
### `type_inference.go`
- **`lowerMyGoTypeString`**: Converts MyGO types (`Option`, `Ref`, `Slice`, `Map`, `Set`, `Int`, `String`, etc.) to Go types recursively.
- **`splitTopLevelArgs`**: Comma-separated type arg splitting that respects nesting.
- **`myGoPrimitiveCompatible`**: Cross-compatibility check between MyGO and Go primitive types.
- **`normalizeMyGoPrimitiveType`**: Normalizes type names (e.g., `"string"` ↔ `"String"`).
- **`goTypeCompatible`**: Now recognizes `Any` as universally compatible.
- **`goType` / `hktArgType`**: Properly handle `Option`, `Result` with prelude prefixing.

### `jennifer_gen.go`
- **`jenTypeExpr`**: Translates `Ref`→`*`, `Slice`→`[]`, `Map`→`map[...]`, `Set`→`map[...]struct{}`, `Any`→`any`.
- **`genJenIds`**: Handles pointer types (`*T` → `Op("*").Id("T")`).
- **`bracketArgs`**: Uses `Types()` instead of `Custom()` for correct generic type rendering.

### `compiler_impl.go`
- **Struct field names**: Don't `exportName()` struct fields — preserve original case.
- **Impl type params**: Uses `Id(fnName).Types(...)` instead of `Func().Custom(...)`.
- **Function body compilation**: Block expressions use `translateFunctionBlock` directly.
- **Error propagation**: `genFunc`/`genImpl` propagate translation errors instead of `panic("translate error")`.
- **`callAny` helper**: Fixes `return` syntax.

### `generate.go`
- **Prelude dot-import**: Auto-injects `import . "github.com/mygo-lang/mygo/lib/prelude"` and `var _ = None[any]()` for non-prelude packages.
- **Go import keep-alive**: `goImportKeepAlive` ensures unused Go imports aren't stripped by the compiler.
- **Render fixes**: Regex post-processing for `return func(` spacing, generic type bracket spacing.

### `go_package.go`
- **`goTypeToMyGoType`**: Converts Go types to MyGO types (e.g., `string` → `String`, `[]T` → `Slice[T]`, `*T` → `Ref[T]`).
- **`variadicElemMyGoType`**: Handles variadic parameter type conversion.

## 6. Typeclass System
- **`typeclassBindingForReceiver`**: Filters bindings by receiver type for precise method dispatch.
- **`translateTypeclassCall`**: Uses first arg as receiver (method-call style) instead of last arg.
- **`translateEnumConstructor`**: Uses `lowerMyGoTypeString` for type args.
- **Import alias resolution**: Import aliases are usable as identifiers.
- **Better error messages**: `None` type inference failure includes expected/retType context.
- **`translateIdent`**: Returns hard error for unknown identifiers instead of silent `jen.Id(name)`.

## 7. Prelude — Uppercase API & Interface Renames
- **Uppercase methods**: All interface methods capitalised (`show`→`Show`, `equals`→`Equals`, `each`→`Each`, `map`→`Map`, `filter`→`Filter`, etc.).
- **`Enumerable` → `IEnumerable`**: Renamed for naming consistency.
- **`IEnumerable` interface**: Added `Len(c) Int` method, removed HKT methods `map[B]`/`fold[B]`.
- **`typeKeyFromType` removed**: Unused utility function.
- **Generated prelude** (`zz_prelude.gen.go`): Updated to match new interface definitions.

## 8. LSP — Extraction & Cleanup
- **Separate binary**: LSP moved to `cmd/mygo-lsp/main.go` (independent binary).
- **`cmd/mygo/main.go`**: Removed `case "lsp"`, cleaned up imports — no LSP dependency.
- **`Range` field rename**: `range` → `textRange` (avoids Go keyword conflict).
- **Map method syntax**: `store.docs.set()` → `store.docs.Set()`.
- **`writeOneMessage`**: Return type `Result[(), error]`, simplified go[] block.
- **`wordAtPosition`**: Returns `Ref[Range]` instead of `Option[Ref[Range]]`.
- **`readOneMessage`**: Simplified go[] block, removed unused variables.
- **`parseDocumentURI`**: Pure MyGO function instead of go[] block.
- **`handleCompletion`**: Field access via go[] block.

## 9. Tests
- **`TestLoadPreludeDoesNotDuplicatePreludeDecls`**: New test ensuring no decl duplication.
- **`TestParseFileSupportsLiteralSwitchPatterns`**: New parser test for literal patterns.
- **`TestParseFileKeepsNestedSwitchCasesSeparate`**: New parser test for nested switches.
- **`TestParseFileBasic`**: Updated for `{}` → `MapLitExpr` change.
- **`TestCompileDirSupportsStringSwitchOnStructField`**: New integration test.
- **`TestCompileDirSupportsSwitchOnLetBoundStructField`**: New integration test.
- **`TestCompileDirSupportsNestedStringAndOptionSwitches`**: New integration test.
- **`TestCompilePrelude`**: Updated `Enumerable` → `IEnumerable`, `optionFlatMap` → `OptionFlatMap`.
- **Go FFI auto-wrap tests**: Three new tests for `go[T]` auto-wrapping.

## 10. Misc
- **`api.go`**: Removed prelude decl injection from `loadPackage` (now handled via dot-import in generated code).
- **`types.go`**: Added `TargetType` to `typeclassBinding`, `switchVarSeq` to `generator`.
- **`examples/data-structure.mygo`**: Removed stray blank line.
