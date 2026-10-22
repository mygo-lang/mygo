# bootstrap-diagnostics Specification

## Purpose

Establish a consistent diagnostic contract for the bootstrap compiler, preserving truthful source provenance through parsing, inference, imports, generation, and Go builds so users can find the source responsible for a failure.

## Requirements

### Requirement: Source failures retain precise primary locations

Bootstrap SHALL report every source-related failure produced before final Go compilation with the real source filename and positive, one-based line and UTF-8 byte column of the most specific responsible source construct. This SHALL cover syntax, declaration validation, expressions, constraints, imports, FFI, and generated Go rendering. Final Go build diagnostics are governed by the generated-code requirement below. Outer function or package context MUST NOT replace an already known more specific location. Message punctuation MUST NOT determine whether a diagnostic is located.

#### Scenario: Nested expression failure containing a colon
- **WHEN** an expression inside a function fails with an explanation containing a colon
- **THEN** the primary location identifies that expression and the function is supplementary context

#### Scenario: Declaration and deferred constraint failures
- **WHEN** a declaration is invalid or a constraint fails after its expression has been traversed
- **THEN** the diagnostic identifies the responsible declaration component or originating constraint use

### Requirement: Source identity survives transformations and package boundaries

Bootstrap MUST preserve original file identity and source provenance across AST transformations, synthetic nodes, prelude injection, ordinary and external test packages, and dependency compilation. Conflicts involving two source constructs SHALL include the other construct as a related location where known. Imports SHALL retain the triggering import location without replacing a dependency's own failure location.

#### Scenario: Dependency failure
- **WHEN** an imported package contains an invalid expression
- **THEN** its source file and expression coordinates are primary and the importing declaration is related context

#### Scenario: Unresolvable import
- **WHEN** a source import cannot be resolved or participates in an import cycle
- **THEN** the diagnostic identifies the triggering import and explains the unresolved path or cycle

#### Scenario: Cross-file conflict and synthetic lowering
- **WHEN** a conflict spans two files or a failing generated construct originates in a source transformation
- **THEN** the diagnostic retains the actual source origins, including the other declaration for a cross-file conflict

### Requirement: Coordinates are truthful and deterministic

Coordinates produced before final Go compilation SHALL use one-based lines and UTF-8 byte columns consistently; EOF SHALL identify the actual end of input. Final build diagnostics SHALL preserve the source file and line, while their column MAY be approximate because it follows generated formatting. Source names supplied to source-string compilation SHALL be retained, with `<input>` reserved for unnamed input. Missing provenance MUST NOT be disguised using invented `1:1`, zero coordinates, an arbitrary first file, or a substituted filename on unrelated coordinates.

#### Scenario: Unicode and EOF
- **WHEN** an error follows non-ASCII characters, a tab, or CRLF input, or occurs at EOF
- **THEN** its coordinates resolve to the responsible source byte or the actual EOF using the documented convention

#### Scenario: Named source string
- **WHEN** compilation receives an explicit source name
- **THEN** errors retain that name and positions within that source

### Requirement: Generated Go failures remain traceable

Bootstrap SHALL make failures in generated Go rendering traceable to the responsible MyGO construct with exact file, line and UTF-8 byte column coordinates. Emitted Go SHALL carry line directives that make subsequent Go compilation report the responsible MyGO file and source line. The final build column MAY be approximate when the failing token's position follows generated formatting; it MUST NOT be treated as an exact source column. Diagnostics SHALL retain underlying tool explanations. Compiler-created support code without a direct source construct MUST be explicitly attributed to its generated file and MUST NOT invent a MyGO position.

#### Scenario: Go compiler rejects emitted source
- **WHEN** the final build rejects a Go expression emitted from a MyGO construct
- **THEN** the failure identifies that MyGO file and line and preserves the Go explanation
- **AND** its column may reflect generated formatting, while an earlier rendering failure for the same construct retains an exact source column

#### Scenario: Rendering or generated support failure
- **WHEN** rendering fails for a source-backed node or a Go failure occurs in compiler-created support code
- **THEN** the former identifies its exact source origin and the latter explicitly identifies generated support and any known originating sources

### Requirement: Operational failures identify their actual context

Failures without a natural source position SHALL identify the stage, operation, affected path or tool, and underlying reason, and explicitly indicate that no source location is available. A known source trigger SHALL be supplied as a related location. Operational errors MUST NOT acquire fabricated source coordinates.

#### Scenario: Filesystem or tool failure
- **WHEN** reading a directory, writing generated output, or starting the Go tool fails without a source trigger
- **THEN** output identifies the operation and target, preserves the cause, and states that no source location is available

### Requirement: Public boundaries present consistent diagnostics

Bootstrap SHALL use structured diagnostic data internally and render source errors with a leading `file:line:column: error [code]: message` followed by related notes and stage context. Existing public Go compilation function signatures and nonzero CLI failure behavior SHALL remain compatible. Underlying type mismatches SHALL retain expected and actual types, and output ordering SHALL be deterministic.

#### Scenario: Same failure through library and CLI
- **WHEN** the same source fails through a public compilation entry point and through the CLI
- **THEN** both expose the same primary source identity, coordinates, code and explanation without duplicate location prefixes

#### Scenario: Type mismatch
- **WHEN** two incompatible types cause compilation to fail
- **THEN** output names both types and identifies the offending source construct with related origins when available
