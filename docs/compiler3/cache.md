# codegen3 build cache and `test` command

`codegen3` never writes generated Go beside `.mygo` sources and no longer uses a
project-local `build/.mygo3` directory. Generated files live in a user-level
content-addressed cache; each build consumes them through a temporary Go
`-overlay`.

## Commands

```sh
mygo --codegen3 build -o app ./cmd/mygo   # binary at ./app
mygo --codegen3 test -run TestX ./pkg     # go test against cached sources
mygo cache clean                          # remove entries and temp overlays
mygo clean -cache                         # Go-compatible spelling
mygo --codegen3 --codegen3-emit=/tmp/gen build ./pkg  # debug/IDE copy
```

`build` and `test` accept the same package patterns and flags as `go build`
and `go test`; the first directory argument selects the working root. Test
failures propagate the toolchain exit status. `test` also generates external
`_test` packages.

## Cache root

- `MYGO_CACHE` selects the root. Unset or empty means the operating system's
  user cache directory plus `mygo` (for example `~/.cache/mygo`).
- `MYGO_CACHE=off` (also `0`/`false`) disables caching; builds still work.
- Deleting the cache only costs compilation time. It never changes output.

## Entries

```text
$MYGO_CACHE/
  build/<xx>/<key>-d/
    manifest.json
    zz_*.gen.go
    zz_*.gen_test.go
    zz_mygo_metadata.gen.go
  summaries/<xx>/<source-fingerprint>.json
  tmp/                 # per-build staging and the temporary overlay
  lock                 # shared/exclusive build lock
```

An entry is immutable. `manifest.json` records the cache format, key, compiler
identity, representation/adapter version, optimization level, and each file's
size and SHA-256. Readers verify the manifest and every file; incomplete or
corrupt entries are ignored, removed, and regenerated.

Entries are published by writing a temporary directory and renaming it into
place. Concurrent writers of the same key converge on one complete entry; the
loser discards its temporary copy.

`zz_mygo_metadata.gen.go` is a metadata-only Go file stored in the cache. It
lets the compiler recover a package's importer-facing declarations from a
verified hit without reparsing its sources. It is not written beside the
`.mygo` sources. `--codegen3-emit` includes it with the other generated files
for IDE/debug materialization.

## Key

The key is a SHA-256 over, in a fixed order:

1. cache format version;
2. compiler binary identity (executable hash, Go version, GOOS/GOARCH);
3. representation version and adapter ABI;
4. optimization level and backend options;
5. the package's `(relative source path, content digest)` pairs for `.mygo`
   sources and non-generated hand-written Go files;
6. dependency keys, each paired with its MyGO import path (Merkle-style, so a
   dependency change invalidates importers);
7. `go.mod`/`go.sum` digests (the `module` line is ignored) and the Go
   toolchain version, target/build environment (`GOOS`, `GOARCH`, `GOAMD64`,
   `GOARM`, `CGO_ENABLED`, `GOFLAGS`, `GOEXPERIMENT`, `GOROOT`, and
   `GOTOOLCHAIN`).
8. path-relative hashes of hand-written Go/native sources elsewhere in the
   main module and of local `replace` targets.

Absolute module roots, checkout locations, timestamps, and user names never
enter the key. Two modules with identical sources, compiler, options, and
dependencies share entries even when their module paths differ.

## Summary inputs and early hits

Before parsing, the compiler computes a cheap source fingerprint from the
package identity, source digest, compiler identity, Go input digest, backend
options, and summary format. The matching summary records the full dependency
input keys and the generated-output key.

A summary is reusable only when all of these still match:

1. `.mygo` sources and non-generated Go files in the package directory;
2. direct MyGO dependencies, recursively validated by their own summaries;
3. the canonical prelude, unless `--no-prelude` is active;
4. build/test source selection, including external `_test` packages;
5. hand-written Go/native files in the main module and local `replace` targets;
6. `go.mod`, `go.sum`, compiler identity, representation/adapter version;
7. optimization level and Go target/build environment (`GOOS`, `GOARCH`,
   `GOFLAGS`, `GOEXPERIMENT`, cgo, `GOROOT`, and toolchain selection).

The summary's checksum covers all recorded fields. Missing, truncated, corrupt,
incompatible, or stale summaries are ordinary misses and use the normal
compilation path. A successful fallback repairs the summary for the next warm
build.

On a verified early hit, the compiler loads exported declarations from the
cached metadata, reuses generated files, and skips full source parsing, Go FFI
signature collection, type inference, IR lowering, optimization, and code
generation for that package. The cache-on result is covered against cache-off
behavior for changed local and transitive MyGO sources, public APIs, Go inputs,
test sources, options, and corrupt records.

## Overlay

Each build writes `<cache>/tmp/<build>/overlay.json` mapping canonical source
paths (for example `prelude/zz_option.gen.go`) to cache entry files. The file is
deleted when the build ends; nothing is written under the project except the
Go toolchain's own `go.mod`/`go.sum` updates.

`--codegen3-emit=<dir>` copies the generated files for the current build into
`<dir>/<package-relative-path>` for debugging or IDE indexing. Ordinary builds
and tests never materialize generated sources.

## Locking and cleaning

Builds hold a shared lock on `<cache>/lock`; `mygo cache clean` takes the
exclusive lock, waits briefly for active builds, and then removes every entry
and temporary overlay. If the lock cannot be acquired it exits non-zero without
deleting anything. With `MYGO_CACHE=off`, cleaning is a successful no-op.

## Tests

The compiler suite covers early-hit stage counters, cold/warm determinism,
corrupt summary fallback and repair, transitive invalidation, local Go input and
`replace` invalidation, cross-module hits, `MYGO_CACHE=off`,
`--codegen3-emit`, cleaning, and the shared/exclusive lock interaction in
`internal/mygo/compiler/codegen3_cache_test.mygo`.
