## Purpose

Defines the guarantees of the IR-based Go backend: behavioral equivalence, per-file provenance, control-flow lowering, tail-call safety, pattern-match lowering, imports, source-derived dependency information, and boundary handling.

## ADDED Requirements

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

### Requirement: No metadata artifact is emitted

codegen3 SHALL NOT emit a package metadata file. Cross-package declarations and dynamic-dispatch information SHALL be derived from source declarations compiled in the same build.

#### Scenario: No metadata file is produced
- **WHEN** a package is compiled by codegen3
- **THEN** no `zz_mygo_metadata.gen.go` or equivalent metadata artifact is written

#### Scenario: Dependencies are source-derived
- **WHEN** a package imports another MyGO package
- **THEN** the importing build obtains that package's declarations from source rather than a metadata file

### Requirement: FFI and dynamic dispatch remain functional

codegen3 SHALL generate the adapters required by Go FFI calls and dynamic-typeclass dispatch for canonical representations. Go `error` results SHALL map to `Result` variants, and dynamic wrappers SHALL continue to dispatch to the selected implementation.

#### Scenario: Go error becomes Err
- **WHEN** a generated FFI call receives a non-nil Go error
- **THEN** the MyGO-visible result is `Err` carrying that error

#### Scenario: Dynamic implementation is selected
- **WHEN** a dynamic typeclass value wraps a concrete implementation and a method is invoked
- **THEN** the generated dispatch calls the wrapped implementation

### Requirement: Generated output builds outside the source tree

The complete set of generated Go files for a package SHALL compile with the project's supported Go toolchain and SHALL not depend on generated files being checked into the repository. When `codegen3` is enabled the compiler SHALL write generated Go only to the user-level content-addressed cache described below and SHALL invoke the Go toolchain directly (`go build` / `go test`) through a temporary overlay to produce binaries, test binaries, and test results. The compiler SHALL NOT create `zz_*.gen.go`, `zz_*.gen_test.go`, or a persistent project-local generated directory such as `build/.mygo3` inside the repository. Project module inputs (`go.mod`, and `go.sum` when present) SHALL remain in place, and Go dependencies SHALL be downloaded through `GOPROXY`.

#### Scenario: Package build succeeds
- **WHEN** a package and its dependencies are compiled by the bootstrap pipeline
- **THEN** the generated Go files form a buildable package inside the cache entry that the temporary overlay exposes

#### Scenario: No checked-in generated files required
- **WHEN** generated files are produced only at build time
- **THEN** the package still compiles and no generated file appears under the source tree

#### Scenario: No project-local generated directory
- **WHEN** a package is built or tested with `codegen3`
- **THEN** the repository contains no `build/.mygo3` directory, and the temporary overlay exists only inside the cache's temporary area

#### Scenario: Direct Go toolchain invocation
- **WHEN** `codegen3` builds or tests a package
- **THEN** the compiler runs `go build` or `go test -overlay <cache overlay>` against the generated output without writing `zz_*.gen.go` or `zz_*.gen_test.go` beside the `.mygo` sources, and a binary is written to the requested `-o` path or to the same default location `go build` would use

#### Scenario: Module context and dependency download
- **WHEN** the generated package requires a Go module dependency
- **THEN** the build uses the project's `go.mod`/`go.sum` and resolves the dependency through `GOPROXY`

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
