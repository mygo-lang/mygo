## Context

See proposal.md for the reported formatter output. The formatter renders from parser2's lossless CST; case branches and their body blocks are represented structurally, and the renderer has a dedicated branch-body path. Existing formatter coverage checks multi-statement arrow cases, but does not exercise a `let` binding followed by a final expression in the reported form.

The formatter CLI reproduces the issue on the current source: within `fieldsForStructInEnvAt`, the binding and recursive call are emitted on the same line. Inspection confirms the CST already has separate `LetStatement` and `ExprStatement` children under the case body's `Block`. The loss occurs later: the wide-arrow case path recursively flattens that `Block`, then renders its statements as a generic expression sequence, which normalizes their separating newline to a space. The existing formatter requirement for multiline case bodies already states the desired behavior, so no specification delta is needed.

## Goals / Non-Goals

**Goals:**

- Keep each statement in a multiline case body on its own line, including a binding followed by a final expression.
- Preserve the case body structure and produce idempotent output.
- Use the reported source shape as a regression fixture.

**Non-Goals:**

- Change case syntax, parser semantics, or formatter width policy.
- Modify the formatter's public API or add a new formatting capability.

## Decisions

- Add regression coverage before changing the renderer. The fixture should assert the expected line break between the `let` and recursive call, successful formatting of the result, and idempotence.
- Keep a wide arrow case's structured `Block` body intact through rendering and render its statements as a body, rather than flattening them into one expression sequence. Keep the existing generic flattened path for inline single-expression cases.
- Keep formatter implementation changes in MyGO source and regenerate checked-in Go output through the repository's established generation workflow.

## Risks / Trade-offs

- A fix to generic statement-boundary handling could alter other case layouts. → Run the formatter regression and existing formatter coverage, and check representative source formatting for unintended changes.
- The checked-in generated Go output can drift from MyGO source. → Regenerate it from the MyGO implementation and include both source and generated output in the change.
