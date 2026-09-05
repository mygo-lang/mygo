## ADDED Requirements

### Requirement: Bootstrap wraps lone-error Go FFI results into Result[(), error]
When a bootstrap-compiled function calls a go:`-imported Go function or method
whose recorded signature returns only a trailing `error` (a single result of
type `error`, for example `func Foo() error` or a `Flush() error` method), the
call SHALL be decoded at the boundary into `Result[(), error]`: the generated
Go SHALL produce `Ok` with a unit payload when the call returns `nil` and
`Err` with the yielded error value when it returns non-nil. The empty tuple
`()` is the unit payload; in generated Go it SHALL be represented as `struct{}`
inside the generic `Result` type (for example `Ok[struct{}, error](struct{}{})`).
This matches the existing rule that every error-bearing Go FFI call is
`Result`-shaped, and it does not alter the `(T, error)` -> `Result[T, error]`
or the `(T, bool)` -> `Option[T]` paths.

#### Scenario: Package function returning a lone error
- **WHEN** a function evaluates `os.Chdir(dir)` where `os.Chdir` has the Go
  signature `func Chdir(dir string) error`
- **THEN** the call SHALL type-check as `Result[(), error]`, and the generated
  Go SHALL invoke `os.Chdir(dir)`, check its returned error, and emit `Ok`
  with a unit (`struct{}{}`) payload when the error is `nil` or `Err` with the
  error value otherwise

#### Scenario: Method returning a lone error on an imported Go type
- **WHEN** a function calls a method whose Go signature returns only `error`
  (for example a `Flush() error` or `Close() error` method on an imported Go
  type receiver) in an expression
- **THEN** the call SHALL lower through the same boundary `Result[(), error]`
  wrapping used for package functions, and the generated Go SHALL call the
  method and branch on its returned error to produce `Ok(())` or `Err(e)`

#### Scenario: Discarding the lone-error Result in statement position
- **WHEN** a lone-error Go FFI call is used solely for its side effect (for
  example `os.Chdir(dir)` as a statement)
- **THEN** the generated Go SHALL emit the raw call as a statement and discard
  its `error` return, rather than building a `Result` value that Go would
  reject as unused
