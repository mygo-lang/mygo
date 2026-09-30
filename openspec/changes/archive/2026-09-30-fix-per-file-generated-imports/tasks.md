## 1. Source-file import environments and type identity

- [x] 1.1 Index imports by source file and use each declaration's file-local aliases during inference; verify sibling aliases are invisible, same aliases can map to different paths across files, and same-file conflicts still fail.
- [x] 1.2 Preserve canonical Go and MyGO type origins through imported signatures, aliases, substitutions, and generic/tuple/function types; verify a caller can infer an imported return type without importing its package.
- [x] 1.3 Resolve Go method sets and package-wide MyGO typeclass/inherent impls from canonical types across files; verify chained method and typeclass calls on returned values.

## 2. Per-file generated imports

- [x] 2.1 Collect imports from each generated Go file's emitted declarations and expressions, including qualified types and selectors; verify used imports remain and unused package-wide imports disappear.
- [x] 2.2 Parse inline Go fragments to collect package-qualified references against the owning source file's import table; verify inline package calls retain their import without importing sibling-file dependencies.
- [x] 2.3 Include compiler-synthesized Prelude, test-package, reflection, and helper dependencies in per-file import selection; verify existing generation scenarios remain buildable and import-clean.

## 3. Self-hosted compiler parity and integration

- [x] 3.1 Apply source-file import visibility, canonical imported type propagation, and per-file import filtering to the self-hosted compiler path; verify its focused inference and codegen cases match the package compiler behavior.
- [x] 3.2 Add an end-to-end multi-file fixture covering an imported typeclass signature, an unrelated file with no reference, and a caller using a returned imported type's method; verify each generated Go file has only the imports it needs and the package builds.
- [x] 3.3 Run the relevant compiler and bootstrap test suites and validate the change artifacts; verify no generated file has unused imports and no import-resolution regressions remain.
