# Spec Delta

## MODIFIED Requirements

### Requirement: Package a value with its typeclass evidence
The compiler SHALL package a concrete value of type `T` together with the selected implementation of dynamic interface `I` whenever either an explicit `as I` conversion is used or a supported expression has an expected type of `I`. Packaging SHALL require exactly one compatible implementation and available constructor metadata, and SHALL preserve the association between the payload and its evidence.

#### Scenario: Package a concrete implementation
- **WHEN** a value has type `T`, exactly one implementation satisfies interface `I[T]`, and the source explicitly converts the value with `as I`
- **THEN** generated Go SHALL call the dynamic constructor for `I` and the selected implementation rather than emit a Go type conversion

#### Scenario: Implicitly package at a known target type
- **WHEN** an expression of concrete type `T` is used at a supported site whose expected type is dynamic interface `I`, and exactly one implementation satisfies `I[T]`
- **THEN** the compiler SHALL infer the packaging without requiring `as I`, and generated Go SHALL call the matching dynamic constructor

#### Scenario: Use dynamic values as function arguments
- **WHEN** a concrete value with a unique implementation is passed to a parameter of dynamic interface type `I`
- **THEN** the argument SHALL be packaged for `I` and the generated Go call SHALL pass the resulting wrapper

#### Scenario: Reject missing or ambiguous evidence
- **WHEN** a value is explicitly or implicitly packaged but no unique implementation of the requested interface applies to its concrete type
- **THEN** compilation SHALL report the missing or ambiguous implementation at the conversion or expected-type use site

#### Scenario: Reject unavailable constructor metadata
- **WHEN** packaging requires a dynamic constructor that is absent or incompatible in imported package metadata
- **THEN** compilation SHALL report the dependency and the required constructor metadata or ABI version

#### Scenario: Do not infer packaging without a target
- **WHEN** concrete expressions have no expected dynamic interface type, such as branches of an unannotated conditional expression
- **THEN** the compiler SHALL NOT choose a dynamic interface or implementation solely to join those expressions

#### Scenario: Preserve ordinary static typeclass calls
- **WHEN** a typeclass method is called on a value whose concrete type is statically known and no dynamic interface target is expected
- **THEN** existing static typeclass resolution and generated dispatch SHALL remain unchanged
