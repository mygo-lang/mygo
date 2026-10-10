# language/dynamic-typeclass-values Specification

## Purpose

Allow values of different concrete types that implement the same MyGO typeclass interface to share a library-level runtime representation, while retaining the selected implementation and supporting callers that compile against already-built dependency packages.

## Requirements

### Requirement: Package a value with its typeclass evidence
The compiler SHALL package a concrete value of type `T` together with the selected implementation of dynamic interface `I` when an explicit `as I` conversion is used or a supported expression has expected type `I`. Packaging SHALL require exactly one compatible implementation and available constructor metadata, and SHALL preserve the association between the payload and its evidence.

#### Scenario: Package a concrete implementation
- **WHEN** a value has type `T`, exactly one implementation satisfies interface `I[T]`, and the source explicitly converts the value with `as I`
- **THEN** generated Go calls the dynamic constructor for `I` and the selected implementation rather than emitting a Go type conversion

#### Scenario: Implicitly package at a known target type
- **WHEN** an expression of concrete type `T` is used at a supported site whose expected type is dynamic interface `I`, and exactly one implementation satisfies `I[T]`
- **THEN** the compiler infers packaging without requiring `as I`, and generated Go calls the matching dynamic constructor

#### Scenario: Use dynamic values as function arguments
- **WHEN** a concrete value with a unique implementation is passed to a parameter of dynamic interface type `I`
- **THEN** the argument is packaged for `I` and the generated Go call passes the resulting wrapper

#### Scenario: Reject missing or ambiguous evidence
- **WHEN** a value is explicitly or implicitly packaged but no unique implementation of the requested interface applies to its concrete type
- **THEN** compilation reports the missing or ambiguous implementation at the conversion or expected-type use site

#### Scenario: Reject unavailable constructor metadata
- **WHEN** packaging requires a dynamic constructor that is absent or incompatible in imported package metadata
- **THEN** compilation reports the dependency and the required constructor metadata or ABI version

#### Scenario: Do not infer packaging without a target
- **WHEN** concrete expressions have no expected dynamic interface type, such as branches of an unannotated conditional expression
- **THEN** the compiler does not choose a dynamic interface or implementation solely to join those expressions

#### Scenario: Preserve ordinary static typeclass calls
- **WHEN** a typeclass method is called on a value whose concrete type is statically known and no dynamic interface target is expected
- **THEN** existing static typeclass resolution and generated dispatch remain unchanged

### Requirement: Store packages for one interface uniformly
Packages for the same typeclass interface SHALL have a common library-level runtime type regardless of their concrete payload types, and SHALL be usable as elements of the same collection and as function arguments or results.

#### Scenario: Generate one ordinary wrapper API per interface
- **WHEN** the compiler compiles a package that declares a typeclass interface `I`
- **THEN** it emits an ordinary wrapper type and ordinary construction/access functions for `I`, without requiring a Go interface declaration or new MyGO syntax

#### Scenario: Store distinct implementations in one collection
- **WHEN** values of two different concrete types implement the same interface and are packaged
- **THEN** both packages can be stored in one collection and dispatched through the library API

### Requirement: Dispatch through the packaged implementation
The library API SHALL invoke methods through the implementation evidence associated with the packaged value. A dynamic operation MAY use the package's stored payload for the operand whose type is hidden by the package. It SHALL NOT require callers to provide another value of that hidden type or open/pattern-match the package. This capability SHALL NOT add or change MyGO syntax.

Implementation adapters SHALL be ordinary package-level functions that call the package-level helpers generated from MyGO `impl` methods. They SHALL NOT depend on Go interface method sets or methods attached to the implementation struct.

#### Scenario: Dispatch one packaged value
- **WHEN** a caller invokes a dynamically supported operation on a package
- **THEN** the operation dispatches to the adapter for that package's selected implementation

#### Scenario: Keep additional hidden-type operands static
- **WHEN** an interface method requires another argument of the package's hidden type, such as `Equals(left: A, right: A)` when the package supplies only one `A`
- **THEN** the dynamic API does not expose that call, while ordinary static typeclass dispatch remains available when `A` is known

#### Scenario: Repackage a hidden-type result
- **WHEN** a dynamically supported method returns a value of the package's hidden type and the same implementation evidence applies to that result
- **THEN** the library API returns the result as a dynamic package carrying that evidence

#### Scenario: Preserve static typeclass calls
- **WHEN** a typeclass method is called on a value whose concrete type is statically known
- **THEN** existing typeclass resolution and dispatch behavior remain unchanged

### Requirement: Publish package metadata with generated Go sources
A compiled MyGO package SHALL include compiler-readable metadata in its generated Go source artifacts. The metadata SHALL describe the exported MyGO declarations and dynamic implementation adapters needed by a caller, and SHALL be consumable without the dependency's `.mygo` source files.

#### Scenario: Compile against an implementation package without MyGO sources
- **WHEN** an implementation package has generated Go sources containing compatible MyGO metadata and the caller imports that package
- **THEN** the caller can resolve the exported declarations and dynamic implementation adapters without reading dependency `.mygo` files

#### Scenario: Add a compiled implementation to a caller
- **WHEN** a caller adds a dependency on a package whose metadata describes a compiled implementation of interface `I`
- **THEN** recompiling the caller makes that implementation available to its dynamic dispatch without recompiling unchanged dependencies

#### Scenario: Reject missing or incompatible metadata
- **WHEN** dynamic packaging or dispatch requires metadata that is missing or has an incompatible schema or adapter ABI
- **THEN** compilation reports the dependency and the required metadata or ABI version

### Requirement: Support generic interface methods through generated adapters
For each dynamically supported generic interface method that does not require additional caller-supplied hidden-type values, the implementation package SHALL publish an adapter that retains the method's type parameters, and the caller SHALL dispatch through that adapter using the call-site type arguments. Method parameters that nest the hidden type, such as callbacks accepting `A`, are outside the dynamic API unless a separate adapter ABI is specified.

An implementation whose target contains hidden implementation type parameters MAY still be packaged. A generic interface method that would require capturing such a hidden type parameter in a Go generic function value SHALL be diagnosed as unsupported for dynamic invocation at compile time.

#### Scenario: Dispatch a generic method from a compiled dependency
- **WHEN** a caller dynamically invokes a supported generic method and the dependency metadata describes its adapter
- **THEN** the caller-generated dispatch instantiates that adapter with the call-site method type arguments

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
