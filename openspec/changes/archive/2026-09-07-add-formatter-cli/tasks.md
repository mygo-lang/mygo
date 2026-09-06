## 1. Formatter foundation

- [x] 1.1 Inspect the current parser2/ast2 position and comment representations, define the thin Go bridge boundary, and verify it with a focused package API test
- [x] 1.1a Add the formatter core source in MyGO, including its public source-to-source API and generated-code entry point
- [x] 1.1b Extend parsec state and parser2 lexeme/trivia handling to collect positioned tokens and raw trivia without discarding source spans
- [x] 1.2 Implement the MyGO protected-token/layout traversal for top-level declarations, blocks, expressions, types, literals, and struct syntax; verify representative golden fixtures produce the expected layout
- [x] 1.2a Add complete source spans to ast2 nodes and combine AST layout decisions with token/trivia preservation for whole-file output
- [x] 1.2b Add parser-owned nested layout events, structural depth, and positioned-token anchors for `case`/`else` boundaries
- [x] 1.3 Implement canonical single-line and multiline `if` formatting, including line breaks after `=>` and `else` and before `end`; verify dedicated golden fixtures
- [x] 1.4 Implement canonical single-line and multiline `case` formatting, using `=>` only for single-line cases and `then` blocks for multiline cases; verify dedicated golden fixtures
- [x] 1.4a Enforce the preferred 100-column width when selecting compact versus multiline layouts; verify long-expression golden fixtures
- [x] 1.4b Add generic function-literal and long-control-flow golden fixtures that verify protected spans remain exact without function-specific rules
- [x] 1.5 Preserve comments, strings, inline Go, and source constructs through formatting; verify fixtures containing each construct retain exact protected contents
- [x] 1.6 Add parse-error propagation with source locations and verify invalid fixtures return errors without a successful formatted result

## 2. Formatter correctness tests

- [x] 2.1 Add MyGO-authored golden tests for valid MyGO files covering declarations, nested blocks, collections, patterns, and multiline syntax; verify expected output matches checked-in fixtures
- [x] 2.2 Add idempotence tests asserting formatting already formatted output produces byte-identical output
- [x] 2.3 Add semantic preservation checks for representative formatted fixtures by parsing them successfully and running the existing relevant test suite

## 3. CLI integration

- [x] 3.1 Add the thin Go `mygo fmt` subcommand and argument handling without changing existing `sync` and `build` behavior; verify command help and existing CLI tests
- [x] 3.2 Implement in-place formatting for one or more files with path-aware diagnostics and safe writeback; verify valid files change and invalid files remain unchanged
- [x] 3.3 Implement stdin/stdout formatting; verify piped valid input produces formatted output and parse errors return a non-zero status
- [x] 3.4 Implement `--check` mode with non-zero status for differences and zero status for already formatted files; verify no files are modified in either check case

## 4. End-to-end verification

- [x] 4.1 Run formatter unit, golden, and CLI integration tests together and verify all pass
- [x] 4.2 Run the repository's existing Go test suite and verify the new CLI does not regress compiler, bootstrap, or LSP behavior

## 5. Lossless parser and generic formatter revision

- [x] 5.1 Extend parsec with reusable positioned token/trivia primitives and preserve raw token text, offsets, and line/column spans
- [x] 5.2 Update parser2 to return the lossless syntax stream while retaining the existing AST-only compatibility API
- [x] 5.3a Add `Spanned[T]` parser results without breaking existing AST constructors
- [x] 5.3b Flatten complete declaration, statement, and expression source spans
- [x] 5.3c Flatten complete type, pattern, and literal spans with structural paths
- [x] 5.4a Refactor formatter traversal to consume AST shape, `NodeSpan` paths, and layout events
- [x] 5.4b Consume positioned-token anchors for protected-source emission and `case`/`else` boundaries
- [x] 5.4c Remove the remaining source-line block inference from the formatter
- [x] 5.5 Remove all function-name, function-body, call-specific, and source-line syntax special cases
- [x] 5.6 Replace formatter golden and idempotence tests with generic cases covering arbitrary function literals, nested calls, comments, strings, inline Go, long control flow, and event anchors
- [x] 5.7 Run parser2, formatter, CLI, and repository-wide regression tests together

## Evidence (2026-09-07)

The parser now owns indentation semantics (`LayoutEvent.AffectsIndent`,
`LayoutEvent.ExpandsAfter`, branch `end` anchors resolved through the enclosing
`if`) and the formatter projects those events onto a composed line map with
explicit expansion roles (`head`/`body`/`else`/`after`). Same-line block bodies
(`struct Point ... end`, `func f() -> Int 1 end`), inline `if ... then`
expansions, wide arrow `if`/`case` expansions, and multiline `switch/case`
bodies all render with canonical indentation; nested single-line function
literals inside calls remain byte-identical (no function-name/body special
cases).

Verification commands, all passing:

```bash
GOCACHE=/tmp/mygo-gocache go run ./cmd/mygo --bootstrap sync internal/mygo/
GOCACHE=/tmp/mygo-gocache go test ./internal/mygo/parser2 ./internal/mygo/formatter -count=1
GOCACHE=/tmp/mygo-gocache go test ./... -count=1        # 13 packages ok
go run ./cmd/mygo fmt <file> && go run ./cmd/mygo fmt --check <file>
```

Formatted output is re-parseable (`TestFormatterOutputRemainsParser2Valid`,
`TestFormatterWideControlRemainsValidMyGO`) and idempotence tests pass.
