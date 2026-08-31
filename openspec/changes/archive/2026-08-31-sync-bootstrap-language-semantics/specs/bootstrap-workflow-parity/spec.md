## Purpose

Defines bootstrap compilation and generation workflow behavior so the
self-hosted pipeline remains a usable verification path for the normal CLI and
Go APIs.

## ADDED Requirements

### Requirement: Prelude selection is honored by bootstrap
The `--no-prelude` option SHALL apply when bootstrap compilation is selected.
Bootstrap compilation without that option SHALL supply the built-in prelude to
non-prelude packages; compilation with the option SHALL not resolve, infer, or
emit a prelude dependency automatically.

#### Scenario: Bootstrap compiles without the prelude
- **WHEN** `mygo --bootstrap --no-prelude sync <package>` is invoked for a
  package that does not use prelude names
- **THEN** the package SHALL compile without requiring a resolvable prelude

### Requirement: External test packages are isolated
Bootstrap compilation SHALL classify each source file from its declared package
name before inference or generation. A `*_test.mygo` file declaring
`package name` SHALL remain an internal test of the main package. A file
declaring `package name_test` SHALL form a separate external Go test package,
distinct from `package name`.

For an external test package, bootstrap SHALL infer the main package first and
make its exported declarations available as the test package's external
environment. Bootstrap SHALL add the corresponding main-package Go dot import
as generated-file metadata. This synthesized Go import SHALL NOT be represented
as a MyGO import or participate in MyGO dependency traversal.

#### Scenario: An external test refers to an exported main-package symbol
- **WHEN** a `name_test` MyGO file refers to an exported symbol from `name`
- **THEN** bootstrap generation SHALL produce buildable Go test output without
  requiring the source to declare the automatic main-package import

#### Scenario: An internal test remains in the main package
- **WHEN** a `*_test.mygo` file declares `package name`
- **THEN** bootstrap SHALL infer and generate it as part of `name`, emit a
  Go-recognized test filename, and SHALL NOT add an import of `name` into
  itself

### Requirement: Convenience generation includes normal prelude context
`compiler.GenerateSource` and `compiler.GenerateSourceAt` SHALL provide the
same default prelude context as bootstrap package compilation for non-prelude
source. Their generated Go SHALL include any required prelude import.

#### Scenario: A convenience generation uses Option
- **WHEN** a non-prelude input supplied to `compiler.GenerateSourceAt` uses `Option`
- **THEN** generation SHALL succeed and the resulting Go source SHALL include
  the required prelude import

### Requirement: Bootstrap results remain observable
Bootstrap sync and build commands SHALL retain the normal generated-file naming
and reporting behavior, including `_test.go` output for test sources and a
deterministic returned list of written files.

#### Scenario: A package has ordinary and test MyGO sources
- **WHEN** bootstrap sync compiles a package containing both source classes
- **THEN** it SHALL report a deterministic list containing the generated Go
  files for each class with Go-recognized test-file names
