## Why

Two enums in one MyGO package may legitimately use the same variant name. A
qualified named-variant literal can currently resolve its constructor through a
bare variant symbol, allowing a later declaration to overwrite the intended
constructor. This breaks valid source such as `AgentState.Running { ... }` when
another enum also declares `Running`.

## What Changes

- Make qualified enum-variant construction resolve by both enclosing enum and
  variant name, without depending on a colliding bare variant binding.
- Preserve bare variant resolution only where the surrounding target enum
  already disambiguates it, such as patterns.
- Add equivalent production-compiler and bootstrap-compiler regression
  coverage for a named payload variant colliding with a zero-payload variant.
- Deliver the production and bootstrap fixes as two independently reviewable
  commits; the production commit must be cherry-pickable onto its maintenance
  branch without requiring the bootstrap commit.

## Capabilities

### New Capabilities

- `language/qualified-enum-variant-resolution`: Defines collision-safe
  resolution of qualified enum-variant construction.

### Modified Capabilities

- `language/typed-enum-variants`: Qualified named-struct variant construction
  must remain correct when another enum declares the same variant name.
- `language-semantics-parity`: Production and bootstrap compilers must agree
  on same-package enum-variant-name collision behavior.

## Impact

- Production inference and its enum-variant regression tests.
- Bootstrap `typeinference2` symbol registration and named-variant inference,
  plus bootstrap compilation tests.
- No source syntax, public API, or runtime representation changes.
