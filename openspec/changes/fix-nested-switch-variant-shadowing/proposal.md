## Why

A `switch` over a nested enum variant, written as `case Ok(Some(x))` and
`case Ok(None)`, fails to compile with
`declared and not used: __mygo_match___mygo_expr_N`.

Parser2 is context-free and cannot know whether a bare identifier pattern is a
binding or a zero-field enum constructor, so it deliberately lowers every bare
identifier to `BindPattern`. Top-level `case None` already works because type
inference resolves that ambiguity from the switch target's enum declaration
(`resolveBareVariantPattern`). The resolver only inspects the outermost
pattern: it never descends into `VariantPattern` arguments, so `Ok(None)`
reaches codegen as `VariantPattern("Ok", [BindPattern("None")])` instead of
`VariantPattern("Ok", [VariantPattern("None", [])])`.

Codegen then sees a payload argument it must bind, allocates an outer assertion
binding for the `Ok(None)` branch, and the branch body never reads it —
producing Go that does not compile.

## What Changes

- Make `typeinference2.resolveBareVariantPattern` recurse into nested pattern
  arguments, resolving each bare identifier against the owning field type of
  its parent variant or tuple element.
- Keep parser2 unchanged: a bare identifier stays a `BindPattern` at parse
  time, and the type-directed resolution stays in type inference.
- No change to generated Go for patterns that already type-check. The existing
  codegen predicates (`patternArgsNeedValueName`, `patternArgsUsedInBody`)
  produce correct output once inference hands them a real
  `VariantPattern("None", [])`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen-correctness`: add a requirement that a bare identifier in
  a nested pattern position is resolved against the owning variant field type,
  so nested variant patterns with nullary constructors always produce Go that
  compiles.

## Impact

- `internal/mygo/typeinference2/infer.mygo` — `resolveBareVariantPattern` and
  its new recursion over `VariantPattern` / `TuplePattern` arguments.
- `internal/mygo/typeinference2` must be re-synced before `codegen2`
  (bootstrap ordering).
- Downstream MyGO projects that worked around this by restructuring their own
  `switch` (e.g. `tutorial-video-generation`) can drop the workaround.
