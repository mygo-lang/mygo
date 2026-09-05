## Purpose

Defines the combinator surface of the prelude `Result[A, E]` and `Option[A]` types:
predicates, transforms, chaining, extraction, and conversion methods modeled after the
Rust standard library, so error-bearing flows read without exhaustive `switch`
boilerplate.

## ADDED Requirements

### Requirement: Result matches via predicates
The prelude SHALL provide `IsOk` and `IsErr` methods on `Result[A, E]` returning
whether the value is the `Ok` or `Err` variant. The methods SHALL NOT evaluate or
consume the payload.

#### Scenario: Ok value predicates
- **WHEN** a `Result` value is `Ok(v)`
- **THEN** `IsOk()` is `true`, `IsErr()` is `false`, and `v` remains accessible

#### Scenario: Err value predicates
- **WHEN** a `Result` value is `Err(e)`
- **THEN** `IsOk()` is `false`, `IsErr()` is `true`, and `e` remains accessible

### Requirement: Result map family
The prelude SHALL provide `Map`, `MapErr`, `MapOr`, and `MapOrElse` methods on
`Result[A, E]`. `Map` applies a function to the `Ok` payload; `MapErr` applies a
function to the `Err` payload. `MapOr` converts `Ok(v)` via a function or returns a
`defaultVal` on `Err`; `MapOrElse` returns the result of a function of the error on
`Err`. The other variant SHALL pass through unchanged.

#### Scenario: Map transforms Ok payload
- **WHEN** `Ok(v).Map(fn)` is called
- **THEN** the result is `Ok(fn(v))`

#### Scenario: Map passes Err through
- **WHEN** `Err(e: E).Map(fn: func(A) -> B)` is called
- **THEN** the result is `Err(e)` with error type `E` unchanged

#### Scenario: MapErr transforms Err payload
- **WHEN** `Err(e).MapErr(fn)` is called
- **THEN** the result is `Err(fn(e))`

#### Scenario: MapOr supplies default on Err
- **WHEN** `Err(e).MapOr(defaultVal, fn)` is called
- **THEN** the result is `defaultVal`

#### Scenario: MapOrElse computes fallback from error
- **WHEN** `Err(e).MapOrElse(fn: func(E) -> A, mapFn)` is called
- **THEN** the result is `fn(e)`

### Requirement: Result chaining
The prelude SHALL provide `AndThen`, `OrElse`, `And`, `Or`, and `Flatten` methods on
`Result`. `AndThen` applies a `Result`-returning function to `Ok`; `OrElse` applies a
`Result`-returning function to `Err`. `And` keeps the second `Result` when the receiver
is `Ok`; `Or` keeps the second `Result` when the receiver is `Err`. `Flatten` collapses
`Result[Result[A, E], E]` to `Result[A, E]`.

#### Scenario: AndThen chains Ok results
- **WHEN** `Ok(v).AndThen(fn: func(A) -> Result[B, E])` is called
- **THEN** the result is `fn(v)`

#### Scenario: AndThen short-circuits Err
- **WHEN** `Err(e).AndThen(fn)` is called
- **THEN** the result is `Err(e)` and `fn` is not invoked

#### Scenario: OrElse recovers from Err
- **WHEN** `Err(e).OrElse(fn: func(E) -> Result[A, E2])` is called
- **THEN** the result is `fn(e)`

#### Scenario: OrElse passes Ok through
- **WHEN** `Ok(v).OrElse(fn)` is called
- **THEN** the result is `Ok(v)` and `fn` is not invoked

#### Scenario: Flatten collapses nested Ok
- **WHEN** `Ok(Ok(v)).Flatten()` is called
- **THEN** the result is `Ok(v)`

#### Scenario: Flatten preserves Err
- **WHEN** `Err(e: E).Flatten()` is called on `Result[Result[A, E], E]`
- **THEN** the result is `Err(e)`

### Requirement: Result extraction
The prelude SHALL provide `Unwrap`, `UnwrapOr`, `UnwrapOrElse`, `UnwrapErr`,
`Expect`, and `ExpectErr` methods on `Result`. `Unwrap`, `UnwrapOr`, `UnwrapOrElse`,
and `Expect` extract the `Ok` payload. `UnwrapErr` and `ExpectErr` extract the `Err`
payload. The unwrap variants SHALL panic on the unexpected variant; `Expect` and
`ExpectErr` SHALL panic including the supplied message.

#### Scenario: Unwrap returns Ok payload
- **WHEN** `Ok(v).Unwrap()` is called
- **THEN** the result is `v`

#### Scenario: Unwrap panics on Err
- **WHEN** `Err(e).Unwrap()` is called
- **THEN** program execution panics

#### Scenario: Expect panics with message on Err
- **WHEN** `Err(e).Expect("db failed")` is called
- **THEN** program execution panics and the panic message includes "db failed"

#### Scenario: UnwrapOr returns default on Err
- **WHEN** `Err(e).UnwrapOr(defaultVal)` is called
- **THEN** the result is `defaultVal`

#### Scenario: UnwrapOrElse computes from error
- **WHEN** `Err(e).UnwrapOrElse(fn: func(E) -> A)` is called
- **THEN** the result is `fn(e)`

#### Scenario: UnwrapErr returns Err payload
- **WHEN** `Err(e).UnwrapErr()` is called
- **THEN** the result is `e`

#### Scenario: UnwrapErr panics on Ok
- **WHEN** `Ok(v).UnwrapErr()` is called
- **THEN** program execution panics

### Requirement: Default-constrained extraction (DEFERRED)
> **Deferred during implementation**: the prelude SHALL NOT ship
> `UnwrapOrDefault` in this change.  The gated smoke test (task 4.1) recorded
> that inherent-impl `using Default[A]` dispatch does not resolve
> (`unknown identifier Default`); the methods were dropped and the blocker was
> recorded in `KNOWN_ISSUES.md`.  The requirement stays on the books for a
> follow-up change once the compiler supports zero-arg typeclass members on
> inherent impls.

The prelude SHALL provide `UnwrapOrDefault` on `Result[A, E]` where `A` implements
`Default`, returning the payload on `Ok` and `Default[A]()` on `Err`.

#### Scenario: UnwrapOrDefault returns payload on Ok
- **WHEN** `Ok(v).UnwrapOrDefault()` is called with `A` implementing `Default`
- **THEN** the result is `v`

#### Scenario: UnwrapOrDefault returns default on Err
- **WHEN** `Err(e).UnwrapOrDefault()` is called with `A` implementing `Default`
- **THEN** the result is `Default[A]()`

### Requirement: Result and Option conversion
The prelude SHALL provide `ToOption` (existing) and `ToErr` on `Result`, and `OkOr`
and `OkOrElse` on `Option`. `Result.ToOption` yields `Some` for `Ok`; `Result.ToErr`
yields `Some` for `Err`. `Option.OkOr` converts `Some(v)` to `Ok(v)` and `None` to
`Err(errVal)`; `Option.OkOrElse` computes the error from a function on `None`.

#### Scenario: ToErr yields Some for Err
- **WHEN** `Err(e).ToErr()` is called
- **THEN** the result is `Some(e)`

#### Scenario: ToErr yields None for Ok
- **WHEN** `Ok(v).ToErr()` is called
- **THEN** the result is `None`

#### Scenario: OkOr converts None to Err
- **WHEN** `None.OkOr(errVal: E)` is called on `Option[A]`
- **THEN** the result is `Err(errVal)`

#### Scenario: OkOr converts Some to Ok
- **WHEN** `Some(v).OkOr(errVal)` is called
- **THEN** the result is `Ok(v)`

#### Scenario: OkOrElse computes error from function
- **WHEN** `None.OkOrElse(fn: func() -> E)` is called
- **THEN** the result is `Err(fn())`

### Requirement: Result transpose
The prelude SHALL provide a standalone conversion transforming
`Option[Result[A, E]]` into `Result[Option[A], E]`: `Some(Ok(v))` becomes `Ok(Some(v))`,
`Some(Err(e))` becomes `Err(e)`, and `None` becomes `Ok(None)`.

#### Scenario: Transpose Some Ok to Ok Some
- **WHEN** `Some(Ok(v))` is transposed
- **THEN** the result is `Ok(Some(v))`

#### Scenario: Transpose None to Ok None
- **WHEN** `None` of type `Option[Result[A, E]]` is transposed
- **THEN** the result is `Ok(None)`

#### Scenario: Transpose Some Err to Err
- **WHEN** `Some(Err(e))` is transposed
- **THEN** the result is `Err(e)`

### Requirement: Option predicates
The prelude SHALL provide `IsSome` and `IsNone` methods on `Option[A]` returning
whether the value is the `Some` or `None` variant.

#### Scenario: Some predicates
- **WHEN** an `Option` value is `Some(v)`
- **THEN** `IsSome()` is `true` and `IsNone()` is `false`

#### Scenario: None predicates
- **WHEN** an `Option` value is `None`
- **THEN** `IsSome()` is `false` and `IsNone()` is `true`

### Requirement: Option chaining and extraction
The prelude SHALL provide `AndThen`, `OrElse`, `Flatten`, `Unwrap`, `UnwrapOrElse`,
`UnwrapOrDefault`, `Expect`, `MapOr`, and `MapOrElse` methods on `Option[A]` with the
same shape as their `Result` counterparts: chaining functions run only on `Some`;
unwrap variants panic on `None`; `Expect` includes the supplied message.

#### Scenario: AndThen chains Some
- **WHEN** `Some(v).AndThen(fn: func(A) -> Option[B])` is called
- **THEN** the result is `fn(v)`

#### Scenario: AndThen short-circuits None
- **WHEN** `None.AndThen(fn)` is called
- **THEN** the result is `None` and `fn` is not invoked

#### Scenario: Flatten collapses nested Some
- **WHEN** `Some(Some(v)).Flatten()` is called
- **THEN** the result is `Some(v)`

#### Scenario: Unwrap panics on None
- **WHEN** `None.Unwrap()` is called
- **THEN** program execution panics

#### Scenario: UnwrapOrDefault returns default on None
- **WHEN** `None.UnwrapOrDefault()` is called with `A` implementing `Default`
- **THEN** the result is `Default[A]()`

## REMOVED Requirements

### Requirement: Standalone OptionFilter function
**Reason**: Duplicates `OptionIEnumerable.Filter`, which has identical behavior as a
typeclass method on `Option`.

**Migration**: Replace calls to `OptionFilter(opt, fn)` with method form `opt.Filter(fn)`.
