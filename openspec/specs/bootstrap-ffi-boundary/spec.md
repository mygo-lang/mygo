# bootstrap-ffi-boundary Specification

## Purpose

Defines the behavior of the self-hosted bootstrap compiler (compiler2) at the
Go FFI boundary: struct field access, package-qualified struct literals,
multi-value returns whose elements include package-local type names, and
values of Go named function types.

## Requirements

### Requirement: Bootstrap type-checks member chains on Go-FFI struct fields
When a bootstrap-compiled function accesses a field of a go:`-imported struct
type and then calls a method declared on that field's type, bootstrap
inference SHALL resolve both the field and the method from the imported
package's type surface, and the generated Go SHALL call the method directly
on the field value.

#### Scenario: Chained method call on a Go-FFI field
- **WHEN** a function with a `Ref[http.Request]` parameter evaluates
  `r.Header.Set("X-Test", "1")`
- **THEN** inference SHALL accept the expression, and the generated Go SHALL
  contain a direct method call on the `Header` field value that the Go
  compiler accepts (no undefined helper symbol)

#### Scenario: Field read from a Go-FFI struct
- **WHEN** a function binds `let h = r.Header` with `r: Ref[http.Request]`
- **THEN** the binding SHALL type-check with `h`'s type being the field's Go
  type (`http.Header`), and the generated Go SHALL read the field directly

### Requirement: Bootstrap dispatches method calls on go:-imported interface values
When a bootstrap-compiled function calls a method on a value whose type is a
go:`-imported Go **interface** (for example an `http.ResponseWriter` handler
parameter, an `io.Reader`-typed binding, or an interface reached through an
exported Go type alias), bootstrap SHALL resolve the method from the imported
package's interface method surface and the generated Go SHALL emit a direct
method call on the value (`w.Write(...)`), not a struct field access.

#### Scenario: Writing an HTTP response through a ResponseWriter
- **WHEN** a function with an `http.ResponseWriter` parameter `w` evaluates
  `w.Header().Set("Content-Type", "text/plain")`, `w.WriteHeader(200)`, and
  `w.Write(bytes)`
- **THEN** bootstrap inference SHALL accept each expression (no
  `unknown field http.ResponseWriter.Write` error) and the generated Go SHALL
  contain direct `w.Header()`, `w.WriteHeader(...)`, and `w.Write(...)` calls
  that the Go compiler accepts

#### Scenario: Reading through an io.Reader interface
- **WHEN** a function binds `let reader = r.Body` with `r: Ref[http.Request]`
  and then evaluates `reader.Read(buf)` where `reader`'s type resolves to the
  `io.Reader` interface
- **THEN** inference SHALL accept the call and the generated Go SHALL contain a
  direct `reader.Read(...)` call

#### Scenario: Interface method call via an exported type alias
- **WHEN** a go:`-imported package exports a type alias whose target is a Go
  interface (for example `type ResponseFlusher = http.ResponseWriter`), and a
  function receiving a value of the alias type evaluates a method on it
- **THEN** bootstrap SHALL resolve the method from the aliased interface's
  method surface and the generated Go SHALL emit a direct call on the alias-typed
  value

#### Scenario: (T, error) result of an interface method wraps into Result
- **WHEN** a function calls an interface method whose recorded signature returns
  `(T, error)` (for example `io.Reader.Read` returning `(int, error)`) as part of
  an expression
- **THEN** the call SHALL lower through the same boundary `Result[T, error]`
  wrapping already used for struct methods and package functions, and the
  generated Go SHALL call the method and check its returned error

### Requirement: Bootstrap preserves raw multi-value returns with package-local result types
When a tuple destructuring binding (`let (a, b) = call()`) targets a Go FFI
call whose result list contains a type name defined inside the imported
package, bootstrap SHALL still lower the binding to Go's native multi-value
assignment (`a, b := call()`) rather than falling back to an anonymous tuple
value, and each bound name SHALL receive its result element type.

#### Scenario: Tuple binding of context.WithTimeout
- **WHEN** a function evaluates
  `let (ctx, cancel) = context.WithTimeout(context.Background(), dur)` where
  the Go signature results are `(context.Context, CancelFunc)` and
  `CancelFunc` is a package-local type of `go:context`
- **THEN** the generated Go SHALL contain `ctx, cancel :=
  context.WithTimeout(...)` and both names SHALL type-check at their Go result
  types

### Requirement: Values of Go named function types are callable
When a value's type is a Go named type whose underlying type is a function
signature (for example `context.CancelFunc`), bootstrap inference SHALL treat
the value as callable with the signature's parameters and results, and the
generated Go SHALL emit a direct call of the value.

#### Scenario: Calling a CancelFunc value
- **WHEN** a bootstrap-compiled function binds `cancel` from a `go:context`
  call result typed `CancelFunc`, assigns it to a `let` binding of that type,
  and then evaluates `cancel()`
- **THEN** inference SHALL accept the call (no
  `cannot unify ... with function type` error) and the generated Go SHALL
  contain the direct invocation `cancel()`

### Requirement: Qualified Go-FFI struct literals keep their package qualifier
When a struct literal names a type from a go:`-imported package using its
qualified source spelling, bootstrap codegen SHALL emit the qualified Go
composite literal so the generated code remains valid Go.

#### Scenario: Empty qualified struct literal
- **WHEN** a function binds `let client = http.Client { }`
- **THEN** the generated Go SHALL contain `http.Client{}` (not a bare
  `Client{}`)

#### Scenario: Qualified struct literal with a field
- **WHEN** a function binds `let client = http.Client { Timeout: time.Second }`
- **THEN** the generated Go SHALL contain `http.Client{Timeout: time.Second}`
  and SHALL type-check against the Go package

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

### Requirement: Bootstrap imports exported package-level Go variables
When a bootstrap-compiled program selects an exported package-level `var`
through a `go:`-imported package (for example `gorm.ErrRecordNotFound` from
`go:gorm.io/gorm`, or `os.ErrNotExist` from `go:os`), bootstrap SHALL treat the
selector as a value of the variable's declared Go type, exactly as it already
does for an exported package-level `const`. A package that exports error
values as `var`s rather than `const`s SHALL be fully usable at this boundary.

#### Scenario: Selecting a GORM sentinel error variable
- **WHEN** a function evaluates `gorm.ErrRecordNotFound` where `gorm` is a
  `go:gorm.io/gorm` import
- **THEN** inference SHALL accept the selector and bind the expression to the
  variable's declared type (`error`), and the generated Go SHALL reference
  `gorm.ErrRecordNotFound` directly (no `unknown Go package member` error)

#### Scenario: Variable and constant selectors agree
- **WHEN** a function selects an exported package-level `var` and an exported
  package-level `const` from the same `go:`-imported package
- **THEN** both SHALL type-check as values of their respective declared types,
  with no behavioral difference between the two at this boundary

### Requirement: Bootstrap resolves embedded members with Go promotion semantics
When a struct declares an embedded type via the `embed` marker, bootstrap
inference SHALL make the embedded type's exported members reachable on the
embedding struct exactly as Go does, for both MyGO and `go:`-imported embedded
types, and at any nesting depth.

An embedded type SHALL be reachable by two distinct paths, and both SHALL
type-check and agree on types:

1. **Promoted access** — an exported member of an embedded type is reachable
   directly on the embedding value (`b.F1`), transitively through any number of
   embedding levels, and across MyGO/Go embedding boundaries in either
   combination.
2. **Qualified access** — the embedded field itself is addressable by the
   embedded type's name (`b.A`, and for a Go embed `u.Model`), so its members
   can be reached through the explicit path (`b.A.F1`).

Promotion SHALL cover exported **fields and methods** alike. Only members Go
itself promotes are in scope; unexported members remain invisible across the
package boundary.

When a member name is reachable at more than one depth, bootstrap SHALL adopt
Go's resolution rules:

- The shallowest reachable member wins.
- A member declared directly on the embedding struct wins over any promoted
  member of the same name.
- Ties at the same depth that originate from different embedded types SHALL be
  reported as an ambiguous selector rather than resolved arbitrarily.
- The qualified path (`b.A.F1`) is always unambiguous and never participates
  in ambiguity.

Generated Go SHALL read both paths as direct selectors so the Go compiler
applies its own promotion rules.

#### Scenario: Reading a field promoted from an embedded GORM model
- **WHEN** a function with a `User` parameter, where `struct User` contains
  `embed gorm.Model`, evaluates `u.ID` or `u.CreatedAt`
- **THEN** inference SHALL accept the selector and bind it to the embedded
  type's field type (an integer identifier and a timestamp respectively)
  rather than reporting `unknown field User.ID`, and the generated Go SHALL
  contain the direct selector expression `u.ID` / `u.CreatedAt`

#### Scenario: Reaching an embedded field by its type name
- **WHEN** a function with a `B` parameter, where `struct B` contains
  `embed A` and `struct A` declares `F1: Int`, evaluates `b.A` or `b.A.F1`
- **THEN** inference SHALL bind `b.A` to the embedded type `A` and `b.A.F1` to
  `A`'s declared field type, and the generated Go SHALL contain the direct
  selector expressions `b.A` / `b.A.F1`

#### Scenario: Multi-level promotion across MyGO embeds
- **WHEN** `struct A` declares `F1: Int`, `struct B` contains `embed A`, and
  `struct C` contains `embed B`, so that embedding reaches `F1` at depth two
- **THEN** `c.F1` SHALL type-check at the field's own declared type

#### Scenario: Multi-level promotion across a Go embed
- **WHEN** a MyGO struct embeds a `go:`-imported struct whose own underlying
  struct embeds a further struct, so that a field sits two levels down
- **THEN** the field SHALL be reachable directly on the MyGO struct value at
  its own declared type

#### Scenario: Promotion through a mixed MyGO and Go embed chain
- **WHEN** a MyGO struct embeds another MyGO struct which embeds a
  `go:`-imported struct (or the reverse ordering), so that a member sits two or
  more levels down across the FFI boundary
- **THEN** that member SHALL be reachable directly on the outermost MyGO value
  at its own declared type

#### Scenario: Promoted method call on an embedded Go type
- **WHEN** a function with a `User` parameter, where `struct User` contains
  `embed gorm.Model`, calls a method exported by `gorm.Model` on `u`
- **THEN** inference SHALL resolve the method on the `User` receiver and bind
  its result to the method's declared result type, and the generated Go SHALL
  contain a direct method call on the value

#### Scenario: Shallowest member wins at greater depth
- **WHEN** an outer struct declares a member with the same name as a member
  reachable through its embedded type, and also embeds a second type that
  contributes that name at a shallower depth
- **THEN** the shallower member SHALL be selected, matching Go's
  shallowest-wins rule

#### Scenario: Ambiguous promotion at equal depth
- **WHEN** a struct embeds two types that both contribute a member of the same
  name at the same depth, and the name is not declared on the struct itself
- **THEN** inference SHALL fail with an ambiguous selector error naming the
  member, rather than silently picking one of the two

#### Scenario: Own fields remain unaffected
- **WHEN** a function selects a field declared directly on the MyGO struct
  that also embeds a type contributing the same name
- **THEN** that field SHALL continue to resolve at its own declared type, as it
  did prior to embedding, and the generated Go SHALL contain the same direct
  selector

#### Scenario: Bare imported Go struct members still resolve
- **WHEN** a function selects a field or calls a method directly on a value of
  a `go:`-imported struct type, without any MyGO struct embedding it
- **THEN** that member SHALL continue to resolve as before, confirming the
  promotion work did not change the pre-existing path
