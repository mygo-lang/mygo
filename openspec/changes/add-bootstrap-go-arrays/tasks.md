## 1. Array type model

- [ ] 1.1 Add TypeExpr/MonoType array variant and check length representation, update equality, substitution, free variables, generic instantiation, alias and package qualification processing; pass element type substitution and non-uniform directional testing of different lengths.
- [ ] 1.2 Update MyGO type display, Go string and Go AST type generation; verify that `Int32[5][3]`, `Slice[Int32[5]]` and `T[5]` generate correct Go types respectively.

## 2. Grammar and formatting

- [ ] 2.1 Add array CST nodes, suffix classification and lowering, retain trivia and length tokens; pass generic/array combination, continuous suffix, zero length, illegal length and lossless source text replay tests.
- [ ] 2.2 Support expected concatenation of `[0] as Int[1] * 5` and update formatter; verify AST equivalent to bracketed form, double formatting idempotence, and existing generics and numeric multiplication regressions.

## 3. Strict array initialization

- [ ] 3.1 Provides array expected type propagation for ascriptions, variable annotations, function parameters, return values, and fields, recursively checks elements, and handles numeric literals; passes per-context, alias, generic element, and nested array positive test.
- [ ] 3.2 Reject few elements, multiple elements, mismatched element types, arrays of different lengths, and Slice-to-array ascription, bypassing the loose ascription fallback; verification errors include source location and expected/actual length, and unannotated literals are still Slice.
- [ ] 3.3 Update literal code generation to use inferred array types; verify that generation of exact literals and `[] as Int[0]` Go compiles and values ​​are correct.

## 4. Repeated operations on arrays

- [ ] 4.1 Add repeated inference, integer literal count check and target int length product check assigned by left array type; pass `Int[1] * 5`, sequence repeat, zero count and illegal count/overflow tests, and verify that numeric multiplication maintains the original behavior.
- [ ] 4.2 Use temporary variables and bounded loops to generate repeated results to avoid expanding the source code by K; perform generated code verification sequence order, left-side single evaluation (including K=0), value element copying and reference element sharing.

## 5. Go interop

- [ ] 5.1 Recursively parse Go `[N]T` function and field signatures and preserve package-qualified element types and lengths; pass FFI-directed testing of array parameters, return values, nested arrays, and imported fields.
- [ ] 5.2 Added cross-package end-to-end use cases, covering array aliases, generic elements and Go array value copying; verifying that generated Go unpackaged conversions and length mismatches will report an error in the MyGO stage.

## 6. Bootstrap and documentation

- [ ] 6.1 Synchronize the affected MyGO packages and generate Go in dependency order, run the corresponding ast2/parser2/typeinference2/codegen2/formatter/compiler test; verify that the newly built CLI can synchronize the affected packages again and keep the generated results stable.
- [ ] 6.2 Update README.md and docs/compiler/collections.md, add fixed array, exact initialization and repeated operation examples; verify that the examples can be compiled using the current bootstrap CLI, and error examples are indeed rejected.
- [ ] 6.3 Run cross-layer array regression with `openspec validate add-bootstrap-go-arrays --strict`; confirm that all spec scenarios have evidence of testing or explicit validation.