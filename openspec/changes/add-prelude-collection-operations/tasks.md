## 1. Dispatch spike and interface scaffolding

- [x] 1.1 Add a minimal probe to the prelude that calls an argument-taking
  `Ord[A]` member (`Less`/`Compare`) and an `Eq[A]` member (`Equals`) from
  inside an inline `go[...]` body, once on a typeclass impl and once on an
  inherent impl. Verify by running `mygo --no-prelude sync prelude/` and
  inspecting the generated Go for the dictionary function references
  (`LessFn`/`CompareFn`/`EqualsFn`). Record which impl form resolves.
- [x] 1.2 Declare the new sequence, ordering, and de-duplication/lookup
  interfaces in `prelude/prelude.mygo` with their full method signatures,
  including the `using` constraints. Verify the prelude still syncs and the
  existing `IEnumerable`/`IIndexable`/`IAssignable` declarations are byte-for-byte
  unchanged.
- [x] 1.3 Confirm no existing call site changes resolution by rebuilding the
  compiler and running `go test ./internal/mygo/...` before any new method is
  implemented, so later failures cannot be attributed to the scaffolding.
- [x] 1.4 Confirm the de-duplication probe works from an inline `go[...]` body:
  add a `go:reflect` import to the prelude, sync it, and verify the generated Go
  compiles and that the probe reports `false` for an element type that is not
  comparable in its Go representation.

## 2. Slice sequence operations

- [x] 2.1 Implement `Concat` so it allocates a fresh slice and copies both
  inputs, and `Reverse` returning a new slice. Verify with a prelude test that
  the concatenated result is correct and that both inputs are unchanged
  afterwards.
- [x] 2.2 Implement `Take`, `Skip`, `TakeWhile`, and `SkipWhile` with clamped
  counts. Verify with prelude tests covering in-range counts, counts past the
  end, negative counts, and an empty receiver.
- [x] 2.3 Implement the shared sequence interface for `Slice`, and wire
  `ToSlice` so a slice converts to a slice by identity content. Verify with a
  prelude test that method syntax (`xs.Concat(ys)`) resolves through the
  interface implementation.

## 3. Slice combination and aggregation

- [x] 3.1 Implement `Zip` and `ZipWith` with shorter-input truncation. Verify
  with prelude tests covering equal lengths, a longer left input, and an empty
  input.
- [x] 3.2 Implement `Flatten` for `Slice[Slice[T]]` and `Partition` returning
  two slices. Verify with prelude tests including empty inner slices.
- [x] 3.3 Implement `Any`, `All`, `Chunk`, `First`, and `Last`. Verify with
  prelude tests covering the non-multiple-of-`n` chunk case and `First`/`Last`
  on empty slices returning `None`.

## 4. Slice ordering operations

- [x] 4.1 Implement stable `Sort` (in place) and `Sorted` (copying) using the
  `Ord[T]` constraint and the dispatch form confirmed in 1.1. Verify with
  prelude tests asserting the ordering result, that `Sorted` leaves the receiver
  unchanged, and that `Sort` orders the receiver in place.
- [x] 4.2 Implement `SortBy`/`SortedBy` over a supplied comparison function and
  verify with a prelude test on a slice of records.
- [x] 4.3 Implement `Min` and `Max` returning `Option`, and verify with prelude
  tests covering a mixed slice and an empty slice.
- [x] 4.4 Verify stability explicitly with a prelude test whose elements compare
  equal but carry a distinguishable secondary field, asserting the original
  relative order survives.

## 5. Slice de-duplication and lookup

- [x] 5.1 Implement `Distinct` with one entry point that probes the element
  type's Go comparability and selects the `map[any]struct{}` hash pass or the
  `EqFn` scan, preserving first-occurrence order in both branches. Verify with
  prelude tests on a primitive element slice, on a non-comparable element type
  that implements `Eq`, and on an empty slice.
- [x] 5.2 Pin the documented consequence with a test asserting that a
  comparable element type whose `Eq` instance differs from Go equality
  de-duplicates by Go equality.
- [x] 5.3 Implement `IndexOf` and `LastIndexOf` returning `Option`. Verify with
  prelude tests covering a repeated element and a missing element.

## 6. List, Map, Set, and String helpers

- [x] 6.1 Implement `List.Append`, `List.Prepend`, `List.ToSlice`, and
  `List.FromSlice`. Verify with a prelude test that `FromSlice` followed by
  `ToSlice` round-trips the order and that both insertions land at the expected
  end.
- [x] 6.2 Implement the shared sequence interface for `List` and `String`
  (`Concat`, `Take`, `Skip`, `TakeWhile`, `SkipWhile`, `Reverse`, `ToSlice`),
  returning `List` and `String` results respectively. Verify with prelude tests
  for both types.
- [x] 6.3 Implement `Map.Keys`, `Map.Values`, `Map.Merge`, `Map.FilterKeys`,
  `Map.MapValues`, `Map.ContainsKey`, and `Map.FromPairs`. Verify with prelude
  tests asserting merge precedence, key preservation under `MapValues`, and
  `FromPairs` duplicate handling.
- [x] 6.4 Implement `Set.Union`, `Set.Intersect`, `Set.Difference`,
  `Set.IsSubsetOf`, `Set.ToSlice`, and `Set.FromSlice`. Verify with prelude
  tests covering overlapping, disjoint, and duplicate inputs.
- [x] 6.5 Implement `String.SplitLines`, `String.PadLeft`, `String.PadRight`,
  and `String.Count`. Verify with prelude tests including a string already wider
  than the pad width and a non-overlapping occurrence count.

## 7. Regeneration and end-to-end validation

- [x] 7.1 Run `mygo --no-prelude sync prelude/` and confirm every touched
  `prelude/zz_*.gen.go` reflects the new surface with no stale file left behind.
- [x] 7.2 Run the prelude tests through the bootstrap pipeline
  (`go test ./prelude/...`) and confirm all new and existing cases pass.
- [x] 7.3 Compile a small MyGO program that chains several of the new
  operations (`Concat`, `Take`, `Sorted`, `Distinct`, `ToSlice`) and confirm it
  compiles and produces the expected values, exercising the operations through
  the normal import path rather than direct test helpers.
- [x] 7.4 Run the broader compiler test suite (`go test ./internal/mygo/...`)
  and confirm no regressions in existing method resolution or generated output.
