# compiler2 (bootstrap pipeline)

> See [differences.md](differences.md) for current differences from
> `docs/spec.md`, known gaps, and reproduction commands. `docs/spec.md` is
> authoritative for language behavior; bootstrap behavior must not expand it.

`compiler2` is the self-hosted MyGO compiler pipeline. It is implemented in
MyGO itself and uses the MyGO parser-combinator library under
`lib/text/parsec` for parsing.

The bootstrapping path is:

```text
parser2 -> ast2 -> typeinference2 -> codegen2 -> .gen.go
```

It is deliberately separate from the legacy compiler path:

| Legacy compiler | Bootstrap compiler | Notes |
| --- | --- | --- |
| `internal/mygo/parser` (yacc + Go) | `internal/mygo/parser2` (parsec + MyGO) | parser2 is the self-host parser |
| `internal/mygo/ast` (Go types) | `internal/mygo/ast2` (MyGO types) | ast2 is the compact AST shared by the bootstrap pipeline |
| `internal/mygo/typeinference` (Go) | `internal/mygo/typeinference2` (MyGO) | HM/Scheme-based inference over ast2 |
| `internal/mygo/codegen` (Go) | `internal/mygo/codegen2` (MyGO) | AST-to-Go lowering using `internal/mygo/codegen2/goast` |

The bootstrap pipeline is the CLI default for sync and build. `--bootstrap`
remains a compatibility alias. Legacy generation requires `--legacy sync`;
`--legacy` is rejected for build and fmt and cannot be combined with either
bootstrap flag. Compilation errors do not trigger an automatic legacy fallback.
The Go APIs retain their existing pipeline semantics: `Sync` and `CompileDir`
use legacy, while the explicitly named bootstrap APIs use bootstrap.

## Commands

Compile a MyGO package with the self-hosted bootstrap compiler:

```bash
go run ./cmd/mygo sync <package path>
```

Compile with per-stage timing:

```bash
go run ./cmd/mygo --bootstrap-timing sync <package path>
```

Compile and then run `go build` on the generated files:

```bash
go run ./cmd/mygo build <package path>
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

### Go FFI argument assignability

At a Go FFI call boundary, bootstrap inference first uses normal MyGO type
unification. If that fails for named imported Go types, it asks Go's `go/types`
assignability service whether the argument is valid for the parameter. This
preserves Go's rules for interface satisfaction, aliases, and named types.
MyGO `Ref[T]` represents Go `*T`, so its pointer method set is used when
checking whether it implements an interface parameter. The generated call
passes the original expression directly. This compatibility check is limited
to Go FFI calls; it does not change general MyGO unification.

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
