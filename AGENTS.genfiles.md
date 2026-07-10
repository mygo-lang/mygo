# AGENTS.genfiles.md — Code generations

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
