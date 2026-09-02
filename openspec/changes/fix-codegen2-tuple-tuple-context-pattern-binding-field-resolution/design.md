## Context

`codegen2` lowers a pattern-matching `switch (state, event)` whose case bodies
have a tuple return type into an anonymous `struct{F0 T0; F1 T1}{...}` value
that holds the tuple elements. Case bodies that destructure an **enum variant**
(e.g. `RunStarted { RunID, ..., InitialMessage }`) are expected to bind each
field to the corresponding Go field produced by that variant's type assertion.

Investigation (`/tmp/qmin`, self-contained, no project code involved) shows the
emitted Go for a variant field referenced inside a function call argument in a
tuple-returning case is produced as:

```go
struct{ F0 AgentState; F1 AgentEvent }{F0: state, F1: event}.F1.InitialMessage
```

The `event` element is correctly lifted to `.F1`, but the `.InitialMessage`
field access is emitted **directly on the enum value** instead of on the
result of the `.(RunStarted)` type assertion for the matched variant. This
happens only when (a) the switch subject is a multi-element tuple, (b) the
case body's expected return type is a tuple, and (c) the pattern-bound field
is referenced in a function-call argument position. A single-element switch or
a non-tuple return type does not reproduce.

Relevant code paths: `bindPattern` / `bindPlainStructPatternFields` /
`bindStructPatternFieldsWithTypes` in `internal/mygo/codegen2/translate_ast.mygo`,
and the anonymous-tuple lowering in `translateTupleLitAst` / `translateSwitchAst`.

## Goals / Non-Goals

**Goals:**

- Fix the codegen so a pattern-bound enum variant field referenced in a
  tuple-returning switch case compiles to the bound local value.
- Keep generated Go for all currently-correct source shapes byte-for-byte
  unchanged where possible.
- Add a regression test that reproduces `(State, Slice[Command])` return +
  enum variant destructuring + function-argument use.

**Non-Goals:**

- New MyGO language features or syntax.
- Changing the type checker or parser; the input AST and mono types are correct.
- Performance optimization of generated switch lowering.

## Decisions

### 1. Emit the variant type assertion when lowering tuple-subject switch cases

The field accessor for an enum variant field must be built as a selector whose
base is `(<lifted-tuple-elem>).(<Variant>)`, not a bare
`<lifted-tuple-elem>.<Field>` selector on the enum value. In `bindPattern`
paths that receive an enum variant (`StructVariantPattern`), the `valueName`
passed to `bindPlainStructPatternFields` / `bindStructPatternFieldsWithTypes`
must be `(<tupleElem>).(<Variant>)` so the emitted Go dereferences through the
variant.

- **Why**: matches what the existing `VariantIf`-style lowering does for the
  non-tuple case (see `translateSwitchBranchesStmt`), which already inserts
  the type assertion. The tuple path just misses the variant step.
- **Alternative considered**: hoisting each tuple element into a local `goast`
  typed variable before binding. Rejected: more invasive than needed and would
  change generated code shape for all tuple cases, not just the broken path.

### 2. Prefer fixing in the shared binding helper over the switch emitter

Make the tuple-element binding helper (`bindPlainStructPatternFields` /
`bindStructPatternFieldsWithTypes`) aware that its `valueName` refers to an
enum variant and needs a type assertion, rather than special-casing each
call site in `translateSwitchBranchesStmt`.

- **Why**: keeps the fix localized to where the incorrect selector is built.

### 3. Regression test mirrors the reducer shape

The test must combine a two-element enum subject, tuple return type, and a
case body that passes the destructured field into a regular function call.

## Risks / Trade-offs

- [Loose coupling risk if the variant-lookahead is done in binding helpers]
  → Centralize the enum variant type lookup in one helper that returns the
    type-asserted base, and unit-test it against the minimal repro before
    running the full bootstrap corpus.
- [The exact expected code shape may shift slightly for the affected case]
  → The spec only requires that the generated Go compiles and semantics are
    preserved, not a specific spelling of the assertion.
