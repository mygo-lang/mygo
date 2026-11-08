## 1. Reproduce and isolate

- [ ] 1.1 Add equivalent direct-call and `WithSession(1, 2, _) <|` multiline callback fixtures; run parsing, inference, generation, Go compilation and execution with codegen2 and codegen3 and record the first failing stage for each.
- [ ] 1.2 Add a focused regression at each confirmed frontend or codegen2 failure boundary, fix the cause if present, and verify both the direct and piped fixtures pass on codegen2.

## 2. Implement typed partial applications in IR3

- [ ] 2.1 Lower direct placeholder arguments into a typed residual function with fresh parameters in source order and original-call body; verify stored, immediate, nested and multiple-placeholder cases generate valid Go and execute correctly.
- [ ] 2.2 Preserve instantiated types, explicit type arguments, source locations and concrete-type diagnostics; verify generic and annotated cases and unresolved-generic rejection.
- [ ] 2.3 Preserve method, constructor and dictionary dispatch and fixed-arity Go FFI conversions by reusing ordinary call paths; verify representative runtime fixtures against equivalent direct calls.
- [ ] 2.4 Preserve per-invocation callee, receiver and supplied-argument evaluation and existing dictionary resolution; verify repeated-invocation effect traces and parameter order.

## 3. Pipes and integration

- [ ] 3.1 Verify and, where necessary, correct left-to-right pipe operand evaluation for both directions through IR normalization and codegen3 optimization; run effect-trace tests with optimization disabled and enabled.
- [ ] 3.2 Run backward multiline callback and forward callback placeholder fixtures on both backends; assert result, callback count, and equivalence to direct calls after Go compilation and execution.
- [ ] 3.3 Test incompatible pipe arguments and existing invalid underscore, variadic and intrinsic cases; verify source-located diagnostics and absence of invalid generated Go.
- [ ] 3.4 Run affected parser, inference, IR and codegen suites plus compiler end-to-end tests; document commands and results, including any environment limitations.
