## Why

`switch (flagA, flagB)` cannot currently match `case (true, true)`: `true` and `false` are keyword tokens that the pattern grammar does not recognize, so parser2 silently leaves them out of the tuple pattern and typeinference2 reports a misleading `tuple pattern arity mismatch`. Boolean literal patterns are the natural complement to the existing number, string, and rune literal patterns, and are needed for flag tuples such as the Commune rich-text converter's `(hardWraps, allowRawHTML)` switch. Separately, a documented `then`-form guard whose guard expression is a parenthesized `if` expression (`case x if (if c then a else b end) then ... end`) mispairs the case's `end` because the structural case-header scan ignores delimiter depth; the equivalent `=>` form only works by accident.

## What Changes

- Recognize `true` and `false` as boolean literal patterns in parser2, including inside tuple patterns, variant payloads, nested patterns, and guarded cases. Lower them as `LiteralPattern("bool", "true" | "false")` while preserving every source byte in the CST and formatter.
- Type boolean literal patterns as `Bool` in typeinference2. A mismatch against a non-`Bool` switch target is reported as a pattern/target type error rather than being dropped or treated as an integer literal.
- Generate boolean equality for literal patterns in codegen2's plain switch, guarded switch, tuple, nested-variant, and tail-call paths. In codegen3, lower the IR tag `literal:bool:true|false` to the Go boolean constant and reject unknown literal kinds instead of emitting them raw.
- Fix parser2 case-header structural scanning to track delimiter depth and to scan from the current `case`. A `then`-form guarded case whose guard contains a parenthesized `if` expression SHALL pair its own `end`; the existing arrow form SHALL keep its behavior.
- Keep boolean switches exempt from enum exhaustiveness checking, consistent with existing `Int` and `String` literal switches. An unguarded boolean switch without a wildcard remains accepted.
- Do not extend the legacy `--legacy sync` parser, type inference, or code generator; that pipeline remains retired and unchanged.

## Capabilities

### New Capabilities

- `language/bool-literal-patterns`: boolean literal pattern syntax, lossless parser2 representation and lowering, `Bool` typing, matching semantics in single-value and tuple switches, and code generation in codegen2 and codegen3.

### Modified Capabilities

- `language/switch-case-guards`: a parenthesized `if` expression in a `then`-form guard must remain part of the guard and must not cause the case's structural `end` to be mispaired.

## Impact

- Parser2 pattern parsing, literal-pattern lowering, case-header structural pairing, recovery, and formatter coverage.
- Typeinference2 literal-pattern typing and pattern/target unification diagnostics.
- Codegen2 literal-pattern expression builders in `translate_ast.mygo`, `guarded_switch.mygo`, and `tailcall.mygo`; codegen3 literal tag/source handling in `codegen3/match.mygo` (and any shared IR tag helper that needs an explicit boolean kind).
- Compiler-generated `zz_*.gen.go` files for the changed MyGO sources, regenerated in dependency order; the legacy pipeline and its generated files are untouched.
- Tests and documentation: parser2 CST/lowering, formatter idempotence, typeinference2 acceptance and mismatch diagnostics, codegen2/codegen3 runtime behavior, guarded `then`/`=>` forms, and the switch section of `docs/compiler/semantics.md`.
