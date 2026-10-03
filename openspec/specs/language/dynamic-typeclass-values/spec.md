# language/dynamic-typeclass-values Specification

## Purpose

Allow values of different concrete types that implement the same MyGO typeclass interface to share a library-level runtime representation, while retaining the selected implementation and supporting callers that compile against already-built dependency packages.

## Requirements

### Requirement: Package a value with its typeclass evidence
The library SHALL provide an ordinary API that packages a value of concrete type `T` together with the selected implementation of interface `I[T]`. Packaging SHALL require an unambiguous implementation and SHALL preserve the association between the payload and its evidence.

#### Scenario: Package a concrete implementation
- **WHEN** a value has type `T` and exactly one implementation satisfies interface `I[T]`
- **THEN** the library API can package the value for dynamic use through `I`

#### Scenario: Reject missing or ambiguous evidence
- **WHEN** a value is packaged but no unique implementation of the requested interface applies to its concrete type
- **THEN** compilation reports the missing or ambiguous implementation

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
