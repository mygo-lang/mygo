## Why

The codegen3 cache currently calls Unix-only `syscall.Flock` APIs from
`codegen3_cache.mygo`. The release build therefore fails for Windows with
undefined `syscall.Flock` and `LOCK_*` symbols, preventing the supported
Windows artifact from being produced.

## What Changes

- Replace direct Unix syscall-based cache locking with a cross-platform Go FFI
  file-lock implementation.
- Keep build operations on a shared lock and cache cleaning on an exclusive
  lock, including the existing bounded wait and busy diagnostic.
- Keep all authored compiler logic in `.mygo`; do not add platform-specific Go
  source files and do not use inline Go for this change.
- Add a compatible Go FFI dependency for cross-platform file locking.
- Verify codegen3 builds for Windows and macOS targets and run the focused
  cache-lock tests on supported hosts.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen3`: Strengthen the build-cache concurrency requirement so
  the shared/exclusive cache lock behaves consistently on every supported
  release platform, including Windows, without changing cache correctness or
  the repository-clean contract.

## Impact

- `internal/mygo/compiler/codegen3_cache.mygo`
- Generated `internal/mygo/compiler/zz_codegen3_cache.gen.go`
- `go.mod` and `go.sum`
- Codegen3 cache lock and clean behavior on Windows, macOS, and Linux
- Release builds for `windows/amd64`, `darwin/amd64`, and `darwin/arm64`
