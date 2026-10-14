## Why

The `mygo fmt` function currently breaks the source code: `using` constraint clauses in block declarations are rendered as `usingToString`, and after re-parsing, the `using` keyword disappears, constraints are lost, and pseudo-statements are generated; in line-wrapped delimiters, trailing comments are repeatedly appended with commas, never converging (a single `fmt` adds 6-7 commas due to a maximum of 6 rounds of internal fixpoints). Both issues are reproduced on HEAD, with 14 `.mygo` files in the repository containing this form of `using` signature.

## What Changes

- When delimiters wrap, the delimiter is only appended after the actual item text and before comments: trailing comments remain unchanged, do not absorb delimiters, and are no longer mistaken for items.

- Comments within an item area are on separate lines and maintain their indentation level, preventing subsequent items from losing indentation; rendering of line-wrapped delimiters converges within a single pass.

- Fixed a critical whitespace separator (the space between `using` and its constraints) that was missing in the declaration header rendering, ensuring that `using ToString[Int]` remains unchanged and no longer produces `usingToString[Int]` that will be re-lexically analyzed into a single identifier.

- Added output structure preservation constraint: the result of a single rendering must have an equivalent structure to the input after re-parsing; fixpoints must not solidify destructural output.

- Extended cross-module AST equivalence checks to the repository scope and incorporated the `using` clause and comment separator into fixtures; removed existing non-idempotent exceptions after the fix.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `formatter-cli`: Placement of separators and comments in line break-separated structures, rendering of `using` constraint clauses, and the constraint that "formatted output must preserve parsed structure".

## Impact

Affects the rendering of delimited items, handling of canonical whitespace in the `internal/mygo/formatter` declaration header, and formatter testing and repository-level equivalence checks. It does not modify the lossless CST shape of parser2 (the `Constraints`/`Constraint` node structure is preserved), nor does it change the AST, downgrade, or CLI interface; no new dependencies are added.