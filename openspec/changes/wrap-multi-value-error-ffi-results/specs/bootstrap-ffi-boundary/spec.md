## MODIFIED Requirements

### Requirement: Bootstrap wraps lone-error Go FFI results into Result[(), error]
When a bootstrap-compiled function calls a go:`-imported Go function or method whose recorded signature returns only a trailing `error` (a single result of type `error`, for example `func Foo() error` or a `Flush() error` method), the call SHALL be decoded at the boundary into `Result[(), error]`: the generated Go SHALL produce `Ok` with a unit payload when the call returns `nil` and `Err` with the yielded error value when it returns non-nil. The empty tuple `()` is the unit payload; in generated Go it SHALL be represented as `struct{}` inside the generic `Result` type (for example `Ok[struct{}, error](struct{}{})`).

For a Go FFI signature `(T, error)`, the call SHALL be exposed as `Result[T, error]`. For a signature `(T1, T2, ..., Tn, error)` with at least two non-error results, the call SHALL be exposed as `Result[(T1, T2, ..., Tn), error]`, with the successful values represented as one MyGO tuple payload. In each case, generated Go SHALL evaluate the Go call once, check the trailing error, and construct `Ok` with the complete success payload or `Err` with the error.

An explicit tuple destructuring binding on a Go FFI call SHALL preserve the raw Go multi-return values, including a trailing error, and SHALL require the pattern arity to equal the Go signature result count. A two-result `(T, bool)` signature SHALL continue to map to `Option[T]` for ordinary expression use. In statement position, a discarded error-bearing FFI call SHALL remain a raw Go call with its results discarded.

#### Scenario: Package function returning a lone error
- **WHEN** a function evaluates `os.Chdir(dir)` where `os.Chdir` has the Go signature `func Chdir(dir string) error`
- **THEN** the call SHALL type-check as `Result[(), error]`, and generated Go SHALL invoke `os.Chdir(dir)`, check its returned error, and emit `Ok` with a unit (`struct{}{}`) payload when the error is `nil` or `Err` with the error otherwise

#### Scenario: Method returning a lone error on an imported Go type
- **WHEN** a function calls a method whose Go signature returns only `error` in an expression
- **THEN** the call SHALL lower through the same boundary `Result[(), error]` wrapping used for package functions, and generated Go SHALL call the method and branch on its returned error to produce `Ok(())` or `Err(e)`

#### Scenario: Two-result value and error call
- **WHEN** a function uses a Go FFI call with signature `(T, error)` as an ordinary expression
- **THEN** inference SHALL expose the call as `Result[T, Error]`, and generated Go SHALL evaluate it once and wrap its value or error in `Ok` or `Err`

#### Scenario: Multiple success values followed by error
- **WHEN** a function uses a Go FFI call with signature `(T1, T2, ..., Tn, error)` where `n >= 2` as an ordinary expression
- **THEN** inference SHALL expose it as `Result[(T1, T2, ..., Tn), Error]`, and generated Go SHALL evaluate it once, check the trailing error, and wrap all success values together as the tuple payload of `Ok` or wrap the error as `Err`

#### Scenario: Explicit tuple binding preserves raw multi-return values
- **WHEN** a function binds `let (a, b, err) = goCall()` and the Go signature returns `(A, B, error)`
- **THEN** generated Go SHALL preserve the native multi-assignment `a, b, err := goCall()` and inference SHALL assign each binding its corresponding Go result type

#### Scenario: Tuple binding arity mismatch is diagnosed
- **WHEN** a tuple destructuring pattern for a Go FFI multi-return call has a different number of entries than the Go signature's result count
- **THEN** compilation SHALL report a tuple binding result-count mismatch

#### Scenario: Two-result value and bool remains Option
- **WHEN** a function uses a Go FFI call with signature `(T, bool)` as an ordinary expression
- **THEN** inference SHALL continue to expose it as `Option[T]` without treating the boolean as an error

#### Scenario: Discarding an error-bearing result in statement position
- **WHEN** an error-bearing Go FFI call is used solely as a statement
- **THEN** generated Go SHALL emit the raw call as a statement and discard its returned values without constructing an unused `Result`

#### Scenario: Discarding the lone-error Result in statement position
- **WHEN** a lone-error Go FFI call is used solely for its side effect (for example `os.Chdir(dir)` as a statement)
- **THEN** generated Go SHALL emit the raw call as a statement and discard its `error` return, rather than building a `Result` value that Go would reject as unused
