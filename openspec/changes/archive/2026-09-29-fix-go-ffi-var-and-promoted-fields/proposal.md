## Why

Two gaps in the Go FFI type-surface collector make real third-party Go
libraries (verified against `gorm.io/gorm v1.31.2`) partially unusable:

1. Exported package-level `var`s are never imported, so selectors over them
   fail with `unknown Go package member`. GORM exports its whole error surface
   this way (`ErrRecordNotFound`, `ErrInvalidData`, `ErrDuplicatedKey`), and
   so do standard-library packages (`os.ErrNotExist`).
2. Go's embedding (promotion) semantics are not implemented. A MyGO struct that
   embeds a Go type via `embed gorm.Model` cannot reach the embedded type's
   exported fields (`u.ID`, `u.CreatedAt`) or its exported methods (`u.Save()`);
   a MyGO struct embedding another MyGO struct (`struct B` / `embed A`) does
   not reach `A`'s members at all; and promotion is neither transitive nor
   depth-aware, so members two or more levels down are unreachable and a
   same-depth conflict has no defined outcome.

Package-level `const`s, interface method dispatch, `(T, error)` boundary
wrapping, and `embed`-field codegen were all verified working, so this change
closes the two remaining holes rather than redesigning the boundary.

## What Changes

- The Go FFI surface collector gains a package-level `var` pass, mirroring
  the existing `const` pass: exported `*types.Var` objects are recorded as
  values of their declared type, so `gorm.ErrRecordNotFound` and
  `os.ErrNotExist` type-check like ordinary selectors.
- Embedded members are promoted with Go's semantics: transitively, across any
  mix of MyGO and Go embedding, for fields and methods alike, with the
  shallowest member winning and same-depth conflicts reported as ambiguous.
- An embedded type is also reachable by name (`b.A.F1`, `u.Model.F1`), which is
  how Go disambiguates an otherwise-ambiguous selector.
- The hand-written `typeinference` FFI loader is kept in step with the
  bootstrap loader so the two pipelines agree at this boundary.

One intentional behaviour change: a selector Go reports as ambiguous now fails
here with an ambiguous-selector error rather than resolving arbitrarily. No
program Go accepts changes behaviour.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: extend the imported Go type surface to include
  exported package-level variables, and define Go's embedding semantics for
  member promotion (fields and methods, transitive, depth-resolved) at the
  bootstrap boundary.

## Impact

- `internal/mygo/compiler/go_ffi_import.go` — the bootstrap surface collector
  (its `*types.Const` pass and its `goTypeFields` helper).
- `internal/mygo/typeinference/go_imports.go` — the hand-written loader's
  equivalent constant pass, for pipeline parity.
- `internal/mygo/typeinference2/env.mygo`, `types.mygo`, `infer.mygo` — the
  embedding-aware symbol construction and the selector-time promotion
  resolver. This is the bulk of the work; the current one-level field-only
  flattening there is replaced.
- `GoTypeSignature` gains an embedded-type list, which changes its literal
  construction sites in `internal/mygo/compiler` and
  `internal/mygo/typeinference`.
- No changes to parser2, codegen2, or the prelude. `embed` syntax and its
  codegen were verified working and are out of scope.
- Regression surface: the `bootstrap-ffi-boundary` spec scenarios must keep
  passing unchanged.
