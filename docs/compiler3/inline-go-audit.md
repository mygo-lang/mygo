# Inline Go and boundary audit

Recorded from all checked-in `.mygo` sources during the IR3 baseline. The raw
inventory with every occurrence is checked in as
[`inline-go-inventory.txt`](inline-go-inventory.txt).

## Summary

- Total `go[...]` occurrences: **240**
- `compatible` (native Go shapes only): **207**
- `needs-adaptation` (mentions or constructs Option/Result/List/dynamic shapes): **33**
- `unsupported` (requires the old interface/type-assertion shape): **0 found in the initial scan**

The classification is an initial pass over the occurrence line plus the
following 25 lines. The raw inventory is the authoritative per-occurrence list;
review the surrounding block before changing a snippet.

## Needs adaptation

| File:line | Declared result type | Why |
| --- | --- | --- |
| [`internal/mygo/compiler/go_ffi_import.mygo:412`](../../internal/mygo/compiler/go_ffi_import.mygo:412) | `Result[BootstrapGoPackageInfo, String]` | Result becomes a value struct instead of an interface. |
| [`internal/mygo/compiler/go_ffi_import.mygo:426`](../../internal/mygo/compiler/go_ffi_import.mygo:426) | `Result[Map[String, BootstrapGoPackageInfo], Error]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap.mygo:239`](../../internal/mygo/compiler/bootstrap.mygo:239) | `Result[Slice[String], Error]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap.mygo:299`](../../internal/mygo/compiler/bootstrap.mygo:299) | `Result[String, Error]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap.mygo:982`](../../internal/mygo/compiler/bootstrap.mygo:982) | `Result[Slice[String], Error]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap.mygo:1005`](../../internal/mygo/compiler/bootstrap.mygo:1005) | `Result[String, Error]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap.mygo:1049`](../../internal/mygo/compiler/bootstrap.mygo:1049) | `Result[String, Error]` | Snippet spells `Result[string, error]` and calls `Ok`/`Err`. |
| [`internal/mygo/compiler/bootstrap.mygo:1176`](../../internal/mygo/compiler/bootstrap.mygo:1176) | `Result[String, Error]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap.mygo:1198`](../../internal/mygo/compiler/bootstrap.mygo:1198) | `Result[BootstrapInputs, ast2.Diagnostic]` | Result becomes a value struct. |
| [`internal/mygo/compiler/bootstrap_diagnostic.mygo:28`](../../internal/mygo/compiler/bootstrap_diagnostic.mygo:28) | `Option[ast2.Diagnostic]` | Option becomes a value struct. |
| [`internal/mygo/compiler/bootstrap_diagnostic.mygo:79`](../../internal/mygo/compiler/bootstrap_diagnostic.mygo:79) | `Option[String]` | Option becomes a value struct. |
| [`internal/mygo/codegen2/codegen2.mygo:124`](../../internal/mygo/codegen2/codegen2.mygo:124) | `Result[Slice[ast2.Decl], ast2.Diagnostic]` | Result becomes a value struct. |
| [`internal/mygo/codegen2/codegen2.mygo:148`](../../internal/mygo/codegen2/codegen2.mygo:148) | `Result[Map[String, String], ast2.Diagnostic]` | Result becomes a value struct. |
| [`internal/mygo/codegen2/codegen2.mygo:176`](../../internal/mygo/codegen2/codegen2.mygo:176) | `Result[Map[String, String], ast2.Diagnostic]` | Result becomes a value struct. |
| [`internal/mygo/codegen2/panic_boundary.mygo:29`](../../internal/mygo/codegen2/panic_boundary.mygo:29) | `Result[Slice[goast.Stmt], ast2.Diagnostic]` | Result becomes a value struct. |
| [`internal/mygo/codegen2/panic_boundary.mygo:42`](../../internal/mygo/codegen2/panic_boundary.mygo:42) | `Result[AstExprResult, ast2.Diagnostic]` | Result becomes a value struct. |
| [`internal/mygo/codegen2/panic_boundary.mygo:53`](../../internal/mygo/codegen2/panic_boundary.mygo:53) | `Result[Slice[goast.Decl], ast2.Diagnostic]` | Result becomes a value struct. |
| [`lib/concurrency/channel.mygo:117`](../../lib/concurrency/channel.mygo:117) | `Option[T]` | Option becomes a value struct. |
| [`lib/concurrency/channel.mygo:133`](../../lib/concurrency/channel.mygo:133) | `Option[T]` | Option becomes a value struct. |
| [`lib/concurrency/channel.mygo:155`](../../lib/concurrency/channel.mygo:155) | `Option[T]` | Option becomes a value struct. |
| [`lib/concurrency/channel.mygo:171`](../../lib/concurrency/channel.mygo:171) | `Option[T]` | Option becomes a value struct. |
| [`lib/sort/sort.mygo:51`](../../lib/sort/sort.mygo:51) | `Option[Int]` | Option becomes a value struct. |
| [`prelude/set.mygo:74`](../../prelude/set.mygo:74) | `Option[Ref[A]]` | Canonical layout is `*A`; `nil` denotes `None`. |
| [`prelude/slice.mygo:76`](../../prelude/slice.mygo:76) | `Option[Ref[T]]` | Canonical layout is `*T`; snippet currently spells `Option[*T]`. |
| [`prelude/slice.mygo:171`](../../prelude/slice.mygo:171) | `Option[T]` | Option becomes a value struct. |
| [`prelude/map.mygo:74`](../../prelude/map.mygo:74) | `Option[Ref[V]]` | Canonical layout is `*V`; `nil` denotes `None`. |
| [`prelude/map.mygo:112`](../../prelude/map.mygo:112) | `Option[V]` | Option becomes a value struct. |
| [`prelude/slice_combine.mygo:103`](../../prelude/slice_combine.mygo:103) | `Option[T]` | Option becomes a value struct. |
| [`prelude/slice_combine.mygo:118`](../../prelude/slice_combine.mygo:118) | `Option[T]` | Option becomes a value struct. |
| [`prelude/slice_order.mygo:45`](../../prelude/slice_order.mygo:45) | `Option[T]` | Option becomes a value struct. |
| [`prelude/slice_order.mygo:66`](../../prelude/slice_order.mygo:66) | `Option[T]` | Option becomes a value struct. |
| [`prelude/slice_search.mygo:68`](../../prelude/slice_search.mygo:68) | `Option[Int]` | Option becomes a value struct. |
| [`prelude/slice_search.mygo:85`](../../prelude/slice_search.mygo:85) | `Option[Int]` | Option becomes a value struct. |

## Boundary surfaces that are not inline Go

These are part of the same representation boundary and must be addressed by
`codegen3`:

- `internal/mygo/codegen2/dynamic_metadata.go` and `internal/mygo/codegen2/decls.mygo`:
  dynamic wrapper/adapter emission. `codegen3` must derive this information
  from source declarations and must not emit a metadata file.
- `internal/mygo/typeinference2/dynamic_boxing.mygo`: currently reads
  `DynamicMetadataJSON` and calls `metadata.LookupDynamicImplementationID` /
  `metadata.DynamicBoxingImplementations`. This dependency must be replaced by
  source-derived package information.
- `internal/mygo/compiler/metadata_import.go`: currently reconstructs exported
  declarations from generated Go comments. The `codegen3` path must load
  dependency declarations from source instead.
- `internal/mygo/compiler/go_ffi_import.mygo`: FFI signatures remain relevant,
  but no metadata digest or `adapterABI` field is emitted by `codegen3`.
- `zz_mygo_metadata.gen.go`: not produced by `codegen3`. Existing `codegen2`
  output may keep it temporarily for rollback and differential testing.

## Verification

- Regenerate the inventory when inline Go snippets change.
- Add compile-time checks that no `codegen3` output contains
  `zz_mygo_metadata.gen.go`.
- Add round-trip tests for `Result[T, error]`, `Option[Ref[T]]`, and
  source-derived dynamic dispatch before `codegen3` becomes the default.
