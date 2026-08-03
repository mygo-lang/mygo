# compiler2 (bootstrap pipeline)

`compiler2` is the self-hosted MyGO compiler pipeline. It is implemented in
MyGO itself and uses the MyGO parser-combinator library under
`lib/text/parsec` for parsing.

The bootstrapping path is:

```text
parser2 -> ast2 -> typeinference2 -> codegen2 -> .gen.go
```

It is deliberately separate from the production compiler path:

| Production compiler | Bootstrap compiler | Notes |
| --- | --- | --- |
| `internal/mygo/parser` (yacc + Go) | `internal/mygo/parser2` (parsec + MyGO) | parser2 is the self-host parser |
| `internal/mygo/ast` (Go types) | `internal/mygo/ast2` (MyGO types) | ast2 is the compact AST shared by the bootstrap pipeline |
| `internal/mygo/typeinference` (Go) | `internal/mygo/typeinference2` (MyGO) | HM/Scheme-based inference over ast2 |
| `internal/mygo/codegen` (Go) | `internal/mygo/codegen2` (MyGO) | AST-to-Go lowering using `internal/mygo/codegen2/goast` |

The production pipeline remains the default. The bootstrap pipeline is opt-in
through the `--bootstrap` flag.

## Commands

Compile a MyGO package with the self-hosted bootstrap compiler:

```bash
go run ./cmd/mygo --bootstrap sync <package path>
```

Compile with per-stage timing:

```bash
go run ./cmd/mygo --bootstrap-timing sync <package path>
```

Compile and then run `go build` on the generated files:

```bash
go run ./cmd/mygo --bootstrap build <package path>
```

Compile without the automatic MyGO prelude:

```bash
go run ./cmd/mygo --no-prelude sync <package path>
```

The corresponding Go API is exposed by `internal/mygo/compiler`:

- `CompileDirBootstrap`
- `CompileDirBootstrapWithTiming`
- `SyncBootstrap`
- `SyncBootstrapWithTiming`

For example:

```go
written, err := compiler.SyncBootstrap("examples/main")
```

## Module Layout

### `parser2`

`internal/mygo/parser2/parser.mygo` is a recursive-descent parser written on
top of `lib/text/parsec`. It owns source locations, declaration parsing, and
expression/statement parsing for the self-hosted format.

Supported syntax includes:

- package declarations, imports, package-level `let`/`var`
- function and `impl` declarations
- struct, enum, interface, type alias, and nominal type declarations
- `let`, `var`, `letrec`, tuple destructuring, assignment, `while`, `return`,
  `break`, and `continue`
- `if`/`else`, `switch`/`case` patterns, function literals
- struct literals, generic struct literals, slice/map/set literals
- inline Go embedding via `go[T] { ... }`
- `using` constraints

Entry points:

- `ParseFile`
- `ParseFileAt`

`ParseFileAt` retains a source name in diagnostics.

### `ast2`

`internal/mygo/ast2` defines the compact AST used by the bootstrap pipeline.
It uses value-typed enums instead of Go interfaces:

- `Decl`, `Stmt`, `ExprKind`, `TypeExpr`, `Pattern`
- `MonoType` for monomorphic type representations
- `SourcePos` and expression IDs for diagnostics and inference lookup

The AST is intentionally smaller and more direct than the production
`internal/mygo/ast` package.

### `typeinference2`

`internal/mygo/typeinference2` is the self-hosted inference engine. It does a
HM/Scheme-style pass over `ast2` declarations.

Important behavior:

- `predeclareFunctions` and symbol tables let forward references resolve.
- `InferFile`, `InferPackage`, `InferPackageWithGoPackages`, and
  `InferPackageWithExternal` are the main entry points.
- The package receives both Go FFI signatures and MyGO package signatures
  through `GoPackageEntry` and `MyGoPackageInfo`.
- The `Env` implementation uses persistent layers and batch bindings so
  imported packages do not require repeatedly copying the whole environment.
- Typeclass constraints collect into `PackageInfo` for `codegen2`.
- The result is a `PackageInfo` containing typed declarations, external
  declarations, inferred field/type information, and resolved constraints.

### `codegen2`

`internal/mygo/codegen2` lowers `ast2` plus `typeinference2.PackageInfo` to Go
source strings.

- `GenerateFiles` merges all source files so cross-file method dispatch works.
- `GenerateSource` and `GenerateSourceAt` are convenient string-to-string
  entry points for tests and tooling.
- Go output is rendered through `internal/mygo/codegen2/goast`, which creates
  `go/ast` nodes rather than concatenating source text.
- `decls.mygo` emits declarations, `translate_ast.mygo` lowers statements and
  method bodies, and `tailcall.mygo` implements tail-call trampoline lowering.
- The prelude is represented externally by `ExternalTypedDecls`, and
  non-prelude packages automatically dot-import the generated prelude when
  needed.

## Bootstrap Orchestration

The actual package build driver is `internal/mygo/compiler/bootstrap.mygo`.
It coordinates:

1. Reading and parsing `.mygo` files with `parser2`.
2. Building `codegen2.SourceFileInput` and `typeinference2.PkgDeclSource`
   structures with matching source paths.
3. Resolving the built-in prelude and MyGO imports.
4. Loading Go FFI signatures for `go:` imports.
5. Running `typeinference2.InferPackageWithExternal`.
6. Generating one `.gen.go` file per MyGO source file with `codegen2.GenerateFiles`.
7. Writing generated files next to the source files.

The state machine also:

- caches compiled and in-progress packages,
- detects import cycles,
- skips code generation while walking a dependency graph,
- supports `--bootstrap-timing` output.

Imports come in two forms:

```mygo
import fmt "go:fmt"
import ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"
```

`go:` imports are Go FFI imports. Other imports are MyGO package imports and
are compiled before type inference of the importer.

## Generated Go Files

Like the production compiler, `compiler2` emits one generated file per MyGO
source file:

```text
main.mygo          -> zz_main.gen.go
foo_test.mygo      -> zz_foo_test.gen.go
```

Generated files are marked with:

```go
// Code generated by mygo; DO NOT EDIT.

package main
```

The generated Go for prelude code and self-hosted MyGO packages is checked in
alongside the source as `.gen.go` files.

## Testing

Important test entry points:

```bash
go test ./internal/mygo/parser2/...
go test ./internal/mygo/typeinference2/...
go test ./internal/mygo/codegen2/...
go test ./internal/mygo/compiler/... -run Bootstrap
```

The bootstrap compiler is exercised end-to-end by tests in
`internal/mygo/compiler/bootstrap_test.go`:

- parser2 compiles the self-hosted parser sources
- basic source-to-Go generation
- root-package `sync`
- prelude HKT declarations
- Go FFI functions and `testing.T`-style method calls
- option/error boundary conversion
- MyGO package imports

Rerun generated files after changing MyGO sources:

```bash
go run ./cmd/mygo --bootstrap sync internal/mygo/parser2
go run ./cmd/mygo --bootstrap sync internal/mygo/ast2
go run ./cmd/mygo --bootstrap sync internal/mygo/typeinference2
go run ./cmd/mygo --bootstrap sync internal/mygo/codegen2
```

Keep the checked-in generated files in sync when the corresponding MyGO
sources change.
