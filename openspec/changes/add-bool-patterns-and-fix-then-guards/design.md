## Context

See `proposal.md` for motivation and `specs/` for the behavior contract. Three current constraints shape the implementation.

First, parser2's pattern leaf grammar recognizes `_`, number/string/rune literal tokens, and identifier tokens. `true` and `false` are keyword tokens, so they are neither a recognized literal pattern nor a bind pattern. Inside a tuple group, `parseGroupItems` leaves an unparsed boolean token as a raw child, but pattern lowering only collects child nodes that are pattern kinds. The tuple therefore loses elements before type inference, and `envWithTuplePattern` reports an arity mismatch rather than a boolean-pattern problem.

Second, typeinference2 assigns `LiteralPattern` types by kind: string to `String`, rune to `Rune`, and every other kind to `Int`. Code generation has the mirror assumption. Codegen2 has four separate literal-to-Go mappings (plain switch, guarded switch, tuple/variant paths, and tail-call lowering), while codegen3 builds an IR tag of the form `literal:<kind>:<value>` and re-emits non-string, non-rune values raw.

Third, parser2's structural `case` pairing uses `syntaxCaseHeaderIndex(items)` and `syntaxRawKeywordCount` while delimiter groups are still flat tokens. Neither scan tracks delimiter depth. For `case x if (if c then a else b end) then ... end`, the scan finds the inner `then` first and counts both the guard introducer and the inner `if`. The structural pass concludes the case does not own an `end`, so the case's real `end` pairs with the enclosing switch and the switch's `end` becomes a stray identifier. The arrow form works only because an arrow case never opens a structural block.

The legacy `--legacy sync` parser, type checker, and code generator are retired for this work and are deliberately left unchanged.

## Goals / Non-Goals

**Goals:**

- Make boolean literal patterns a first-class bootstrap pattern form in parser2, typeinference2, codegen2, and codegen3, including tuple, nested variant, guarded, statement, expression, and tail-call positions.
- Correct then-form guard structural pairing for parenthesized `if` expressions without changing arrow-form behavior.
- Keep parser2 lossless and formatter-idempotent for both changes.
- Keep codegen2 and codegen3 behavior aligned for the same source program.

**Non-Goals:**

- Requiring a boolean switch to cover both `true` and `false`. Boolean switches follow the existing non-enum literal-switch rule.
- Adding a new AST pattern variant or a new CST pattern kind for booleans.
- Changing the legacy pipeline or its generated files.
- Turning every unparsed tuple-pattern element into a dedicated diagnostic. Boolean patterns no longer take that path; broader pattern-diagnostic cleanup is separate work.
- Adding runtime equality checks for literal patterns in `let` destructuring. Existing number, string, and rune literal patterns keep their current destructuring behavior.

## Decisions

### Reuse `LiteralPattern("bool", "true" | "false")`

The pattern AST already stores a literal as a kind/value pair. Representing booleans as a new kind keeps every pattern traversal, pretty-printer, and codegen dispatch on one enum shape and avoids adding a `BoolPattern` alternative to `ast2.Pattern` and its generated metadata. The alternative of a dedicated pattern node was rejected because it would multiply pattern walks in parser2, typeinference2, IR lowering, and both backends for no behavioral gain.

### Classify booleans at the parser leaf, not as identifiers

Extend the parser2 literal classification so a keyword token whose raw text is `true` or `false` is a boolean literal pattern. Lowering maps that raw token directly to `LiteralPattern("bool", ...)`; no text re-scan is needed. A new CST node kind is unnecessary because the existing `LiteralPattern` CST kind already carries the raw leaf.

Boolean expressions keep their `BoolExpr` treatment. The distinction is contextual: the same keyword is a boolean expression in expression position and a boolean literal pattern in pattern position. This ordering also ensures a boolean token can never fall through to bind-pattern handling.

### Type boolean patterns explicitly and reject unknown kinds

Typeinference2 maps the `"bool"` kind to `TCon("Bool")`. It should also make the kind mapping explicit for `"number"`, `"string"`, and `"rune"`, and report an unsupported-literal-kind diagnostic for any other kind instead of silently defaulting to `Int`. The silent `Int` fallback is how a parser gap can turn into a confusing downstream type error.

Tuple positions and variant payloads need no new inference machinery: they already route each subpattern through `envWithPattern`, so adding the boolean kind is sufficient. A mismatch against an `Int` or `String` target uses the existing pattern/target unification diagnostic.

### Centralize literal-to-Go conversion in codegen2

Make `literalPatternGoExpr` the single mapping for literal patterns and have the plain switch, guarded switch, tuple/variant, and tail-call paths call it. For `"bool"`, emit the Go identifier `true` or `false`; the generated comparison is `subject == true` or `subject == false`, which preserves first-match-wins and guard fallthrough behavior without a special boolean lowering. `goast.Ident(literal)` is sufficient and avoids adding a new AST helper.

Patching the four existing inline mappings separately was rejected because they can drift; one helper also makes the non-legacy policy explicit. The legacy codegen package is not changed.

### Make codegen3's boolean literal handling explicit

IR lowering already encodes a literal pattern as `literal:<kind>:<value>`, and codegen3 already switches on that tag when a switch contains a literal case. Add an explicit `"bool"` branch that emits the value as a Go boolean constant, keeping the raw fallback only for compatibility with unknown kinds that should be unreachable. Because typeinference2 now rejects unknown literal kinds, the bootstrap pipeline cannot deliver an unknown kind to codegen3. A nested literal payload continues to use the same tag path.

The alternative of adding a new IR literal expression was rejected because the existing tag contract already distinguishes kind and value and supports nested payloads.

### Find case headers relative to the current case with delimiter depth

Add a case-relative header scan that starts after the current `case` and walks raw elements while tracking `()`, `[]`, and `{}` depth. The first depth-zero `then` or `=>` is the case header terminator. Count guard-introducing `if` tokens with the same depth awareness, so a parenthesized guard expression is opaque to the count.

Use this scan in the structural `case` pairing decision. The existing grouped branch path can keep using the same helper because grouped slices no longer contain raw delimiter tokens. The alternative of special-casing `guardIfs >= 2` would only hide this instance and would still mis-handle nested parentheses or a guard expression containing another `then` inside parentheses.

Arrow-form cases continue to open no structural block. A then-form case still opens an `end` block, and the closer pairing now sees only the case's own header terminator.

### Preserve boolean non-exhaustiveness semantics

`Bool` is a built-in non-enum type. Exhaustiveness lookup for a `TCon("Bool")` target finds no enum variants and accepts the switch, matching the existing `Int` and `String` behavior. Nested enum coverage in the other tuple positions is unchanged. No new coverage requirement is introduced for `true` and `false`.

## Risks / Trade-offs

- [Boolean tokens become valid literal patterns in `let` and declaration patterns as well as switch cases] -> Keep behavior consistent with existing number/string literal patterns, add a focused parser test for declaration positions, and document that literal patterns assert in `switch` and are ignored as destructuring slots in `let`, as before.
- [Changing the structural case header scan regresses nested switches or arrow cases] -> Test then, arrow, mixed, nested-switch, multi-line, single-line, and parenthesized-expression guard forms; retain byte-lossless replay and formatter idempotence checks.
- [A parenthesized guard's inner `if` is still counted as a structural block opener] -> The recovery and structural paths use the same depth-aware classification; test a guard containing a nested `if` inside two levels of parentheses and a guard on a tuple pattern.
- [Codegen2 and codegen3 drift on boolean comparisons] -> Share one literal mapping in codegen2 and add the same source fixture to both backends, checking generated Go and runtime results.
- [Boolean literal patterns are mistaken for enum variants named `true`/`false` in exhaustiveness handling] -> `true` and `false` are keywords and cannot be variant names; add a test that a partial boolean switch produces no missing-variant diagnostic.
- [Self-hosted regeneration pollutes the checkout or changes unrelated generated files] -> Regenerate in dependency order with a task-local writable cache, compare the generated file list, run compile-only tests after each stage, and avoid generating into the repository's benchmark/module root.

## Migration Plan

1. Add parser2 boolean literal classification, lowering, case-relative depth-aware header scanning, and focused CST/lowering/guard tests.
2. Add typeinference2 boolean literal typing plus the explicit kind mapping and unknown-kind diagnostic; add acceptance, mismatch, tuple, nested, and boolean-exhaustiveness tests.
3. Centralize codegen2 literal conversion and add codegen3's explicit boolean branch; verify both backends generate compilable Go for single, tuple, guarded, and nested boolean patterns.
4. Add an end-to-end fixture shaped like the Commune `BuildRichConverter` flag tuple, run it through the bootstrap and codegen3 pipelines, and compare results across all four flag combinations.
5. Update `docs/compiler/semantics.md` for boolean literal patterns and then-form parenthesized guard expressions.
6. Regenerate the changed `.mygo`-backed `zz_*.gen.go` files in dependency order, rebuild consumers, run focused parser2/typeinference2/codegen2/codegen3/IR3 tests, then run the broader suite with a task-local `GOCACHE`.
7. Run `openspec validate add-bool-patterns-and-fix-then-guards --strict` before implementation is considered complete.

Rollback is additive: revert the parser2/typeinference2/codegen changes and their generated files. Existing programs do not use boolean literal patterns, and arrow-form guard behavior is unchanged.
