## Purpose

Defines how MyGO implementation code represents missing indexed elements so absence cannot be silently confused with a valid fallback value.

## Requirements

### Requirement: Missing indexed values are represented explicitly
MyGO implementation code under `internal/mygo` SHALL represent the absence of an indexed element as `Option[T]` rather than substituting a value solely to continue a recursive or indexed traversal. Callers that consume such results SHALL handle `None` explicitly.

#### Scenario: An indexed helper reaches a missing element
- **WHEN** an index or recursive helper requests an element that is not present
- **THEN** the helper SHALL return `None` instead of a fabricated element value
- **AND** its caller SHALL explicitly handle the `None` case

#### Scenario: A default is intentional
- **WHEN** code uses `UnwrapOr(default)` to express an intentional default, such as initializing a missing accumulator or applying a documented fallback
- **THEN** the code MAY retain that default behavior
- **AND** it SHALL NOT be classified as a missing-index fallback solely because it uses `UnwrapOr`

#### Scenario: Existing valid source behavior is preserved
- **WHEN** MyGO source is compiled or formatted after the refactor
- **THEN** valid input behavior SHALL remain unchanged
- **AND** absence in internal indexed data SHALL follow the explicitly handled `None` path
