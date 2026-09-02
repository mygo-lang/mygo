## MODIFIED Requirements

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
