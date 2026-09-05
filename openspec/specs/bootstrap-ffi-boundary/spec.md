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
