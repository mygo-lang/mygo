## 1. Parser2 boolean literal patterns

- [x] 1.1 Extend `cstElementPatternLiteralKind` and `cstTokenPatternLiteralKind` to classify the keyword tokens `true` and `false` as the `"bool"` literal kind, and lower them to `LiteralPattern("bool", "true" | "false")`; verify `loweredPatternOf("true")` and `loweredPatternOf("false")` return the matching boolean literal pattern.
- [x] 1.2 Verify a tuple pattern retains boolean elements: `loweredPatternOf("(true, _)")` must contain two elements, and `loweredPatternOf("(true, false)")` must contain two boolean literal patterns rather than an empty or one-element tuple.
- [x] 1.3 Add parser2 CST and lossless tests for `switch flag` with `case true`/`case false`, `case (true, false)`, and `case (Some(true), false)`; verify the CST exposes literal pattern nodes, replays the original bytes, and formats idempotently.
- [x] 1.4 Check declaration-position behavior for a boolean literal pattern and confirm it matches existing number/string literal patterns; record the accepted or rejected behavior in a focused parser test so the shared pattern grammar does not change silently.

## 2. Parser2 then-form guard structural fix

- [x] 2.1 Add a case-relative, delimiter-depth-aware case-header scan that starts after the current `case` and returns the first depth-zero `then` or `=>`; use it in the `structuralCloseAt` case decision and verify `case x if (if x > 0 then true else false end) then ... end` pairs with its own `end`.
- [x] 2.2 Make the guard-`if` count in the same structural decision depth-aware so parenthesized `if` expressions are opaque; verify a guard nested two parentheses deep and a guard on a tuple pattern still open exactly one case block.
- [x] 2.3 Verify arrow-form guarded cases keep their existing behavior and that a then-form guarded case followed by another case or fallback parses as siblings; add parser2 CST tests for both forms.
- [x] 2.4 Verify the existing malformed form `case P if a && if b then c else d end then ... end` is still rejected with the guard-parenthesization diagnostic, and add a test for that diagnostic.
- [x] 2.5 Verify lossless replay and formatter idempotence for then-form guarded cases with parenthesized `if` expressions, including comments in the case header.

## 3. Typeinference2 boolean typing

- [x] 3.1 Replace the implicit `else Int` mapping in `envWithPatternTarget`'s `LiteralPattern` handling with explicit `"number"`, `"string"`, `"rune"`, and `"bool"` branches and an unsupported-literal-kind diagnostic; verify `switch b` over `Bool` accepts `case true` and `case false`.
- [x] 3.2 Verify a non-Boolean target is rejected: an `Int` switch with `case true` must report a pattern/target mismatch, and the diagnostic must not be a tuple arity error.
- [x] 3.3 Verify tuple and nested positions: `(Bool, Bool)` with `case (true, false)` infers both literals as `Bool`, `(Bool, Int)` with a boolean in the second slot reports a type mismatch, and `case (Some(true), false)` infers without an arity or payload error.
- [x] 3.4 Verify boolean switches remain exempt from enum exhaustiveness: a `Bool` switch with only `case true` compiles, a `(Bool, Int)` switch without a wildcard compiles, and a `(Color, Bool)` switch still uses the existing enum coverage result.
- [x] 3.5 Add a typeinference2 test that an unknown literal kind is rejected explicitly rather than defaulting to `Int`, so a future pattern-lowering gap cannot repeat the original misleading failure.

## 4. Codegen2 boolean matching

- [x] 4.1 Make `literalPatternGoExpr` the single literal-to-Go mapping and add a `"bool"` branch that emits `goast.Ident("true" | "false")`; replace the duplicate inline mappings in `translate_ast.mygo`, `switchLiteralGoExpr` in `guarded_switch.mygo`, and the tail-call mapping in `tailcall.mygo` with calls to it.
- [x] 4.2 Add a codegen2 test for a four-arm `(Bool, Bool)` switch and verify the generated Go compiles and returns the four distinct bodies for all input combinations.
- [x] 4.3 Verify guarded boolean cases (`case (true, _) if flag then ... end`) and nested payloads (`case Some(true) then ... end`) generate boolean comparisons and do not match the opposite value.
- [x] 4.4 Verify statement-form and tail-position boolean switches use the same mapping and preserve their existing unmatched behavior.

## 5. Codegen3 and IR3 boolean matching

- [x] 5.1 Add an explicit `"bool"` branch in codegen3's `literalSource` so `literal:bool:true` and `literal:bool:false` emit Go boolean constants; verify an IR dump from a boolean switch contains the boolean literal tags.
- [x] 5.2 Add a codegen3 test for the same four-arm `(Bool, Bool)` switch and verify generated Go compiles and returns results matching codegen2.
- [x] 5.3 Verify guarded and nested boolean patterns (`case (true, _) if flag`, `case Some(true)`) lower through the existing literal tag path in codegen3 and select only matching values.
- [x] 5.4 Verify an unknown literal kind cannot reach codegen3 through the bootstrap pipeline because typeinference2 rejects it, and that codegen3 does not treat a boolean literal as a numeric literal.

## 6. Documentation and end-to-end coverage

- [x] 6.1 Update `docs/compiler/semantics.md` with boolean literal pattern syntax, tuple/nested examples, non-exhaustiveness for `Bool`, and then-form parenthesized `if` guards; verify every added example compiles.
- [x] 6.2 Add an end-to-end fixture matching the Commune `BuildRichConverter` flag tuple shape (four `(Bool, Bool)` arms returning distinct values) and run it through the bootstrap and codegen3 pipelines for all four flag combinations.
- [x] 6.3 Reproduce the original failing shape with a minimal `switch (hardWraps, allowRawHTML)` using `case (true, true)` and verify the CLI no longer reports `tuple pattern arity mismatch`.
- [x] 6.4 Verify the Commune `rich.mygo` pattern shape compiles against the updated compiler using a stand-in for its Go FFI imports if the full backend module is not built in the sandbox.

## 7. Self-host regeneration and validation

- [x] 7.1 Regenerate the changed `zz_*.gen.go` files in dependency order (parser2, typeinference2, then codegen2/codegen3/IR3 consumers) and verify the generated file list and compile-only tests after each stage.
- [x] 7.2 Run the focused parser2, typeinference2, codegen2, codegen3, IR3, and compiler test suites with a task-local writable `GOCACHE`; verify all focused suites pass.
- [x] 7.3 Run the broader relevant test suite and verify no unrelated generated files changed and no existing unguarded or arrow-form guard behavior regressed.
- [x] 7.4 Run `openspec validate add-bool-patterns-and-fix-then-guards --strict` and verify the change artifacts are valid.
