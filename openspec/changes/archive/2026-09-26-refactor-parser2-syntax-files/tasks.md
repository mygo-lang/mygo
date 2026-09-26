## 1. Partition syntax sources

- [x] 1.1 Map top-level declarations in `syntax.mygo` into focused same-package source files by tree/navigation, lexing, recovery/tree construction, grammar parsing, and lowering responsibilities; verify every existing declaration is assigned exactly once.
- [x] 1.2 Move declarations into the selected files without algorithm changes; regenerate parser2 sources and verify the parser2 package builds.

## 2. Clarify stored nodes and navigation views

- [x] 2.1 Rename `GreenNode` to `CstNode` throughout parser2 MyGO sources and consumers, preserving all fields and CST structure; verify no stale source references remain.
- [x] 2.2 Rename `SyntaxNode` to `SyntaxNodeView` throughout sources, tests, and consumers; organize cohesive navigation functions in a struct impl where supported and verify parser2 generation succeeds.
- [x] 2.3 Review existing interfaces and introduce interface impl only for genuine shared contracts; verify no artificial abstraction or behavior change was added.
- [x] 2.4 Rename all stored `Green*` types and `green*` helper prefixes to their `Cst*` / `cst*` equivalents across parser2, tests, formatter consumers, and generated output; verify no old identifiers remain.

## 3. Verify behavior preservation

- [x] 3.1 Regenerate parser2 Go files and verify `go test ./internal/mygo/parser2` passes.
- [x] 3.2 Verify formatter integration with `go test ./internal/mygo/formatter`.
- [x] 3.3 Regenerate parser2 and formatter Go files, run both package test suites, and verify the stored-tree naming migration preserves behavior.
