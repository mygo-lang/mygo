## 1. CLI selection and dispatch

- [x] 1.1 Parse explicit pipeline selectors and reject legacy/bootstrap conflicts in either order and legacy commands other than sync before side effects; verify focused CLI cases return usage exit status 2 and leave fixtures unchanged.
- [x] 1.2 Route plain sync and build to bootstrap, preserve explicit bootstrap compatibility and existing no-Prelude/timing precedence, and retain explicit legacy sync dispatch; verify isolated generation fixtures identify the intended pipeline and default build produces a Go artifact.
- [x] 1.3 Preserve generated-path reporting ownership and stop on bootstrap errors without fallback or Go build; verify paths are not duplicated and failing generation does not invoke the build subprocess.

## 2. Help and migration documentation

- [x] 2.1 Update CLI usage to describe the bootstrap default, bootstrap compatibility alias, and sync-only legacy selector; verify captured usage contains all three behaviors.
- [x] 2.2 Update README and compiler workflow documentation, including conflicting default claims discovered by a targeted search; verify documented default commands and the legacy sync followed by Go build recovery workflow match the delta spec.

## 3. Focused acceptance verification

- [x] 3.1 Add or extend CLI regression coverage for default sync/build, explicit bootstrap, legacy sync, both no-Prelude paths, timing, invalid selectors, and generation failure using temporary packages; run targeted `go test ./cmd/mygo` with a writable Go cache and verify all cases pass without repository-wide regeneration.
- [x] 3.2 Validate the completed change with `openspec validate switch-default-pipeline-to-bootstrap --strict` and inspect the final diff; verify only intended CLI, documentation, tests, and change artifacts are included and compiler API semantics remain unchanged.
