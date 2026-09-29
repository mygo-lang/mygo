## Context

See proposal.md for motivation and specs/per-file-import-resolution/spec.md for the behavior contract. Package loading currently merges import aliases across source files, inference processes declarations in one package environment, and generation gives each output file the package-wide import set. The change must keep package declarations and impls shared while making source import resolution local to each declaration's origin.

## Goals / Non-Goals

**Goals:**

- Resolve explicit package names against the import environment of the declaration's source file.
- Keep imported type identity independent from a file-local alias, so signatures and inferred values can cross file boundaries safely.
- Keep package-wide declarations, typeclasses, and impl metadata available to inference in every file.
- Emit a minimal, correct import set independently for every generated Go file.
- Keep the normal and self-hosted compiler paths behaviorally aligned.

**Non-Goals:**

- Change Go or MyGO package export rules, or make another file's local import alias usable as an expression qualifier.
- Change typeclass method lowering or Go FFI method semantics beyond making their imported type metadata available across files.
- Change import behavior for generated files outside the source file that owns their declarations.

## Decisions

### Build a source-file import environment before inference

Index each `ImportDecl` by its source file and resolve each declaration using that file's imports. Keep package declarations and impls in the shared package environment. Detect conflicting aliases within one file; allow the same alias to map to different paths in different files. Dot imports follow the same file scope.

This is preferred over retaining a package-wide alias environment because package-wide lookup makes sibling imports accidentally visible and makes alias meaning depend on unrelated files.

### Store canonical type identity separately from source spelling

Represent a foreign type in inferred signatures by its canonical origin (Go import path plus exported type name, or MyGO package identity plus exported type name), not by a source alias. Resolve that identity while processing the owning declaration, using its file-local imports. Preserve enough origin metadata to load Go method sets and to look up package-wide MyGO impls after the value crosses into another file. At a use site, a type's origin grants method/impl resolution but does not introduce the source import alias into that file's name environment.

This extends existing qualified-type handling rather than rewriting source signatures to depend on aliases shared across files. Imported function signatures and aliases must be normalized recursively through generic arguments, tuples, and function types.

### Select imports from each file's emitted dependencies

Keep a package-wide import-path registry for resolving type origins and building import candidates, but do not pass that package-wide list directly to each renderer. For each generated file, collect actual package references from emitted Go AST nodes, including qualified types and selectors, then resolve those references to that file's candidate imports. Include compiler-synthesized dependencies such as Prelude and test-package imports only when emitted declarations require them. Run Go formatting/import normalization after this filtering.

The import collector is preferred over relying on `goimports` to repair package-wide imports: erased declarations and package aliases can leave no reliable type reference in the rendered file for that tool to classify.

### Parse inline Go package references

Inline Go code SHALL be parsed to identify package-qualified references, such as `mail.ParseAddress`, and those references SHALL contribute to the generated file's import set. The parser must account for substituted value and type operands without treating selectors on ordinary values as package qualifiers; resolution is against the current `.mygo` file's import aliases. Inline Go users do not need a separate dependency declaration. If the inline fragment cannot be parsed as a standalone expression, use the inline-Go grammar/lowering context to extract its package references rather than conservatively importing every package in the MyGO package.

This is preferred over a manual dependency list because the snippet already contains the package qualifier and declarations should have one source of truth. It also avoids retaining unrelated imports from the same file.

### Apply one contract to both compiler pipelines

Implement and verify the behavior in the package compiler/type inference and code generation paths, then carry the same source-file import context and import filtering contract into the self-hosted compiler pipeline. Shared canonical type identities should be the boundary between inference and generation so neither path reconstructs identity from a globally visible alias.

## Risks / Trade-offs

- [A type may lose its origin while being generalized or expanded through an imported signature] -> Add focused tests for nested generic, tuple, alias, and function types crossing file boundaries; keep origin metadata through substitutions and scheme instantiation.
- [Typeclass impl lookup may key on alias spellings today] -> Normalize impl receiver and constraint keys to canonical type identity before lookup, and test both direct and chained calls.
- [Import alias collisions that were previously rejected package-wide may become valid across files] -> Validate collisions per file, while retaining errors for conflicting declarations within the same file.
- [Inline Go contains selectors that are not package references] -> Resolve parsed qualifiers against the current file's import table and use expression/type context to distinguish package names from value selectors; add tests for both forms.
- [Compiler-synthesized imports may be dropped if filtering only sees source imports] -> Treat Prelude, test-package, reflection, and helper imports as candidates in the same per-file reference analysis and cover each synthesis path.
- [Bootstrap and self-hosted codegen may drift] -> Add equivalent cross-file inference and generated-import scenarios to both workflows and compare buildable output.

## Migration Plan

No source migration is intended for code that declares imports in every file where their aliases are used. Packages that relied on an import alias leaking from a sibling file will need to add that import to the file that directly names the package. Calls that only consume a value returned with an imported type should continue to work without repeating the type's import. Regenerate generated Go files after upgrading; rollback consists of reverting the compiler change and regenerating them.
