## 1. Dependency and FFI Surface

- [ ] 1.1 Add `github.com/gofrs/flock` at a version compatible with the project's Go version; verify `go.mod` and `go.sum` contain the dependency and `go list -m github.com/gofrs/flock` resolves it.
- [ ] 1.2 Confirm Go FFI exposes `flock.New` plus the `TryLock`, `TryRLock`, `Path`, and `Unlock` methods with the expected `Result` boundary shapes; verify with a focused compiler test or FFI loading check.

## 2. MyGO Lock Implementation

- [ ] 2.1 Update `codegen3_cache.mygo` to import `go:github.com/gofrs/flock`, store `Option[Ref[flock.Flock]]`, and remove the `syscall` import and integer lock-mode path; verify the source still type-checks.
- [ ] 2.2 Implement bounded shared and exclusive acquisition with `TryRLock()` for builds and `TryLock()` for cleaning, preserving the 20 ms retry interval and busy diagnostic; verify the exclusive-lock wait test passes.
- [ ] 2.3 Release the lock through `flock.Unlock()` and clear the stored handle; verify repeated shared and exclusive lock acquisition succeeds in the focused cache tests.
- [ ] 2.4 Regenerate `zz_codegen3_cache.gen.go`; verify it imports `github.com/gofrs/flock`, contains no `syscall.Flock` or `LOCK_*` references, adds no platform-specific `.go` source files, and introduces no inline Go blocks.

## 3. Cross-Platform Verification

- [ ] 3.1 Run the focused compiler cache tests with a writable `GOCACHE`; verify all `TestCodegen3Cache*` tests pass.
- [ ] 3.2 Run the broader compiler test suite; verify cache correctness, concurrent builds, and repository-clean behavior remain unchanged.
- [ ] 3.3 Build `./cmd/mygo` for `windows/amd64`, `darwin/amd64`, and `darwin/arm64` with `CGO_ENABLED=0`; verify all three targets compile successfully.
- [ ] 3.4 Run the focused shared/exclusive lock tests on Windows and macOS runners where available; verify builds acquire compatible shared locks and cleaning cannot race an active build.
- [ ] 3.5 Run `openspec validate --strict fix-codegen3-cross-platform-cache-lock`; verify the change artifacts remain valid.
