# Tasks

## 1. Define contained package output mapping

- [x] 1.1 Map in-module packages using the existing module-relative layout and out-of-module packages under deterministic module/import-path namespaces; verify MyGO path tests cover module-cache paths, local replacements, encoding collisions, and missing module identities.
- [x] 1.2 Add a shared output-root containment check and reject parent traversal, absolute paths, and symlink escapes before filesystem operations; verify MyGO tests confirm rejected paths create no directories or files.

## 2. Stage external MyGO modules safely

- [x] 2.1 Copy each external dependency module once under the active build root while preserving ordinary Go sources and package layout; verify MyGO tests confirm paths stay contained and source-module contents remain unchanged.
- [x] 2.2 Create a per-build temporary `-modfile` with local replacements to staged modules and pass it to package discovery and Go build/test commands; verify MyGO tests confirm the project's `go.mod` and `go.sum` remain byte-identical and replacements resolve to staging paths.

## 3. Route generated files and overlays

- [x] 3.1 Write external package and external test outputs into their validated staged module package directories; verify generated files remain under the active root and source module directories stay untouched.
- [x] 3.2 Align cache restore/publication, optional emit paths, manifests, and overlays with the staged package mapping; verify cache hits restore generated files into the staged copy and overlays contain no target beneath `GOMODCACHE`.

## 4. Verify external dependency builds

- [x] 4.1 Add a MyGO integration fixture importing a MyGO dependency from the module cache and verify codegen3 build/test succeeds through the temporary replacement module file without creating or modifying files in the dependency.
- [x] 4.2 Run focused codegen3 compiler tests and verify generated and staged paths remain under the active output root, module inputs remain unchanged, and cold/cache-hit builds both work.
