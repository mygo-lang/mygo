# Design

## Context

See proposal.md for motivation. The formatter renders declarations from parser2's lossless Syntax Tree. Parenthesized groups already wrap on rendered width, while function declaration and implementation method nodes own their signature header and body boundaries.

## Goals / Non-Goals

**Goals:**

- Use the existing preferred width of 100 columns to decide when a declaration header needs multiline layout.
- Emit one parameter per line with stable indentation and preserve the return type after the closing parenthesis.
- Apply the same rule to ordinary function declarations and implementation methods, while keeping short headers compact.
- Make function-call wrapping account for the full rendered call expression, including the callee text that precedes its argument group.
- Automatically wrap one-line calls that exceed the preferred width, without requiring users to pre-format the source.

**Non-Goals:**

- Change function literal layout, parser syntax, or CLI behavior.
- Reflow comments or protected source content within the header.

## Decisions

- Make the header layout decision at the function declaration or implementation method boundary, using the complete flattened header width (function name through return type). A delimiter-only decision cannot account for a long function name or return type that makes the whole declaration exceed the width.
- When the flattened header exceeds 100 columns, render the parameter list as one parameter per line. Keep the opening parenthesis attached to the function name, align the closing parenthesis with the declaration's indentation, and render any return type after it. This yields the requested shape and avoids leaving a long return type on a separate, ambiguous continuation line.
- For calls, include the rendered callee and its argument group in the width calculation. If the full call exceeds 100 columns, render the argument group in multiline form while retaining the callee before the opening parenthesis; do not rely only on the argument group's standalone width.
- Leave function literal layout unchanged. Calls whose complete expression fits within 100 columns retain the current compact form.
- Continue using CST structure and rendered text rather than function-name matching, and preserve comments/source spans according to existing formatter rules.

## Risks / Trade-offs

- [A comment inside a signature may make flat-width measurement or per-parameter splitting ambiguous] → Preserve comment trivia with its associated parameter and add fixtures for comments if the current CST representation can represent them there.
- [An already multiline header could be reflowed differently on later passes] → Treat the canonical multiline form as stable and verify byte-identical repeated formatting.
- [Call wrapping based only on delimiter width can miss calls made long by the callee] → Measure the owning call expression and add a fixture whose callee pushes an otherwise short argument list over the limit.
