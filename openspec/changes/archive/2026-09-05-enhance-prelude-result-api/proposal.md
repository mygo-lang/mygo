## Why

The prelude's `Result` type has only `ToOption`, `Flatten`, and `Eq` support, while the
self-hosted compiler consumes `Result` almost entirely through verbose `switch` pattern
matching. Compared with Rust's standard library `Result` API, the missing combinator set
(`map`, `and_then`, `or_else`, `unwrap_or`, `is_ok`, ...) forces callers into boilerplate
and leaves the boundary type less ergonomic than its `Option` counterpart.

## What Changes

- Create `prelude/result.mygo` hosting all `Result` impls (existing
  `ToOption`/`Flatten`/`ResultEq` moved from `prelude.mygo`, plus a focused combinator
  set): `IsOk`, `IsErr`, `Map`, `MapErr`, `AndThen`, `OrElse`, `And`, `Or`, `Unwrap`,
  `UnwrapOr`, `UnwrapOrElse`, `Expect`, `UnwrapErr`, `ExpectErr`, `ToErr`,
  `Transpose`, and `MapOr` (Tier 2 adds `UnwrapOrDefault` and `MapOrElse` where
  typeclass constraints are verified).
- Add the symmetric set to `impl[A] Option[A]` in `prelude/option.mygo`: `IsSome`,
  `IsNone`, `OkOr`, `OkOrElse`, `AndThen`, `OrElse`, `Flatten`, `Unwrap`, `UnwrapOrElse`,
  `Expect`, `MapOr` (Tier 2 adds `UnwrapOrDefault` and `MapOrElse`).
- Promote the standalone `OptionToResult` helper to method form (`Option.OkOr`); the
  standalone function is kept for compatibility.
- **Tier 2 deferral**: the `Default`-constrained methods (`UnwrapOrDefault`,
  `MapOrElse` fallback branch) were dropped during implementation — the gated
  smoke test (task 4.1) confirmed inherent-impl `using Default[A]` dispatch does
  not resolve (`unknown identifier Default`).  The spec's "Default-constrained
  extraction" requirement is marked DEFERRED and the blocker is recorded in
  `KNOWN_ISSUES.md` for a follow-up change.
- Remove the duplicate standalone `OptionFilter` function, which is identical to
  `OptionIEnumerable.Filter`.
- Leave the `Into` interface and the `?` early-return operator out of scope (follow-up
  design work).

## Capabilities

### New Capabilities
- `prelude`: Result and Option combinator API surface in the built-in prelude package
  (methods, naming conventions, and panic behavior of the added functions).

### Modified Capabilities

## Impact

- New file `prelude/result.mygo` (Result impls); `prelude/option.mygo` gains the
  symmetric Option methods; `prelude/prelude.mygo` loses the moved `Result` impls and
  the removed `OptionFilter` (pure MyGO `switch` implementations; Tier 2 methods may
  use `using Default[A]` constraints).
- Generated Go: `prelude/zz_result.gen.go` (new), `prelude/zz_option.gen.go`,
  `prelude/zz_prelude.gen.go` regenerated via the bootstrap pipeline
  (`mygo --bootstrap sync`).
- Known issue surface: multi-parameter inherent impls are already proven by `ToOption`/
  `Flatten`; typeclass-constrained methods (`UnwrapOrDefault`) need verification against
  `matchTypeclassHelper` dispatch before acceptance.
- Docs: `docs/compiler/ffi.md` Option/Result section updated with the method set; method
  naming follows the existing CamelCase convention (`IsOk`, not `is_ok`).
