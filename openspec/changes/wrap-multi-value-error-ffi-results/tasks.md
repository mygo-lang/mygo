## 1. Result Shape Inference

- [ ] 1.1 Extend Go FFI signature inference in both compiler pipelines to map signatures with multiple values followed by `error` to `Result[tuple-of-values, Error]`; verify inference fixtures cover two or more success values, `(T, error)`, lone `error`, and `(T, bool)`.
- [ ] 1.2 Preserve package-qualified and package-local named types in every tuple payload element; verify fixtures assert the resulting tuple element types.

## 2. Go Call Lowering

- [ ] 2.1 Update Result boundary lowering to bind all success values and the trailing error from one call, constructing `Ok` with the complete payload or `Err` with the error; verify generated Go parses and preserves a single call evaluation.
- [ ] 2.2 Keep explicit tuple destructuring on the raw Go multi-return path, including the trailing error and exact arity diagnostics; verify raw generated assignment and mismatch cases.
- [ ] 2.3 Cover package functions and imported methods, along with discarded statement calls; verify both expression wrapping and statement behavior in codegen tests.

## 3. Cross-Pipeline Conformance

- [ ] 3.1 Add or update shared conformance cases for Result tuple matching and extraction, and verify bootstrap and self-hosted outputs agree on inferred shape and behavior.
- [ ] 3.2 Run the relevant compiler and bootstrap FFI test suites and confirm existing lone-error, two-result error, and `(T, bool)` cases remain passing.
