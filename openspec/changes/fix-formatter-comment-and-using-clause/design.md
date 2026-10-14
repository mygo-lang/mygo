## Context

See proposal.md for motivation. The following facts, confirmed by a temporary CST probe, form the basis for design and trade-offs:

- The CST of `func f(x: Int) -> String using ToString[Int]` is `[func, name, FunctionParameters, FunctionReturnType, Constraints, Block, end]`, where `Constraints` precedes `Block`, and the header boundary `memberHeaderEnd` also includes it (`headerEnd` points to the trivia before `Block`). In other words, the "clause running into the body" is not a header boundary miscalculation.

- A single `FormatSyntaxBlocks` outputs the header of `func f(x: Int) -> String usingToString[Int]` for the same input: structurally correct, but missing the space between `using` and the constraint.

- In the second round of `FormatSource`'s fixpoint, the text is re-lexically analyzed into a single identifier `usingToString`. Therefore, the `using` keyword disappears, `Constraints` nodes are no longer generated, clauses fall into `Block`, and subsequent rendering rounds maintain this corrupted shape.

- The mechanism for missing spaces: parser2's `functionUsingChildren` leaves the delimiter trivia inside the `Constraint` child node; when the child node is rendered, this trivia becomes the first trivia, and `syntaxCanonicalWhitespace` returns `""` when there is no preceding token in the same element list, thus discarding spaces.

- Delimiters for delimited structures are always appended after the item text (`renderSyntaxDelimitedItems`). When an item ends with a line comment, the comma following the comment is swallowed by the comment, and another comma is added in the next round; `formatSyntaxFixpoint` has a maximum of 6 rounds, so a single `mygo fmt` adds 6-7 commas. Comments between two items do not accumulate, but the latter item loses indentation.

## Goals / Non-Goals

**Goals:** Fix these two defects internally in the formatter; maintain the parsed structure of the output in a single render; incorporate this property into repository-level validation.

**Non-Goals:** Do not change the lossless CST shape or lexical of parser2 (the trivia affixation of `Constraints`/`Constraint` remains unchanged); do not change the formatting rules of pipes/long expressions; do not change the degradation, AST, and CLI interfaces; do not rewrite the legacy renderer.

## Decisions

### 1. Whitespace Defect Fixed in Formatter (Recommended)

When rendering nodes, pass the "previous valid token of the parent" to the canonical whitespace check, so that the separators of the leading trivia are calculated according to the parent context: `using` followed by constraint nodes results in whitespace.

Alternative Solution 1: Move the separator trivia from the `Constraint` node back to the parent list. Rejected—the lossless CST is already correct (`TestFunctionDeclarationOwnsUsingConstraints` asserts kinds and bytes). Moving the trivia will change the shape of the CST and affect degradation and existing CST tests; the benefits do not outweigh the risks.

Alternative Solution 2: Post-process the rendered result to insert whitespace. Rejected—fragile and introduces a second set of lexical cognitions, easily breaking the structure again.

### 2. Placement Rules for Separators and Comments

Item areas are interpreted as "preceding comment line + valid content + ending comment": comments are rendered on a separate line and indented by item; separators are only appended after valid content. If the area contains only trivia, no separator is appended, and the comment is output as is. This prevents ending comments from swallowing separators (no accumulation), and items following comments no longer lose indentation.

### 3. Structure Preservation as a First-Class Constraint

The output of a single pass must be re-parsed into an equivalent structure (ignoring position and comments). Because the violation in this example occurs during a *single* rendering (`usingToString`), validation only after the fixpoint is insufficient. The implementation extends "AST equivalence before and after formatting" to the repository scope (ignoring reflection comparison of position fields) and adds fixtures separated by `using` clauses and comments; other existing defects are explicitly recorded instead of silently excluded.

### 4. Validation and Regression

Use the existing formatter corpus and golden; the newly added comment separators and `using` test case assertions for byte-by-byte output and secondary formatting remain unchanged. After the fix, remove the non-idempotent exception currently reserved only for `internal/mygo/typeinference2/env.mygo`.

## Risks / Trade-offs

- [Parent context input may change whitespace in other locations] → Run the existing formatter corpus and golden first, then run repository-level equivalence checks; confirm differences one by one instead of accepting them in batches.

- [Comments on separate lines may change existing comment golden] → Intentionally update golden, while simultaneously asserting byte equality in secondary formatting.

- [Repository-level equivalence checks may expose other existing defects] → Explicitly record the list in tasks as subsequent changes rather than silently expanding the scope of this change.

- [Changes to `using` clause output] → The current output is destructive; the fix will only make these files safe to format (14 files in the repository are affected).

## Migration Plan

First, fix the formatter rendering (space and delimiter rules), then synchronously generate Go code using `internal/mygo/formatter` in bootstrap order; run `go test ./internal/mygo/parser2 ./internal/mygo/formatter`, then run a full run of `go test ./...`; finally, perform repository-level idempotency and equivalence scans and supplement documentation. During rollback, restore both handwritten source code and generated artifacts.