## Why

Combining a placeholder partial application with a pipe and a multiline callback should work under the existing language semantics, but the reported codegen2 failure has not yet been localized and codegen3 currently lowers placeholder calls as ordinary calls. This prevents users from reliably using callback-style APIs across backends.

## What Changes

- Reproduce the reported callback expression through parsing, inference, code generation, Go compilation, and execution on both backends; fix any confirmed frontend or codegen2 defect.
- Support placeholder partial applications in the codegen3 IR path, including their use as either pipe's function operand, while preserving existing placeholder and pipe evaluation rules.
- Add backend parity and regression coverage for callback literals, ordinary partial applications, evaluation order, supported dispatch and FFI boundaries, and existing diagnostics.
- Preserve existing syntax, typing, precedence, and underscore meanings; this change does not introduce a new pipe insertion rule.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-pipe-operators`: Explicitly require placeholder partial applications combined with callback pipes to work on both bootstrap backends.
- `language/placeholder-partial-application`: Explicitly require backend parity for established partial-application semantics, including use outside pipes.

## Impact

Potentially affects parser2, typeinference2, codegen2, IR3 lowering and codegen3, as determined by reproduction; adds compiler and runtime regression coverage. No public syntax or dependency changes are intended.
