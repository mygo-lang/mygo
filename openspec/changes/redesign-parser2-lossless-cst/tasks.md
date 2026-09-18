## 1. Lossless lexical and tree foundation

- [ ] 1.1 Define bootstrap-compilable token, trivia, diagnostic, green-node, and Syntax Node/Token data models with byte-offset and line/column ranges; verify model unit tests cover every child/range invariant.
- [ ] 1.2 Replace the independent lossless character scan with one lexer that recognizes all current MyGO lexical modes, including comments, ordinary/raw/triple-quoted literals and inline-Go payloads; verify token-plus-trivia concatenation equals each fixture input byte-for-byte.
- [ ] 1.3 Add CST construction for files, declarations, expressions, types, patterns, delimited constructs, and keyword-delimited blocks; verify CST dumps cover representative `parser2`, `prelude`, and nested-control fixtures.
- [ ] 1.4 Implement red Syntax Node/Token traversal and typed syntax accessors over immutable green nodes; verify parent, sibling, ordered-child, and source-range navigation on nested fixtures.

## 2. Recovery and strict parsing boundary

- [ ] 2.1 Implement grammar-context recovery and error nodes at top-level declaration, branch/end, and delimiter synchronization points; verify multiple-error fixtures retain following valid declarations in source order.
- [ ] 2.2 Preserve malformed token and literal bytes in recovered CST regions and emit location-bearing diagnostics; verify malformed ordinary, raw, and triple-quoted literal fixtures preserve their original text.
- [ ] 2.3 Add the public syntax-parse API that returns CST plus diagnostics while keeping the existing AST-only API signature stable; verify syntax parsing recovers and AST parsing reports the first useful error for the same malformed inputs.

## 3. CST-to-ast2 lowering and compatibility

- [ ] 3.1 Implement diagnostic-free CST-to-ast2 lowering with semantic source spans derived from Syntax Tree ranges; verify every supported declaration, expression, type, and pattern fixture lowers successfully.
- [ ] 3.2 Differential-test valid parser2 inputs against the legacy parser for AST shape, literal values, and spans, then route `ParseFile` and `ParseFileAt` through CST lowering; verify parser2 package tests pass with the new route.
- [ ] 3.3 Remove or isolate legacy post-parse scanning, reconstructed node-span, delimited-span, and layout-event production after equivalent CST queries exist; verify no formatter-facing production path depends on them.

## 4. Syntax-tree formatter migration

- [ ] 4.1 Introduce Syntax Tree formatter traversal that re-emits opaque comments, literals, and inline-Go ranges unchanged while normalizing only formatable trivia gaps; verify byte preservation goldens for each opaque source form.
- [ ] 4.2 Migrate delimiter and block layout to CST ownership, including function bodies, `if`/`elsif`/`else`, `switch`/`case`, and keyword terminators; verify existing formatter golden tests and nested-layout regressions remain canonical.
- [ ] 4.3 Remove the formatter's reconstructed layout-event and line-mapping authority after CST traversal covers all layout families; verify `mygo fmt` remains deterministic and a second formatting pass is byte-identical.
- [ ] 4.4 Preserve strict formatter failure on syntax diagnostics; verify recoverable malformed input returns a location-bearing error and no formatted output.

## 5. Bootstrap and end-to-end validation

- [ ] 5.1 Add MyGO-authored parser2 and formatter regressions for CST source coverage, recovery, strict lowering, literals, and nested branches; verify focused parser2 and formatter test packages pass.
- [ ] 5.2 Synchronize parser2 and formatter generated Go through the bootstrap workflow without hand-editing generated files; verify a second sync produces no generated-file drift and regenerated packages compile.
- [ ] 5.3 Run parser2, formatter, command, and full-project validation plus formatter byte-identity/idempotence checks on representative real files; verify all commands pass and invalid-input CLI behavior remains non-successful.
- [ ] 5.4 Validate `redesign-parser2-lossless-cst` with `openspec validate redesign-parser2-lossless-cst --strict`; verify the OpenSpec change is internally consistent before implementation handoff.
