## 1. Typed dependency information

- [x] 1.1 Add a per-build typed-source cache to the bootstrap state and store
  each package's `TypedDeclSources` after inference in both the codegen2 and
  codegen3 paths. Verify a direct and a transitive dependency both have typed
  declarations cached before the importer is inferred.
- [x] 1.2 Make imported-package collection prefer cached typed declarations
  and fall back to parsed sources when no typed record exists. Verify an
  unannotated binding in a transitive dependency resolves through the source
  path, and that a package with only parsed sources still loads.
- [x] 1.3 Add a package value index to `MyGoPackageInfo` (name to mutable
  flag), build it from flattened declarations, and populate it wherever the
  enum index is built, including metadata reconstruction. Verify existing
  package-info construction tests and a new index test for `let`/`var`
  entries.

## 2. Imported binding surface

- [x] 2.1 Split imported-package seeding into a declaration phase, refreshed
  twice like package predeclaration, followed by a value-binding phase that
  registers `LetDecl` and `VarDecl` schemes from `bind.Type` or, when absent,
  `bind.Value.Type`. Verify annotated and unannotated fixtures, including a
  binding whose annotation names a type or alias declared later in the file.
- [x] 2.2 Project exported bindings from `exportMyGoPackageEntry`, filtering on
  the ASCII-uppercase rule and applying `wrapPkgTypesInMonoType` to the value
  type. Verify bare `Box` and `List[Box]` both resolve as the defining
  package's type when the importer declares its own `Box`.
- [x] 2.3 Confirm an uppercase binding with no annotation compiles without a
  new diagnostic and is exported with its inferred type, while a lowercase
  unannotated binding remains private and compiles locally. Verify both
  behaviors with type-inference fixtures and assert no annotation-required
  diagnostic exists.

## 3. Code generation

- [x] 3.1 In IR3 lowering, set a package value declaration's annotation from
  `binding.Type` when present, otherwise from `binding.Value.Type`. Verify the
  lowered IR carries the inferred type for an unannotated value.
- [x] 3.2 In codegen3, emit a package value from the IR annotation or the
  lowered initializer's type, and reserve the error for a value with no
  resolved type. Verify an unannotated package value compiles and runs through
  codegen3 without an annotation-required diagnostic.
- [x] 3.3 Confirm the codegen2 package binding path still emits `var X T` plus
  an initialization assignment for annotated and unannotated values. Verify the
  generated Go for both forms.

## 4. Imported mutability rules

- [x] 4.1 Teach assignment inference to consult the value index for
  `FieldExpr(IdentExpr(alias), name)` targets and reject assignment to an
  imported `let`. Verify passing and failing fixtures: imported `var`
  assignment succeeds, imported `let` assignment reports immutable, wrong-type
  assignment reports a type mismatch, and a local binding that shares the
  import alias follows the same resolution order as ordinary field inference.

## 5. Metadata

- [x] 5.1 Add an additive `Values` export list to package metadata with name,
  canonical resolved type, and mutable flag; rebuild `let`/`var` declarations
  in `ExportedDecls`. Verify a Go round-trip test reconstructs the same names,
  types, and mutability for annotated and inferred values.
- [x] 5.2 Pass typed declarations to the codegen2 metadata writer and add a
  MonoType-to-metadata converter that preserves named package identity,
  prelude names, and Go FFI paths. Verify generated metadata for builtin,
  local struct, prelude collection, generic, and third-party types, and that
  metadata-only loading resolves an unannotated binding with the recorded type.
- [x] 5.3 Preserve compatibility with metadata that has no `Values` field.
  Verify such a document loads successfully and exposes no package-level value
  members.

## 6. End-to-end behavior and documentation

- [x] 6.1 Add a two-package codegen2 fixture covering annotated and
  unannotated reads of exported `let` and `var`, assignment to the exported
  `var`, a wrong-type assignment, and an assignment to the exported `let`.
  Verify the successful program's output and each expected diagnostic.
- [x] 6.2 Assert that a file whose only use of `dep` is a value reference keeps
  the import in the generated Go and builds successfully. Verify by inspecting
  the generated file and running the package build.
- [x] 6.3 Add an initialization-order fixture where an importer's package
  binding is initialized from an exported binding. Verify the importer observes
  the dependency's initialized value rather than a zero value on codegen2.
- [x] 6.4 Run the same fixtures through codegen3 in an isolated temporary
  module, including an unannotated package value. Verify equal runtime output,
  no annotation-required diagnostic, and that generated files stay outside the
  repository.
- [x] 6.5 Add the declaration-order boundary fixture: a function declared
  before an unannotated binding still reports an unresolved identifier, while
  the same function can reference an annotated binding through predeclaration.
  Verify both outcomes.
- [x] 6.6 Update `docs/compiler/semantics.md` with the uppercase rule, optional
  annotations and inferred exported types, cross-package type qualification,
  imported `let`/`var` assignment rules, and the unchanged declaration-order
  boundary. Verify the documented examples compile.

## 7. Validation

- [x] 7.1 Regenerate affected packages with the bootstrap compiler in
  dependency order and run focused Go tests for `typeinference2`, `metadata`,
  `ir3`, `codegen2`, and `compiler`. Verify all focused tests pass with a
  task-local `GOCACHE`.
- [x] 7.2 Run the full repository test suite after the focused pass. Verify no
  regressions in existing package-binding, metadata, codegen2, IR3, or codegen3
  coverage.
- [x] 7.3 Run `openspec validate export-package-bindings --strict` and verify
  the change is valid with this spec and no skipped artifacts.
