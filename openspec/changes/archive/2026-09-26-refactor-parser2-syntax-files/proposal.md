## Why

`internal/mygo/parser2/syntax.mygo` has grown into a single 8,000-line source file that combines lossless CST storage and navigation, lexing, recovery, grammar parsing, and lowering. Splitting these responsibilities into focused files will make parser maintenance easier while retaining the current behavior and public entry points.

## What Changes

- Split the syntax implementation into multiple same-package MyGO files organized by responsibility.
- Migrate stored tree types and helpers from Green terminology to Cst terminology (`CstNode`, `CstElement`, `CstToken`, `CstTrivia`, and related kinds and helpers); rename `SyntaxNode` to `SyntaxNodeView` to distinguish stored CST nodes from navigation views.
- Use struct impl and interface impl where they clarify cohesive operations without changing parser behavior.
- Preserve parsing, recovery diagnostics, CST shape, lowering results, and formatter-visible behavior.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This is a behavior-preserving refactor and does not change parser requirements.

## Impact

- Affected sources: `internal/mygo/parser2/syntax.mygo` and new same-package `.mygo` files in `internal/mygo/parser2`.
- Generated parser2 and formatter Go files will be refreshed from the MyGO sources.
- No dependency or language behavior changes are intended.
