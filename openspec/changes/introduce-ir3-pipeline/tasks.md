## 1. Baseline and contracts

- [x] 1.1 Add an allocation, stack-depth, and compile-time benchmark harness for representative Option/List/dictionary/tail-recursion workloads and record baseline results; verify the harness runs locally and the baseline numbers are checked in.
- [x] 1.2 Freeze the canonical representation matrix for Option, Result, List, Ref, enums, recursion, generics, and HKT in a checked-in design note; verify every type category in `design.md` has a concrete layout and recursion rule.
- [x] 1.3 Audit every `go[T] { ... }`, FFI signature, and dynamic-typeclass adapter for assumptions about the previous interface-based ADT shapes; verify the audit lists each occurrence and classifies it as compatible, needs-adaptation, or unsupported.
- [x] 1.4 Define a compiler-level representation version and include it in build/cache keys without emitting a metadata file; verify repeated builds reuse only matching compiler versions and no metadata artifact is required.
- [x] 1.5 Add CLI/API scaffolding for `--codegen3`, optimization level selection, and IR dump/verification output without changing default behavior; verify CLI parsing tests and a no-op package build.
- [x] 1.6 Add a determinism check that compiles a package twice and compares generated files and IR dumps byte-for-byte; verify it fails on a deliberately unordered traversal.

## 2. IR core (`ir3`)

- [x] 2.1 Define the IR data model for declarations, expressions, call kinds, continuations, effects, representations, and source positions under `internal/mygo/ir3/`; verify the package builds under the bootstrap pipeline.
- [x] 2.2 Add a textual IR printer with stable formatting; verify golden IR snapshots for declarations, expressions, calls, and continuations.
- [x] 2.3 Add an IR verifier for control-flow scope, type presence, representation consistency, and effect annotations; verify invalid jump and representation-mismatch tests are rejected before code generation.
- [x] 2.4 Implement lowering for declarations, literals, bindings, assignments, direct/closure/method calls, `if`, and `while`; verify IR golden tests for each construct.
- [x] 2.5 Implement pattern-match lowering to decision trees with `block`/`jump`/`raise`, including guard fallback and merged identical patterns; verify guard-failure and nested-variant IR tests.
- [x] 2.6 Lower dictionary calls, typeclass dispatch, dynamic boxing, and Go FFI/Inline Go operations into explicit IR kinds; verify IR tests for static and dynamic dispatch plus FFI.
- [x] 2.7 Preserve per-declaration source paths and positions and group IR declarations by originating file; verify a cross-file call retains its caller's provenance.
- [x] 2.8 Introduce deterministic traversal helpers for maps, worklists, and SCCs; verify repeated IR dumps and generated output are identical.

## 3. Canonical value representation

- [x] 3.1 Implement representation computation for named types and detect direct and mutual recursion; verify recursive enums use indirection and non-recursive enums use tagged values.
- [x] 3.2 Map `Option`, `Result`, `List`, `Ref`, `Slice`, `Map`, `Set`, arrays, and first-order generic parameters to canonical Go layouts; verify generated type-string tests for each category.
- [x] 3.3 Implement FFI, dynamic-typeclass, and inline-Go boundary adapters for canonical representations; verify `Result[T, error]`, `Option[Ref[T]]`, and dynamic-dispatch round-trip tests.
- [x] 3.4 Emit a diagnostic when an inline Go snippet or FFI signature cannot be adapted to the canonical representation; verify the incompatible-shape test fails with a located diagnostic.
- [x] 3.5 Ensure representation computation depends only on type declarations and the compiler version, with no metadata input; verify packages compiled in one build agree on layouts and mixed compiler versions are rejected.

## 4. Go backend (`codegen3`)

- [x] 4.1 Implement IR-to-Go declaration and expression emission using canonical layouts; verify backend unit tests for declarations, literals, calls, and field access.
- [x] 4.2 Implement per-file declaration grouping, exact import inference, source line directives, and stable formatting; verify generated-file golden tests and source-provenance assertions.
- [x] 4.3 Implement structured control-flow reconstruction for reducible joins and a label/`goto` fallback with hoisted locals for guard fallback and complex joins; verify guarded-switch and nested-join output compiles.
- [x] 4.4 Implement decision-tree pattern-match emission with single subject evaluation and guard fallback; verify nested variant/struct pattern tests.
- [x] 4.5 Implement tail-call SCC lowering to loops/state machines while preserving exported symbols and signatures; verify large mutual-recursion stack tests and symbol-presence tests.
- [x] 4.6 Ensure codegen3 emits no metadata file and cross-package inference/dynamic dispatch use source declarations; verify the codegen3 output contains no metadata artifact and dependency builds succeed from source.
- [x] 4.7 Wire `codegen3` into the bootstrap pipeline and CLI/API selection points without mixing backends within a package; generated Go is written to a build output directory (never beside `.mygo` sources; the initial repository-local materialization is superseded by the shared cache in 4.11) and the compiler invokes `go build`/`go test` directly, keeping `go.mod`/`go.sum` and resolving dependencies through `GOPROXY`; verify explicit selection, rollback, and repository-clean output.
- [x] 4.8 Add a differential harness that compiles the same package with `codegen2` and `codegen3` and compares observable behavior; verify the existing package test suite passes through both backends.
- [x] 4.9 Add a repository-clean build test: build a representative package with `codegen3` from a clean checkout and verify the binary/test result succeeds while no `zz_*.gen.go` or `zz_*.gen_test.go` is created under the repository; verify `go.mod`/`go.sum` remain unchanged and a missing dependency can be fetched through `GOPROXY`.
- [x] 4.10 Add `test` to the CLI (`mygo test`, `mygo --codegen3 test`) with the same root and package-argument handling as `build`, `go list -deps -test` selection, external `_test` package generation, `go test` flag passthrough, and exit-status propagation; verify a passing package, a failing test, and repository-clean output.
- [x] 4.11 Replace the project-local `build/.mygo3` materialization with a user-level content-addressed cache: `MYGO_CACHE`, the documented key composition, immutable entries, manifest verification, atomic publication, shared/exclusive locking, a temporary overlay, and a canonical-package set derived from the build manifest. Verify cross-module hits, cold/warm determinism, corrupt-entry recovery, and `MYGO_CACHE=off`.
- [x] 4.12 Add `--codegen3-emit=<dir>` as the explicit debug/IDE materialization path; verify it copies the selected entries without changing what an ordinary build or test consumes.
- [x] 4.13 Add `mygo cache clean` and `mygo clean -cache` with exclusive locking, bounded waiting, summary output, and a successful no-op under `MYGO_CACHE=off`; verify cleaning during a concurrent build deletes nothing.

## 5. Optimizer (`optimizer3`)

- [x] 5.1 Implement the pass driver, fixpoint control, optimization levels, and pass selection; verify O0 correctness and deterministic pass ordering tests.
- [x] 5.2 Implement `Match3` decision-tree compaction and redundant-test elimination; verify IR snapshots and guard-fallback behavior.
- [x] 5.3 Implement `Simplify3` constant folding, copy propagation, DCE, CSE, and effect-aware hoisting; verify FFI calls are never removed or reordered and pure unused values are eliminated.
- [x] 5.4 Implement `Tailcall3` statically known tail-call graph construction and SCC lowering; verify stack-depth tests for self and mutual recursion.
- [x] 5.5 Implement `Unbox3` local Option/Result multi-return conversion and non-escaping `Ref.new` elimination; verify allocation benchmarks and behavioral equivalence.
- [x] 5.6 Implement `Closure3` free-variable analysis, lambda lifting, and immediate-call beta reduction; verify closure capture and mutation tests.
- [x] 5.7 Implement `Inline3` intra-package inlining with recursion and code-size budgets; verify inlining tests and a compile-time budget regression check.
- [x] 5.8 Implement `Specialize3` intra-package dictionary and first-order generic specialization; verify specialized call sites remove known dictionary parameters and preserve behavior.
- [x] 5.9 Add cross-pass determinism tests that run the full optimizer twice and compare IR dumps; verify map-backed passes produce identical output.

## 6. Self-hosting and rollout

- [x] 6.1 Compile `ir3`, `optimizer3`, and `codegen3` with `codegen2`; verify the packages build and their unit tests pass.
- [ ] 6.2 Compile `ir3`, `optimizer3`, and `codegen3` with `codegen3` and run the resulting compiler; verify its output matches the `codegen2`-built compiler on the test suite.
- [ ] 6.3 Run the full existing MyGO test suite through the `codegen3` pipeline; verify behavioral parity with the established backend.
- [ ] 6.4 Measure allocation, stack-depth, and compile-time results against the Phase 1 baseline and record them in a checked-in report; verify each optimization has a measured effect or is documented as deferred.
- [ ] 6.5 Make `codegen3` the default after parity and keep an explicit rollback to `codegen2`; verify default builds and rollback builds both succeed, and that the default build writes generated Go only to the shared codegen3 cache and leaves the source tree clean.
- [ ] 6.6 Add a self-host CI job that compiles the compiler with `codegen3` from a clean checkout (dependencies via `GOPROXY`) and runs the compiler test suite; verify the job fails on self-host regressions and fails if any `.gen.go`/`.gen_test.go` is written into the repository.
- [x] 6.7 Run `mygo --codegen3 test ./...` from two module fixtures that share a cold `MYGO_CACHE`; verify the first run is cold, the second reuses entries, both runs pass, and neither tree contains generated files or a `build/.mygo3` directory.

## 7. Documentation and validation

- [ ] 7.1 Document the IR contract, effect rules, representation matrix, optimizer passes, and codegen3 control-flow mapping under `docs/compiler3/`; verify the documents cover every spec requirement.
- [ ] 7.2 Update `docs/compiler2/core.md`, `docs/compiler/collections.md`, and `docs/compiler/mutual-tailcall.md` with the new representation and tail-call behavior; verify stale references to the interface representation are removed.
- [ ] 7.3 Update `KNOWN_ISSUES.md` and `HISTORY.md` with the deferred items (large-payload threshold, unknown-closure tail calls); verify each deferred item has a follow-up note and that the repository-clean build contract is documented as in scope, not deferred.
- [x] 7.4 Run `openspec validate introduce-ir3-pipeline --strict` and fix all reported issues; verify the command exits successfully.
- [ ] 7.5 Run `go test ./...` and the MyGO self-host test suite with both backends; verify all tests pass, no generated files are required to be checked in, and enabling `codegen3` writes no `.gen.go`/`.gen_test.go` into the source tree.
- [x] 7.6 Document the `test` command, the cache root and `MYGO_CACHE`, key composition, entry layout, temporary overlay lifetime, cross-module sharing, locking, `cache clean`, and `--codegen3-emit`; verify the docs state that deleting the cache only costs time.
