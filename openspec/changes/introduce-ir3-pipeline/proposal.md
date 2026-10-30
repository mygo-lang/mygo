## Why

The bootstrap pipeline currently lowers typed `ast2` directly to Go source. That couples value representation, pattern matching, dictionary passing, and tail-call lowering inside `codegen2`, which forces interface boxing for ADTs, type-assertion-based matching, duplicated tail-position logic, and prevents representation-level optimization.

The project can now accept generated-Go ABI changes and treats `.gen.go` files as build artifacts. This removes the main blocker for a real middle end: a package-level IR with explicit representation and effect information, followed by a dedicated optimizer and Go backend modeled on OCaml's Lambda, Closure, and Flambda 2 layers but adapted to Go's structured control flow.

## What Changes

- Add `ir3`, a package-level structured-ANF IR with `let`/`letrec`/`function`/`apply`/`if`/`switch`, join continuations (`block`/`jump`), static match failure (`raise`), explicit call kinds, tail markers, effect/coeffect annotations, source positions, and canonical value representations.
- Add `optimizer3`, a fixpoint pass pipeline covering pattern-match decision trees, tail-call and SCC lowering, simplification/DCE/CSE, local unboxing, closure conversion, and intra-package inlining/specialization. Cross-package inlining is explicitly out of scope.
- Add `codegen3`, an IR-to-Go backend that preserves per-file output, line directives, imports, FFI/dynamic-typeclass adapters, and Go build correctness without emitting a metadata artifact. Generated Go is written to the compiler's content-addressed user cache and consumed through a temporary overlay; it is never written beside the source files.
- **BREAKING** Adopt canonical value representations for generated Go: `Option[A]` and `Result[A, E]` become value structs, `Option[Ref[T]]` maps to `*T`, `List[A]` becomes a pointer node, enums become tagged structs with recursion broken by pointers, and generic parameters remain real Go type parameters instead of collapsing to `any`.
- Add an opt-in `--codegen3` pipeline switch plus IR dump/verification tooling; keep `codegen2` as the differential baseline until behavioral parity is established.
- Add a `test` subcommand (`mygo test`, and the codegen3 form `mygo --codegen3 test`) that mirrors `go test` argument handling and package selection, generates test-only and external `_test` packages into the build cache, and runs the Go toolchain directly against the generated overlay. Test failures propagate the `go test` exit status, and no test wrapper is written into the repository.
- Add a content-addressed, user-level codegen3 build cache modeled on `GOCACHE`. The cache root is selected by `MYGO_CACHE` (defaulting to the OS user cache directory), entries are keyed by compiler identity, representation and adapter versions, optimization level, normalized source and dependency digests, and the Go module/toolchain inputs used for FFI. Entries are immutable, verified, atomically published, safe for concurrent builds, reusable offline, and shared across different Go modules because the key never contains an absolute module root.
- Add cache lifecycle commands: `mygo cache clean` (with `mygo clean -cache` accepted as the Go-compatible spelling) removes the codegen3 build entries and temporary overlays under the active cache root. Cleaning takes an exclusive cache lock, so it never deletes data under an in-flight build; a cold build after cleaning reproduces the same output.
- Make `codegen3` emit no metadata artifact. Cross-package declarations are derived from source in the same build, and representation compatibility is guaranteed by a single compiler version rather than a generated metadata file.
- Enable repository-clean builds: when `codegen3` is enabled the compiler SHALL NOT write `zz_*.gen.go` or `zz_*.gen_test.go` into the repository and SHALL NOT create a project-local generated directory such as `build/.mygo3`. Generated Go lives only in the user-level content-addressed cache; a per-build overlay inside the cache maps logical source paths to those entries and is removed when the build ends. The compiler invokes `go build` / `go test` directly, so binaries and test results are written to the requested `-o` path or to Go's default location. `go.mod`/`go.sum` remain in the project, and dependencies are downloaded through `GOPROXY`.

## Capabilities

### New Capabilities

- `bootstrap-ir3-pipeline`: Package-level IR construction, pass contracts, effect annotations, tail-position semantics, source attribution, and IR verification.
- `bootstrap-value-representation`: Canonical generated-Go layouts for algebraic data types, `Option`, `Result`, `List`, enums, generics, and their FFI/dynamic-boundary adaptations.
- `bootstrap-codegen3`: Behavioral equivalence, per-file emission, line directives, imports, source-derived dependency information, pattern-match lowering, tail-call lowering, and FFI handling for the IR-based Go backend.

### Modified Capabilities

None. The change preserves language-level semantics; existing specs such as `bootstrap-codegen-correctness`, `bootstrap-ffi-boundary`, and `switch-exhaustiveness` continue to apply as acceptance constraints.

## Impact

- New compiler packages: `internal/mygo/ir3/`, `internal/mygo/optimizer3/`, `internal/mygo/codegen3/`.
- Modified pipeline orchestration in `internal/mygo/compiler/bootstrap.mygo` and CLI/API surfaces in `cmd/mygo` and `internal/mygo/compiler`.
- Generated Go ABI changes for all MyGO packages; dependents must be recompiled with the same compiler version.
- Prelude, dynamic-typeclass adapters, and FFI boundary code must be regenerated and revalidated.
- Inline Go snippets (`go[T] { ... }`) that assume the old interface/type-assertion shapes require an audit.
- Cross-package inference and dynamic dispatch must be source-derived; precompiled metadata artifacts are not part of the `codegen3` model.
- Existing `codegen2` remains available for differential tests and rollback until `codegen3` reaches parity.
- Build outputs move out of the source tree: generated Go files and test wrappers are produced in the compiler build cache, while `go.mod`/`go.sum` and dependency resolution through `GOPROXY` stay unchanged.
- The codegen3 cache becomes a shared machine-level artifact: `MYGO_CACHE` selects its location, `mygo cache clean` manages its lifetime, and the repository no longer contains a persistent `build/.mygo3` directory. `--codegen3-emit=<dir>` remains the explicit opt-in debug path for inspecting generated files.
