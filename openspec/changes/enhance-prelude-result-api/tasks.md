## 1. File Reorganization

- [ ] 1.1 Create `prelude/result.mygo` and move `impl[A, E] Result[A, E]`
  (`ToOption`, `Flatten`) and `impl[A, E] ResultEq[A, E]` out of
  `prelude/prelude.mygo` unchanged; verify a `go build ./...` passes after
  regeneration
- [ ] 1.2 Regenerate prelude via the bootstrap pipeline (`mygo --bootstrap sync`)
  so `zz_result.gen.go` appears and `zz_prelude.gen.go` drops the moved impls; verify
  `go build ./...` and `go test ./internal/mygo/...` still pass (pure relocation, no
  behavior change)

## 2. Result Combinators (constraint-free)

- [ ] 2.1 Add `IsOk` and `IsErr` to `impl[A, E] Result[A, E]` in `result.mygo`;
  verify a test asserts `Ok(v).IsOk()` is true, `Err(e).IsErr()` is true, and payloads
  remain accessible
- [ ] 2.2 Add `Map` and `MapErr`; verify tests assert `Ok(v).Map(fn)` is `Ok(fn(v))`,
  `Err(e).Map(fn)` passes `Err(e)` through with `E` unchanged, and
  `Err(e).MapErr(fn)` is `Err(fn(e))`
- [ ] 2.3 Add `AndThen`, `OrElse`, `And`, and `Or`; verify tests assert chain functions
  run only on the matching variant and the other variant short-circuits unchanged
- [ ] 2.4 Add `Unwrap`, `UnwrapOr`, `UnwrapOrElse`, `Expect`, `UnwrapErr`, and
  `ExpectErr`; verify `Unwrap()`/`UnwrapErr()` return payloads, the unwrap variants
  panic on the unexpected variant, and `Expect("msg")` panics with the message
  included
- [ ] 2.5 Add `MapOr`; verify `Ok(v).MapOr(def, fn)` is `fn(v)` and
  `Err(e).MapOr(def, fn)` is `def`
- [ ] 2.6 Add `ToErr`; verify `Err(e).ToErr()` is `Some(e)` and `Ok(v).ToErr()` is
  `None`
- [ ] 2.7 Add `Transpose` (`Option[Result[A, E]]` to `Result[Option[A], E]`), as an
  `Option`-side method if method sugar resolves there, else a standalone function;
  verify `Some(Ok(v))` → `Ok(Some(v))`, `Some(Err(e))` → `Err(e)`, `None` → `Ok(None)`

## 3. Option Symmetric Methods

- [ ] 3.1 Add `IsSome` and `IsNone` to `impl[A] Option[A]` in `option.mygo`; verify
  `Some(v).IsSome()` is true and `None.IsNone()` is true
- [ ] 3.2 Add `OkOr` and `OkOrElse` (superseding the standalone `OptionToResult`,
  which is kept for compatibility); verify `Some(v).OkOr(e)` is `Ok(v)` and
  `None.OkOr(e)` is `Err(e)`
- [ ] 3.3 Add `AndThen`, `OrElse`, and `Flatten`; verify chain functions run only on
  `Some` and `Some(Some(v)).Flatten()` is `Some(v)`
- [ ] 3.4 Add `Unwrap`, `UnwrapOrElse`, `Expect`, and `MapOr`; verify payload
  extraction, panic on `None`, and default/fallback paths

## 4. Default-Constrained Methods (gated)

- [ ] 4.1 Smoke-test one constrained method first: add `UnwrapOrDefault` to
  `Result` and `Option` using `Default[A]`; verify the generated call resolves the
  `Default` constraint and a run-time test returns `Default[A]()` on the unexpected
  variant — if inherent-impl `using` dispatch does not resolve, drop both methods,
  re-scope the spec's "Default-constrained extraction" requirement, and record the
  blocker in KNOWN_ISSUES.md
- [ ] 4.2 If 4.1 passes, add `MapOrElse` to `Result` and `Option`; verify the same
  test style covers the fallback branch computed from the error/unit

## 5. Cleanup and Docs

- [ ] 5.1 Remove the standalone `OptionFilter` from `prelude/prelude.mygo` after
  grepping `lib/`, `internal/`, `examples/` for callers; verify none remain and
  regeneration keeps `go build ./...` green
- [ ] 5.2 Update `docs/compiler/ffi.md` Option/Result section with the final method
  list and the `result.mygo` file layout; verify the doc lists every method added
- [ ] 5.3 Final integration pass: regenerate all prelude `zz_*.gen.go`, run
  `go build ./...` and `go test ./internal/mygo/...`; verify no test regressions and
  the bootstrap compiler still rebuilds the prelude cleanly
