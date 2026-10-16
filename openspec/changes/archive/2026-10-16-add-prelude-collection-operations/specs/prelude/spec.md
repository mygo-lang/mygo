## ADDED Requirements

### Requirement: Slice concatenation and sequence operations
The prelude SHALL provide sequence-shaping methods on `Slice[T]`: `Concat`,
`Take`, `Skip`, `TakeWhile`, `SkipWhile`, and `Reverse`. `Concat` SHALL
return a newly allocated slice containing the receiver's elements followed by
the other slice's elements, and SHALL NOT write into either input's backing
array. `Take(n)` SHALL return the first `n` elements and `Skip(n)` the
remaining elements; a negative or oversized `n` SHALL be clamped to the valid
range rather than failing. `TakeWhile` and `SkipWhile` SHALL split the
receiver on the first element for which the predicate stops holding. `Reverse`
SHALL return a new slice with the elements in reverse order.

#### Scenario: Concat joins two slices
- **WHEN** `[1, 2].Concat([3, 4])` is called
- **THEN** the result is `[1, 2, 3, 4]`

#### Scenario: Concat leaves inputs unchanged
- **WHEN** `Concat` produces its result
- **THEN** neither input slice's elements are modified and both keep their
  original length

#### Scenario: Take stops at the end of a short slice
- **WHEN** `[1, 2].Take(5)` is called
- **THEN** the result is `[1, 2]`

#### Scenario: Skip beyond the end yields an empty slice
- **WHEN** `[1, 2].Skip(5)` is called
- **THEN** the result is empty

#### Scenario: TakeWhile stops at the first failing element
- **WHEN** `[1, 2, 5, 1].TakeWhile(fn)` is called with a predicate testing for
  values below 5
- **THEN** the result is `[1, 2]`

#### Scenario: Reverse returns a new ordering
- **WHEN** `[1, 2, 3].Reverse()` is called
- **THEN** the result is `[3, 2, 1]`

### Requirement: Slice combination and aggregation
The prelude SHALL provide `Zip`, `ZipWith`, `Flatten`, `Any`, `All`,
`Partition`, `Chunk`, `First`, and `Last` on `Slice[T]`. `Zip` SHALL pair
elements positionally up to the shorter input; `ZipWith` SHALL combine each
position with a function. `Flatten` SHALL collapse `Slice[Slice[T]]` into a
single slice. `Any` and `All` SHALL evaluate the supplied predicate across the
elements. `Partition` SHALL return the matching and non-matching elements as two
slices. `Chunk(n)` SHALL split the receiver into consecutive groups of at most
`n` elements, with a trailing shorter group when the length is not a multiple of
`n`. `First` and `Last` SHALL return the first and last element as an
`Option`.

#### Scenario: Zip stops at the shorter input
- **WHEN** `[1, 2, 3].Zip(["a", "b"])` is called
- **THEN** the result contains the pairs `(1, "a")` and `(2, "b")`

#### Scenario: Flatten collapses nested slices
- **WHEN** `[[1], [2, 3]].Flatten()` is called
- **THEN** the result is `[1, 2, 3]`

#### Scenario: All is false when one element fails
- **WHEN** `[2, 4, 5].All(fn)` is called with an even-number predicate
- **THEN** the result is `false`

#### Scenario: Chunk keeps a short trailing group
- **WHEN** `[1, 2, 3, 4, 5].Chunk(2)` is called
- **THEN** the result is `[[1, 2], [3, 4], [5]]`

#### Scenario: First on a non-empty slice
- **WHEN** `[7, 8].First()` is called
- **THEN** the result is `Some(7)`

#### Scenario: Last on an empty slice
- **WHEN** `[]` typed as `Slice[Int]` has `Last()` called on it
- **THEN** the result is `None`

### Requirement: Slice ordering operations
The prelude SHALL provide ordering methods on `Slice[T]` for any `T` that
implements `Ord`: `Sort`, `Sorted`, `SortBy`, `SortedBy`, `Min`, and
`Max`. Ordering SHALL be stable, so elements that compare equal keep their
relative order. `Min` and `Max` SHALL return an `Option` that is `None` for an
empty slice. `Sort`, `Sorted`, `Min`, and `Max` SHALL require an `Ord[T]`
implementation at the call site; `SortBy` and `SortedBy` SHALL take a
comparison function and need no `Ord` instance.

#### Scenario: Sorted orders elements
- **WHEN** `[3, 1, 2].Sorted()` is called
- **THEN** the result is `[1, 2, 3]`

#### Scenario: Sorted leaves the receiver unchanged
- **WHEN** `[3, 1, 2].Sorted()` is called
- **THEN** the receiver still reads `[3, 1, 2]` afterwards

#### Scenario: Sort orders the receiver in place
- **WHEN** `let xs = [3, 1, 2]` then `xs.Sort()` is called
- **THEN** `xs` reads `[1, 2, 3]` afterwards

#### Scenario: Ordering uses a supplied comparison
- **WHEN** `SortedBy` is called with a comparison function on a slice of records
- **THEN** the result is ordered by that comparison

#### Scenario: Min and Max on an empty slice
- **WHEN** `Min()` or `Max()` is called on an empty `Slice[T]`
- **THEN** the result is `None`

#### Scenario: Ordering respects equal elements
- **WHEN** a stable ordering runs over elements that compare equal
- **THEN** those elements keep their original relative order

### Requirement: Slice de-duplication and positional lookup
The prelude SHALL provide `Distinct`, `IndexOf`, and `LastIndexOf` on
`Slice[T]` for any `T` that implements `Eq`. `Distinct` SHALL remove later
duplicates and preserve first-occurrence order through a single method entry
point: when the element type is comparable in its Go representation it SHALL
de-duplicate through a hash-based pass, and otherwise it SHALL fall back to the
`Eq[T]` instance. The hash-based pass SHALL compare elements by Go equality,
so for an element type whose `Eq` instance differs from Go equality the outcome
is defined by the hash-based pass. `IndexOf` SHALL return the index of the first matching element and
`LastIndexOf` the index of the last, both as an `Option` that is `None` when no
element matches.

#### Scenario: Distinct keeps first occurrences
- **WHEN** `[1, 2, 1, 3, 2].Distinct()` is called
- **THEN** the result is `[1, 2, 3]`

#### Scenario: Distinct on a non-comparable element type
- **WHEN** `Distinct()` is called on a slice whose element type is not
  comparable in its Go representation but implements `Eq`
- **THEN** the result keeps the first occurrence of each `Eq`-equal element

#### Scenario: Distinct on an empty slice
- **WHEN** `Distinct()` is called on an empty slice
- **THEN** the result is empty

#### Scenario: IndexOf finds the first match
- **WHEN** `["a", "b", "a"].IndexOf("a")` is called
- **THEN** the result is `Some(0)`

#### Scenario: LastIndexOf finds the last match
- **WHEN** `["a", "b", "a"].LastIndexOf("a")` is called
- **THEN** the result is `Some(2)`

#### Scenario: Lookup reports a missing element
- **WHEN** `[1, 2].IndexOf(9)` is called
- **THEN** the result is `None`

### Requirement: Cross-container sequence typeclass
The prelude SHALL define a typeclass interface describing the sequence
operations of the Slice requirements above (`Concat`, `Take`, `Skip`,
`TakeWhile`, `SkipWhile`, `Reverse`, and a conversion to `Slice`), and
`Slice`, `List`, and `String` SHALL each implement it with their own result
type. A `List` operation SHALL return a `List` and a `String` operation SHALL
return a `String`; the conversion method SHALL return the elements as a
`Slice`.

#### Scenario: List supports the shared sequence operations
- **WHEN** `Concat`, `Take`, or `Reverse` is called on a `List`
- **THEN** the result is a `List` with the corresponding element ordering

#### Scenario: String supports the shared sequence operations
- **WHEN** `Take`, `Skip`, or `Reverse` is called on a `String`
- **THEN** the result is a `String` built from the corresponding runes

#### Scenario: Any collection converts to a slice
- **WHEN** the conversion method is called on a `Slice`, `List`, or `String`
- **THEN** the result is a `Slice` holding the same elements in the same order

### Requirement: List helpers
The prelude SHALL provide `Append`, `Prepend`, `ToSlice`, and `FromSlice` on
`List[T]`. `Append` SHALL add an element at the end of the list and
`Prepend` at the front, both returning the resulting list. `ToSlice` SHALL
return the list elements as a `Slice` in list order; `FromSlice` SHALL build a
`List` holding the slice elements in order. `List[A]` SHALL be a two-variant
enum whose `Nil` variant represents the empty list, so an operation whose
result has no elements SHALL return `Nil` rather than failing.

#### Scenario: An empty slice yields the empty list
- **WHEN** `List.FromSlice([] as Slice[Int])` is called
- **THEN** the result is `Nil` and converting it back with `ToSlice()` is empty

#### Scenario: FromSlice preserves order
- **WHEN** `List.FromSlice([1, 2, 3])` is called and then converted back with
  `ToSlice()`
- **THEN** the result is `[1, 2, 3]`

#### Scenario: Append adds at the end
- **WHEN** a list built from `[1, 2]` has `Append(3)` called on it and is
  converted with `ToSlice()`
- **THEN** the result is `[1, 2, 3]`

#### Scenario: Prepend adds at the front
- **WHEN** a list built from `[2, 3]` has `Prepend(1)` called on it and is
  converted with `ToSlice()`
- **THEN** the result is `[1, 2, 3]`

### Requirement: Map helpers
The prelude SHALL provide `Keys`, `Values`, `Merge`, `FilterKeys`,
`MapValues`, `ContainsKey`, and `FromPairs` on `Map[K, V]`. `Keys` and
`Values` SHALL return slices holding the map's keys and values, without
guaranteeing an order. `Merge` SHALL combine two maps, with the right-hand
entries winning on key conflict. `FilterKeys` SHALL keep the entries whose key
satisfies the predicate and `MapValues` SHALL transform values while keeping the
keys. `ContainsKey` SHALL report whether a key is present. `FromPairs` SHALL
build a map from a slice of key-value pairs, with later duplicates winning.

#### Scenario: Keys and Values expose entries
- **WHEN** `Keys()` and `Values()` are called on a map with two entries
- **THEN** both results hold two elements taken from the map's entries

#### Scenario: Merge prefers the right-hand value
- **WHEN** `{"a": 1}.Merge({"a": 2, "b": 3})` is called
- **THEN** the key `"a"` maps to `2` and `"b"` maps to `3`

#### Scenario: MapValues transforms values
- **WHEN** `MapValues` is called with a doubling function
- **THEN** the keys are unchanged and each value is doubled

#### Scenario: ContainsKey reports presence
- **WHEN** `ContainsKey` is called with a present key and with an absent key
- **THEN** the results are `true` and `false` respectively

### Requirement: Set helpers
The prelude SHALL provide `Union`, `Intersect`, `Difference`, `IsSubsetOf`,
`ToSlice`, and `FromSlice` on `Set[A]`. `Union` SHALL return the elements
present in either set, `Intersect` the elements present in both, and
`Difference` the elements present in the receiver but not the argument.
`IsSubsetOf` SHALL report whether every element of the receiver is also in the
argument. `ToSlice` SHALL return the elements as a `Slice` and `FromSlice`
SHALL build a `Set` from a `Slice`, dropping duplicates.

#### Scenario: Union combines sets
- **WHEN** `{"a", "b"}.Union({"b", "c"})` is called
- **THEN** the result contains `"a"`, `"b"`, and `"c"`

#### Scenario: Intersect keeps shared elements
- **WHEN** `{"a", "b"}.Intersect({"b", "c"})` is called
- **THEN** the result contains only `"b"`

#### Scenario: Difference removes shared elements
- **WHEN** `{"a", "b"}.Difference({"b", "c"})` is called
- **THEN** the result contains only `"a"`

#### Scenario: IsSubsetOf reports containment
- **WHEN** `IsSubsetOf` is called with a superset and with a disjoint set
- **THEN** the results are `true` and `false` respectively

#### Scenario: FromSlice drops duplicates
- **WHEN** `Set.FromSlice(["a", "a", "b"])` is called and converted with
  `ToSlice()`
- **THEN** the result holds `"a"` and `"b"` exactly once each

### Requirement: String helpers
The prelude SHALL provide `SplitLines`, `PadLeft`, `PadRight`, and `Count` on
`String`. `SplitLines` SHALL split on line boundaries. `PadLeft` and
`PadRight` SHALL pad the string to a minimum width with the supplied pad
character, leaving a string that already meets the width unchanged. `Count`
SHALL return the number of non-overlapping occurrences of the given substring.

#### Scenario: SplitLines separates lines
- **WHEN** `"a\nb\nc".SplitLines()` is called
- **THEN** the result is `["a", "b", "c"]`

#### Scenario: PadLeft reaches the requested width
- **WHEN** `"7".PadLeft(3, "0")` is called
- **THEN** the result is `"007"`

#### Scenario: Padding keeps a wider string unchanged
- **WHEN** `"abcd".PadRight(2, " ")` is called
- **THEN** the result is `"abcd"`

#### Scenario: Count finds non-overlapping occurrences
- **WHEN** `"aaaa".Count("aa")` is called
- **THEN** the result is `2`
