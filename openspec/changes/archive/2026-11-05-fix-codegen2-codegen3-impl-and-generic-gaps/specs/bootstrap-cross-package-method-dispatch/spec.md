## ADDED Requirements

### Requirement: Consumer-declared impls on imported types remain callable
When a consumer package declares an impl whose target type belongs to an
imported MyGO package, bootstrap inference and codegen SHALL resolve that impl
and lower its receiver method calls to the helper emitted in the declaring
consumer package. The generated Go SHALL NOT require a Go method on the
imported type.

#### Scenario: A consumer implements an imported struct
- **WHEN** package `app` imports `lib`, declares `impl lib.Box` with an instance
  method `Double`, and calls `box.Double()` on a value returned by `lib.Make`
- **THEN** inference SHALL accept the call
- **AND** generated Go SHALL invoke `app`'s emitted `Double` helper instead of
  emitting `box.Double()`
- **AND** `go build` SHALL succeed

#### Scenario: The same lowering is used by codegen2 and codegen3
- **WHEN** the consumer-declared imported-type impl fixture is compiled through
  codegen2 and through codegen3
- **THEN** both paths SHALL accept it and produce buildable Go with equivalent
  runtime results

### Requirement: Receiver identity distinguishes same-named package types
Impl lookup SHALL identify a receiver by both its defining package and its
member name. A consumer-local type and an imported type with the same short
name SHALL select their own impl methods and helpers.

#### Scenario: Local Box and imported lib.Box coexist
- **WHEN** a consumer declares its own `Box`, imports `lib.Box`, and declares
  impl methods with the same method name on each type
- **THEN** a call on the local value SHALL select the local impl
- **AND** a call on `lib.Box` SHALL select the impl targeting `lib.Box`
- **AND** the generated Go SHALL compile without an ambiguous method lookup
