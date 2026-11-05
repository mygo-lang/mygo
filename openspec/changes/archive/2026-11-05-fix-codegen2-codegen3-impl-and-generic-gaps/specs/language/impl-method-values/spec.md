## Purpose

Defines how impl methods can be used as bound function values or unbound
receiver-first function values, including passing them to higher-order
functions and calling them immediately, with buildable behavior in both
bootstrap backends.

## ADDED Requirements

### Requirement: Bound method values capture the receiver once
For an impl method `Method` whose receiver is `T`, an expression `value.Method`
SHALL produce a function value with the receiver removed from its parameter
list. The receiver expression SHALL be evaluated once when the method value is
created, not on each invocation.

#### Scenario: A local method value is stored and called
- **WHEN** source binds `let fn = rect.Area` and later calls `fn()`
- **THEN** the generated Go SHALL invoke the mangled `Area` helper with `rect`
- **AND** the call SHALL return the same value as `rect.Area()`

#### Scenario: A temporary receiver is evaluated once
- **WHEN** a method value is created from an expression such as `makeBox().Size`
- **THEN** `makeBox()` SHALL run exactly once at method-value creation
- **AND** repeated calls to the resulting function SHALL reuse that receiver

### Requirement: Unbound method expressions expose the receiver parameter
For an impl method `Method` on `T`, the expression `T.Method` SHALL produce a
function value whose first parameter is the receiver, followed by the method's
declared parameters. `T.Method(receiver, args...)` SHALL be accepted as a
direct call of that function.

#### Scenario: A type-qualified method is stored as a function
- **WHEN** source binds `let fn: func(Rectangle) -> Int = Rectangle.Area`
- **THEN** the generated Go SHALL use the mangled `Area` helper as the function
  value
- **AND** calling `fn(rect)` SHALL equal `rect.Area()`

#### Scenario: A type-qualified instance method is called directly
- **WHEN** source calls `Rectangle.Area(rect)`
- **THEN** the generated Go SHALL directly call the mangled helper with `rect`
- **AND** SHALL NOT emit a Go method expression selector

### Requirement: Method values compose with higher-order functions
Impl method values SHALL be accepted wherever a function with the same
receiver-adjusted signature is expected, including call arguments and return
values.

#### Scenario: A bound method is passed as a callback
- **WHEN** a function expecting `func() -> Int` receives `rect.Area`
- **THEN** callers SHALL be able to invoke the callback and receive the method
  result

#### Scenario: An unbound method is passed as a callback
- **WHEN** a function expecting `func(Rectangle) -> Int` receives
  `Rectangle.Area`
- **THEN** the callback SHALL invoke the helper with its supplied receiver

### Requirement: Method-value resolution preserves field precedence
A selector without a call SHALL continue to resolve declared struct fields
before impl methods. A function-typed field SHALL remain a field read and SHALL
NOT be rewritten as an impl method value.

#### Scenario: A struct has a field with the method's name
- **WHEN** a receiver has a declared field named `Area` and also an impl method
  named `Area`
- **THEN** `receiver.Area` SHALL read the field
- **AND** only the method call form `receiver.Area()` SHALL invoke the method

### Requirement: Method values support qualified and generic impl methods
Method values SHALL retain the owning package qualification and any required
generic type arguments or typeclass dictionaries. Imported-type methods and
methods on generic receivers SHALL produce buildable Go equivalent to their
direct-call forms.

#### Scenario: A method value targets an imported type
- **WHEN** a consumer creates `lib.Box.Size` or binds a value returned by
  `lib.Make` as `let fn = box.Size`
- **THEN** generated Go SHALL reference the helper in the package that declares
  the impl
- **AND** `go build` SHALL succeed

#### Scenario: A generic method value is emitted
- **WHEN** an impl method on `Box[A]` is used as a value at a concrete receiver
  type
- **THEN** the generated helper SHALL receive the same explicit type arguments
  as the equivalent direct method call

### Requirement: Both bootstrap backends agree on method values
The codegen2 and codegen3 paths SHALL accept the same bound and unbound method
value fixtures and SHALL produce equivalent observable results.

#### Scenario: The method-value fixture runs on both backends
- **WHEN** one fixture stores, passes, and invokes both `value.Method` and
  `Type.Method`
- **THEN** both backends SHALL generate buildable Go
- **AND** both SHALL return the same values and evaluate receivers the same
  number of times
