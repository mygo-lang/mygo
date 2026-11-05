## 1. Freeze the three failure baselines

- [ ] 1.1 Add a focused inference test for `apply[T, U, E]` with a
  `func(T) -> Result[U, E]` callback and explicit `Int`, `String`, `Error`
  arguments; verify it fails with the current `String`/`Error` mismatch before
  the fix and passes afterward
- [ ] 1.2 Add an alias variant using `Pair[A, B] = Result[A, B]` and
  `Pair[U, E]`; verify it exercises the same argument-order rule
- [ ] 1.3 Add a local `Box` plus imported `lib.Box` fixture where both types
  declare a method with the same name; verify the current generated call is
  missing or ambiguous
- [ ] 1.4 Add `value.Method`, `Type.Method`, higher-order argument, and direct
  `Type.Method(receiver, args...)` fixtures; verify codegen2 and codegen3
  currently emit Go selectors and `go build` fails

## 2. Correct generic type application

- [ ] 2.1 Implement a simultaneous substitution path for applying a generic
  type's bound parameters, keeping unification substitution behavior separate;
  verify new unit tests cover direct and nested applications
- [ ] 2.2 Route `applySchemeTypeArgs` and the enum/alias application path
  through the simultaneous form; verify `Result[U, E]` remains
  `Result[U, E]` when `U` and `E` are caller parameters
- [ ] 2.3 Verify the explicit and inferred callback fixtures generate
  `apply[int, string, error]` with `Result[string, error]` and `go build`
  succeeds
- [ ] 2.4 Run focused `typeinference2` and existing generic-call tests; verify
  no regression in chained explicit type arguments or Result/Option methods

## 3. Canonical receiver identity in codegen2

- [ ] 3.1 Add a canonical receiver identity carrying defining package path and
  member name to the shared instance-method entry/candidate data; verify unit
  tests distinguish local `Box` from `lib.Box`
- [ ] 3.2 Resolve an impl target's MyGO package alias to its canonical path
  when seeding inherent candidates; verify `impl lib.Box` produces a
  path-qualified identity and a local helper name
- [ ] 3.3 Update codegen2 receiver matching to rebuild the same identity from
  the typed receiver and match MyGO paths as well as Go FFI paths; verify
  `box.Double()` lowers to the emitted helper
- [ ] 3.4 Add the local/imported same-name test and verify each receiver calls
  its own helper without changing helper ownership
- [ ] 3.5 Re-run the existing imported-package impl fixtures; verify the
  already-working alias-qualified helper calls remain unchanged

## 4. Align codegen3 receiver identity

- [ ] 4.1 Carry receiver path and member through `ir3.InstanceMethod` and the
  codegen3 table conversion; verify a unit test preserves `lib.Box` rather than
  reducing it to `Box`
- [ ] 4.2 Replace bare-name comparison in instance-method lookup with canonical
  identity matching; verify the consumer-declared imported-type fixture builds
  through codegen3
- [ ] 4.3 Verify codegen2 and codegen3 produce equivalent calls for both local
  and imported impl ownership
- [ ] 4.4 Run codegen3 with an isolated `MYGO_CACHE`; verify no generated Go or
  metadata appears in the checkout

## 5. Represent impl method values in the typed AST

- [ ] 5.1 Add a typed method-value form that records bound receiver or nominal
  type, method name, receiver-adjusted type, and explicit type arguments; verify
  every affected exhaustive switch compiles
- [ ] 5.2 Make inference emit the marker only when impl-method lookup wins over
  struct-field lookup; verify a same-named function-typed field remains a field
  read
- [ ] 5.3 Verify inferred types are `func() -> Int` for `rect.Area` and
  `func(Rectangle) -> Int` for `Rectangle.Area`, including unannotated bindings
- [ ] 5.4 Preserve generic type arguments and `using` requirements on the
  method-value form; verify a generic/typeclass fixture reaches codegen with the
  required evidence

## 6. Lower method values in codegen2

- [ ] 6.1 Emit a bound method value as a closure that evaluates the receiver
  once and calls the helper; verify generated Go compiles and returns the direct
  call's value
- [ ] 6.2 Emit an unbound method expression as the helper function value and
  lower `Type.Method(receiver, args...)` directly to the helper call; verify the
  generated Go contains no method-expression selector
- [ ] 6.3 Verify bound and unbound values work as higher-order arguments and
  returned function values
- [ ] 6.4 Verify receiver expressions with side effects run exactly once when
  the method value is created
- [ ] 6.5 Verify cross-package and generic method values reuse the existing
  helper qualification, explicit type arguments, and dictionary arguments

## 7. Lower method values in codegen3

- [ ] 7.1 Lower the typed method-value form to an IR closure for a bound value
  and to a helper variable for an unbound method; verify IR verification passes
- [ ] 7.2 Teach instance rewriting to process method values in expression,
  argument, return, and let-binding positions; verify none remain as Go
  selectors
- [ ] 7.3 Verify direct calls, higher-order arguments, supplied receivers, and
  receiver side-effect count match codegen2
- [ ] 7.4 Verify cross-package, generic, and dictionary-using method values
  produce buildable Go and the same runtime results as direct calls
- [ ] 7.5 Emit codegen3 output to a temporary directory and verify the checkout
  remains free of generated files

## 8. Documentation and end-to-end validation

- [ ] 8.1 Update `docs/compiler/semantics.md` to document bound method values,
  unbound method expressions, field precedence, receiver evaluation once, and
  direct-call equivalence; verify the examples match the implemented behavior
- [ ] 8.2 Add bootstrap tests in MyGO where practical and use inline Go only to
  reach `compileDirBootstrapEntry` or codegen3 test entry points
- [ ] 8.3 Run focused `typeinference2`, `codegen2`, `ir3`, `codegen3`, and
  `compiler` tests; verify all pass
- [ ] 8.4 Run the generic callback, cross-package impl, and method-value
  fixtures through codegen2 and codegen3 with `go build` and runtime assertions
- [ ] 8.5 Run the full bootstrap suite and inspect `git status`; verify no
  unexpected generated artifacts or unrelated changes remain
