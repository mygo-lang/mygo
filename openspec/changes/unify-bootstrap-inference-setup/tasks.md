## 1. Shared inference setup

- [ ] 1.1 Extract a typeinference2 internal setup helper that seeds imports, predeclares effective package declarations, and derives structural symbols from the predeclared environment; verify all helper callers retain their prior base-environment semantics.
- [ ] 1.2 Route `InferFile`, ordinary package inference, and raw external-declaration inference through the shared setup helper; verify aliases declared in a later package source resolve in struct and enum-variant field symbols.
- [ ] 1.3 Route already-inferred external-package inference through the same helper while preserving its batched newest-first external environment and inherited fields; verify external test-package inference remains isolated.

## 2. Regression coverage and validation

- [ ] 2.1 Add an `InferPackageWithExternal` regression using separate sources with a forward `RunID = String` alias and a struct literal field initialized by `""`; verify the focused typeinference2 test passes.
- [ ] 2.2 Regenerate any required self-hosted generated Go artifacts and run the focused typeinference2 suite; verify generated sources are synchronized with the MyGO implementation.
- [ ] 2.3 Run bootstrap sync or build against the external package and the relevant repository validation suite; verify no `RunID` versus `String` struct-field unification error remains.
