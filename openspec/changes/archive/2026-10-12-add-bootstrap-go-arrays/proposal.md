## Why

The Bootstrap compiler currently cannot represent fixed-length Go arrays using the MyGO type, limiting the use of array fields, parameters, and Go FFI. Adding `ElementType[N]` and explicit array repetition operations maintains the native Go type identity while avoiding the need to manually write a large number of repetitive elements.

## What Changes

- Supports fixed-length array type `T[N]`, generating Go `[N]T`; the length is a non-negative integer literal, supporting zero-length and nested arrays.

- Array literals must provide exactly N elements; implicit zero padding is prohibited; list literals that do not provide the expected array type are inferred as Slices.

- Supports `T[N] * K` array repetition operations, resulting in `T[N * K]`; for example, `[0] as Int[1] * 5`. The repetition count is a non-negative integer literal, and the left side is evaluated only once. - Supports array representation in Go FFI array signatures, type aliases, generic element types, formatting, and type explicitation.

- Retains existing generic, slice, and numeric multiplication behavior; does not add generic length parameters, length constant expressions, or implicit slice/array conversions.

## Capabilities

### New Capabilities

- `bootstrap-go-arrays`: Fixed-length array syntax, type checking, strict initialization, repeated operations, and Go interoperability.

### Modified Capabilities

None.

## Impact

Involves parser2 CST/AST lowering, AST2 type model, typeinference2 type unification and expected type propagation, codegen2 and Go AST type generation, Go FFI signature parsing, formatter, related type explicitation, and regression testing. Implementation requires synchronous bootstrap generation files; older compilers are not included in this scope. No new runtime dependencies are required.