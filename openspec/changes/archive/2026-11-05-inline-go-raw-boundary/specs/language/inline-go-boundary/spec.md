## Purpose

Gives inline Go blocks an explicit way to declare the raw Go result shape the
snippet produces, so the compiler can adapt nilable pointers, comma-ok results,
and error-bearing results into MyGO `Option` and `Result` values without
changing the behavior of existing inline Go blocks.

## ADDED Requirements

### Requirement: Inline Go blocks accept an optional raw result type

An inline Go block SHALL accept an optional `raw: <type>` field alongside
`code:`, `in`, and `type` fields. The type in `go[T]` SHALL remain the
MyGO result type of the expression. When `raw:` is present, it SHALL declare
the Go-visible result shape produced by the snippet; the compiler SHALL adapt
that shape to the declared MyGO result type.

When `raw:` is absent, the Go-visible result shape SHALL be the declared
result type itself, preserving the behavior of every existing inline Go block.

#### Scenario: Omitting raw keeps an existing native Go result

- **WHEN** a block declares `go[Int] { code: "{x} + 1"; in x = n }` without
  `raw:`
- **THEN** the expression SHALL continue to lower as a raw Go `int` value and
  compile as before

#### Scenario: Declaring a raw shape different from the result type

- **WHEN** a block declares `go[Option[Ref[rsa.PublicKey]]]` with
  `raw: Ref[rsa.PublicKey]` and a snippet that yields a Go
  `*rsa.PublicKey`
- **THEN** the expression SHALL have MyGO type
  `Option[Ref[rsa.PublicKey]]`, the value SHALL be observed as `Some` or
  `None`, and generated Go SHALL match the selected ABI's representation

#### Scenario: Raw type resolves enclosing type parameters

- **WHEN** a generic function declares `go[Option[Ref[T]]]` with
  `raw: Ref[T]` and `T` is a function type parameter
- **THEN** `raw:` SHALL resolve `T` from the enclosing type environment and
  the generated adaptation SHALL be valid for every instantiation

### Requirement: A raw Ref adapts to Option[Ref]

When the declared result type is `Option[Ref[U]]` and `raw:` is
`Ref[U]`, the compiler SHALL evaluate the snippet once, produce `None` when
the resulting Go pointer is nil, and produce `Some` with that pointer when it
is non-nil.

#### Scenario: Non-nil pointer becomes Some

- **WHEN** a block declares `go[Option[Ref[rsa.PublicKey]]]` with
  `raw: Ref[rsa.PublicKey]` and the snippet returns a non-nil
  `*rsa.PublicKey`
- **THEN** the result SHALL be `Some` containing that exact pointer

#### Scenario: Nil pointer becomes None

- **WHEN** the same block's snippet returns `nil`
- **THEN** the result SHALL be `None`

#### Scenario: Raw pointer expression is evaluated once

- **WHEN** the snippet is a function call with an observable side effect and
  returns a pointer
- **THEN** generated Go SHALL evaluate that call exactly once before the
  value is observed as `Some` or `None`

### Requirement: A comma-ok raw shape adapts to Option

When the declared result type is `Option[U]` for a non-reference `U` and
`raw:` is `(U, Bool)`, the compiler SHALL evaluate the snippet once, produce
`Some(value)` when the boolean is true, and produce `None` when it is false.

#### Scenario: Present comma-ok value becomes Some

- **WHEN** a block declares `go[Option[Int]]` with `raw: (Int, Bool)` and
  the snippet returns `(42, true)`
- **THEN** the result SHALL be `Some(42)`

#### Scenario: Absent comma-ok value becomes None

- **WHEN** the same block's snippet returns `(0, false)`
- **THEN** the result SHALL be `None`

### Requirement: An error tuple adapts to Result

When the declared result type is `Result[U, Error]` and `raw:` is
`(U, Error)`, the compiler SHALL evaluate the snippet once, produce
`Ok(value)` when the Go error is nil, and produce `Err(err)` otherwise.
When the declared error type is `String`, the `Err` payload SHALL be the
result of `err.Error()`.

#### Scenario: Successful error-tuple call becomes Ok

- **WHEN** a block declares `go[Result[Int, Error]]` with
  `raw: (Int, Error)` and the snippet returns `(7, nil)`
- **THEN** the result SHALL be `Ok(7)`

#### Scenario: Failed error-tuple call becomes Err

- **WHEN** the same block's snippet returns `(0, err)` with non-nil `err`
- **THEN** the result SHALL be `Err(err)`

#### Scenario: String error result preserves the message

- **WHEN** a block declares `go[Result[Int, String]]` with
  `raw: (Int, Error)` and the snippet returns a non-nil error whose message
  is `"failed"`
- **THEN** the result SHALL be `Err("failed")`

### Requirement: A lone error adapts to a unit Result

When the declared result type is `Result[(), Error]` or
`Result[(), String]` and `raw:` is `Error`, the compiler SHALL produce
`Ok(())` when the Go error is nil and `Err(err)` otherwise. For a
`String` error type, the payload SHALL be `err.Error()`.

#### Scenario: Nil error becomes Ok unit

- **WHEN** a block declares `go[Result[(), Error]]` with `raw: Error` and
  the snippet returns nil
- **THEN** the result SHALL be `Ok(())`

#### Scenario: Non-nil error becomes Err

- **WHEN** the same block's snippet returns a non-nil error
- **THEN** the result SHALL be `Err` carrying that error

### Requirement: An explicit raw type equal to the result type is an identity

When `raw:` names the same type as the declared result type, the compiler
SHALL emit the snippet without any boundary adaptation. This SHALL allow an
inline Go block to keep constructing `Option` or `Result` directly in Go
when that is intentional.

#### Scenario: Direct Option construction remains a raw value

- **WHEN** a block declares `go[Option[Int]]` with `raw: Option[Int]` and
  the snippet returns an `Option[int]` value built from `Some` and
  `None`
- **THEN** generated Go SHALL use the snippet's value directly and SHALL NOT
  wrap it a second time

#### Scenario: Direct Result construction with a rich error remains raw

- **WHEN** a block declares `go[Result[Data, Diagnostic]]` with
  `raw: Result[Data, Diagnostic]` or without `raw:`, and the snippet calls
  a Go helper that returns that Result type
- **THEN** generated Go SHALL use the helper's value directly

### Requirement: Unsupported raw and result combinations are rejected

When `raw:` is present but the pair of declared result type and raw type does
not match one of the supported adaptations or the identity case, compilation
SHALL fail with a diagnostic that names both types. The compiler SHALL NOT
guess a conversion or emit code whose Go types disagree.

#### Scenario: Incompatible raw type

- **WHEN** a block declares `go[Option[Int]]` with `raw: String`
- **THEN** compilation SHALL report an unsupported inline-Go boundary
  conversion naming `Option[Int]` and `String`

#### Scenario: Error tuple cannot produce a rich error payload

- **WHEN** a block declares `go[Result[Data, Diagnostic]]` with
  `raw: (Data, Error)`
- **THEN** compilation SHALL reject the conversion because a Go `error`
  cannot be converted back into `Diagnostic`

### Requirement: Existing inline Go behavior remains unchanged without raw

Inline Go blocks without `raw:` SHALL retain their existing raw-result and
tuple-conversion behavior. This includes native Go values, direct
`Some` / `None` / `Ok` / `Err` construction, rich-error Result helpers,
and the existing `go[(U, Bool)]` and `go[(U, Error)]` tuple spellings.

#### Scenario: Existing direct Option construction still compiles

- **WHEN** an existing block declares `go[Option[T]]` and its snippet builds
  `Some` / `None` values directly
- **THEN** the block SHALL compile with the same generated behavior as before

#### Scenario: Existing tuple conversion still compiles

- **WHEN** an existing block declares `go[(Int, Bool)]` in a context expecting
  `Option[Int]`
- **THEN** inference SHALL continue to expose `Option[Int]` and generated Go
  SHALL continue to choose `Some` or `None`

#### Scenario: Existing rich-error Result helper still compiles

- **WHEN** an existing block declares `go[Result[Data, Diagnostic]]` without
  `raw:` and the snippet calls a Go helper returning that Result
- **THEN** the block SHALL compile exactly as before

### Requirement: Codegen3 adapts raw shapes to its selected ABI

When codegen3 compiles an inline Go block with `raw:`, it SHALL produce the
representation required by the prelude ABI selected for the build. Under the
canonical ABI, `Option[Ref[U]]` SHALL remain the raw `*U` value, so
`raw: Ref[U]` requires no wrapper and nil SHALL be observed as `None`.
Under the legacy prelude ABI, `Option[Ref[U]]` SHALL be constructed with the
ABI's `Some` / `None` helpers. For `Option[U]` and `Result[U, E]`,
codegen3 SHALL construct the selected ABI's `Option__Some` / `Option__None`
/ `Result__Ok` / `Result__Err` helpers or legacy `Some` / `None` /
`Ok` / `Err` helpers. The generated value SHALL expose the same
`Some` / `None` / `Ok` / `Err` observations as codegen2.

#### Scenario: Canonical Option[Ref] uses the pointer itself

- **WHEN** codegen3 compiles `go[Option[Ref[rsa.PublicKey]]]` with
  `raw: Ref[rsa.PublicKey]` against a canonical prelude
- **THEN** generated Go SHALL use the returned `*rsa.PublicKey` directly, a
  nil pointer SHALL be observed as `None`, and a non-nil pointer SHALL be
  observed as `Some`

#### Scenario: Legacy Option[Ref] uses nil-check constructors

- **WHEN** codegen3 compiles the same block against a legacy prelude
- **THEN** generated Go SHALL branch on the pointer and construct
  `Some[*rsa.PublicKey]` or `None[*rsa.PublicKey]`

#### Scenario: Codegen3 Option comma-ok follows the ABI

- **WHEN** codegen3 compiles `go[Option[Int]]` with
  `raw: (Int, Bool)` against a canonical prelude
- **THEN** generated Go SHALL call `Option__Some[int]` or
  `Option__None[int]` according to the boolean, and the same block against a
  legacy prelude SHALL call `Some[int]` or `None[int]`

#### Scenario: Codegen3 Result tuple follows the ABI

- **WHEN** codegen3 compiles `go[Result[Int, String]]` with
  `raw: (Int, Error)` against a canonical prelude
- **THEN** generated Go SHALL call `Result__Ok[int, string]` or
  `Result__Err[int, string]`, with the error message produced by
  `err.Error()`; against a legacy prelude it SHALL call `Ok` or `Err`

#### Scenario: Codegen3 lone error follows the ABI

- **WHEN** codegen3 compiles `go[Result[(), Error]]` with `raw: Error`
  against either selected ABI
- **THEN** generated Go SHALL produce the ABI's `Ok` unit value or `Err`
  value

### Requirement: Raw declarations require expression-form inline Go

`raw:` SHALL apply to inline Go blocks whose body is a Go expression. A block
whose body is a statement list and also declares `raw:` SHALL fail with a
diagnostic; the compiler SHALL NOT try to reinterpret statements as a value.

#### Scenario: Expression-form raw block compiles

- **WHEN** a `raw:` block's code is a single Go expression, such as a
  function-literal call
- **THEN** the compiler SHALL accept the block and apply the selected
  boundary adaptation

#### Scenario: Statement-form raw block is rejected

- **WHEN** a `raw:` block's code contains statements such as
  `x := ...` or `return ...` rather than a single expression
- **THEN** compilation SHALL fail with a diagnostic naming the
  expression-form requirement

#### Scenario: Statement-position raw block is rejected

- **WHEN** a `raw:` block is used solely as a statement and its value is
  discarded
- **THEN** compilation SHALL fail with a diagnostic that a raw declaration
  requires a value context

### Requirement: The legacy backend rejects raw declarations explicitly

The legacy `--legacy` pipeline SHALL keep supporting inline Go blocks without
`raw:`. When it encounters a block that declares `raw:`, it SHALL report an
unsupported-feature diagnostic instead of ignoring the field or emitting
incompatible Go.

#### Scenario: Legacy backend rejects raw

- **WHEN** `--legacy` compiles a block that declares `raw:`
- **THEN** compilation SHALL fail with a diagnostic that raw declarations are
  not supported by the legacy backend
