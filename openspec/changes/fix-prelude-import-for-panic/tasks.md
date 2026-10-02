# Tasks

## 1. Track prelude declaration references

- [ ] 1.1 Update codegen2 import analysis to identify references to prelude declarations by resolved declaration identity or equivalent stable ownership metadata, including declarations whose FFI types are Go built-ins; verify with a focused regression test for a direct `Panic` call.
- [ ] 1.2 Preserve per-generated-file import selection and avoid adding unused prelude imports; verify with regression coverage for a file without prelude references and run the focused codegen2 tests.

## 2. Verify generated output

- [ ] 2.1 Regenerate or reproduce the listener case and confirm its generated Go includes the prelude import alongside `Panic`; verify generated package compiles with the expected import resolution.
