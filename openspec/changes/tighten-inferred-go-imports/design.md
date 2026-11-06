## Context

See proposal.md for motivation. The current pipeline has three relevant
stages:

1. bootstrap.mygo collects Go packages from every declaration in the
   inference source set, including declarations appended from dependency MyGO
   packages (bootstrapAppendMyGoPackageSources, then
   bootstrapCollectGoPackages).
2. codegen2.generateOneFile passes that complete list to inferredGoImports,
   which appends an import for every entry not already present.
3. After lowering, goast.ReferencedImportAliases scans the generated Go for
   selector text and keeps imports whose alias appears. The scan has no scope
   information, so a local variable named like an alias keeps the import.

Foreign MyGO nominal types already follow a different path: their canonical
paths are collected from the file's typed declarations
(inferredTypeImportPaths) and only emitted when the file references them.
Qualified Go types are collected by the same traversal, but their imports are
currently seeded by the unconditional stage 2 list instead.

## Goals / Non-Goals

**Goals:**

- Generated files import only Go packages that their own emitted code
  references.
- Dependency declarations remain usable for FFI and signature resolution
  without granting the consumer package an import.
- Import usage detection distinguishes package references from local bindings
  that reuse the alias.

**Non-Goals:**

- Changing FFI signature collection or Go package metadata loading.
- Renaming identifiers in MyGO sources.
- Replacing the generated-source validation in codegen2 dynamic metadata.
- Changing explicit import syntax or alias precedence rules.

## Decisions

### Decision: infer Go imports from referenced types, not from every collected package

Resolve Go imports the same way foreign MyGO types are resolved: collect the
qualified type paths that actually appear in the file's typed declarations,
then map each go: path to a Go package entry from the resolution set. The
unconditional loop over info.GoPackages is removed from generateOneFile.

Why: the file's typed declarations are the authoritative record of what the
emitted Go can mention. A package that a dependency imports but the consumer
never references cannot appear in emitted code, so it must not produce an
import.

Alternatives considered:

- Filter info.GoPackages by alias text before emission. Rejected because the
  same local-binding collision that breaks the final filter would also make
  this filter keep a false positive.
- Keep the list and rely on the existing final filter. Rejected because that
  filter is exactly the component that misreads local bindings.

### Decision: keep two Go package roles explicit in the pipeline

The package entry list keeps serving signature resolution and alias choice.
Import emission separately consumes only referenced paths. If the inference
data model needs to expose both, add a package-level field for the consumer's
own source imports rather than overloading GoPackages.

Why: dependency Go imports are needed to type-check dependency signatures
(for example a dependency returning a value typed from go/net/http), but they
are not evidence that the consumer emits that package.

Alternatives considered:

- Drop dependency Go packages entirely from the consumer's inference input.
  Rejected because signature resolution would lose type identity for
  third-party return types.

### Decision: detect usage where the generator emits a qualified reference

Record import usage when the generator materializes a package-qualified
reference (foreign type qualification, Go FFI selector, package constant or
function). Filter emitted imports by that recorded set; keep the existing AST
scan only as a diagnostic check that the recorded set covers the rendered
source.

Why: the generator already knows which alias belongs to which package at the
moment it writes the identifier. Recording there avoids reconstructing scopes
from generated text.

Alternatives considered:

- Scope analysis over the generated Go AST. Rejected because go/ast parsing
  does not populate object resolution, and go/types would require loading the
  generated file's imports.
- Excluding AST-visible local bindings from the alias scan. Rejected because
  shadowing is scoped: a name bound in one function must not hide a genuine
  package use in another.

## Risks / Trade-offs

- A missed emission site would drop a required import and surface as a Go
  compile error in generated code. Mitigation: keep the rendered-source check
  and cover FFI selectors, foreign types, and prelude helper references in
  tests.
- Inference may reference a Go type through an erased declaration, in which
  case no import is emitted. This matches the existing erased-typeclass rule
  and is covered by a spec scenario.
- The resolution set still contains dependency Go packages, so alias
  selection can differ from a set restricted to the consumer's imports.
  Mitigation: choose aliases deterministically from explicit imports first,
  then from referenced packages, and assert stable output in tests.
