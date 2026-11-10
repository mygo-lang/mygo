# Compiler2 implementation notes

[`docs/spec.md`](../spec.md) is the authoritative language contract. This
document records implementation notes for the self-hosted
`parser2 -> ast2 -> typeinference2 -> codegen2` pipeline.

## Current status

The shared conformance fixtures and bootstrap workflow tests cover the pipeline
surface.

## Implementation notes

- Generated Go identifiers, dictionary parameters, trampoline placement, and
  other lowering details are implementation details, not language differences.
- Generated Go is not guaranteed to be byte-identical across compiler versions
  or backends.
- If a behavioral difference is observed, the language contract in
  `docs/spec.md` takes precedence.

The compiler verification commands are documented in [`core.md`](core.md).
