## Why

Two gaps in the Go FFI type-surface collector make real third-party Go
libraries (verified against `gorm.io/gorm v1.31.2`) partially unusable:

1. Exported package-level `var`s are never imported, so selectors over them
   fail with `unknown Go package member`. GORM exports its whole error surface
   this way (`ErrRecordNotFound`, `ErrInvalidData`, `ErrDuplicatedKey`), and
   so do standard-library packages (`os.ErrNotExist`).
2. Promoted fields of embedded Go structs are not expanded, so a MyGO struct
   that embeds a Go type via `embed gorm.Model` cannot reach the embedded
   type's own exported fields (`u.ID`, `u.CreatedAt`).

Package-level `const`s, interface method dispatch, `(T, error)` boundary
wrapping, and `embed`-field codegen were all verified working, so this change
closes the two remaining holes rather than redesigning the boundary.

## What Changes

- The Go FFI surface collector gains a package-level `var` pass, mirroring
  the existing `const` pass: exported `*types.Var` objects are recorded as
  values of their declared type, so `gorm.ErrRecordNotFound` and
  `os.ErrNotExist` type-check like ordinary selectors.
- Go struct field collection expands embedded fields, so a field promoted
  from an embedded Go struct resolves on the MyGO struct that embeds it.
- The hand-written `typeinference` FFI loader is kept in step with the
  bootstrap loader so the two pipelines agree at this boundary.

No breaking changes: the collector only gains members it previously dropped,
so programs that compiled before continue to compile.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: extend the imported Go type surface to include
  exported package-level variables and the promoted fields of embedded Go
  structs.

## Impact

- `internal/mygo/compiler/go_ffi_import.go` — the bootstrap surface collector
  (its `*types.Const` pass and its `goTypeFields` helper).
- `internal/mygo/typeinference/go_imports.go` — the hand-written loader's
  equivalent constant pass, for pipeline parity.
- `internal/mygo/typeinference2/` — no change expected; it consumes the
  collector output. Verified that `GoConstSignature` already carries a type
  string that a `var` entry can reuse.
- No changes to parser2, codegen2, or the prelude. `embed` syntax and its
  codegen were verified working and are out of scope.
- Regression surface: the `bootstrap-ffi-boundary` spec scenarios must keep
  passing unchanged.
