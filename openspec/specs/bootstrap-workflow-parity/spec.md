# bootstrap-workflow-parity Specification

## Purpose

Defines bootstrap compilation and generation workflow behavior so the
self-hosted pipeline remains a usable verification path for the normal CLI and
Go APIs.

## Requirements

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
deterministic returned list of written files. Bootstrap package inference SHALL
also resolve a package-local type alias before it derives the types of struct
fields and named enum-variant fields, regardless of the source file that
declares the alias or whether prelude declarations participate as external
context. For valid bootstrap-accepted MyGO source, every generated Go file
SHALL compile with its generated package output and resolved dependencies;
per-file automatic imports SHALL be sufficient and free of unused imports.

#### Scenario: A package has ordinary and test MyGO sources
- **WHEN** bootstrap sync compiles a package containing both source classes
- **THEN** it SHALL report a deterministic list containing the generated Go
  files for each class with Go-recognized test-file names

#### Scenario: A later file defines an alias used by a struct field
- **WHEN** bootstrap compilation infers a package where one source file
  declares a struct field of type `RunID`, a later source file declares
  `type RunID = String`, and a function initializes the field with `""`
- **THEN** inference SHALL accept the field value as the alias target and
  SHALL NOT report a `RunID` versus `String` unification error

#### Scenario: Alias resolution includes external declarations
- **WHEN** bootstrap compilation infers the same package together with
  external prelude declarations
- **THEN** the package-local alias SHALL remain transparent while deriving
  struct and named enum-variant field types

#### Scenario: Independently generated application files use and omit Prelude
- **WHEN** a bootstrap build generates one file that lowers Prelude helper
  calls and another that contains only erased Prelude-typed interfaces
- **THEN** both generated files SHALL compile together, with the required
  Prelude import present only in the file that references emitted Prelude code

### Requirement: CLI compilation defaults to bootstrap
The CLI SHALL use the bootstrap pipeline for `sync` and the generation phase of `build` when no pipeline selection flag is supplied. Bootstrap errors SHALL be reported without automatically retrying through legacy. Default compilation SHALL honor `--no-prelude` and existing generated-file reporting behavior.

#### Scenario: Default sync
- **WHEN** `mygo sync <package>` is invoked
- **THEN** the CLI SHALL generate the package through bootstrap

#### Scenario: Default build
- **WHEN** `mygo build <package>` is invoked
- **THEN** the CLI SHALL generate through bootstrap and run Go build only after generation succeeds

#### Scenario: Default sync without Prelude
- **WHEN** `mygo --no-prelude sync <package>` is invoked for a package that does not use Prelude names
- **THEN** bootstrap SHALL compile without resolving or injecting Prelude

#### Scenario: Bootstrap compilation fails
- **WHEN** default sync or build encounters a bootstrap compilation error
- **THEN** the CLI SHALL report failure without falling back to legacy or invoking Go build

### Requirement: Legacy generation requires explicit sync selection
The CLI SHALL select the legacy pipeline for `mygo --legacy sync <package>`, including its existing generated-file reporting and `--no-prelude` behavior. `--legacy` SHALL be accepted only for `sync`; another command with that flag SHALL fail with a usage error before formatting, generation, or Go build occurs.

#### Scenario: Explicit legacy sync
- **WHEN** `mygo --legacy sync <package>` is invoked
- **THEN** the CLI SHALL generate through the legacy pipeline

#### Scenario: Legacy sync without Prelude
- **WHEN** `mygo --legacy --no-prelude sync <package>` is invoked
- **THEN** legacy generation SHALL disable automatic Prelude resolution and injection

#### Scenario: Unsupported legacy command
- **WHEN** `mygo --legacy build <package>` or `mygo --legacy fmt <file>` is invoked
- **THEN** the CLI SHALL report a usage error before performing command work

### Requirement: Pipeline flags have deterministic compatibility
The CLI SHALL retain `--bootstrap` as an explicit compatibility alias for default compilation and retain `--bootstrap-timing` for bootstrap stage timing. Combining `--legacy` with either bootstrap flag SHALL produce a usage error before generation, regardless of flag order. CLI help SHALL describe bootstrap as the default and `--legacy sync` as the legacy generation command.

#### Scenario: Existing bootstrap invocation
- **WHEN** `mygo --bootstrap sync <package>` or `mygo --bootstrap build <package>` is invoked
- **THEN** pipeline selection SHALL match the corresponding default command

#### Scenario: Timing remains available
- **WHEN** `mygo --bootstrap-timing sync <package>` is invoked with normal Prelude selection
- **THEN** bootstrap SHALL generate the package and print stage durations to standard error

#### Scenario: Conflicting selectors
- **WHEN** global flags contain `--legacy` and either `--bootstrap` or `--bootstrap-timing` in either order
- **THEN** the CLI SHALL report a usage error before generation

#### Scenario: Help explains migration
- **WHEN** CLI usage is displayed
- **THEN** it SHALL identify bootstrap as the default, the compatibility alias, and the sync-only legacy selector
