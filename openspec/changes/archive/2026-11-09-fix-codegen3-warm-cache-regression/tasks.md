## 1. Reproduction and attribution

- [x] 1.1 Reproduce cold and warm pre-`cc5d576e9f` and current self-host build/test workloads in isolated trees; record exact commands, identities, cache state, repeated timing and timeouts.
- [x] 1.2 Trace on-demand dependency loading and metadata completeness for imports, dynamic dispatch, source positions and test inputs; document the verified fallback and cycle requirements.
- [x] 1.3 Add or use focused stage/work counters for parsing, FFI, inference, summary validation, hashing and metadata work; demonstrate the repeated work on a shared-dependency fixture and attribute the self-host warm path.
- [x] 1.4 Record the current `List.Cons` and `ListToSlice` failures and compare `TestPipelineIR3Scaffolding` and `TestPipelineCLITestCommand` with the pre-change baseline under the same conditions.

## 2. Dependency cache reuse

- [x] 2.1 Integrate verified early reuse into on-demand codegen3 dependency declaration loading with guarded fallback; verify a changed importer can consume a warm dependency without repeating its parse, FFI or inference.
- [x] 2.2 Preserve transitive imports, package identities, dynamic metadata, diagnostics, source positions and internal/external test behavior; compare cold, warm and cache-off outputs and diagnostics on focused fixtures.
- [x] 2.3 Preserve cycle and error handling and the existing overlay-aware non-codegen3 path; run dependency-cycle and unsaved imported-signature/overlay-close regression tests.
- [x] 2.4 Separate verified generated-output hits from typed-declaration hits; verify an output hit does not populate source or typed caches with placeholder declarations and an incomplete declaration hit falls back to source.
- [x] 2.5 Define and version the complete typed dependency representation, including imports, inferred bindings, inherent methods, instances, constraints, dynamic information, test input identity and package-relative declaration locations; test round trips and reject missing or old-format typed summaries.
- [x] 2.6 Add an explicit pretyped dependency path to inference and code generation, including prelude, without inferring stub bodies; compare method and instance resolution, inferred types and diagnostics against cache-off fixtures.
- [x] 2.7 Test implicit prelude types, `List.Cons`, `ListToSlice`, generic methods, inferred bindings and changed importers; run `TestPipelineIR3Scaffolding` and `TestPipelineCLITestCommand` and compare any failures against the baseline.
- [x] 2.8 Verify diagnostics retain source paths, lines and columns after typed-summary reuse in a second checkout, and that unsaved overlays bypass disk-derived summaries.
- [x] 2.9 Fix codegen3 emission of imported enum constructors in closures with source-spelled return types; verify canonical and legacy constructor selection and compile a cross-package fixture.

## 3. Bounded validation and cache input work

- [x] 3.1 Implement per-build successful-summary validation memoization with separate active traversal tracking and full identity checks; verify diamond graphs validate each shared package once and invalid/cyclic entries are not trusted.
- [x] 3.2 Reuse source fingerprints and avoid duplicate source digest calculations during summary construction and lookup; verify work counters and input-change invalidation.
- [x] 3.3 Avoid cache-only module and replacement digest scans with `MYGO_CACHE=off`, while retaining full enabled-cache input coverage; verify cache-off scan counts and Go/module/replacement change tests.
- [x] 3.4 Measure metadata staging, manifest verification, scanning and FFI costs, then remove material repeated work without weakening verification; verify isolated work counts and cache-off parity.
- [x] 3.5 If stored cache data or validity semantics change, version affected entries and verify old, missing, corrupt and interrupted entries safely miss without manual cleanup.

## 4. Correctness and regression gates

- [x] 4.1 Add focused tests for shared dependencies, changed implementation/signature, test-only imports, transitive sources, relevant Go inputs, local replacements, compiler/toolchain/options and corrupt caches; compare diagnostics and generated output with cache disabled.
- [x] 4.2 Verify concurrent cache access and output containment, including external modules, without writing generated Go or modifying module files in the checkout.
- [x] 4.3 Regenerate affected MyGO packages in dependency order with a writable Go cache; run focused compiler/cache tests, LSP overlay regressions, and relevant codegen3 build/test integration checks.

## 5. Performance acceptance and documentation

- [x] 5.1 Run alternating repeated baseline/final self-host build and test comparisons with cold, warm and disabled caches and implementation/signature/test-input incremental cases; record binary and source identities, Go cache state, stage counts, wall time and resource measurements.
- [x] 5.2 Demonstrate that warm self-host build and test no longer regress against the pre-`cc5d576e9f` baseline on the same host and workload; investigate remaining material hotspots and keep this task open if results time out or are inconclusive.
- [x] 5.3 Document reproducible commands, medians/ranges, MyGO versus Go toolchain time, correctness/output parity and limitations in the performance documentation; validate this change with `openspec validate fix-codegen3-warm-cache-regression --strict`.
