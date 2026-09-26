## 1. Lossless lexical and tree foundation

- [x] 1.1 Define bootstrap-compilable token, trivia, diagnostic, green-node, and Syntax Node/Token data models with byte-offset and line/column ranges; verify model unit tests cover every child/range invariant.
- [x] 1.2 Replace the independent lossless character scan with one lexer that recognizes all current MyGO lexical modes, including comments, ordinary/raw/triple-quoted literals and inline-Go payloads; verify token-plus-trivia concatenation equals each fixture input byte-for-byte.
- [x] 1.3 Add CST construction for files, declarations, expressions, types, patterns, delimited constructs, and keyword-delimited blocks; verify CST dumps cover representative `parser2`, `prelude`, and nested-control fixtures.
- [x] 1.4 Implement red Syntax Node/Token traversal and typed syntax accessors over immutable green nodes; verify parent, sibling, ordered-child, and source-range navigation on nested fixtures.

## 2. Recovery and strict parsing boundary

- [x] 2.1 Implement grammar-context recovery and error nodes at top-level declaration, branch/end, and delimiter synchronization points; verify multiple-error fixtures retain following valid declarations in source order.
- [x] 2.2 Preserve malformed token and literal bytes in recovered CST regions and emit location-bearing diagnostics; verify malformed ordinary, raw, and triple-quoted literal fixtures preserve their original text.
- [x] 2.3 Add the public syntax-parse API that returns CST plus diagnostics while keeping the existing AST-only API signature stable; verify syntax parsing recovers and AST parsing reports the first useful error for the same malformed inputs.

## 3. CST-to-ast2 lowering and compatibility

- [x] 3.1 Define the lowering contract: enumerate the supported CST node kinds, add typed Syntax accessors for every semantic child, and make diagnostic-free unsupported nodes return an explicit lowering error without source reparsing or partial `ast2` output.
- [x] 3.2 Complete declaration-header CST and lowering for package, import, let, var, function, type alias, struct, enum, interface, and impl declarations; preserve names, modifiers, parameter/result slots, annotations, and source spans.
- [x] 3.3 Complete recursive type CST and lowering for named, tuple, generic, nested generic, function, collection, and constraint types; add regressions for tuple and nested generic type arguments.
- [x] 3.4 Complete recursive pattern CST and lowering for bind, wildcard, literal, tuple, and variant patterns; add regressions for tuple and variant patterns in declarations and branches.
- [x] 3.5 Complete expression-atom CST and lowering for identifiers, numeric, boolean, string, rune, raw, triple-quoted, inline-Go, tuple/unit, and parenthesized expressions; preserve decoded semantic values and raw-source spans.
- [x] 3.6 Complete operator and postfix expression CST and lowering for unary/binary operators, calls, member access, indexing/slicing, generic calls, and pipe forms; add precedence and nested-postfix regressions.
- [x] 3.7 Complete delimited expression CST and lowering for list/slice, map/set, tuple, and `TypeName { ... }` struct literals; preserve item, key/value, field, separator, and delimiter ownership.
- [x] 3.8 Complete block-expression CST and lowering for function bodies, `if`/`elsif`/`else`, `switch`/`case`, branch headers, bodies, terminators, and nested control flow; add nested-branch source-span regressions.
- [x] 3.9 Lower declaration bodies, implementation members, generic parameters, and constraints through the same Syntax Tree path; verify each supported declaration, expression, type, and pattern fixture lowers successfully.
- [x] 3.10 Differential-test the completed valid-source corpus against the legacy parser for AST shape, literal values, declaration membership, and spans; record and resolve each intentional compatibility difference before cutover.
- [x] 3.11 Route `ParseFile` and `ParseFileAt` through diagnostic-free CST lowering after the differential gate passes; verify parser2 package tests and compiler-facing parse callers pass on the new route.
- [x] 3.12 Isolate and then remove legacy semantic-parser fallback from production parse APIs; retain it only as a test oracle until the migration is complete.
- [x] 3.13 Remove or isolate legacy post-parse scanning, reconstructed node-span, delimited-span, and layout-event production after equivalent CST queries exist; verify no formatter-facing production path depends on them.

## 4. Syntax-tree formatter migration

- [x] 4.1 Define a Syntax Tree formatter cursor that consumes ordered green elements, uses an explicit `greenElementIsTrivia(item)` predicate, and recursively skips or normalizes only formatable trivia gaps.
- [x] 4.2 Re-emit comments, ordinary/raw/triple-quoted literals, rune literals, and inline-Go ranges as opaque source; verify byte-preservation goldens for every opaque lexical form.
- [x] 4.3 Migrate delimiter layout to CST ownership for parenthesized, bracketed, and braced groups, calls, generic arguments, collection literals, and struct literals; verify separator and nested-delimiter goldens remain canonical.
- [x] 4.4 Implement block formatting by recursively traversing Syntax Tree nodes and tokens; cover function bodies, `if`/`elsif`/`else`, `switch`/`case`, nested branch bodies, and keyword terminators, and verify canonical output with focused nested-layout regressions.
- [x] 4.5 Preserve strict formatter failure on syntax diagnostics; verify recoverable malformed input returns a location-bearing error and no formatted output.
  - `ParseFileLossless` already returns `Err(syntaxDiagnosticError(...))` whenever `tree.Diagnostics.Len() != 0`, so this task is mostly about confirming the formatter entry point propagates that error; verify the `mygo fmt` CLI exits non-successfully, reports the diagnostic's source location, and writes no formatted output for recoverable malformed input.
- [x] 4.6 Complete generic Syntax Tree rendering for declarations, expressions, and function literals; preserve canonical declaration spacing and correctly format function literals and nested control-flow expressions wherever they occur.
- [x] 4.7 Implement recursive width measurement and delimiter rendering from CST ownership for calls, generic arguments, parenthesized/bracketed/braced groups, collections, and struct literals; wrap long items and nested fields, align closing delimiters with their owner, and preserve opaque spans.
- [x] 4.8 Match the existing formatter golden corpus across declaration spacing, long signatures, inline and block conditionals, nested branches, function literals, and nested delimiter layouts; add MyGO-authored focused regressions for every repaired behavior.
- [x] 4.9 Verify deterministic output and byte-identical second-pass formatting on representative real files and protected-source fixtures.
- [x] 4.10 Remove the legacy multi-pass formatter and all production `NodeSpans`/`Delimited`/`LayoutEvents` and reconstructed line/event mapping dependencies after tasks 4.6–4.9 pass; verify formatter production rendering traverses only the Syntax Tree.

## 5. Bootstrap and end-to-end validation

- [x] 5.1 Add MyGO-authored parser2 regressions for semantic CST coverage, strict lowering failures, literals, type/pattern recursion, expression precedence, declaration bodies, and nested branches; keep fixtures self-contained to parser2 rather than importing unrelated modules.
- [x] 5.2 Add MyGO-authored formatter regressions for opaque ranges, trivia normalization, delimited constructs, branches, strict syntax failure, byte identity, and idempotence; extend the existing formatter golden corpus for the Syntax Tree renderer tasks 4.6–4.9.
- [x] 5.3 Synchronize parser2 and formatter generated Go through `./mygo --bootstrap sync internal/mygo/parser2` without hand-editing generated files; verify a second sync produces no generated-file drift and regenerated packages compile.
- [x] 5.4 Run focused parser2 and formatter package tests after each coherent migration frontier, using a dedicated temporary Go cache when needed; preserve the exact failing fixture for regressions.
- [x] 5.5 Run parser2, formatter, command, and full-project validation plus formatter byte-identity/idempotence checks on representative real files; verify all commands pass and invalid-input CLI behavior remains non-successful.
- [x] 5.6 Validate `redesign-parser2-lossless-cst` with `openspec validate redesign-parser2-lossless-cst --strict`; verify the OpenSpec change is internally consistent before implementation handoff.
