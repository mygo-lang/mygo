# Tasks

## 1. Define contained package output mapping

- [ ] 1.1 Map in-module packages using the existing module-relative layout and out-of-module packages under a deterministic import-path namespace; verify path unit tests cover module-cache paths, local replacements, encoding collisions, and missing import paths.
- [ ] 1.2 Add a shared output-root containment check and reject parent traversal, absolute paths, and symlink escapes before filesystem operations; verify focused tests confirm rejected paths create no directories or files.

## 2. Apply mapping to codegen3 outputs

- [ ] 2.1 Route main and external test package writes and stale-output cleanup through the validated output mapping; verify generated outputs for both package kinds remain under the active root.
- [ ] 2.2 Align cache publication, manifests, optional emit paths, and overlay targets with the mapped package directory; verify the overlay resolves each generated file to the mapped path and cache hits remain usable.

## 3. Verify external dependency builds

- [ ] 3.1 Add an integration fixture importing a MyGO dependency from a read-only module-cache directory and verify codegen3 build/test succeeds without creating or modifying files in that dependency.
- [ ] 3.2 Run the focused codegen3 compiler tests and verify all generated package paths are contained by the active output root, including after cache hits and stale-output cleanup.
