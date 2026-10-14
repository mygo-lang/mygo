## Context

See proposal.md for motivation. The reported compiler revision is `ec912b1849a3b5be95f254376bb29e0f8518ee2f`, which was also the inspected checkout. Imported impl signatures are resolved through their owning package environment in `myGoImplMethodSymbolsOne`; `wrapPkgTypesInMonoType` preserves existing qualified paths. The exact typed AST for the external application has not been inspected, so the first implementation step must distinguish an inference identity loss from the confirmed rendering defect.

`generateOneFile` builds candidate imports from `info.GoPackages` only. Both string and structured MonoType renderers look up `TQualifiedName` paths in a file alias table and currently emit the inner bare name when no alias exists. These two facts explain the reported output even when inference has retained the package identity.

Imported signature projection also recurses into the nominal constructor inside an existing qualified type. When the exporting package declares a type with the same name, this can introduce an incorrect second qualification. Protect the already-owned constructor while still qualifying locally supplied generic arguments.

This design is required because imported signature handling, file import planning, and multiple rendering paths interact.

## Goals / Non-Goals

**Goals:**
- Keep canonical package paths in semantic types until file-specific lowering chooses aliases.
- Give string and structured lowering one consistent contextual package resolution policy.
- Keep import planning deterministic, file-local, and limited to dependencies already represented by inferred types.

**Non-Goals:**
- Changing source import syntax, generic dispatch rules, or package visibility.
- Fixing unrelated legacy generator behavior or modifying external application source.
- Importing every transitive dependency unconditionally.

## Decisions

### 1. Verify semantic identity at the imported API boundary

Create a minimal three-package fixture: auth defines Identity, storage imports auth and exposes a repository method and function returning it inside Result, and server imports storage only. Assert the canonical auth path inside inferred return and generic argument types. If that assertion fails, repair signature projection in the owning environment before rendering. Preserve existing qualified paths; do not infer ownership from a bare name in the consumer environment.

Alternative: prepend the method-owning package alias to the return name. Rejected because storage does not define Identity and same-named types must remain distinct.

### 2. Plan imports before lowering and filter them after emission

Add a recursive semantic type dependency collector for the current file's typed declarations and expressions. Visit qualified names and nested applications, arrays, tuples, functions, and variadic functions. Use canonical paths and available MyGO package metadata to create candidate imports alongside existing Go FFI candidates. The collector must cover type arguments selected during method lowering through their inferred receiver, argument, and result types.

Reserve explicit aliases first, reuse imports by path, and allocate stable aliases for new paths using the existing collision policy. Exclude the current package's canonical path; obtain it from resolved package/compiler metadata, not the package's short name. Feed the complete alias table into lowering, then retain only imports referenced by emitted Go AST, using the existing referenced-alias filtering. Candidate imports are permitted to over-approximate semantic usage; final imports must reflect emitted usage.

Alternative: insert imports after rendering by searching generated text. Rejected because package ownership is semantic information and bare names no longer encode it. Importing all cached packages also risks unused imports and alias-dependent changes unrelated to this file.

### 3. Share contextual qualification rules across type renderers

Resolve each qualified type as local, explicitly dot-imported, normally imported, or unresolved. Local types render bare; normal imports use their allocated alias. Unresolved foreign paths produce a generation diagnostic containing the path and nominal type, propagated through the file-generation error channel. Keep context-free diagnostic formatting separate from contextual emitted-type rendering so an empty diagnostic alias table does not become a compilation error.

String and structured renderers must use the same resolver and preserve nested generic parameter context. Do not simply change the current missing-alias fallback globally: it also serves callers without a file context.

Alternative: keep falling back to bare names. Rejected because it reports successful MyGO generation while producing undefined or incorrectly bound Go types.

### 4. Validate generated code and behavior

Use compiler integration fixtures authored through the existing test conventions. Verify canonical inference identity, qualified helper type arguments, emitted imports, and `go test -buildvcs=false` for the generated module. Include alternate explicit aliases, alias collisions between distinct paths with same-named nominal types, nested type shapes, per-file import filtering, current-package rendering through a synthetic type-rendering probe, and unresolved-package diagnostics.

The current-package probe is synthetic because a literal three-package source graph returning the consumer's type can introduce a forbidden Go import cycle; self-import prevention must not depend on constructing such a cycle.

## Risks / Trade-offs

- [A type required only by lowering escapes dependency collection] -> Cover selected impl type arguments explicitly and diagnose missing mappings rather than emit bare foreign types.
- [Short package names confuse current-package detection] -> Compare canonical paths and test same-named foreign packages.
- [A global rendering change breaks context-free callers] -> Apply strict resolution only to emitted types with file context.
- [Alias allocation becomes unstable] -> Allocate in deterministic traversal order and test repeat generation.
- [Existing bootstrap artifacts mask changed behavior] -> Refresh generated compiler packages in dependency order and test the rebuilt compiler.

## Migration Plan

No source migration is required. Existing explicit-import/type-annotation workarounds remain compatible. Implement in handwritten MyGO files, regenerate affected Go artifacts through bootstrap sync, and validate inference, codegen, compiler integration, and the runnable main example with a writable task-local GOCACHE. Rollback reverts the handwritten change and its regenerated artifacts together.
