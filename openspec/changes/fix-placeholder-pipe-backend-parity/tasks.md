## 1. Reproduce and isolate

- [x] 1.1 Add equivalent direct-call and `WithSession(1, 2, _) <|` multiline callback fixtures; run parsing, inference, generation, Go compilation and execution with codegen2 and codegen3 and record the first failing stage for each.
- [x] 1.2 Add a focused regression at each confirmed frontend or codegen2 failure boundary, fix the cause if present, and verify both the direct and piped fixtures pass on codegen2.

## 2. Implement typed partial applications in IR3

- [x] 2.1 Lower direct placeholder arguments into a typed residual function with fresh parameters in source order and original-call body; verify stored, immediate, nested and multiple-placeholder cases generate valid Go and execute correctly.
- [x] 2.2 Preserve instantiated types, explicit type arguments, source locations and concrete-type diagnostics; verify generic and annotated cases and unresolved-generic rejection.
- [x] 2.3 Preserve method, constructor and dictionary dispatch and fixed-arity Go FFI conversions by reusing ordinary call paths; verify representative runtime fixtures against equivalent direct calls.
- [x] 2.4 Preserve per-invocation callee, receiver and supplied-argument evaluation and existing dictionary resolution; verify repeated-invocation effect traces and parameter order.

## 3. Pipes and integration

- [x] 3.1 Verify and, where necessary, correct left-to-right pipe operand evaluation for both directions through IR normalization and codegen3 optimization; run effect-trace tests with optimization disabled and enabled.
- [x] 3.2 Run backward multiline callback and forward callback placeholder fixtures on both backends; assert result, callback count, and equivalence to direct calls after Go compilation and execution.
- [x] 3.3 Test incompatible pipe arguments and existing invalid underscore, variadic and intrinsic cases; verify source-located diagnostics and absence of invalid generated Go.
- [x] 3.4 Run affected parser, inference, IR and codegen suites plus compiler end-to-end tests; document commands and results, including any environment limitations.

## Verification

- `go test -count=1 ./internal/mygo/parser2 ./internal/mygo/typeinference2 ./internal/mygo/ir3 ./internal/mygo/codegen2 ./internal/mygo/codegen3 ./internal/mygo/compiler`
  - parser2, typeinference2, ir3, codegen2, and codegen3 passed.
  - The compiler package hit its default 10-minute timeout in `TestBootstrapIR3Codegen3StackDepth` while copying a module tree. A temporary-directory cleanup error was also reported after the timeout.
- `go test ./internal/mygo/compiler -run '^(TestPlaceholderPipeBackendParity|TestCodegen3RejectsUnresolvedPlaceholderType|TestPlaceholderPipeInvalidFormsRejected)$' -count=1 -v -timeout=10m` passed.
- `go test ./internal/mygo/compiler -run '^TestCodegen3DifferentialBehavior$' -count=1 -v -timeout=5m` passed.
- `go test ./internal/mygo/parser2 -run '^TestSwitchTargetKeepsMultilineCallbackPipe$' -count=1 -v` passed.
- `go test ./internal/mygo/ir3 -run 'TestIr3LowerPlaceholder' -count=1 -v` passed.
