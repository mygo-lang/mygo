## ADDED Requirements

### Requirement: Chained receiver-method calls spell non-inferable type arguments
Bootstrap SHALL apply the type arguments written at a call site during type
inference, binding the callee's type parameters to those types, so the call's
inferred type is concrete even when the type parameter appears only in the
return type. When it then lowers a receiver-style impl or typeclass method call
whose helper has a type parameter that Go cannot infer, it SHALL emit the
helper's explicit type arguments resolved from the inferred receiver type, even
when the call is part of a chain and the source declares no type annotation on
the receiver or the result. A method's type parameter is non-inferable when it
appears only inside an interface-shaped container such as `Option[A]` or
`Result[A, E]` and not in any position Go can unify from a bare argument. The
generated Go SHALL compile without requiring the user to add an intermediate
annotation.

#### Scenario: Written type argument is the only source of a type parameter
- **WHEN** valid MyGO source calls `empty[Result[Int, String]]()` where `empty`
  is `func empty[A]() -> Option[A]` (the type parameter appears only in the
  return type) and then evaluates `.Unwrap().Unwrap()`, with no declared type on
  any binding
- **THEN** bootstrap SHALL infer the call as `Option[Result[Int, String]]` and
  the generated helpers SHALL carry `[Result[int, string]]` and `[int, string]`,
  and the generated Go SHALL compile with `go build`

#### Scenario: Nested Option and Result unwrap on a channel receive
- **WHEN** valid MyGO source binds `let results = concurrency.MakeChan[Result[PostRecord, String]](2)`
  and then evaluates `results.Receive().Unwrap().Unwrap()` with no declared type
  on any binding
- **THEN** the generated call chain SHALL pass `Result[PostRecord, string]` to the
  option unwrap helper and `[PostRecord, string]` to the result unwrap helper, and
  the generated Go SHALL compile with `go build`

#### Scenario: Receiver defined in the Prelude
- **WHEN** a non-annotated chain unwraps an `Option[Result[Int, String]]` value
  produced by another method call and all types come from the Prelude
- **THEN** the generated helpers SHALL carry the explicit type arguments
  `Result[int, string]` and `[int, string]` and the generated Go SHALL compile

#### Scenario: Declared annotation no longer changes codegen
- **WHEN** the same chain is written either with or without a declared receiver
  type such as `Option[Result[PostRecord, String]]`
- **THEN** both forms SHALL emit equivalent helper calls carrying the same
  explicit type arguments and both SHALL compile
