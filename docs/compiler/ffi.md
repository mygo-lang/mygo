# ffi.md — Go FFI and import "go:…"

## Go FFI foundations

- Use `import "go:pkg/name"` for Go packages.
- Allow an optional alias form like `import fmt "go:fmt"` when the Go package name should be explicit.
- Package-qualified selectors such as `fmt.Sprint(...)` should lower as Go selectors, not as struct field access.
- The built-in prelude provides common typeclasses such as `ToString[A]` and `Eq[A]`; prefer using those protocols rather than ad hoc `any` formatting or conversion.
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
- The prelude owns both types (`enum Option[A]`, `enum Result[A, E]` in `prelude/prelude.mygo`); use them rather than redeclaring local variants.

### File layout

- `prelude/prelude.mygo` — enums, interfaces, and standalone helpers (`OptionToResult`, `Panic`, `Zero`, ...).
- `prelude/option.mygo` — all `Option` impls (`OptionIEnumerable`, `impl[A] Option[A]`, `OptionEq`, and the `impl[A, E] Option[Result[A, E]]` transpose impl).
- `prelude/result.mygo` — all `Result` impls (`impl[A, E] Result[A, E]`, `ResultEq`) and the new combinators.

### `Result[A, E]` methods (`impl[A, E] Result[A, E]`)

Predicates: `IsOk`, `IsErr`.

Transforms: `Map[B](fn: func(A) -> B) -> Result[B, E]`, `MapErr[E2](fn: func(E) -> E2) -> Result[A, E2]`, `MapOr[B](defaultVal: B, fn: func(A) -> B) -> B`.

Chaining: `AndThen[B](fn: func(A) -> Result[B, E]) -> Result[B, E]`, `OrElse[E2](fn: func(E) -> Result[A, E2]) -> Result[A, E2]`, `And[B](other: Result[B, E]) -> Result[B, E]`, `Or[E2](other: Result[A, E2]) -> Result[A, E2]`, `Flatten() -> Result[A, E]`.

Extraction: `Unwrap() -> A`, `UnwrapOr(defaultVal: A) -> A`, `UnwrapOrElse(fn: func(E) -> A) -> A`, `Expect(msg: String) -> A`, `UnwrapErr() -> E`, `ExpectErr(msg: String) -> E`.

Conversion: `ToOption() -> Option[A]` (existing), `ToErr() -> Option[E]`.

### Automatic `Result` wrapping for Go FFI calls

- Any Go FFI call whose recorded signature returns `(T, error)` — a
  package-level function such as `myos.ReadFile(...)` or a method on an
  imported Go type such as `client.Do(req)` / `req.Cookie(name)` — is lowered
  at the boundary into `Result[T, error]`. This is the dedicated shape for Go
  error-bearing flows (see `wrapGoErrorResultCall` in the bootstrap codegen and
  `translateFFIResultCall` in the self-hosted codegen2).

```mygo
func DoRequest(client: Ref[http.Client], req: Ref[http.Request]) -> Result[Ref[http.Response], Error]
  client.Do(req)
end
```

generates a `Result[*http.Response, error]`-shaped body that calls
`client.Do(req)`, checks the returned error, and produces `Ok`/`Err` — instead
of leaking the raw two-value Go call into the return statement. The receiver's
type must name an imported Go package type (`Ref[T]` or value form), and the
method is resolved from the Go method-set table collected for that package.

- A Go FFI call whose signature is a lone trailing `error` (`func Foo() error`,
  e.g. `os.Chdir`, an `io.Writer.Flush`-style method) also lowers to a `Result`
  value: `Result[(), error]`, generated as `Result[struct{}, error]`. The unit
  payload collapses the Go side's "no value, only error" convention, so the
  MyGO caller pattern-matches `case Ok(_)` / `case Err(e)` exactly like a
  `(T, error)` call. `goSignatureResultShape`/`GoSignatureType` widen the
  two-result `(T, error)` rule to the arity-1 case, and
  `translateFFIResultCall` binds just the single error and synthesizes a
  `struct{}{}` unit payload for the `Ok` arm. In statement position (result
  discarded) the call is emitted as the raw Go call instead of an unused
  `Result`.

### `Option[A]` methods (`impl[A] Option[A]`)

Predicates: `IsSome`, `IsNone`.

Transforms: `MapOr[B](defaultVal: B, fn: func(A) -> B) -> B` (note `Map`/`Filter`/`Fold`/`Find`/`Len`/`Each`/`Contains` come from `OptionIEnumerable`).

Chaining: `AndThen[B](fn: func(A) -> Option[B]) -> Option[B]`, `OrElse(fn: func() -> Option[A]) -> Option[A]`, `Flatten() -> Option[A]`.

Extraction: `Unwrap() -> A`, `UnwrapOr(defaultVal: A) -> A` (existing), `UnwrapOrElse(fn: func() -> A) -> A`, `Expect(msg: String) -> A`.

Conversion: `OkOr[E](errVal: E) -> Result[A, E]`, `OkOrElse[E](fn: func() -> E) -> Result[A, E]`; the standalone `OptionToResult` helper is kept for compatibility.

### `Option[Result[A, E]]` methods (`impl[A, E] Option[Result[A, E]]`)

- `Transpose() -> Result[Option[A], E]` — `Some(Ok(v))` → `Ok(Some(v))`, `Some(Err(e))` → `Err(e)`, `None` → `Ok(None)`.

### Removed / deferred

- The standalone `OptionFilter` was removed (it duplicated `OptionIEnumerable.Filter`); use method form `opt.Filter(fn)`.
- `UnwrapOrDefault` and `MapOrElse` are deferred: inherent-impl `using Default[A]` dispatch does not resolve yet (`unknown identifier Default`; see `KNOWN_ISSUES.md`).

## Collection types

- `List[A]` is a singly-linked list with `head: A` and `tail: Option[Ref[List[A]]]`; `None` terminates the list.
- `Slice[A]` is MyGO's canonical slice type spelling and lowers directly to Go's native slice `[]A`.
- `Map[K, V]` is Go's native map `map[K]V`.
- `Set[A]` is Go's native set `map[A]struct{}`.

## IAssignable interface — indexed access for Slice and Map

- `IAssignable[C[A], K, A]` is a generic interface that provides indexed access (read + write) for both `Slice` and `Map`.
- Three type parameters: `C[A]` (the container type, one of `Slice[V]` or `Map[K, V]`), `K` (the index/key type), `A` (the value type).
- Two methods:
  - `func get(c: C[A], index: K) -> Option[A]` — safely read a value by index/key; returns `None` if the index is out of range (Slice) or the key does not exist (Map), `Some(value)` otherwise.
  - `func set(c: C[A], index: K, value: A) -> ()` — write a value at the given index/key.
- Concrete instantiations:
  - `Slice[T]: IAssignable[Slice[T], Int, T]` — `K = Int`
  - `Map[K, V]: IAssignable[Map[K, V], K, V]` — `K` is the map's key type
