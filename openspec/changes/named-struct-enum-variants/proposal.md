## Why

MyGO enum variants can only carry positional (tuple) payloads today. Named
fields make enum values self-documenting, reduce ordering mistakes, and are a
common expectation for sum types in modern languages. This change adds
named-struct enum variants alongside the existing tuple form.

## What Changes

- Add named-struct enum variant syntax: `Name { field: Type, ... }`.
- Allow an enum to mix tuple and named-struct variants, but reject a single
  variant that mixes both payload styles.
- Add struct-style variant construction: `Enum.Variant { field: expr, ... }`.
- Add struct pattern matching: `case Variant { field } =>` (shorthand for
  `field: field`) and `case Variant { field: name } =>`, with `_` to ignore
  selected fields.
- Allow partial struct patterns (matching a subset of fields).
- Preserve all existing tuple variant syntax and behavior.
- Implement the feature in both the production (`parser`/`codegen`/
  `typeinference`) and bootstrap (`parser2`/`codegen2`/`typeinference2`)
  compilers, as two separate commits.

## Capabilities

### New Capabilities

- `language/typed-enum-variants`: Defines the source-language behavior for
  enum variant payloads, including named-struct variants, construction,
  pattern matching, and mixing rules within an enum.

### Modified Capabilities

<!-- No existing capability spec changes. -->

## Impact

- **Parser**: `internal/mygo/parser/parser.y`, `parser2` equivalents: accept
  named-struct `enum_variant` bodies and `Variant { ... }` patterns.
- **AST**: add representation for named fields on enum variant declarations
  and for struct-style variant patterns.
- **Type inference**: infer constructor and destination types for
  named-struct variant construction and pattern binding.
- **Code generation (production)**: emit variant struct literals from
  named-field construction and bind named fields during switch/pattern lowering.
- **Code generation (bootstrap)**: mirror the production backend behavior.
- **Validation**: enforce the no-mixed-payload rule and report unknown or
  duplicate struct field names.
- **Documentation**: update compiler semantics docs for enum variants.
