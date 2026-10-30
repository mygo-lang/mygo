# inference-stage-performance Specification

## Purpose

Defines how self-hosted type inference scales with package declarations, local
bindings, and import surfaces while preserving diagnostics, inferred types,
typed ASTs, environment ordering, and generated output.

## Requirements

### Requirement: Inference cost scales with declarations, bindings, and imports

Type inference SHALL take time roughly proportional to the number of
declarations, local bindings, and imported symbols in a package. Per-unit cost
SHALL NOT grow as those counts grow, and repeating the same import surface
across source files SHALL NOT repeat its full conversion for every inference
phase.

#### Scenario: Declaration-heavy packages stay proportional
- **WHEN** the same declaration shape is inferred at increasing package sizes
- **THEN** the time per declaration stays roughly constant instead of growing
  with the declaration count

#### Scenario: Local-binding-heavy functions stay proportional
- **WHEN** a function body contains increasing numbers of sequential `let` and
  `var` bindings
- **THEN** the time per binding stays roughly constant and inference does not
  degrade quadratically

#### Scenario: Reused imports do not multiply work
- **WHEN** many source files import the same Go and MyGO packages
- **THEN** the inference stage does not repeat the full import conversion once
  per predeclaration pass, symbol-table pass, inference pass, and output pass

### Requirement: Imported environments are reused without changing lookup semantics

The inference setup SHALL build each source file's imported environment once
and reuse it across declaration predeclaration, symbol-table construction,
source inference, and package-output construction. Cached import batches SHALL
include their lookup index and free-variable union, and a cache hit SHALL NOT
rebuild those structures.

#### Scenario: Shadowing and environment order are preserved
- **WHEN** a local declaration, a MyGO import, and a Go import expose the same
  member name
- **THEN** lookup resolves the same binding, in the same precedence order, as
  before the optimization

#### Scenario: Exported environments keep their order
- **WHEN** a package's inferred environment is converted to its ordered entry
  list for downstream consumers
- **THEN** the entry order and duplicate-name precedence are unchanged

#### Scenario: Changed imports invalidate reusable batches
- **WHEN** a Go package signature or a MyGO package declaration surface changes
- **THEN** inference rebuilds the affected cached batch instead of reusing a
  stale lookup index or free-variable union

### Requirement: Observable inference results are unchanged

The type-inference stage SHALL preserve every existing diagnostic, inferred
type, predicate, substitution result, typed AST shape, declaration order, and
environment entry value. Accepted sources SHALL generate byte-identical Go
output after the optimization.

#### Scenario: Differential corpus stays equal
- **WHEN** the repository corpus is inferred through the changed compiler
- **THEN** every file that previously succeeded still succeeds, and every
  diagnostic keeps its text, span, and ordering

#### Scenario: Generated Go stays byte-identical
- **WHEN** the repository is regenerated after the optimization
- **THEN** every pre-existing generated Go file stays byte-identical, with the
  benchmark's own generated test file as the only permitted addition

#### Scenario: Regeneration reaches a fixed point
- **WHEN** the changed compiler is rebuilt and used to sync the repository a
  second time
- **THEN** the second generation matches the first byte for byte

### Requirement: Inference-stage performance is measured

The repository SHALL provide benchmarks for declaration-heavy packages,
local-binding-heavy functions, Go FFI import surfaces, MyGO import surfaces,
and a pinned real package. Each benchmark SHALL report nanoseconds per
operation, bytes per operation, and allocations per operation, and SHALL make
cold and warm import-cache behavior observable.

#### Scenario: Benchmarks report time and allocations
- **WHEN** the inference benchmarks run with allocation reporting enabled
- **THEN** each workload reports nanoseconds per operation, bytes per operation,
  and allocations per operation

#### Scenario: Import-cache behavior is visible
- **WHEN** a benchmark runs with an empty import cache and again with a warm
  import cache
- **THEN** both results are reported separately so cache reuse cannot hide cold
  inference cost

