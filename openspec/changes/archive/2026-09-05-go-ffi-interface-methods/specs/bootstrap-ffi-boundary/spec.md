## ADDED Requirements

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
