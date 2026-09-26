## 1. Partition syntax sources

- [ ] 1.1 Map top-level declarations in `syntax.mygo` into focused same-package source files by tree/navigation, lexing, recovery/tree construction, grammar parsing, and lowering responsibilities; verify every existing declaration is assigned exactly once.
- [ ] 1.2 Move declarations into the selected files without algorithm changes; regenerate parser2 sources and verify the parser2 package builds.

## 2. Clarify stored nodes and navigation views

- [ ] 2.1 Rename `GreenNode` to `CstNode` throughout parser2 MyGO sources and consumers, preserving all fields and CST structure; verify no stale source references remain.
- [ ] 2.2 Rename `SyntaxNode` to `SyntaxNodeView` throughout sources, tests, and consumers; organize cohesive navigation functions in a struct impl where supported and verify parser2 generation succeeds.
- [ ] 2.3 Review existing interfaces and introduce interface impl only for genuine shared contracts; verify no artificial abstraction or behavior change was added.

## 3. Verify behavior preservation

- [ ] 3.1 Regenerate parser2 Go files and verify `go test ./internal/mygo/parser2` passes.
- [ ] 3.2 Verify formatter integration with `go test ./internal/mygo/formatter` and inspect the final diff for changes beyond source organization and the two type renames.
