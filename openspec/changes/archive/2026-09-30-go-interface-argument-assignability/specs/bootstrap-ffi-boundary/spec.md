## MODIFIED Requirements

### Requirement: Bootstrap dispatches method calls on go:-imported interface values
When a bootstrap-compiled function calls a method on a value whose type is a
go:`-imported Go **interface** (for example an `http.ResponseWriter` handler
parameter, an `io.Reader`-typed binding, or an interface reached through an
exported Go type alias), bootstrap SHALL resolve the method from the imported
package's interface method surface and the generated Go SHALL emit a direct
method call on the value (`w.Write(...)`), not as a struct field access. At a
Go FFI call boundary, a value of a named imported Go interface type SHALL also
be accepted for a parameter of a different named imported Go interface type
when the source interface's method set satisfies the parameter interface's
method set. Satisfaction SHALL use Go method-set rules, including methods
promoted through embedded interfaces; sharing a name or some methods alone
SHALL NOT make incompatible interfaces assignable.

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
  type, and the source interface's method set contains every method required by
  the parameter interface with matching signatures
- **THEN** bootstrap inference SHALL accept the argument and generated Go SHALL
  pass the value directly

#### Scenario: Interface metadata comes from an imported MyGO signature
- **WHEN** a package imports a MyGO function whose exported signature contains
  a named Go interface from a `go:` import that the caller does not import
  directly (for example, `Result[io.ReadCloser, Error]`), destructures that
  result, and passes the bound interface value to a compatible Go interface
  parameter
- **THEN** bootstrap SHALL preserve the interface's canonical Go import path,
  load its method-set metadata through the imported MyGO dependency graph, and
  accept the call when the method set satisfies the target interface

#### Scenario: Embedded interface methods satisfy a parameter interface
- **WHEN** a source Go interface embeds another interface that declares a
  required method, and a value of the source interface is passed to a named
  interface parameter requiring that method
- **THEN** bootstrap inference SHALL include the promoted method in the source
  method set, accept the argument when all parameter methods are satisfied, and
  generate a direct argument pass

#### Scenario: Interface argument is rejected when a required method is missing
- **WHEN** a value of a named Go interface type is passed to a named interface
  parameter whose method set includes a method absent from the source interface
- **THEN** bootstrap inference SHALL reject the call with a type mismatch
  identifying the incompatible argument, even if the source and parameter
  interfaces have related names or share some methods

#### Scenario: Passing one named Go interface to another named interface parameter
- **WHEN** a function passes a value of a named imported Go interface type to
  an imported Go function or method parameter of a different named interface
  type, and the source interface's method set contains every method required by
  the parameter interface with matching signatures
- **THEN** bootstrap inference SHALL accept the argument and generated Go SHALL
  pass the value directly

#### Scenario: Embedded interface methods satisfy a parameter interface
- **WHEN** a source Go interface embeds another interface that declares a
  required method, and a value of the source interface is passed to a named
  interface parameter requiring that method
- **THEN** bootstrap inference SHALL include the promoted method in the source
  method set, accept the argument when all parameter methods are satisfied, and
  generate a direct argument pass

#### Scenario: Interface argument is rejected when a required method is missing
- **WHEN** a value of a named Go interface type is passed to a named interface
  parameter whose method set includes a method absent from the source interface
- **THEN** bootstrap inference SHALL reject the call with a type mismatch
  identifying the incompatible argument, even if the source and parameter
  interfaces have related names or share some methods
