## ADDED Requirements

### Requirement: Bootstrap preserves anonymous Go interface types from FFI signatures
When a Go FFI signature contains a non-empty anonymous interface type,
bootstrap SHALL recognize the complete `interface{...}` type before resolving
package-qualified names. It SHALL preserve the interface's Go identity and
complete method set, including methods promoted from embedded named interfaces.
The empty `interface{}` type SHALL continue to map to `Any`.

#### Scenario: Anonymous interface returned by a Go function
- **WHEN** an imported Go function returns
  `interface { renderer.Option; html.Option }` and the MyGO source binds or
  passes that result
- **THEN** bootstrap SHALL treat the result as an anonymous Go interface, not as
  a package name containing `interface{...}`, and SHALL preserve both embedded
  interface method sets

#### Scenario: Anonymous interface nested in another signature type
- **WHEN** an FFI parameter or result contains an anonymous interface inside a
  slice, map, pointer, channel, or function signature
- **THEN** bootstrap SHALL recognize the anonymous interface at its nested
  position instead of splitting it as a qualified type name

#### Scenario: Empty interface remains Any
- **WHEN** an FFI signature contains `interface{}`
- **THEN** bootstrap SHALL continue to represent it as `Any` with the existing
  behavior

### Requirement: Anonymous Go interface arguments use Go assignability
At a Go FFI call boundary, a value whose inferred type is an anonymous Go
interface SHALL be accepted for a parameter interface when Go determines that
the value's type is assignable to the parameter type. This SHALL include
assignability established by embedded interfaces and the pointer method set of
`Ref[T]`. The generated Go SHALL pass the original value without an unnecessary
conversion or helper call. Ordinary MyGO unification SHALL NOT be broadened by
this check.

#### Scenario: Passing an anonymous embedded interface to a named interface parameter
- **WHEN** a function passes the result of a Go call returning
  `interface { renderer.Option; html.Option }` to a variadic parameter of type
  `...renderer.Option`
- **THEN** bootstrap inference SHALL accept the argument and generated Go SHALL
  pass the returned value directly

#### Scenario: Pointer method set satisfies an anonymous-interface-derived parameter
- **WHEN** `Ref[T]` represents Go `*T`, `T`'s pointer method set implements the
  target interface, and the value is used at a Go FFI boundary
- **THEN** bootstrap SHALL accept the argument according to Go assignability and
  emit a direct argument pass

#### Scenario: Missing or mismatched method is rejected
- **WHEN** an anonymous interface value is passed to a parameter interface that
  requires a method the source interface does not contain, or contains it with a
  different signature
- **THEN** bootstrap SHALL reject the call with a type-mismatch diagnostic that
  identifies the argument and retains the anonymous source type in the
  diagnostic

### Requirement: Bootstrap does not reinterpret other anonymous Go types as qualified names
When a Go FFI signature contains an anonymous struct or another anonymous
composite type that is not already modeled, bootstrap SHALL classify it as an
anonymous Go type rather than as a package-qualified name. Operations that are
not supported for that anonymous type SHALL fail with a diagnostic naming the
type; they SHALL NOT be silently accepted through a fabricated package path.

#### Scenario: Anonymous struct in a Go signature
- **WHEN** an imported Go function returns `struct { Value int }`
- **THEN** bootstrap SHALL not split the type at its internal dot or fabricate a
  package qualifier, and any unsupported operation on the value SHALL report the
  anonymous struct type

#### Scenario: Existing named Go types are unchanged
- **WHEN** an FFI signature contains a named imported type such as
  `renderer.Option` or `time.Time`
- **THEN** bootstrap SHALL continue to resolve it to its canonical Go package
  path and preserve existing unification and assignability behavior
