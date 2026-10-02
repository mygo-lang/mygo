# Tasks

## 1. Track prelude declaration references

- [x] 1.1 Update codegen2 import analysis to recognize prelude function names even when their FFI signatures use Go built-in types; verify with a focused regression test for a direct `Panic` call.
- [x] 1.2 Preserve per-generated-file import selection and avoid adding unused prelude imports; verify with regression coverage for a file without prelude references and run the focused codegen2 tests.

## 2. Verify generated output

- [x] 2.1 Reproduce the listener case in a temporary project copy and confirm its generated Go includes the prelude import alongside `Panic`; verify the generated package compiles with the expected import resolution.
