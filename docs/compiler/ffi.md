# ffi.md — Go FFI and import "go:…"

## Go FFI foundations

- Use `import "go:pkg/name"` for Go packages.
- Allow an optional alias form like `import fmt "go:fmt"` when the Go package name should be explicit.
- Package-qualified selectors such as `fmt.Sprint(...)` should lower as Go selectors, not as struct field access.
- The built-in prelude provides common typeclasses such as `Show[A]` and `Eq[A]`; prefer using those protocols rather than ad hoc `any` formatting or conversion.
- The built-in prelude also owns foundational algebraic data types like `Option[A]` and `Result[A, E]`; use those rather than redeclaring them in example packages.
- Generated Go should only include helper imports when they are actually needed; `reflect` is now a fallback for truly dynamic `any` function calls, not a blanket import.
- Typeclass-style `impl` blocks should lower to standalone helper functions plus explicit function parameters at call sites, not to method dictionaries.

## Ref types

- `Ref[T]` is the non-nil reference form at the Go boundary and should lower to `*T` in generated Go.
- `Ref[T]` remains a compiler-recognized boundary type, not a prelude-declared enum or struct.
- `Ref.new(expr)` is the canonical MyGO expression for producing a `Ref[T]`; it lowers to Go address-taking (`&expr`) and should be preferred over exposing raw `&` syntax in MyGO source.
- `Option[Ref[T]]` is the preferred shape for possibly-nil pointer returns and should be preserved rather than collapsed to a bare pointer.

## Option / Result

- `Option` continues to represent absence for nilable Go values and comma-ok style results.
- `Result` is the dedicated shape for Go `error`-bearing flows and should be used instead of encoding failures as `Option`.

## Collection types

- `List[A]` is a singly-linked list with `head: A` and `tail: Option[Ref[List[A]]]`; `None` terminates the list.
- `Slice[A]` is MyGO's canonical slice type spelling and lowers directly to Go's native slice `[]A`.
- `Map[K, V]` is Go's native map `map[K]V`.
- `Set[A]` is Go's native set `map[A]struct{}`.

## IAssignable interface — indexed access for Slice and Map

- `IAssignable[C[A], K, A]` is a generic interface that provides indexed access (read + write) for both `Slice` and `Map`.
- Three type parameters: `C[A]` (the container type, one of `Slice[V]` or `Map[K, V]`), `K` (the index/key type), `A` (the value type).
- Two methods:
  - `func get(c: C[A], index: K) -> Option[Ref[A]]` — safely read a value by index/key; returns `None` if the index is out of range (Slice) or the key does not exist (Map), `Some(ref)` otherwise.
  - `func set(c: C[A], index: K, value: A) -> ()` — write a value at the given index/key.
- Concrete instantiations:
  - `Slice[T]: IAssignable[Slice[T], Int, T]` — `K = Int`
  - `Map[K, V]: IAssignable[Map[K, V], K, V]` — `K` is the map's key type

