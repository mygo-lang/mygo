## 1. Go Interface Metadata

- [ ] 1.1 Record complete method-set metadata for imported named interfaces, including methods promoted through embedded interfaces, and verify loader coverage for named and embedded interfaces.
- [ ] 1.2 Preserve Go method signature identity needed for assignability checks, including package identity where Go requires it, and verify matching and mismatching signature cases.

## 2. FFI Argument Inference

- [ ] 2.1 Add a Go-interface assignability check at imported function and method argument boundaries, leaving ordinary unification unchanged, and verify `io.ReadCloser` is accepted for an `io.Reader` parameter.
- [ ] 2.2 Reject source interfaces missing a target method or declaring an incompatible signature, and verify the diagnostic points to the failing call argument.

## 3. Integration Coverage

- [ ] 3.1 Add inference and generated-Go coverage for distinct named interfaces, embedded interface satisfaction, and the original SMTP `ReadCloser` to `Reader` call; verify generated calls pass the original value directly.
- [ ] 3.2 Run the focused compiler test suites and OpenSpec validation, and verify all interface assignability scenarios pass without changing non-FFI type unification.
