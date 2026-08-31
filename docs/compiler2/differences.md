# Bootstrap and language specification differences

[`docs/spec.md`](../spec.md) is the authoritative language contract. This
document records the intentional differences that remain between the default
compilation chain and the self-hosted bootstrap chain
`parser2 -> ast2 -> typeinference2 -> codegen2`.

## Current status

No confirmed source-language or compilation-workflow difference remains in the
parity surface tracked by the shared conformance fixtures and bootstrap
workflow tests.

## Intentional limitations

- The two pipelines are deliberately independent implementations. Generated Go
  identifiers, dictionary parameters, trampoline placement, and other lowering
  details may differ; those are implementation details, not language
  differences.
- Bootstrap does not guarantee byte-identical generated Go output.
- If a behavioral difference is observed, `docs/spec.md` states that the
  default pipeline takes precedence; bootstrap behavior must not be interpreted
  as a new language promise.

The bootstrap verification commands are documented in
[`core.md`](core.md).
