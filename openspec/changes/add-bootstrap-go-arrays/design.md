## Context

See proposal.md for motivation and specs/bootstrap-go-arrays/spec.md for behavior. parser2 currently classifies bracketed type suffixes as generic arguments, while ast2.TypeExpr and MonoType have no array variant. inferSlice infers Slice values before ascriptions; inferTypeAs has a permissive failure path that must not conceal array incompatibility. codegen2 has both AST and string type emission paths, and Go signature parsing currently recognizes Slice prefixes.

## Goals / Non-Goals

**Goals:** Carry element type and static length through the entire bootstrap pipeline; diagnose errors before Go compilation; retain the existing unannotated list literal behavior.

**Non-Goals:** Introduce dependent types, compile-time expression evaluation, new indexing or collection method syntax, legacy compiler parity, or additional Slice conversion rules. Existing array values can be used through Go interoperation; collection operations beyond this contract can be proposed separately.

## Decisions

### Dedicated array types and lossless suffix parsing

Add an ArrayType CST kind and `ArrayType(Ref[TypeExpr], length)` / `TArray(Ref[MonoType], length)` variants. Preserve the raw length token in CST and normalize its numeric value for type equality. Use a checked representation that preserves valid target-int lengths without host overflow. Recursively update equality, substitution, free variables, alias resolution, qualification, generic instantiation, and type rendering.

Classify a suffix containing an integer literal as an array suffix, and a suffix containing types as a generic application. Reject malformed numeric suffixes explicitly. Support repeated array suffixes and generic types as array elements; preserve the existing same-line suffix rule. Do not encode lengths as fake type names or TApp arguments: those would conflate values and type parameters and complicate diagnostics.

### Expected-type-directed literals

Reuse the existing list literal expression syntax. Add a common expected-type inference route for ascription, annotated bindings, call arguments, returns, and fields. Resolve aliases before deciding whether a literal expects an array. Validate element count before element inference, then infer each item against the expected element type, including nested arrays and numeric literal specialization.

Reject array mismatches explicitly rather than accepting inferTypeAs's general fallback. Do not broaden this work into a global cast redesign. Typed literals must carry an array result type into codegen; extend literal lowering so it does not assume every list literal is a Slice. A separate surface literal syntax was rejected because the expected type already distinguishes arrays and slices.

### Typed repetition using existing multiplication syntax

Dispatch `*` based on the inferred left operand: arrays require an integer-literal right operand and produce a checked N*K length; numeric operands retain existing inference. Accept the unparenthesized requested form with semantics `([0] as Int[1]) * 5`, using the smallest parser precedence adjustment necessary and testing existing numeric/ascription expressions. Preserve source syntax while marking typed array repetition distinctly for lowering.

Lower repetition using a scoped Go expression with a temporary for the left operand, an explicitly typed result array, and bounded loops that copy each input block in order. This avoids generated-code expansion for large K and guarantees one evaluation, including K=0. Reference elements retain sharing through ordinary Go assignment. Runtime repeat counts were rejected because the output length must be statically known. Both N and K, as well as their product, must fit target int; nested array lengths are checked independently.

### Native Go emission and signatures

Emit `[N]T` in both MonoType and TypeExpr emission routes, including Go AST construction. Extend Go signature decoding recursively for array prefixes before generic/named fallback. Propagate array types through Go imported fields and package metadata. Preserve aliases and package identity of element types. Update formatter type-node classification and user-facing MyGO type displays to output postfix syntax.

## Risks / Trade-offs

- [Permissive ascriptions hide mismatches] → Add negative compiler tests for array length, element type, and Slice conversions across all expected-type contexts.
- [New enum variants silently miss traversals] → Audit every TypeExpr/MonoType switch and run focused type-system and compiler tests after regeneration.
- [Precedence changes affect existing expressions] → Verify AST shape and formatter round trips for ascription with numeric multiplication and the requested shorthand.
- [Go rejects oversized array types beyond int bounds] → Validate integer representability and multiplication overflow in MyGO; document any further Go implementation size limits when encountered rather than inventing a language limit.
- [Self-hosting requires a buildable generation order] → Establish a working CLI before source changes, regenerate dependency-first, and verify a fresh CLI can regenerate the changed packages.

## Migration Plan

This is additive source syntax. Implement AST/type support, parser/tooling, inference, and codegen in buildable stages, synchronizing generated Go with the repository bootstrap process. Run targeted regressions and self-hosting verification, then update README and collection documentation. Rollback consists of reverting implementation and generated outputs together; no runtime data migration is required.
