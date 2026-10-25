## Purpose

Defines the guarantees of the IR-based Go backend: behavioral equivalence, per-file provenance, control-flow lowering, tail-call safety, pattern-match lowering, imports, metadata, and boundary handling.

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

### Requirement: Metadata is complete and versioned

codegen3 SHALL emit package metadata describing exported types, implementations, dynamic adapters, generated-source digests, compiler ABI, and representation version. Metadata SHALL be deterministic for identical inputs.

#### Scenario: Digest matches generated source
- **WHEN** codegen3 emits a generated file and its metadata
- **THEN** the recorded digest matches the emitted file content

#### Scenario: Representation version is present
- **WHEN** metadata is emitted
- **THEN** it identifies the representation version used for generated layouts

### Requirement: FFI and dynamic dispatch remain functional

codegen3 SHALL generate the adapters required by Go FFI calls and dynamic-typeclass dispatch for canonical representations. Go `error` results SHALL map to `Result` variants, and dynamic wrappers SHALL continue to dispatch to the selected implementation.

#### Scenario: Go error becomes Err
- **WHEN** a generated FFI call receives a non-nil Go error
- **THEN** the MyGO-visible result is `Err` carrying that error

#### Scenario: Dynamic implementation is selected
- **WHEN** a dynamic typeclass value wraps a concrete implementation and a method is invoked
- **THEN** the generated dispatch calls the wrapped implementation

### Requirement: Generated output builds without checked-in generated files

The complete set of generated Go files for a package SHALL compile with the project's supported Go toolchain and SHALL not depend on generated files being checked into the repository.

#### Scenario: Package build succeeds
- **WHEN** a package and its dependencies are compiled by the bootstrap pipeline
- **THEN** the generated Go files form a buildable package

#### Scenario: No checked-in generated files required
- **WHEN** generated files are produced only at build time
- **THEN** the package still compiles

### Requirement: codegen3 is selectable and reversible

The compiler SHALL allow code generation to be selected per compilation and SHALL NOT mix backends within one package. The established backend SHALL remain available until codegen3 reaches behavioral parity.

#### Scenario: Explicit backend selection
- **WHEN** the user selects codegen3
- **THEN** all files in that package are generated by codegen3

#### Scenario: Rollback
- **WHEN** the user selects the established backend
- **THEN** the package is generated without codegen3 and remains buildable
