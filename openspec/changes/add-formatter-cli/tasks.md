## 1. Formatter foundation

- [x] 1.1 Inspect the current parser2/ast2 position and comment representations, define the thin Go bridge boundary, and verify it with a focused package API test
- [x] 1.1a Add the formatter core source in MyGO, including its public source-to-source API and generated-code entry point
- [ ] 1.1b Collect positioned tokens/trivia from parser2 and associate them with ast2 nodes without losing protected source spans
- [x] 1.2 Implement the MyGO protected-token/layout traversal for top-level declarations, blocks, expressions, types, literals, and struct syntax; verify representative golden fixtures produce the expected layout
- [ ] 1.2a Combine ast2 layout decisions with token/trivia preservation for whole-file output
- [x] 1.3 Implement canonical single-line and multiline `if` formatting, including line breaks after `=>` and `else` and before `end`; verify dedicated golden fixtures
- [x] 1.4 Implement canonical single-line and multiline `case` formatting, using `=>` only for single-line cases and `then` blocks for multiline cases; verify dedicated golden fixtures
- [x] 1.4a Enforce the preferred 100-column width when selecting compact versus multiline layouts; verify long-expression golden fixtures
- [ ] 1.4b Add function-literal and long-control-flow golden fixtures that verify protected spans remain exact
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
