# bootstrap-codegen3 Specification

## Purpose

Defines the guarantees of the IR-based Go backend: behavioral equivalence, per-file provenance, control-flow lowering, tail-call safety, pattern-match lowering, imports, source-derived dependency information, and boundary handling.

## Requirements

### Requirement: Self-host performance is measurable

The project SHALL provide a repeatable comparison of codegen2-built and codegen3-built compiler executables running the same sync workload, including elapsed time, CPU and allocation profiles, and representative type sizes. A measured optimization SHALL preserve observable behavior and report its effect and measurement conditions.

#### Scenario: Comparable self-hosted runs
- **WHEN** both compiler binaries process the same isolated workload under comparable cache conditions
- **THEN** measurement distinguishes the build backend from the runtime-selected backend and records comparable results

#### Scenario: Optimized compiler retains behavior
- **WHEN** a measured hot path is optimized
- **THEN** representative backend parity and self-host checks pass and the performance report identifies the result and limitations

#### Scenario: Self-host runtime and pipeline cost are distinguished
- **WHEN** performance of a compiler built with codegen3 is compared with the codegen3 pipeline itself
- **THEN** the report states the executable's build backend, the runtime-selected backend and optimization level, cache state, workload and toolchain, and compares repeated runs under matching conditions

#### Scenario: Warm cache retains correctness
- **WHEN** a package is rebuilt after a source, dependency, compiler, representation, toolchain or relevant option change
- **THEN** cached output or typed information is reused only when all relevant inputs match, and diagnostics and output remain equivalent to a clean build

#### Scenario: Full backend validation uses compatible tests
- **WHEN** the complete test suite is run with codegen3
- **THEN** hand-written Go tests exercise the canonical ABI without assuming the legacy interface representation, and parity and source-tree cleanliness are verified

### Requirement: codegen3 preserves observable behavior

For every program accepted by the bootstrap pipeline, Go emitted by codegen3 SHALL have the same observable behavior as Go emitted by the established backend, including evaluation order, panics, mutation, reference identity, FFI effects, dynamic dispatch, and returned values.

#### Scenario: Differential behavior
- **WHEN** the same package is compiled and run through both backends
- **THEN** the observable results and side effects match

#### Scenario: Panic ordering
- **WHEN** an expression panics before a later side effect
- **THEN** the codegen3 output panics before performing that side effect

### Requirement: Generated files preserve per-file provenance

codegen3 SHALL emit one generated Go file per MyGO source file. A declaration SHALL be emitted to the file that declared it, and generated line directives SHALL refer to the originating source path and position.

#### Scenario: Declarations stay in their source file
- **WHEN** a package contains declarations in two `.mygo` files
- **THEN** each generated file contains the declarations originating from its matching source file

#### Scenario: Line directives point to source
- **WHEN** generated Go contains a statement lowered from a MyGO expression
- **THEN** a line directive identifies the originating `.mygo` path and line

#### Scenario: Output is independent of the absolute module root
- **WHEN** the same source is compiled from two different module directories
- **THEN** the generated text is byte-identical and line directives name package-relative paths rather than absolute filesystem paths

### Requirement: Control-flow lowering produces valid structured Go

codegen3 SHALL reconstruct reducible joins as ordinary Go control flow. When a join or guard fallback cannot be expressed structurally, codegen3 SHALL emit valid Go control flow using labels, `goto`, and hoisted locals without entering variable scopes illegally.

#### Scenario: Guard fallback compiles
- **WHEN** a switch arm's guard fails and execution must continue at a later arm
- **THEN** the generated Go compiles and reaches the later arm

#### Scenario: Nested joins compile
- **WHEN** multiple nested branches merge into a shared continuation
- **THEN** the generated Go is syntactically valid and preserves the merge value

### Requirement: Tail-call lowering preserves symbols and bounds stack growth

codegen3 SHALL lower tail-call cycles among statically known targets into loops or state machines that do not grow the Go stack per iteration. Source-level exported function symbols SHALL remain available with their original names and signatures. Tail calls through unknown closures or dynamic dispatch MAY remain ordinary calls.

#### Scenario: Mutual recursion is stack-safe
- **WHEN** two mutually recursive functions in one tail-call cycle run for a large number of iterations
- **THEN** execution does not overflow the Go stack

#### Scenario: Exported symbol remains
- **WHEN** an optimized function is exported from its MyGO package
- **THEN** generated Go still exposes a function with the source-level name and signature

### Requirement: Pattern matching uses decision trees

codegen3 SHALL lower pattern matches from the IR decision tree, evaluating the subject once per decision and transferring failed guards to the next applicable arm. It SHALL NOT require repeated interface type assertions for a single match.

#### Scenario: Guard failure reaches the next arm
- **WHEN** the first matching arm's guard is false
- **THEN** the next applicable arm is attempted with the same subject value

#### Scenario: Nested variants match
- **WHEN** a pattern nests variant and struct patterns
- **THEN** the generated Go extracts the required fields and selects the correct arm

### Requirement: Generated imports are exact

Each generated Go file SHALL import exactly the packages it references. It SHALL include the prelude import when a generated declaration references the prelude, and SHALL NOT include an import that the file does not use.

#### Scenario: No unused imports
- **WHEN** a source file references no symbol from a package
- **THEN** its generated file does not import that package

#### Scenario: Prelude helper reference
- **WHEN** a generated declaration calls a prelude-provided helper
- **THEN** the generated file imports the prelude package

### Requirement: Importer metadata is verified and safely staged

codegen3 SHALL derive importer-facing metadata from the compiled package and MAY emit it alongside generated output under the active contained build or cache root. The compiler SHALL use cached dependency information for inference only after verifying its compatibility, integrity, completeness, source and dependency inputs. Signature-only metadata SHALL NOT be treated as a complete typed dependency. If cached information cannot be trusted or does not cover the requested inputs, the compiler SHALL use the correct current source-based compilation path. Reuse MUST preserve cross-package declarations, inherent methods, instances, dynamic dispatch and diagnostics, and MUST NOT override editor overlays.

#### Scenario: Verified metadata supplies a dependency
- **WHEN** a dependency's generated output and importer metadata are compatible, complete and verified for the current inputs
- **THEN** an importer can obtain equivalent declarations and dynamic-dispatch information from that metadata

#### Scenario: Invalid metadata falls back to sources
- **WHEN** metadata is missing, corrupt, incompatible or stale
- **THEN** the compiler uses current source-derived declarations and does not consume the untrusted metadata

#### Scenario: Signature-only metadata cannot seed inference
- **WHEN** cached metadata contains exported signatures but lacks complete typed method or instance information for a dependency needed by inference
- **THEN** the compiler uses current source-derived declarations and does not infer placeholder function bodies or binding values

#### Scenario: Cached dependency retains diagnostic locations
- **WHEN** an importer uses a verified typed dependency from a different checkout of the same source
- **THEN** diagnostics retain the source paths, lines and columns appropriate to that checkout

#### Scenario: Metadata remains contained
- **WHEN** codegen3 emits or stages metadata
- **THEN** it remains under the active build or cache output root and does not create generated files in the source checkout

### Requirement: FFI and dynamic dispatch remain functional

codegen3 SHALL generate the adapters required by Go FFI calls and dynamic-typeclass dispatch for canonical representations. Go `error` results SHALL map to `Result` variants, and dynamic wrappers SHALL continue to dispatch to the selected implementation.

#### Scenario: Go error becomes Err
- **WHEN** a generated FFI call receives a non-nil Go error
- **THEN** the MyGO-visible result is `Err` carrying that error

#### Scenario: Dynamic implementation is selected
- **WHEN** a dynamic typeclass value wraps a concrete implementation and a method is invoked
- **THEN** the generated dispatch calls the wrapped implementation

### Requirement: Generated output builds outside the source tree

The complete set of generated Go files for a package SHALL compile with the project's supported Go toolchain and SHALL not depend on generated files being checked into the repository. When `codegen3` is enabled the compiler SHALL write generated Go only under the active per-build output root in `MYGO_CACHE` (or the system temporary directory when caching is disabled) and SHALL invoke the Go toolchain directly (`go build` / `go test`), using temporary overlays for in-module packages and staged module copies for external MyGO packages. The compiler SHALL NOT create `zz_*.gen.go`, `zz_*.gen_test.go`, or a persistent project-local generated directory inside the repository. Project module inputs (`go.mod`, and `go.sum` when present) SHALL remain unchanged; Go dependencies SHALL be downloaded through `GOPROXY`. Output paths for packages outside the current module SHALL use deterministic import-path-based directories. Before writing or copying, the compiler SHALL verify that the resolved destination is contained within the active output root; it SHALL fail with a diagnostic without writing if containment cannot be established.

#### Scenario: Package build succeeds
- **WHEN** a package and its dependencies are compiled by the bootstrap pipeline
- **THEN** generated Go files are staged under the active codegen3 build/cache output root and the Go toolchain builds them using the configured temporary overlay and module file

#### Scenario: No checked-in generated files required
- **WHEN** generated files are produced only at build time
- **THEN** the package still compiles and no generated file appears under the source tree

#### Scenario: No project-local generated directory
- **WHEN** a package is built or tested with `codegen3`
- **THEN** the repository contains no persistent generated directory, and the temporary overlay exists only inside the `MYGO_CACHE` temporary area (or system temporary area when caching is disabled)

#### Scenario: Direct Go toolchain invocation
- **WHEN** `codegen3` builds or tests a package
- **THEN** the compiler runs `go build` or `go test` with the temporary overlay and, when external modules are staged, the temporary `-modfile`, without writing generated Go beside MyGO sources; a binary is written to the requested `-o` path or the same default location `go build` would use

#### Scenario: Module context and dependency download
- **WHEN** the generated package requires a Go module dependency
- **THEN** the build uses the project's `go.mod`/`go.sum` and resolves the dependency through `GOPROXY`

#### Scenario: External MyGO dependency output stays contained
- **WHEN** an imported MyGO package is resolved from the Go module cache or a local replacement outside the current module root
- **THEN** its source module is staged under a deterministic import-path-based directory within the active codegen3 build/cache output root, generated files are written only in that staged copy, and the dependency source directory remains unchanged

#### Scenario: Go module cache is not an overlay target
- **WHEN** codegen3 builds a MyGO package whose source is under `GOMODCACHE`
- **THEN** the Go overlay contains no replacement target beneath `GOMODCACHE`; a per-build temporary module file redirects the external module to its staged copy, and the project's `go.mod` and `go.sum` remain unchanged

#### Scenario: Staged module preserves ordinary Go sources
- **WHEN** an external MyGO module also contains ordinary Go files or other packages
- **THEN** the staged module preserves the source files needed to compile its imported Go packages alongside generated MyGO output

#### Scenario: Output path traversal is rejected
- **WHEN** an output or staged module path for any package, including an external test package, resolves outside the active codegen3 build/cache output root
- **THEN** codegen3 reports an output-path diagnostic before creating directories or files

### Requirement: codegen3 provides a test command

The compiler SHALL expose `test` as a CLI command. Without `--codegen3` it uses the established backend; with `--codegen3` it SHALL generate the requested packages (including test-only imports and external `_test` packages) into the cache, run `go test -overlay <overlay>` with the user's remaining arguments, and return the Go toolchain's exit status. The repository-clean contract applies to test generation.

#### Scenario: Tests run against generated output
- **WHEN** the user runs `mygo --codegen3 test ./pkg -run TestX`
- **THEN** `go test` runs the package's tests against generated Go and a failing test makes the command exit non-zero

#### Scenario: Test flags are passed through
- **WHEN** the user supplies `-run`, `-count`, `-v`, or other `go test` flags
- **THEN** those flags reach `go test` unchanged while package patterns select the generated packages

#### Scenario: External test package is generated
- **WHEN** a package contains a `package foo_test` source file
- **THEN** the external test package is generated and compiled together with the package under test

#### Scenario: No test wrapper is written to the repository
- **WHEN** a codegen3 test run completes, successfully or not
- **THEN** no `.gen_test.go` file or project-local generated directory appears under the source tree

### Requirement: codegen3 build cache is content-addressed and shared across modules

The compiler SHALL store generated Go in a user-level content-addressed cache. `MYGO_CACHE` SHALL select the cache root and SHALL default to the operating system's user cache directory; `MYGO_CACHE=off` SHALL disable caching. A cache key SHALL cover the cache format version, compiler identity, representation and adapter versions, optimization level, normalized package source content, dependency keys, and the Go module/toolchain inputs used for FFI, and SHALL NOT contain the absolute module root, module checkout path, timestamps, or user-specific paths. Entries SHALL be immutable, verified against a manifest, published atomically, safe for concurrent builds, and reusable offline. Removing, corrupting, or disabling the cache SHALL only cost time, never correctness.

#### Scenario: Cache hit across modules
- **WHEN** two Go modules contain identical package sources and use the same compiler, representation version, and options
- **THEN** the second build reuses the first build's cache entry even though the module roots differ

#### Scenario: Compiler, representation, or option change misses
- **WHEN** the compiler build, representation version, adapter ABI, or optimization level changes
- **THEN** the cache key changes and the old entry is not reused

#### Scenario: Dependency change invalidates importers
- **WHEN** an imported MyGO package changes such that its generated declarations change
- **THEN** importers' keys change through the dependency key and their entries are regenerated

#### Scenario: Concurrent builds
- **WHEN** two compiler processes build the same package at the same time
- **THEN** both produce the same output and the cache contains one complete entry with the other temporary copy discarded

#### Scenario: Corrupt or interrupted entry
- **WHEN** an entry is incomplete or fails its manifest or file digest check
- **THEN** the compiler ignores it, regenerates the package, and does not mix partial output into the build

#### Scenario: Offline reuse
- **WHEN** the inputs are unchanged and the cache is warm
- **THEN** the build or test succeeds without network access

#### Scenario: Cache disabled
- **WHEN** `MYGO_CACHE=off` is set or the cache directory is removed
- **THEN** builds and tests still succeed and produce the same generated text

### Requirement: Cache locking is portable across supported release platforms

The codegen3 compiler SHALL provide cache locking on every platform in the
project's supported release matrix, including Windows and macOS, while
preserving shared locking for builds and exclusive locking for cache cleaning.
The compiler SHALL wait for a bounded period when a compatible lock is held and
SHALL fail without deleting or reading partially written cache data when the
lock cannot be acquired before the deadline.

#### Scenario: Windows release build compiles
- **WHEN** the compiler is built for `windows/amd64`
- **THEN** the generated compiler sources compile without unresolved Unix-only lock symbols

#### Scenario: macOS release builds compile
- **WHEN** the compiler is built for `darwin/amd64` or `darwin/arm64`
- **THEN** the generated compiler sources compile and retain the same shared and exclusive cache-lock behavior

#### Scenario: Concurrent builds share the cache on every supported platform
- **WHEN** multiple codegen3 builds use the same cache root on Windows, macOS, or Linux
- **THEN** each build acquires a compatible shared lock, cache entries remain complete, and builds are not unnecessarily serialized against one another

#### Scenario: Cleaning cannot race an active build on every supported platform
- **WHEN** cache cleaning requests an exclusive lock while a build holds a shared lock
- **THEN** cleaning waits for the bounded timeout or exits with a busy diagnostic without deleting active cache data

### Requirement: codegen3 cache can be cleared safely

The compiler SHALL provide `mygo cache clean` to remove the codegen3 entries and temporary overlays under the active cache root; `mygo clean -cache` SHALL accept the same operation as the Go-compatible spelling. Cleaning SHALL take the cache's exclusive lock, SHALL NOT delete data being read by an in-flight build, and SHALL report the cache root and a summary of what was removed. With caching disabled, cleaning SHALL be a successful no-op.

#### Scenario: Clean removes all entries
- **WHEN** `mygo cache clean` runs with no build in flight
- **THEN** every build entry and temporary overlay under `MYGO_CACHE` is removed and the command exits zero

#### Scenario: Clean after a cold build reproduces output
- **WHEN** the cache is cleaned and the same package is rebuilt
- **THEN** the build succeeds and produces the same generated text as before cleaning

#### Scenario: Clean does not race an active build
- **WHEN** `mygo cache clean` runs while another build holds the shared cache lock
- **THEN** the command waits for a bounded time or exits non-zero with an explanation, and deletes nothing while the build is active

#### Scenario: Clean with caching disabled
- **WHEN** `MYGO_CACHE=off mygo cache clean` runs
- **THEN** the command reports that caching is disabled and exits zero without touching any files

### Requirement: codegen3 is selectable and reversible

The compiler SHALL allow code generation to be selected per compilation and SHALL NOT mix backends within one package. `codegen2` SHALL remain the default backend in this change and SHALL retain its legacy interface-based ADT ABI. codegen3 SHALL activate only through explicit selection.

#### Scenario: Explicit backend selection
- **WHEN** the user selects codegen3
- **THEN** all files in that package are generated by codegen3

#### Scenario: Established backend remains the default
- **WHEN** the user runs a build or test without selecting codegen3
- **THEN** the package uses codegen2 and its legacy generated-Go ABI

#### Scenario: Rollback
- **WHEN** the user selects the established backend
- **THEN** every file in the package is generated without codegen3 and remains buildable through the legacy ABI
