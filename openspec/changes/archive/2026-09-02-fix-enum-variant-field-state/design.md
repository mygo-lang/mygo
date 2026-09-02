## Context

See proposal.md for motivation. The bootstrap inference pipeline recursively
checks fields of named enum-variant literals. Its recursive step retains the
current substitution but does not currently retain the state returned by the
field expression. Inference state owns the fresh type-variable supply.

## Goals / Non-Goals

**Goals:**

- Keep fresh type-variable allocation monotonic across all fields of a named
  enum-variant literal.
- Preserve the existing field validation and substitution behavior.
- Cover the collision pattern with a focused bootstrap inference regression
  test.

**Non-Goals:**

- Change MyGO collection syntax or contextual typing rules.
- Redesign type-variable allocation across unrelated inference paths.
- Alter code generation or the default compiler pipeline.

## Decisions

### Propagate the field result state through the recursive variant-field walk

The recursive call will use the `InferState` returned while inferring the
current field expression. This mirrors ordinary struct-literal field inference
and preserves the latest fresh-variable counter for subsequent fields and the
enclosing expression.

Alternative considered: reserve a fixed number of type variables before field
inference. This would be brittle because a field expression can allocate a
variable-dependent number of fresh variables.

### Test through the public bootstrap inference entry point

The regression test will parse and infer a small source program containing a
named enum-variant literal followed by an empty slice constrained by a declared
result type. This exercises the actual state propagation boundary rather than
only a helper function.

Alternative considered: unit-test only the recursive helper with manually
constructed inference state. That would not verify parser, AST, and expression
inference integration.

## Risks / Trade-offs

- [State propagation changes inferred-variable ordering] -> Limit the change to
  the named enum-variant field recursion and assert successful inference rather
  than internal variable IDs.
- [A nearby path may have the same stale-state pattern] -> Compare the changed
  path with ordinary struct-literal recursion and retain a narrowly scoped
  regression test.

## Migration Plan

No migration is required. The change is an internal bootstrap compiler fix;
rollback consists of reverting the inference and test changes together.
