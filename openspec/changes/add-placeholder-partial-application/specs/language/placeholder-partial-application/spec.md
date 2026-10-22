## Purpose

Defines how a bare `_` in a direct call argument produces a function that
supplies the remaining arguments, including typing, evaluation order, method
and constructor calls, and Go FFI fixed-arity calls.

## ADDED Requirements

### Requirement: Placeholder call syntax
A bare `_` used as a direct argument of a call SHALL be a placeholder. A
call containing one or more placeholders SHALL evaluate to a function whose
parameters correspond to the placeholders in source order, and whose body is
the original call with each placeholder replaced by the corresponding
parameter. A call without placeholders SHALL keep its existing meaning.

#### Scenario: One placeholder in a two-argument call
- **WHEN** a function `add(a: Int, b: Int) -> Int` is called as `add(1, _)`
- **THEN** the expression SHALL have type `func(Int) -> Int` and SHALL
  compute `add(1, x)` when the resulting function is applied to `x`

#### Scenario: Several placeholders keep source order
- **WHEN** `sub(a: Int, b: Int) -> Int` is called as `sub(_, _)`
- **THEN** the result SHALL be a two-parameter function `func(Int, Int) -> Int`
  that passes its first argument as `a` and its second as `b`

#### Scenario: Placeholder in an earlier argument position
- **WHEN** `sub(_, 3)` is applied to `8`
- **THEN** the result SHALL be `sub(8, 3)`

#### Scenario: A call without placeholders is unchanged
- **WHEN** an existing call supplies exactly the declared number of arguments
- **THEN** its typing, evaluation order, and generated behavior SHALL remain
  as before

### Requirement: Placeholder scope
A placeholder SHALL bind to the innermost enclosing call whose direct
argument list contains it. A placeholder SHALL only be accepted as a direct
argument. It SHALL NOT be accepted as a callee, as an operand of another
expression, as a field receiver, or as an argument to a compiler-intrinsic
call form.

#### Scenario: Nested calls bind to the innermost call
- **WHEN** `outer(inner(_), 2)` is evaluated
- **THEN** `inner(_)` SHALL produce a function that is supplied as the first
  argument of `outer`, and the outer call SHALL have no placeholder parameter

#### Scenario: Placeholder as an operand is rejected
- **WHEN** a call argument is written as `1 + _` or `_.Field`
- **THEN** compilation SHALL fail with a diagnostic stating that `_` is only
  supported as a direct call argument

#### Scenario: Placeholder in callee position is rejected
- **WHEN** source writes `_(1)`
- **THEN** compilation SHALL fail with a diagnostic stating that `_` cannot
  be called

#### Scenario: Placeholder in an intrinsic call is rejected
- **WHEN** source writes `Ref.new(_, value)`
- **THEN** compilation SHALL fail with a diagnostic stating that the
  intrinsic call form does not accept placeholders

### Requirement: Re-evaluation semantics
The generated function SHALL re-evaluate every already supplied argument each
time it is invoked. Arguments SHALL be evaluated left to right, and the
callee SHALL be evaluated before the arguments that remain in the call body.
No supplied argument SHALL be captured at the point where the placeholder
expression is created.

#### Scenario: Supplied side-effecting argument runs per invocation
- **WHEN** `f(next(), _)` is evaluated and the resulting function is invoked
  twice
- **THEN** `next()` SHALL run twice, once per invocation, in left-to-right
  argument order

#### Scenario: Callee is re-evaluated per invocation
- **WHEN** a placeholder call selects its callee through a side-effecting
  expression and the resulting function is invoked twice
- **THEN** the callee expression SHALL run once per invocation

### Requirement: Placeholder typing
Each placeholder SHALL take the type of the corresponding parameter in the
callee's instantiated signature. The result SHALL be a function over exactly
the placeholder parameter types, in source order, returning the callee's
result type. Generic callees SHALL be instantiated as they are for an
ordinary call before placeholder types are assigned. For a monomorphic
callee, a single placeholder covering the whole parameter list SHALL be an
eta-expansion with the callee's own function type. At code generation every
placeholder type SHALL be concrete. An unresolved type variable SHALL fail
with a diagnostic that asks for a type annotation or an immediate application,
rather than emitting a generic function value.

#### Scenario: Generic callee is instantiated first
- **WHEN** `identity[A](value: A) -> A` is called as `identity(_)` and the
  result is applied to a `String`
- **THEN** the result SHALL have type `func(String) -> String`

#### Scenario: Unresolved generic placeholder requires context
- **WHEN** `let g = identity(_)` is written with no surrounding context that
  fixes the type variable
- **THEN** compilation SHALL fail with a diagnostic asking for a type
  annotation or an immediate application of the resulting function

#### Scenario: Annotation fixes a generic placeholder
- **WHEN** `let g: func(String) -> String = identity(_)` is written
- **THEN** the binding SHALL be accepted with the same type as `identity`
  instantiated at `String`

#### Scenario: Function value supports a placeholder
- **WHEN** `let f = add` binds a value of type `func(Int, Int) -> Int` and
  the program evaluates `f(_, 2)(3)`
- **THEN** the call SHALL type-check and produce the same value as `f(3, 2)`

#### Scenario: Placeholder type mismatch is reported at the application site
- **WHEN** a placeholder stands for an `Int` parameter and the resulting
  function is applied to a `String`
- **THEN** compilation SHALL fail with a type mismatch that points at the
  application site

### Requirement: Methods and constructors
Placeholders SHALL be supported for inherent methods, interface impl methods,
static methods, and enum variant constructors. For a receiver method, the
receiver expression SHALL take part in the per-invocation evaluation order
described above. Implicit dictionary arguments introduced by `using`
constraints SHALL be resolved at the call site and SHALL NOT be re-resolved
per invocation.

#### Scenario: Receiver method with a placeholder
- **WHEN** `box.Map(_, f)` selects a method whose remaining parameter is a
  value of type `B`
- **THEN** the expression SHALL produce a function `func(B) -> C`, and the
  generated call SHALL use the same impl-helper dispatch as `box.Map(value, f)`

#### Scenario: Receiver is re-evaluated per invocation
- **WHEN** the receiver in a placeholder method call is a side-effecting
  expression and the resulting function is invoked twice
- **THEN** the receiver expression SHALL run twice

#### Scenario: Enum constructor placeholder
- **WHEN** `Some(_)` is applied to `1`
- **THEN** the result SHALL be `Some(1)` with the same type as an ordinary
  `Some(1)` call

### Requirement: Go FFI fixed-arity calls
Placeholders SHALL be supported for Go FFI package functions, Go FFI methods,
and values of named Go function types when the callee has a fixed arity. The
generated function SHALL preserve the existing Go boundary conversion for the
callee's results, including `(T, error)` to `Result[T, error]`,
`(T, bool)` to `Option[T]`, and lone-error results to
`Result[(), error]`.

#### Scenario: Go package function with a fixed arity
- **WHEN** a fixed-arity Go function `ParseInt(s string, base int) (int, error)`
  is called as `ParseInt(_, 10)` and the result is applied to `"42"`
- **THEN** the expression SHALL have a function type returning
  `Result[Int, error]` and SHALL perform the same boundary conversion as a
  direct call

#### Scenario: Go FFI method value
- **WHEN** a fixed-arity method on an imported Go type is called with a
  placeholder
- **THEN** the generated function SHALL invoke the same method with the same
  argument list and boundary conversion as the corresponding direct call

#### Scenario: Named Go function type value
- **WHEN** a value of a named Go function type with a fixed arity is called
  with a placeholder
- **THEN** the expression SHALL be accepted and the generated function SHALL
  call the value directly

### Requirement: Variadic calls reject placeholders
A call whose callee has a Go variadic signature SHALL reject any placeholder
in that call. The diagnostic SHALL identify the variadic callee and the
placeholder position. Variadic calls without placeholders SHALL keep their
existing behavior.

#### Scenario: Placeholder in a variadic call
- **WHEN** source writes `fmt.Println(_)` or `fmt.Sprintf(format, _)`
- **THEN** compilation SHALL fail with a diagnostic stating that placeholders
  are not supported for variadic calls

#### Scenario: Variadic call without a placeholder
- **WHEN** source writes `fmt.Println("value")`
- **THEN** the call SHALL compile and behave as before

### Requirement: Existing underscore meanings are preserved
An underscore in a let binding, a tuple destructuring pattern, a switch
pattern, a struct pattern, or a lambda parameter list SHALL keep its existing
discard or wildcard meaning. The placeholder rule SHALL apply only to a bare
`_` expression in a direct call argument.

#### Scenario: Let discard remains a discard
- **WHEN** source writes `let _ = f(1, 2)`
- **THEN** the call SHALL be an ordinary call and the result SHALL be
  discarded

#### Scenario: Wildcard pattern remains a wildcard
- **WHEN** source writes `case _ => body`
- **THEN** the case SHALL remain an unconditional wildcard branch

#### Scenario: Tuple discard remains a discard
- **WHEN** source writes `let (_, value) = pair`
- **THEN** the first component SHALL be discarded and `value` SHALL be bound
  as before

### Requirement: Source and editor preservation
The lossless CST SHALL preserve the written `_` token, and formatting a
placeholder call SHALL round-trip byte for byte. Editor analysis SHALL NOT
report a placeholder as an unknown identifier and SHALL expose its inferred
parameter type for hover and diagnostics.

#### Scenario: Formatter round-trip
- **WHEN** a formatted file contains `add(1, _)`
- **THEN** formatting SHALL leave the call unchanged

#### Scenario: Editor does not report an unknown identifier
- **WHEN** the language service analyses `add(1, _)`
- **THEN** it SHALL NOT report `_` as an unknown identifier

#### Scenario: Hover exposes the placeholder parameter type
- **WHEN** the language service hovers the `_` in `add(1, _)`
- **THEN** it SHALL report the remaining parameter type `Int`
