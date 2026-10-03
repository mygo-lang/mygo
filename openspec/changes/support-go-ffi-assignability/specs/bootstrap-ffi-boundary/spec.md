# Spec Delta

## MODIFIED Requirements

### Requirement: Bootstrap dispatches method calls on go:-imported interface values
When a bootstrap-compiled function calls a method on a value whose type is a
go:`-imported Go **interface** (for example an `http.ResponseWriter` handler
parameter, an `io.Reader`-typed binding, or an interface reached through an
exported Go type alias), bootstrap SHALL resolve the method from the imported
package's interface method surface and the generated Go SHALL emit a direct
method call on the value (`w.Write(...)`), not a struct field access.
At a Go FFI call boundary, a value SHALL be accepted for a parameter when Go's
type system determines that the argument type is assignable to the parameter
type. This includes pointer method sets: `Ref[T]`, which represents Go `*T`,
SHALL be accepted for an interface parameter when `*T` implements that
interface. Generated Go SHALL pass the value directly without an unnecessary
conversion. Incompatible arguments SHALL remain type errors with the argument
location identified. Go-specific assignability SHALL NOT broaden ordinary
MyGO unification or erase the distinction between incompatible named Go types.

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

#### Scenario: Passing one named Go interface to another named interface parameter
- **WHEN** a function passes a value of a named imported Go interface type to
  an imported Go function or method parameter of a different named interface
  type, and Go determines that the source interface type is assignable to the
  parameter type
- **THEN** bootstrap inference SHALL accept the argument and generated Go SHALL
  pass the value directly

#### Scenario: Interface metadata comes from an imported MyGO signature
- **WHEN** a package imports a MyGO function whose exported signature contains
  a named Go interface from a `go:` import that the caller does not import
  directly (for example, `Result[io.ReadCloser, Error]`), destructures that
  result, and passes the bound interface value to a compatible Go interface
  parameter
- **THEN** bootstrap SHALL preserve the interface's canonical Go import path,
  make Go type information available through the imported MyGO dependency graph,
  and accept the call when Go determines the argument assignable to the target

#### Scenario: Embedded interface methods satisfy a parameter interface
- **WHEN** a source Go interface embeds another interface that declares a
  required method, and a value of the source interface is passed to a named
  interface parameter requiring that method
- **THEN** bootstrap SHALL accept the argument when Go determines the source
  interface assignable to the target interface, including its promoted method
  set, and generate a direct argument pass

#### Scenario: Interface argument is rejected when it is not assignable
- **WHEN** a value of a named Go interface type is passed to a named interface
  parameter and Go determines that the source type is not assignable to the
  parameter type
- **THEN** bootstrap inference SHALL reject the call with a type mismatch
  identifying the incompatible argument, even if the source and parameter
  interfaces have related names or share some methods

#### Scenario: Interface argument is rejected when a required method is missing
- **WHEN** a value of a named Go interface type is passed to a named interface
  parameter whose method set includes a method absent from the source interface
- **THEN** bootstrap inference SHALL reject the call with a type mismatch
  identifying the incompatible argument, even if the source and parameter
  interfaces have related names or share some methods

#### Scenario: Pointer to a Go type implements an interface parameter
- **WHEN** a Go FFI function or method expects a named interface parameter and
  the caller supplies `Ref[T]` for a Go named type `T` whose pointer method set
  implements that interface
- **THEN** bootstrap SHALL accept the call according to Go assignability and
  generated Go SHALL pass the `*T` value directly

#### Scenario: Pointer method set is required for interface assignment
- **WHEN** a caller supplies `Ref[T]` for a Go named type `T` whose pointer
  method set does not implement the target interface
- **THEN** bootstrap SHALL reject the call with a type mismatch identifying the
  argument

#### Scenario: Incompatible named Go types remain rejected
- **WHEN** a Go FFI call receives a named Go value that Go does not consider
  assignable to the corresponding parameter, despite matching underlying
  representation
- **THEN** bootstrap SHALL reject the call and SHALL NOT treat the values as
  compatible merely because their underlying shapes match
