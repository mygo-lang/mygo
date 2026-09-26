## Context

See `proposal.md` for motivation. The parser source is compiled from multiple `.mygo` files in one package, so same-package file boundaries can be introduced without creating runtime modules or changing visibility. Existing parser2 tests and formatter consumers depend on the syntax tree shape and the behavior of parsing and lowering.

## Goals / Non-Goals

**Goals:**
- Group source declarations by responsibility so readers can find lexer, tree, recovery, parser, and lowering logic independently.
- Use cohesive `struct impl` blocks for operations naturally belonging to a struct, and `interface impl` only for behavior that has a meaningful shared contract.
- Rename the stored recursive node type from `GreenNode` to `CstNode` and the navigable wrapper from `SyntaxNode` to `SyntaxNodeView`.
- Preserve runtime behavior and keep this a source organization and naming change.

**Non-Goals:**
- Change grammar, recovery rules, diagnostic content, CST shape, source spans, or AST lowering.
- Rename `GreenToken`, `GreenTrivia`, `GreenElement`, or `SyntaxTree` as part of this change.
- Add an abstraction merely to increase the number of interface implementations.

## Decisions

### Split files along stable responsibilities

Move declarations into multiple same-package `.mygo` files. A useful initial partition is tree model/navigation, lexing and literal decoding, recovery and delimited tree building, declaration and block parsing, expression/type/pattern parsing, and lowering grouped by declarations/statements/expressions/types/patterns. Keep mutually dependent parser helpers together where separating them would create artificial boundaries. The source compiler assembles files in the package, so these boundaries do not imply separate Go packages.

Alternative considered: make a small number of broad files. That reduces file count but leaves unrelated responsibilities coupled and does little to help navigation. The partition should nevertheless avoid tiny files for isolated helpers.

### Rename only the two requested concepts

Use `CstNode` for the node stored in `GreenElement.NodeElement`, and `SyntaxNodeView` for the wrapper carrying root, node, and path. Update constructors, signatures, comments, tests, and generated output consistently. Retain the existing token/trivia/element and tree result names to constrain the rename surface.

Alternative considered: retain the Green/Red terminology. The new names explain the concrete syntax role and the navigation wrapper directly, without requiring knowledge of color-based tree terminology.

### Keep operations close to their owning structures

Where the language supports the required methods cleanly, group `SyntaxNodeView` navigation operations into `impl SyntaxNodeView`. Use interface implementations only when multiple syntax types share a real behavior contract; do not force lexer, parser, and lowering functions into synthetic interfaces. Top-level functions may remain for package-level algorithms and cross-cutting helpers.

Alternative considered: convert every helper into a method. Many parser and lowering operations work over several inputs and are not naturally owned by one value; forcing them into methods would obscure their dependencies.

### Preserve semantics by relocation and mechanical rename

Move source declarations without rewriting their algorithms, then apply the two type renames across source and generated files. Existing parser2 and formatter-facing tests are the behavior oracle. Any discovered behavior change should be corrected within the refactor before it is considered complete.

## Risks / Trade-offs

- **[Risk] File relocation changes compiler source discovery or generated output ordering.** → Keep all files in the same package directory and follow the established `.mygo` source compilation workflow; verify generated sources and package builds.
- **[Risk] A partial `GreenNode` or `SyntaxNode` rename leaves stale references or generated artifacts.** → Search the parser2 source and generated Go output for both old names after regeneration.
- **[Risk] Overusing impl blocks makes shared algorithms harder to locate.** → Move only operations with a clear receiver; retain package functions for general algorithms.

## Migration Plan

1. Identify logical declaration boundaries and move them into focused same-package source files.
2. Rename `GreenNode` to `CstNode` and `SyntaxNode` to `SyntaxNodeView`, including references in parser2 tests and formatter consumers.
3. Group cohesive view operations in a struct impl where supported; introduce no artificial interface.
4. Regenerate parser2 Go output and run the existing parser2 and formatter validation suites.
5. Review the diff to confirm generated and source changes reflect only relocation, naming, and organization.

Rollback consists of reverting the source moves and corresponding generated output together.
