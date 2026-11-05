## Why

Inline Go currently uses one type for two different jobs: the MyGO result type
and the raw Go result shape. That makes a natural boundary declaration such as
`go[Option[Ref[rsa.PublicKey]]]` unable to return the bare Go value
`*rsa.PublicKey`, even though Go FFI already converts `*T`, `(T, bool)`,
and `(T, error)` into `Option` / `Result`. Existing workarounds construct
the ADT inside the Go snippet, which is verbose and exposes the prelude's Go
representation. At the same time, existing `go[T]` blocks that intentionally
return a native Go value or construct `Option` / `Result` directly must keep
working unchanged.

## What Changes

- Add an optional `raw:` field to inline Go blocks:
  `go[T] { raw: RawType; code: "..."; ... }`.
- Keep the type in `go[T]` as the MyGO result type. `raw:` declares the
  Go-visible result shape produced by the snippet; the compiler adapts it to
  `T` at the boundary.
- Support these explicit boundary conversions:
  - `raw: Ref[U]` with target `Option[Ref[U]]` -> nil check into
    `Some` / `None`.
  - `raw: (U, Bool)` with target `Option[U]` -> comma-ok into
    `Some` / `None`.
  - `raw: (U, Error)` with target `Result[U, Error]` or
    `Result[U, String]` -> `Ok` / `Err`; the `String` target uses
    `err.Error()`.
  - `raw: Error` with target `Result[(), Error]` or
    `Result[(), String]` -> lone-error adaptation.
- When `raw:` is omitted, the raw Go shape is the declared result type, so
  every existing inline Go block retains its current behavior exactly.
- Reject an unsupported `target` / `raw:` pair with a generation
  diagnostic that names both types. Do not guess a conversion.
- Limit `raw:` to expression-form inline Go. A statement-form block that
  declares `raw:` is rejected with a diagnostic instead of reinterpreting
  its `return` statements.
- Codegen3 SHALL adapt the same `target` / `raw:` pairs to its selected
  prelude ABI: canonical `Option[Ref[U]]` remains the raw `*U`, while the
  other adaptations construct the ABI's canonical `Option__*` / `Result__*`
  helpers or legacy `Some` / `None` / `Ok` / `Err` helpers.
- Update parser2, the AST and typed AST, the formatter, the bootstrap entry
  metadata path, IR3, codegen2, codegen3, documentation, and tests. No new
  keyword is needed: `raw` is recognized as a go-block field name in
  context.

## Capabilities

### New Capabilities

- `language/inline-go-boundary`: explicit raw Go result shapes for inline Go
  blocks and the boundary adaptations that turn them into `Option` / `Result`.

### Modified Capabilities

<!-- None: Go FFI behavior itself is unchanged; inline Go gains a new syntax. -->

## Impact

- Parser and AST: `internal/mygo/parser2/`, `internal/mygo/ast2/ast2.mygo`,
  and the legacy `internal/mygo/parser` / `internal/mygo/ast` only as needed
  to keep the shared syntax surface consistent.
- Formatter: preserve and canonicalize the optional `raw:` field.
- Inference: carry the raw type through typed ASTs and substitutions without
  changing the inferred MyGO result type.
- Codegen2 and codegen3: select the boundary adaptation from
  `(target, raw)` and emit each backend's ABI representation. Codegen2 uses
  the existing `Some` / `None` / `Ok` / `Err` lowering; codegen3 uses
  `GoTypeString` plus the canonical or legacy constructor names selected by
  its prelude ABI.
- Generated files: regenerate `zz_*.gen.go`; never hand-edit them.
- Scope: parser2, codegen2, and codegen3 implement adaptation. `--legacy`
  remains out of scope and rejects `raw:` explicitly.
- No breaking change: `go[T]` without `raw:`, including direct
  `Some` / `None` / `Ok` / `Err` construction and rich-error
  `Result[T, Diagnostic]` helpers, keeps its existing semantics.
