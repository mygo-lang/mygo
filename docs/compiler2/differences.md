# Bootstrap and language specification differences

[`docs/spec.md`](../spec.md) is the authoritative language contract. This
document records the intentional differences that remain between the legacy
compilation chain and the self-hosted bootstrap chain
`parser2 -> ast2 -> typeinference2 -> codegen2`.

## Current status

No confirmed source-language or compilation-workflow difference remains in the
parity surface tracked by the shared conformance fixtures and bootstrap
workflow tests, apart from the intentional backward-pipe associativity
divergence listed below.

## Intentional limitations

- The two pipelines are deliberately independent implementations. Generated Go
  identifiers, dictionary parameters, trampoline placement, and other lowering
  details may differ; those are implementation details, not language
  differences.
- Bootstrap does not guarantee byte-identical generated Go output.
- The legacy parser groups chained `<|` left-associatively, as
  `(f <| g) <| value`, and treats `|>` and `<|` as one precedence level. Parser2
  groups `<|` to the right, as `f <| (g <| value)`, and binds `<|` looser than
  `|>`. The legacy parser is frozen, so chained `<|` or mixed pipe source is not
  part of the differential-parity surface. `docs/spec.md` defines the language
  behavior.
- If a behavioral difference is observed, the language contract in
  `docs/spec.md` takes precedence; pipeline behavior must not be interpreted
  as a new language promise.

The bootstrap verification commands are documented in
[`core.md`](core.md).
