## Context

See proposal.md for the failure being addressed. Bootstrap inference already
records dictionary requirements for constrained method calls. Code generation
has separate lowering paths for ordinary calls and receiver-style implementation
calls; the latter currently emits the receiver and explicit arguments without
appending the inferred dictionaries.

## Goals / Non-Goals

**Goals:**
- Preserve the `using` contract declared by a typeclass method in generated
  helper calls.
- Select each dictionary using the inferred receiver/constraint types and
  preserve the helper's argument order.
- Cover one and multiple constraints, including an implementation that accepts
  but does not use its dictionary argument.

**Non-Goals:**
- Change MyGO source syntax or relax interface/typeclass conformance.
- Remove `Eq[A]` from `Set.Contains`; it remains part of the shared
  `IEnumerable.Contains` contract.
- Change the implementation semantics of Set membership.

## Decisions

- Reuse dictionary-call requirements recorded by type inference for receiver
  calls, rather than infer constraints again from the selected implementation
  helper name. Inference has the resolved constraint identities and concrete
  type arguments at the call site.
- Append dictionary arguments after the receiver and explicit source arguments,
  in the same order as the generated helper signature. Resolve each requirement
  through the existing package/Prelude instance candidate metadata and lexical
  dictionaries where applicable.
- Keep ordinary-call dictionary lowering unchanged unless shared helpers are
  needed to avoid diverging argument-order behavior.

## Risks / Trade-offs

- [Requirement metadata may not be available for every receiver dispatch path]
  → Add focused coverage for Prelude methods and methods with multiple
  constraints, and preserve a clear compile-time diagnostic if a required
  dictionary cannot be resolved.
- [A dictionary may be passed even when an implementation ignores it] → This
  is intentional: the generated helper signature follows the declared
  `using` contract, so unused arguments are required for type correctness.
