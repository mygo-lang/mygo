# codegen3-compilation-performance Specification

## Purpose

Reduce repeated codegen3 compilation work while preserving cache correctness, generated output, and reproducible performance evidence.

## Requirements

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

### Requirement: Verified dependency declarations are reused on demand

For a codegen3 build with verified, compatible cached dependency information sufficient for an importer, the compiler SHALL reuse that information when declarations are requested on demand, without repeating full dependency parsing, Go FFI signature collection or type inference. Reuse MUST preserve declarations, diagnostics, test input distinctions and output equivalence with a cache-disabled build; unavailable or untrusted information SHALL trigger safe compilation. Editor overlays MUST retain their existing precedence over disk and cached declarations.

#### Scenario: Warm dependency requested by an importer
- **WHEN** an importer needs declarations from an unchanged dependency whose cached information and transitive inputs verify successfully
- **THEN** the compiler supplies equivalent declarations without repeating the dependency's full parsing, FFI signature collection or type inference

#### Scenario: Dependency cache is unsafe to reuse
- **WHEN** required dependency data is missing, corrupt, incompatible, stale, or cannot represent the selected inputs
- **THEN** the compiler compiles from the correct current sources and preserves uncached diagnostics and output

#### Scenario: Cached prelude and methods support a changed importer
- **WHEN** an importer changes while its prelude and dependencies have complete verified typed summaries containing their inherent methods and instances
- **THEN** inference resolves those methods and instances without inferring placeholder bodies, and diagnostics and output match a cache-disabled build

#### Scenario: Output hit precedes an incomplete declaration hit
- **WHEN** a cached generated output is valid but a later importer needs declaration information absent from its typed summary
- **THEN** the output can be reused and the compiler obtains the missing declarations through source-based compilation

#### Scenario: Unsaved dependency overlay
- **WHEN** editor analysis uses an unsaved imported dependency
- **THEN** it uses the overlay-derived declarations instead of a disk-derived cached summary

### Requirement: Shared cache validation work is bounded per build

For stable inputs during one build, successfully validated package summaries and their source fingerprints SHALL be reusable throughout that build, so a shared dependency is not fully revalidated once per path through the dependency graph. Reuse MUST be scoped to the verified input identity and MUST NOT turn corrupt or changed inputs into trusted results. Cache-disabled builds SHALL NOT perform cache-only input fingerprint scans.

#### Scenario: Shared dependency graph
- **WHEN** a warm build validates a dependency graph in which multiple importers share a dependency
- **THEN** the dependency's successful full validation is performed at most once per applicable input identity during the build

#### Scenario: Later build changes an input
- **WHEN** a source, test input, Go input, local replacement, compiler, toolchain or relevant option changes between builds
- **THEN** the next build revalidates the relevant inputs and matches an uncached build

#### Scenario: Cache is disabled
- **WHEN** caching is disabled
- **THEN** the build avoids cache-only module and replacement fingerprint scans while retaining normal compilation and diagnostics

### Requirement: Representative warm-cache regressions are measured

Performance validation SHALL compare repeated, paired before-and-after self-host compiler build and self-host test workloads using comparable toolchain, source, optimization and cache conditions. It SHALL report cold, unchanged warm, cache-disabled and incremental cases, observable stage work counts, elapsed time and relevant resource measurements, with MyGO generation separated from Go toolchain work. Acceptance SHALL require the warm regression to be eliminated relative to the pre-`cc5d576e9f` baseline on the same workload and host, with correctness and generated output parity established; inconclusive or timed-out comparisons SHALL NOT be reported as a pass.

#### Scenario: Self-host warm build and test
- **WHEN** a warmed self-host build and test are compared against the pre-optimization baseline
- **THEN** the report records repeated timings, the exact cache and executable identities, stage work and correctness results, and demonstrates that the regression is eliminated

#### Scenario: Incremental and failure paths
- **WHEN** implementation, public signature, test-only or relevant Go inputs change, or a cache record is corrupted
- **THEN** validation reports affected work and verifies behavior against cache-disabled execution
