## ADDED Requirements

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
