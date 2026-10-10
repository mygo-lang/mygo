## ADDED Requirements

### Requirement: Render compiler channel types in the dynamic Go API
The dynamic Go API SHALL render the compiler channel types `Chan[T]`, `SendChan[T]`, and `RecvChan[T]` as the Go channel types `chan T`, `chan<- T`, and `<-chan T`. It SHALL NOT render them as a qualified named type from another package, and SHALL NOT depend on that package exporting a matching Go type. This applies to every type the dynamic API renders, including constructor payload types, dispatch parameters, and results.

#### Scenario: Render a bidirectional channel payload
- **WHEN** the dynamic API renders an implementation whose target is `Chan[T]`
- **THEN** the generated Go uses `chan T` and does not reference a `Chan` named type

#### Scenario: Render a send-only channel payload
- **WHEN** the dynamic API renders an implementation whose target is `SendChan[T]`
- **THEN** the generated Go uses `chan<- T`

#### Scenario: Render a receive-only channel payload
- **WHEN** the dynamic API renders an implementation whose target is `RecvChan[T]`
- **THEN** the generated Go uses `<-chan T`

#### Scenario: Read generated declarations back into channel types
- **WHEN** the compiler converts the generated dynamic Go API back into MyGO declarations for inference
- **THEN** `chan T`, `chan<- T`, and `<-chan T` are reconstructed as `Chan[T]`, `SendChan[T]`, and `RecvChan[T]`

#### Scenario: Reconstruct channel types from imported metadata
- **WHEN** an imported package's metadata describes a channel type and the compiler reconstructs that package's exported declarations
- **THEN** the reconstructed declaration is the corresponding channel type rather than a named type from another package

### Requirement: Emit dynamic constructors only for usable implementations
The compiler SHALL emit a dynamic constructor for an implementation only when the implementation can be dispatched through or the compiling package references that interface's dynamic API. An implementation whose interface exposes no dynamically supported method and that the compiling package does not reference SHALL NOT produce a constructor. When neither any constructor nor any dispatch function qualifies, the package SHALL emit no dynamic API declarations.

#### Scenario: Skip implementations with no dispatchable operation
- **WHEN** imported metadata describes an implementation whose interface has no dynamically supported method and the compiling package does not reference that interface's dynamic API
- **THEN** no dynamic constructor is generated for that implementation and the package's dynamic API declarations stay empty

#### Scenario: Keep constructors the package uses
- **WHEN** the compiling package references an interface's dynamic API, or the interface has at least one dynamically supported method
- **THEN** the corresponding dynamic constructors are generated

#### Scenario: Compile a project that only imports the concurrency library
- **WHEN** a MyGO project imports a library that declares channel implementations but the project uses no dynamic interface
- **THEN** the project's generated dynamic API declarations reference no non-existent channel type and `go build ./...` succeeds
