# genfiles.md — Code generations

## Per-Source-File Code Generation

### Overview
The compiler now generates one `.gen.go` file per `.mygo` source file instead of one `zz_mygo.gen.go` per package. Prelude declarations go into `zz_prelude.gen.go`. This supports `_test.mygo` → Go `_test.go` mapping naturally.

### API Changes
- `CompileDir(dir string) (string, error)` → `CompileDir(dir string) ([]string, error)` — returns list of generated file paths.
- `Generate() (string, error)` → `GenerateFiles() (map[string]string, error)` — returns `filename → Go source`.
- `Generate()` kept for backward compat, calls `GenerateFiles()` and concatenates results.

### AST Changes
- `Decl` types (`ImportDecl`, `EnumDecl`, `StructDecl`, `InterfaceDecl`, `ImplDecl`, `FuncDecl`, `LetStmt`) now have `SourceFile string` tracking the source `.mygo` file.
- `Package.Files map[string][]Decl` groups declarations by source filename.
- Prelude-injected declarations have `SourceFile == ""`.

### Naming Convention
- `main.mygo` → `zz_main.gen.go`
- `example_test.mygo` → `zz_example_test.gen.go` (Go auto-recognizes `_test.go`)
- `prelude.mygo` (when loaded directly, not via injection) → `zz_prelude.gen.go`

### HKT Type Generation
- `genHKTType(file *jen.File)` generates `HKTType`, `HKT1[F any]`, `HKT2[A any]`, and `HKT[F any, A any]` types when the package has interfaces using higher-kinded types (e.g. `Enumerable[C, A]`).
- Returns void and writes directly to file to avoid rendering issues with `jen.Code` splat.

## Multi-Line Brace Literal NEWLINE Support

### Problem
`{ "key": value }` and `Struct { field: value }` parsed correctly on a single line but failed with multiline syntax because `NEWLINE` tokens disrupted the `COLON expr` and `COMMA entry` yacc rules.

### Solution
Added `braceDepth` counter in lexer (`parser_lexer.go`):
- `{` → `braceDepth++`, `}` → `braceDepth--`
- When `braceDepth > 0`, `NEWLINE` tokens are skipped in `nextToken()` (recursively read next token instead)
- This makes all NEWLINEs inside braces invisible to the yacc state machine

### Notes
- Only affects NEWLINEs inside LBRACE...RBRACE blocks; other NEWLINEs (statements, let/var) work normally.
- Trailing commas (e.g. `{ "a": 1, \n }`) are still not supported due to original yacc grammar.
- Added `//go:build ignore` to `parser.y` to prevent go 1.26 from attempting to compile `.y` files as Go source.

## MyGo-to-Go Type Normalization (`compiler/type_inference.go`)

### `normalizeMygoTypeToGo(s string) string`

Normalizes MyGo type names to their Go equivalents for type compatibility checking in `goTypeCompatible()`. This is necessary because collection types and primitives have different names in MyGo vs Go.

| MyGo Type | Go Type | Rule |
|-----------|---------|------|
| `Ref[T]` | `*T` | Pointer dereference |
| `Slice[T]` | `[]T` | Slice |
| `Set[T]` | `map[T]struct{}` | Set as map |
| `Map[K, V]` | `map[K]V` | Map |
| `String` | `string` | Primitive |
| `Bool` | `bool` | Primitive |
| `Int`/`Int8`/.../`UInt64` | `int`/`int8`/.../`uint64` | Numeric |
| `Float32`/`Float64` | `float32`/`float64` | Float |
| `Any` | `any` | Any type |

The function handles nesting (e.g., `Slice[Ref[String]]` → `[]*string`) and Map's comma-separated type parameters with bracket-depth tracking.

### Integration

`normalizeMygoTypeToGo` is called at the start of `goTypeCompatible()` before the existing `normalizeMyGoPrimitiveType` normalization, ensuring that collection type parameters and return types are compared in Go-native form during code generation type checks.

### Type Compatibility Extensions

Two additional type compatibility helpers in `goTypeCompatible()`:

- **`isStringRuneSequenceType()`**: Allows MyGo `C[rune]` / `C[int32]` to unify with Go `string`, enabling HM inference of `String` as a sequence of runes. This is used by `PeekRune` and similar String operations.
- **`isRuneGoAliasPair()`**: Recognizes `rune` and `int32` as the same type (Go's `rune` is an alias for `int32`), preventing spurious type mismatches.

### `goSimpleCallRE` / `goSliceFromRE` / `goSliceToLenEqRE`

In `translate_go.go`, three regex patterns parse simple Go expressions for better `jen.Code` generation:
- `goSimpleCallRE`: `^([A-Za-z_])\(([A-Za-z_])\)$` — matches `fn(arg)` calls
- `goSliceFromRE`: `^([A-Za-z_])\[([A-Za-z_]):\]$` — matches `arr[x:]` slice operations
- `goSliceToLenEqRE`: `^([A-Za-z_])\[:len\(([A-Za-z_])\)\]\s*==\s*([A-Za-z_])$` — matches `arr[:len(arr)] == x` patterns
