## 1. Baseline and contracts

- [ ] 1.1 Add an allocation, stack-depth, and compile-time benchmark harness for representative Option/List/dictionary/tail-recursion workloads and record baseline results; verify the harness runs locally and the baseline numbers are checked in.
- [ ] 1.2 Freeze the canonical representation matrix for Option, Result, List, Ref, enums, recursion, generics, and HKT in a checked-in design note; verify every type category in `design.md` has a concrete layout and recursion rule.
- [ ] 1.3 Audit every `go[T] { ... }`, FFI signature, and dynamic-typeclass adapter for assumptions about the previous interface-based ADT shapes; verify the audit lists each occurrence and classifies it as compatible, needs-adaptation, or unsupported.
- [ ] 1.4 Add representation-version and layout-identity fields to generated metadata and include them in cache keys; verify metadata parsing and cache-invalidation tests cover matching and mismatched versions.
- [ ] 1.5 Add CLI/API scaffolding for `--codegen3`, optimization level selection, and IR dump/verification output without changing default behavior; verify CLI parsing tests and a no-op package build.
- [ ] 1.6 Add a determinism check that compiles a package twice and compares generated files and IR dumps byte-for-byte; verify it fails on a deliberately unordered traversal.

## 2. IR core (`ir3`)

- [ ] 2.1 Define the IR data model for declarations, expressions, call kinds, continuations, effects, representations, and source positions under `internal/mygo/ir3/`; verify the package builds under the bootstrap pipeline.
- [ ] 2.2 Add a textual IR printer with stable formatting; verify golden IR snapshots for declarations, expressions, calls, and continuations.
- [ ] 2.3 Add an IR verifier for control-flow scope, type presence, representation consistency, and effect annotations; verify invalid jump and representation-mismatch tests are rejected before code generation.
- [ ] 2.4 Implement lowering for declarations, literals, bindings, assignments, direct/closure/method calls, `if`, and `while`; verify IR golden tests for each construct.
- [ ] 2.5 Implement pattern-match lowering to decision trees with `block`/`jump`/`raise`, including guard fallback and merged identical patterns; verify guard-failure and nested-variant IR tests.
- [ ] 2.6 Lower dictionary calls, typeclass dispatch, dynamic boxing, and Go FFI/Inline Go operations into explicit IR kinds; verify IR tests for static and dynamic dispatch plus FFI.
- [ ] 2.7 Preserve per-declaration source paths and positions and group IR declarations by originating file; verify a cross-file call retains its caller's provenance.
- [ ] 2.8 Introduce deterministic traversal helpers for maps, worklists, and SCCs; verify repeated IR dumps and generated output are identical.

## 3. Canonical value representation

- [ ] 3.1 Implement representation computation for named types and detect direct and mutual recursion; verify recursive enums use indirection and non-recursive enums use tagged values.
- [ ] 3.2 Map `Option`, `Result`, `List`, `Ref`, `Slice`, `Map`, `Set`, arrays, and first-order generic parameters to canonical Go layouts; verify generated type-string tests for each category.
- [ ] 3.3 Implement FFI, dynamic-typeclass, and inline-Go boundary adapters for canonical representations; verify `Result[T, error]`, `Option[Ref[T]]`, and dynamic-dispatch round-trip tests.
- [ ] 3.4 Emit a diagnostic when an inline Go snippet or FFI signature cannot be adapted to the canonical representation; verify the incompatible-shape test fails with a located diagnostic.
- [ ] 3.5 Emit representation version and layout identity into package metadata and validate imported metadata before use; verify mismatched-version and matching-version tests.

## 4. Go backend (`codegen3`)

- [ ] 4.1 Implement IR-to-Go declaration and expression emission using canonical layouts; verify backend unit tests for declarations, literals, calls, and field access.
- [ ] 4.2 Implement per-file declaration grouping, exact import inference, source line directives, and stable formatting; verify generated-file golden tests and source-provenance assertions.
- [ ] 4.3 Implement structured control-flow reconstruction for reducible joins and a label/`goto` fallback with hoisted locals for guard fallback and complex joins; verify guarded-switch and nested-join output compiles.
- [ ] 4.4 Implement decision-tree pattern-match emission with single subject evaluation and guard fallback; verify nested variant/struct pattern tests.
- [ ] 4.5 Implement tail-call SCC lowering to loops/state machines while preserving exported symbols and signatures; verify large mutual-recursion stack tests and symbol-presence tests.
- [ ] 4.6 Emit metadata with exports, implementations, dynamic adapters, generated-source digests, compiler ABI, and representation version; verify digest and determinism tests.
- [ ] 4.7 Wire `codegen3` into the bootstrap pipeline and CLI/API selection points without mixing backends within a package; verify explicit selection and rollback tests.
- [ ] 4.8 Add a differential harness that compiles the same package with `codegen2` and `codegen3` and compares observable behavior; verify the existing package test suite passes through both backends.

## 5. Optimizer (`optimizer3`)

- [ ] 5.1 Implement the pass driver, fixpoint control, optimization levels, and pass selection; verify O0 correctness and deterministic pass ordering tests.
- [ ] 5.2 Implement `Match3` decision-tree compaction and redundant-test elimination; verify IR snapshots and guard-fallback behavior.
- [ ] 5.3 Implement `Simplify3` constant folding, copy propagation, DCE, CSE, and effect-aware hoisting; verify FFI calls are never removed or reordered and pure unused values are eliminated.
- [ ] 5.4 Implement `Tailcall3` statically known tail-call graph construction and SCC lowering; verify stack-depth tests for self and mutual recursion.
- [ ] 5.5 Implement `Unbox3` local Option/Result multi-return conversion and non-escaping `Ref.new` elimination; verify allocation benchmarks and behavioral equivalence.
- [ ] 5.6 Implement `Closure3` free-variable analysis, lambda lifting, and immediate-call beta reduction; verify closure capture and mutation tests.
- [ ] 5.7 Implement `Inline3` intra-package inlining with recursion and code-size budgets; verify inlining tests and a compile-time budget regression check.
- [ ] 5.8 Implement `Specialize3` intra-package dictionary and first-order generic specialization; verify specialized call sites remove known dictionary parameters and preserve behavior.
- [ ] 5.9 Add cross-pass determinism tests that run the full optimizer twice and compare IR dumps; verify map-backed passes produce identical output.

## 6. Self-hosting and rollout

- [ ] 6.1 Compile `ir3`, `optimizer3`, and `codegen3` with `codegen2`; verify the packages build and their unit tests pass.
- [ ] 6.2 Compile `ir3`, `optimizer3`, and `codegen3` with `codegen3` and run the resulting compiler; verify its output matches the `codegen2`-built compiler on the test suite.
- [ ] 6.3 Run the full existing MyGO test suite through the `codegen3` pipeline; verify behavioral parity with the established backend.
- [ ] 6.4 Measure allocation, stack-depth, and compile-time results against the Phase 1 baseline and record them in a checked-in report; verify each optimization has a measured effect or is documented as deferred.
- [ ] 6.5 Make `codegen3` the default after parity and keep an explicit rollback to `codegen2`; verify default builds and rollback builds both succeed.
- [ ] 6.6 Add a self-host CI job that compiles the compiler with `codegen3` and runs the compiler test suite; verify the job fails on self-host regressions.

## 7. Documentation and validation

- [ ] 7.1 Document the IR contract, effect rules, representation matrix, optimizer passes, and codegen3 control-flow mapping under `docs/compiler3/`; verify the documents cover every spec requirement.
- [ ] 7.2 Update `docs/compiler2/core.md`, `docs/compiler/collections.md`, and `docs/compiler/mutual-tailcall.md` with the new representation and tail-call behavior; verify stale references to the interface representation are removed.
- [ ] 7.3 Update `KNOWN_ISSUES.md` and `HISTORY.md` with the deferred items (large-payload threshold, unknown-closure tail calls, source-first `.gen.go` build contract); verify each deferred item has a follow-up note.
- [ ] 7.4 Run `openspec validate introduce-ir3-pipeline --strict` and fix all reported issues; verify the command exits successfully.
- [ ] 7.5 Run `go test ./...` and the MyGO self-host test suite with both backends; verify all tests pass and no generated files are required to be checked in.
