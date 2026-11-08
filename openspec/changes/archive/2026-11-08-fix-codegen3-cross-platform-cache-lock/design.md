## Context

See `proposal.md` for the motivation. The current lock implementation in
`internal/mygo/compiler/codegen3_cache.mygo` imports Go's `syscall` package and
uses `Flock`, `LOCK_EX`, `LOCK_SH`, `LOCK_NB`, and `LOCK_UN`. Those symbols are
available on the Unix targets used here but not on Windows, so the generated
`zz_codegen3_cache.gen.go` cannot compile for `windows/amd64`.

The implementation cannot use platform-specific `.go` files or build
constraints because the project requires authored compiler code to remain in
`.mygo`. Inline Go is also excluded. The selected solution therefore has to
provide portability behind a Go FFI boundary.

## Goals / Non-Goals

**Goals:**

- Restore compilation for Windows and macOS release targets.
- Preserve shared locks for builds and exclusive locks for cache cleaning.
- Preserve the existing bounded retry and busy diagnostic.
- Keep all compiler-authored logic in `.mygo` and use Go FFI only.
- Reuse the existing cache key, manifest, and repository-clean guarantees.

**Non-Goals:**

- Redesigning the cache layout or cache-key algorithm.
- Relaxing cache integrity or concurrent-build guarantees.
- Introducing platform-specific build tags or handwritten Go helper files.
- Supporting targets outside the project's release platform matrix.

## Decisions

### Use `github.com/gofrs/flock` through Go FFI

Import `github.com/gofrs/flock` as `go:github.com/gofrs/flock` from
`codegen3_cache.mygo`. The library encapsulates the platform-specific
implementation and exposes `New`, `TryLock`, `TryRLock`, `Path`, and `Unlock`.
It uses `LockFileEx` on Windows and Unix file locking elsewhere.

This avoids a conditional implementation in generated Go and keeps all
authored logic in `.mygo`. Adding `.go` platform wrappers would violate the
project constraint, while importing and branching on `syscall` symbols cannot
compile on Windows even when the runtime branch is not taken.

### Keep the bounded retry loop in MyGO

Use `TryLock()` for exclusive cleaning and `TryRLock()` for shared builds.
Both calls lower to `Result[Bool, Error]`. The existing MyGO loop remains
responsible for the 20 ms retry interval, deadline, and
`codegen3 cache is busy` diagnostic. This preserves current timeout behavior
without introducing context construction or additional inline Go.

### Store the lock object instead of a file handle and mode

Replace `Option[Ref[os.File]]` with `Option[Ref[flock.Flock]]`. The lock object
owns the underlying file descriptor and remembers whether it is shared or
exclusive, so `Unlock()` can release the correct lock without a separate mode
integer.

### Treat the dependency as part of the compiler build

Add `github.com/gofrs/flock` to `go.mod` and `go.sum`. The compiler identity and
Go-input digest already participate in cache keys, so the dependency update
will not reuse entries produced by the incompatible syscall implementation.

## Risks / Trade-offs

**New external dependency** -> Pin a released version compatible with the
project's Go version and rely on its tested Windows and Unix implementations.

**Platform locking semantics differ** -> Use one lock mode per `Flock` object
and never convert a shared lock into an exclusive lock on the same object.

**Cross-compilation does not exercise runtime locking** -> Build all release
targets and run the focused lock tests on Linux, macOS, and Windows where
available.

**FFI loader may not expose all methods** -> Verify `TryLock`, `TryRLock`,
`Path`, and `Unlock` survive FFI signature collection before regenerating the
checked-in generated source.

## Migration Plan

Replace the syscall calls, regenerate
`internal/mygo/compiler/zz_codegen3_cache.gen.go`, update module files, then run
the focused cache tests and release-target builds. No cache format migration is
required; stale entries remain valid but will normally be bypassed because the
compiler and module inputs change. Rollback consists of restoring the previous
implementation and module files.
