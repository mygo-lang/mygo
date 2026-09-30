## Why

Receiver-style calls to typeclass methods can lower to generated helpers that declare dictionary parameters for their `using` constraints, while the call omits those arguments. This leaves valid MyGO source producing Go that fails to compile, as with `Set.Contains` and its required `Eq[A]` evidence.

## What Changes

- Ensure generated receiver-method calls pass all instance dictionaries required by the selected helper's `using` constraints.
- Preserve matching by constrained type and declaration order when calls require multiple dictionaries.
- Add coverage for receiver calls whose implementations do not themselves use the dictionary, since the declared interface contract still requires it.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen-correctness`: generated calls to typeclass receiver helpers must include required dictionary arguments and remain buildable.

## Impact

- Affects bootstrap type inference metadata and code generation for receiver-style typeclass method calls, plus compiler regression coverage.
- No source syntax or public API change is intended.
