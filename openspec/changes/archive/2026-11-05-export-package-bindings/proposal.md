## Why

A MyGO package can export functions, types, and enum constructors, but an
imported package cannot name a package-level `let` or `var`. The compiler
loads the dependency's declarations and even emits an exported Go variable for
an uppercase binding, yet inference never registers that binding. A reference
such as `config.DefaultTimeout` therefore fails with `unknown identifier
config`, which forces packages to add accessor functions for shared constants
and mutable state.

## What Changes

- A package-level `let` or `var` whose name starts with an uppercase letter
  becomes an importable member and is referenced as `alias.Name`. Lowercase
  bindings stay private to their defining package.
- An explicit type annotation is optional for an exported binding. When the
  annotation is present it is used directly; otherwise the exported type is
  the type inferred for the initializer by the defining package's inference
  pass. A missing annotation alone SHALL NOT produce a new diagnostic.
- The imported binding keeps its defining package's type identity. A value
  whose inferred or declared type is `Box` in `dep` is seen by importers as
  `dep.Box`, so a local type with the same name cannot capture it.
- Package metadata gains exported binding entries containing the binding name,
  its resolved type, and whether it is mutable. Packages loaded from metadata
  alone reconstruct the corresponding `let` or `var` declarations.
- Reading an exported binding is allowed from other packages. Assigning to an
  exported `var` is allowed and type-checked against its resolved type.
  Assigning to an exported `let` through a package qualifier is rejected.
- The codegen2 and codegen3 paths keep equivalent observable behavior for these
  references. Codegen3 no longer rejects an unannotated package value when its
  inferred type is available; it uses that type instead.

This change does not make an unannotated package binding visible to
declarations that appear earlier in its defining package. That declaration
order behavior is unchanged.

## Capabilities

### New Capabilities

- `language/exported-package-bindings`: export and import semantics for
  uppercase package-level `let` and `var` bindings, including annotation-or-
  inference type resolution, type qualification, mutability, and metadata
  support.

### Modified Capabilities

None.

## Impact

- `internal/mygo/typeinference2`: seed and export imported package bindings,
  resolve their types from annotations or inferred value types, qualify those
  types, reject assignment to imported `let` bindings, and reconstruct
  metadata-only dependencies.
- `internal/mygo/compiler`: cache each dependency's typed declarations after
  inference and prefer them when collecting imported package information, so
  unannotated bindings expose their inferred types to importers.
- `internal/mygo/ir3` and `internal/mygo/codegen3`: fall back to the inferred
  package value type when no annotation exists, and reserve the current error
  for values with no resolved type at all.
- `internal/mygo/metadata`: add an additive `Values` export list with the
  resolved type and rebuild `let`/`var` declarations in `ExportedDecls`.
- `internal/mygo/codegen2/dynamic_metadata.go`: emit value exports from typed
  declarations, including a MonoType-to-metadata conversion that preserves
  package identity.
- `docs/compiler/semantics.md`: document cross-package visibility, optional
  annotations, inferred exported types, and imported `let`/`var` assignment
  rules.
- Tests: two-package fixtures compiled through codegen2 and codegen3;
  type-inference coverage for annotated and unannotated bindings,
  qualification, and immutability; metadata round-trip coverage.
- No generated-Go ABI change: uppercase bindings already lower to exported Go
  package variables in both backends.
