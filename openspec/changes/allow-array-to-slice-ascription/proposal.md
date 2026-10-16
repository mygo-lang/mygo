## Why

Fixed-length arrays land in the compiler as native Go arrays (`T[N]` -> `[N]T`), but
there is no way to obtain a `Slice[T]` view of an array value. Writing
`values as Slice[Int]` fails type unification (`cannot unify Int[32] with
Slice[Int]`), and the language has no indexing or slicing expression that the
parser retains, so every call site that needs a slice of an array drops into an
inline `go[Slice[Int]] { code: "{a}[:]" }` block. This leaks Go source into MyGO
programs for an operation Go expresses as `a[:]`.

## What Changes

- Accept `T[N] as Slice[T]` ascription. It lowers to the Go slice expression
  `a[:]`, so the result is a `Slice[T]` that shares the operand's backing storage.
- Require the array element type to unify with the slice element type, including
  nested element types (`Int[3][2] as Slice[Int[2]]` -> Go `[][2]int`).
- Keep rejecting the other directions and mismatches: `Slice[T] as T[N]` fails,
  and element-type mismatches such as `Int[32] as Slice[String]` keep producing a
  source-located diagnostic.
- Add no new indexing/slicing syntax and no prelude helper: the ascription itself
  is the surface, and array values keep their static length everywhere else.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-go-arrays`: adds an array-to-slice ascription requirement so
  `T[N] as Slice[T]` is accepted while the existing Slice-to-array rejection is
  preserved.

## Impact

- `internal/mygo/typeinference2/infer.mygo` (`inferTypeAs`) and its regenerated
  `internal/mygo/typeinference2/zz_infer.gen.go`.
- `internal/mygo/codegen2/translate_ast.mygo` (`translateTypeAsAst`) plus a new
  slice-expression builder in `internal/mygo/codegen2/goast/goast.go`, and their
  regenerated output.
- Bootstrap re-sync order: `typeinference2` -> `codegen2` -> `compiler` with a
  writable `GOCACHE`; the compiler must be rebuilt before re-syncing.
- End-to-end coverage in `internal/mygo/compiler` bootstrap tests.
- `docs/compiler/collections.md` documents the new ascription.

