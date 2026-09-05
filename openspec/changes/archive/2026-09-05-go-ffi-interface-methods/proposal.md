## Why

MyGO's Go FFI lets handlers register on `http.DefaultServeMux`, start an
`http.Server`, and read a request body — but a handler cannot write the
response, because calling methods on an imported Go **interface** value (e.g.
`w.Write(b)` on `http.ResponseWriter`, or `r.Body.Read(...)` on `io.Reader`)
fails with `unknown field http.ResponseWriter.Write`. Go interfaces currently
surface with an empty method table, so a pure-MyGO transport shell that writes
the response head/status/body is blocked.

## What Changes

- During Go FFI package loading, collect the exported method set of Go
  **interface** types (currently only the pointer method set is collected,
  which is empty for interfaces). Interfaces keep their nominal value type
  (no struct/field reinterpretation).
- Include interfaces reached through exported **type aliases**, so a package
  alias to an interface exposes the target's methods too.
- Route `.Method(...)` calls on an interface-typed receiver through the existing
  GoMethod dispatch, lowering to a direct Go interface method call
  (`w.Write(...)`) rather than a struct field access.
- Add tests covering interface method collection, alias-exposed interface
  methods, inference-time dispatch, and generated-Go lowering.

No **BREAKING** changes: behavior only widens (previously-erroring interface
method calls start compiling). Downstream symbol registration, resolution, and
codegen already dispatch Go methods generically and need no changes.

## Capabilities

### New Capabilities

- None. The behavior extends an existing capability rather than introducing a
  new domain.

### Modified Capabilities

- `bootstrap-ffi-boundary`: adds a requirement that method calls on go:`-imported
  Go **interface** values (including interfaces reached via exported type
  aliases) resolve from the imported package's interface method surface and
  lower to direct Go method calls.

## Impact

- `internal/mygo/compiler/go_ffi_import.go` — the `goTypeMethods` collector must
  choose the interface method set (the interface type itself) instead of the
  always-pointer method set for interface-underlying named types. This is the
  single behavioral change; both the exported-alias pass and the named-type pass
  call it, so alias support follows for free.
- `internal/mygo/compiler/go_ffi_import_test.go` — extend with interface and
  alias method-surface assertions.
- `internal/mygo/typeinference2` — resolution (`GoMethodSignatureInPackages`,
  `inferOrdinaryField`) already consumes the populated `Methods` table; add
  tests (and any needed fixtures) where the self-hosted inference is exercised.
- `internal/mygo/codegen2` — lowering of `FieldAccess` to `goast.Selector` is
  already interface-agnostic; add an end-to-end generation test.
- No new external dependencies; no runtime system changes.
