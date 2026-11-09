## Purpose

Reduce repeated codegen3 compilation work while preserving cache correctness, generated output, and reproducible performance evidence.

## ADDED Requirements

### Requirement: Verified warm builds avoid repeated front-end work

When the compiler has sufficient verified, version-compatible cached information to establish that a package and its dependencies are unchanged, codegen3 SHALL reuse the package output without repeating full source parsing, Go FFI signature collection, type inference, or code generation for that package. Cached information SHALL be validated against all inputs affecting dependency discovery, diagnostics, generated output, and FFI. The compiler SHALL fall back to normal compilation whenever it cannot establish validity, including missing, corrupt, incompatible, or disabled cache data.

#### Scenario: Unchanged warm package
- **WHEN** a previously built package and all relevant inputs are unchanged and its cached records verify successfully
- **THEN** the build reuses the output without full package parsing or Go FFI signature collection and produces the same result as a cold build

#### Scenario: A changed dependency or input
- **WHEN** a source, imported package, relevant Go input, toolchain, build option, or compiler identity changes
- **THEN** affected cached data is invalidated or revalidated before reuse and the result matches a cache-disabled build

#### Scenario: Cache record cannot be trusted
- **WHEN** required cached information is missing, malformed, incompatible, or corrupt, or the cache is disabled
- **THEN** the compiler uses the normal compilation path and preserves its diagnostics and output

#### Scenario: Build and test inputs differ
- **WHEN** a build and a test use different source files, external test packages, imports, or build options
- **THEN** a cached result is reused only if it covers the selected inputs and yields the same behavior and diagnostics as an uncached invocation

### Requirement: Multi-file code generation avoids repeated package scans

For a codegen3 package with multiple source files, package-wide emission preparation SHALL be shared across the files and source declarations SHALL be organized so that selecting a file's declarations does not require a full package declaration scan per output file. Generated file membership, ordering, imports, line directives, and text SHALL match the established emission behavior for identical inputs.

#### Scenario: Many-file package
- **WHEN** a package is emitted from multiple source files
- **THEN** every generated file contains its own declarations in the established order and emission does not rescan all package declarations to select each file's declarations

#### Scenario: Imports and source locations
- **WHEN** files use different import aliases, generated helpers, or source locations
- **THEN** each output keeps the correct imports, helpers, and source line mappings and is byte-identical to the pre-change output for the same input

### Requirement: Compilation performance is reproducibly measured

The project SHALL provide repeatable before-and-after measurements for cold, warm, and incremental codegen3 builds and for multi-file emission scaling. Measurements SHALL report workload, file and declaration counts, compiler and Go versions, optimization level, cache state, time, and allocations where available, alongside correctness and output parity results. Results SHALL distinguish MyGO generation from downstream Go toolchain time.

#### Scenario: Warm and incremental measurements
- **WHEN** unchanged, implementation-only, and public-signature changes are benchmarked
- **THEN** the report identifies affected packages, cache conditions, repeated timings, and the observed work avoided or repeated

#### Scenario: File-count scaling
- **WHEN** a benchmark varies source-file count while controlling declaration count
- **THEN** the report exposes the scaling of emission time and allocations and verifies generated output parity
