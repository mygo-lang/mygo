## Context

See proposal.md for motivation. The constraints that shape the approach come
from how the prelude is written and compiled today:

- The prelude is MyGO source under `prelude/`, compiled with
  `mygo --no-prelude sync prelude/` into `prelude/zz_*.gen.go`, so every new
  operation is authored in MyGO and shipped as generated Go.
- Collection dispatch runs through typeclass dictionaries. In an inline
  `go[...]` body, a constraint method declared with `using <Iface>[T]` is
  reachable under the name `<Method>Fn`; `prelude/slice.mygo` already calls
  `EqualsFn(v, item)` inside `Contains`. `Ord` methods follow the same shape,
  so `LessFn`/`CompareFn` are available to generated bodies.
- `Slice`, `Map`, and `Set` lower to Go natives, and the compiler derives
  `comparable` constraints for type parameters that end up as Go map keys.
- Method-call resolution uses `methodSpellCount`, a conservative upper bound
  with a plain-call fallback. Growing an existing interface changes the prefix
  arithmetic for every implementation of that interface.
- `KNOWN_ISSUES.md` records that an inherent-impl method annotated with
  `using Default[A]` could not resolve a zero-argument typeclass member, which
  is why `UnwrapOrDefault` is deferred. Constraint use on inherent impls is
  therefore treated as an open risk rather than an established capability.

## Goals / Non-Goals

**Goals:**

- Give the prelude a usable collection surface in MyGO, without inline Go at
  call sites, for the operations listed in the spec delta.
- Keep each new operation's constraint requirements explicit in its signature
  so ordering and de-duplication are checked at the call site.
- Make the in-place versus copying choice visible in the method name.
- Cover the new surface with prelude-level tests that exercise both the ordinary
  case and the empty/edge cases.

**Non-Goals:**

- Lazy or streaming iterators; the prelude works on materialized collections.
- Changing the existing `IEnumerable`, `IIndexable`, or `IAssignable`
  declarations or the behaviour of the existing methods.
- Changing Go's slice aliasing rules or redefining existing `Append`/`Prepend`.
- Adding numeric-only aggregates such as `Sum` that would require a new numeric
  typeclass; `Min`/`Max` cover the common case through `Ord`.

## Decisions

### New interfaces instead of extending `IEnumerable`

The shared sequence operations go into new interfaces; `IEnumerable` keeps its
current seven methods. Extending it would force every existing implementation
(including `Option` and `Range`, where sequence shaping has no meaning) to
provide the new members and would shift `methodSpellCount` for every call site
that resolves through it. New interfaces leave the existing dispatch paths
untouched and carry only the types where the operations make sense.

Alternatives considered: adding to `IEnumerable` (rejected for the reasons
above); declaring everything as per-type inherent methods (rejected because
`Concat`/`Take`/`Reverse` would then be written three times for `Slice`,
`List`, and `String` with no shared contract).

### Mutability expressed by paired method names

`Sort` and `SortBy` order the receiver's backing array in place; `Sorted`
and `SortedBy` copy first and return the new slice. The choice is a property of
the call, so it belongs in the name.

Alternatives considered: a runtime boolean parameter (rejected: it makes the
in-place case look like a flag on a functional API); a typeclass pair where the
user selects a policy instance (rejected: two unnamed-instantiable
implementations on the same receiver are ambiguous under the current dispatch
rules, and `using` instance selection is not part of the call-site syntax
users already rely on).

### Ordering via `Ord[T]`, de-duplication split by runtime comparability

`Sort`/`Sorted`/`Min`/`Max` take a `using Ord[T]` constraint and call the
dictionary's comparison inside their generated bodies. `Distinct`,
`IndexOf`, and `LastIndexOf` take `using Eq[T]`. This keeps the constraint
requirement in the signature, so a call on an element type without an `Ord` or
`Eq` instance fails at the call site instead of at code generation.

`Distinct` keeps one entry point and selects its strategy at run time. It asks
the reflection layer whether the element type is comparable; when it is, it
de-duplicates through a `map[any]struct{}` hash pass, and when it is not it
scans the elements already emitted with `EqFn`. Both branches preserve
first-occurrence order.

This is possible because `map[any]struct{}` imposes no `comparable`
constraint on `T`; only the reflected comparison needs one, and it runs before
the hash pass is entered, so a non-comparable element type never reaches the
map. A native `map[T]struct{}` would force the emitted Go function to declare
`[T comparable]`, which cannot hold for the fallback branch in the same
generic body.

The cost is that the two branches do not share an equivalence relation. The
hash pass compares with Go equality while the fallback uses the caller's
`Eq[T]` instance, so an element type whose `Eq` instance differs from Go
equality is de-duplicated by Go equality. The spec records that consequence and
a test pins it.

Alternatives considered: a compile-time split that forces a `Map`/`Set` type
position into the signature so the compiler derives `comparable` (rejected: two
branches still cannot share one function body, so it only trades the entry
point for a second method name and an extra argument); a pure `EqFn` scan
(rejected: O(n^2) on the primitive element types that dominate real use).

### `Concat` allocates; container-specific names carry the rest

`Concat` returns a fresh slice so it cannot write into the receiver's spare
capacity. The container-specific operations keep their own names rather than
being folded into `Concat`: `Map.Merge` has to define conflict precedence and
`Set.Union` has to de-duplicate, so neither is the same operation as
concatenation.

### Where each operation lives

| Operation group | Home | Why |
|---|---|---|
| `Concat`, `Take`, `Skip`, `TakeWhile`, `SkipWhile`, `Reverse`, `ToSlice` | new sequence interface, implemented by `Slice`, `List`, `String` | same shape, different result type |
| `Sort`, `Sorted`, `SortBy`, `SortedBy`, `Min`, `Max` | ordering interface with `using Ord[T]` + `Slice` inherent methods | ordering is meaningful for sequences; the in-place pair is slice-specific |
| `Distinct`, `IndexOf`, `LastIndexOf` | de-duplication/lookup interface with `using Eq[T]` | needs element equality; `Distinct` picks its branch by runtime comparability |
| `Zip`, `ZipWith`, `Flatten`, `Any`, `All`, `Partition`, `Chunk`, `First`, `Last` | `Slice` inherent impl | slice-only shape or element-type relationships |
| `Keys`, `Values`, `Merge`, `FilterKeys`, `MapValues`, `ContainsKey`, `FromPairs` | `Map` inherent impl | key/value structure |
| `Union`, `Intersect`, `Difference`, `IsSubsetOf`, `ToSlice`, `FromSlice` | `Set` inherent impl | set algebra |
| `SplitLines`, `PadLeft`, `PadRight`, `Count` | `String` inherent impl | string-only |

### Edge-case conventions

### `SortBy`/`SortedBy` take a comparison function

The first implementation took a key function plus a method-level type parameter
`K` constrained by `using Ord[K]`. Dispatch resolved that dictionary against
the receiver's element type rather than the key type, so a slice of records
whose element type also implemented `Ord` failed with a `Compare` signature
mismatch. The methods now take `less: func(A, A) -> Bool`, which removes both
the method-level type parameter and the dictionary lookup.

### `Partition` returns a named pair

A declared top-level tuple return lowers to a Go multi-return signature, but an
impl-method call site lowers `let (a, b) = ...` to field access on a single
struct value. The two disagree, so `Partition` returns `Partitioned[T]` with
`matched` and `rest` fields instead of `(Slice[A], Slice[A])`.

### `List` becomes a two-variant enum

`List[A]` was a struct whose `head` field was mandatory, so an empty list had
no representation. `Take(0)`, `Skip(len)`, a `TakeWhile` that rejects the
first element, and `FromSlice([])` all had no value to return. The type is now
`enum List[A] { Nil, Node(A, Option[Ref[List[A]]]) }`, which makes the empty
case representable and lets `List` implement the shared sequence interface
honestly. `Cons`, `Head`, and `Tail` keep their signatures so parser2 and the
examples keep compiling; only the struct literal form is gone.

Alternatives considered: returning `Option[List[T]]` from the operations that
can produce an empty result (rejected: it makes every list pipeline branch, and
the interface cannot express it); a placeholder payload on `Nil` (rejected: it
stores a meaningless value in every empty list).

### Compiler prerequisites

`Nil` has no payload, so the generated `List__Nil__Ctor` has no argument from
which Go could infer its type argument, and Go does not infer type arguments
through a generic interface parameter at all. Two fixes were needed:

- `translate_ast` now emits zero-argument enum variant constructions through
  `goast.GenericCall` with `enumTypeArgs`, matching how `Some`/`None` already
  spell their type arguments.
- `typeinference2` resolves `name.field` against an inner local binding before
  the Go package-alias path, so a local whose name collides with a package alias
  follows Go's shadowing rule instead of failing as an unknown package member.

Both are prerequisites for the prelude sources to compile; the compiler must be
rebuilt before the prelude is re-synced.

- `Take`/`Skip` clamp an out-of-range count instead of returning `None`; the
  existing `Slice` indexing path already uses `Option` for out-of-range
  indexes, but clamping keeps prefix/suffix pipelines free of branching.
- `Zip` stops at the shorter input.
- `First`/`Last`/`Min`/`Max`/`IndexOf`/`LastIndexOf` return `Option`.
- `Chunk` emits a trailing partial group.
- `PadLeft`/`PadRight` measure width in runes and leave a longer string alone.

## Risks / Trade-offs

- [Constraint dispatch on inherent impls may not resolve] The deferred
  `UnwrapOrDefault` shows the zero-argument case failing on an inherent impl
  (`KNOWN_ISSUES.md`). The ordering and de-duplication methods use argument-taking
  constraint members (`LessFn`, `CompareFn`, `EqualsFn`), which is a different
  path and already works inside a typeclass impl. → Validate with a minimal spike
  before writing the full surface; if an inherent impl cannot see the
  dictionary, declare those methods on the interface implementation instead of
  the inherent impl and keep the same call syntax.
- [Growing an interface shifts method resolution] Mitigated by leaving
  `IEnumerable` unchanged and confining the new interfaces to their own
  implementations.
- [Reflection and boxing in `Distinct`] The hash branch probes the element type
  with reflection and boxes each element into `any`, so it costs more per
  element than a native `map[T]` would. → The probe runs once per call and the
  fallback branch stays allocation-light; accepted for the common case.
- [`Distinct` equivalence depends on the element type] A comparable type with a
  custom `Eq` instance de-duplicates by Go equality rather than by that
  instance. → Recorded in the spec and pinned by a test.
- [O(n^2) `Distinct` fallback and O(n^2) `IndexOf`] Acceptable for the
  collection sizes the prelude targets, and both keep the `Eq`-only element
  requirement.
- [Surprising aliasing] `Sort` mutates the receiver's backing array, so a slice
  produced earlier by `SliceSlice`/`append` shares the change. Documented by
  the paired naming and covered by a test asserting `Sorted` does not modify the
  receiver.
- [Generated output drift] New MyGO prelude sources require a re-sync and a
  regeneration of `prelude/zz_*.gen.go` in dependency order; a stale generated
  file would silently ship the old surface.
- [Bootstrap cost] The prelude is compiled by the compiler under bootstrap, so
  the new sources must compile through the bootstrap pipeline, not only through
  a host `go build`.
- [Go toolchain inlining bug around a generic interface map] Instantiating an
  inline `map[any]` keyed by a generic element type inside a dictionary-taking
  method made the linker panic with `R_USEIFACE ... type:.eqfunc` once the
  element was a struct. → The hash pass lives in `DistinctHashed`, a separate
  method with no `using` parameters; `Distinct` only branches into it, and the
  same body keeps the `EqFn` scan.

## Migration Plan

1. Spike the `using Ord[T]`/`using Eq[T]` dispatch shape needed by ordering and
   de-duplication, and confirm the generated body references the dictionary
   functions.
2. Add the interfaces and implementations to the prelude sources, then run
   `mygo --no-prelude sync prelude/` and regenerate the `zz_*.gen.go` files.
3. Add `prelude/*_test.mygo` coverage and run it through the bootstrap pipeline.
4. Rollback is a revert of the prelude sources and their regenerated output;
   nothing outside `prelude/` changes.

## Open Questions

- Whether `TakeWhile`/`SkipWhile` should also appear on `Map`/`Set` is left for a
  later change; iteration order there is unspecified, so the operations would
  not have a stable definition.
