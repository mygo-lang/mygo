## Why

The prelude collection surface stops at the seven `IEnumerable` methods
(`Each`/`Len`/`Map`/`Filter`/`Fold`/`Find`/`Contains`) plus a handful of
type-specific helpers (`Slice.Append`/`Prepend`, `Set.New`/`Add`/`Delete`,
`Map.ToPairs`, `List.Cons`/`Head`/`Tail`). Everyday work such as concatenating
two slices, taking a prefix, reversing, sorting, de-duplicating, or zipping still
forces users to drop into an inline `go[...]` block, which leaks Go code into
MyGO call sites.

## What Changes

- Introduce new prelude typeclass interfaces for collection semantics that are
  shared across container types, keeping the existing `IEnumerable` method set
  untouched so current dispatch keeps resolving:
  - sequence operations (`Concat`, `Take`, `Skip`, `TakeWhile`, `SkipWhile`,
    `Reverse`, `ToSlice`) implemented for `Slice`, `List`, and `String`;
  - ordering operations (`Sort`, `Sorted`, `SortBy`, `SortedBy`, `Min`, `Max`)
    constrained by `Ord[A]`;
  - de-duplication and positional lookup (`Distinct`, `IndexOf`, `LastIndexOf`)
    constrained by `Eq[A]`.
- Add the `Slice` combinators that do not generalize across containers:
  `Zip`, `ZipWith`, `Flatten`, `Any`, `All`, `Partition`, `Chunk`,
  `First`, `Last`.
- Expose mutability as a visible choice at the call site through paired method
  names: `Sort`/`SortBy` mutate the receiver's backing array in place, while
  `Sorted`/`SortedBy` return a fresh slice and leave the input untouched.
- Add map operations: `Keys`, `Values`, `Merge`, `FilterKeys`, `MapValues`,
  `ContainsKey`, `FromPairs`.
- Add set operations: `Union`, `Intersect`, `Difference`, `IsSubsetOf`,
  `ToSlice`, `FromSlice`.
- Add list and string helpers that have no cross-container meaning: `List.ToSlice`,
  `List.FromSlice`, `List.Append`, `List.Prepend`, `String.SplitLines`,
  `String.PadLeft`, `String.PadRight`, `String.Count`.
- **BREAKING**: `List[A]` becomes a two-variant enum (`Nil` / `Node`) so an
  empty list is representable. `List.Cons`, `List.Head`, and `List.Tail`
  keep their signatures, but the `List[T] { head: ..., tail: ... }` struct
  literal stops compiling.
- Two compiler fixes are prerequisites for the enum change: a zero-argument
  enum variant constructor must emit its type arguments, and an inner local
  binding must shadow a Go package alias during member resolution.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `prelude`: the prelude collection surface gains concat, sequence, ordering,
  de-duplication, lookup, aggregation, and container-specific combinator
  requirements across `Slice`, `List`, `Map`, `Set`, and `String`.

## Impact

- `prelude/slice.mygo`, `prelude/list.mygo`, `prelude/map.mygo`,
  `prelude/set.mygo`, `prelude/string.mygo`, `prelude/prelude.mygo`, and their
  regenerated `prelude/zz_*.gen.go` output.
- New prelude interfaces are added to `prelude/prelude.mygo`; existing
  `IEnumerable`, `IIndexable`, and `IAssignable` declarations are unchanged.
- The prelude must be re-synced with `mygo --no-prelude sync prelude/` and the
  generated files rebuilt in dependency order.
- Ordering and de-duplication depend on typeclass dictionaries for `Ord[A]` and
  `Eq[A]` inside generated helper bodies; the exact dispatch path for
  constraint-annotated methods is a validation point (see design and tasks).
- `internal/mygo/codegen2/translate_ast.mygo` and
  `internal/mygo/typeinference2/infer.mygo` plus their regenerated output; the
  compiler must be rebuilt before the prelude sources can be re-synced.
- `internal/mygo/parser2` and `examples/data-structure` are regenerated
  because their emitted calls into `List` now need explicit type arguments.
- New behaviour is covered by `prelude/*_test.mygo` regression tests.
