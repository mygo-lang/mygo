## ADDED Requirements

### Requirement: Chained receiver-method calls spell non-inferable type arguments
When bootstrap lowers a receiver-style impl or typeclass method call whose
helper has a type parameter that Go cannot infer, it SHALL emit the helper's
explicit type arguments resolved from the inferred receiver type, even when the
call is part of a chain and the source declares no type annotation on the
receiver or the result. A method's type parameter is non-inferable when it
appears only inside an interface-shaped container such as `Option[A]` or
`Result[A, E]` and not in any position Go can unify from a bare argument. The
generated Go SHALL compile without requiring the user to add an intermediate
annotation.

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

### Requirement: Unresolved impl-method dispatch is diagnosed before Go output
When bootstrap lowers a receiver-style impl or typeclass method call and cannot
resolve the type arguments required by the generated helper, it SHALL report a
generation diagnostic that identifies the method and its source location. It
SHALL NOT emit a Go receiver-method selector such as `x.Unwrap()` for a method
that has no Go method, and SHALL NOT fall back to a plain helper call that relies
on Go's inference for type parameters appearing only inside interface-shaped
containers. Bootstrap SHALL fail generation instead of reporting success for a
source file that produces uncompilable Go.

#### Scenario: Unknown receiver type during dispatch
- **WHEN** a receiver-style method call is lowered while the receiver's inferred
  type is not concrete enough to resolve the required type arguments
- **THEN** bootstrap SHALL fail with a diagnostic naming the method and source
  position and SHALL NOT emit a Go method-selector call for that method

#### Scenario: Plain-call fallback is not silently accepted
- **WHEN** the resolved number of type arguments is fewer than the helper
  requires
- **THEN** bootstrap SHALL report the unresolved type-argument condition rather
  than emitting a plain helper call whose Go compilation depends on inference
