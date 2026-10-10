## Why

Any project that transitively imports `lib/concurrency` currently fails `go build
./...`. The compiler emits dynamic API wrapper declarations into each package's
`zz_mygo_dynamic.gen.go`, and for the concurrency channel implementations it renders
the compiler-internal channel types as Go named types that do not exist:
`concurrency.SendChan[T]`, `concurrency.RecvChan[T]`, and `concurrency.Chan[T]`. Go
reports `undefined: concurrency.SendChan` (and the same for `RecvChan`/`Chan`) in every
package that imports concurrency, including packages that never use a dynamic
interface. `tutorial-video-generation` reproduces this in `internal/queue`,
`internal/httpapi`, and `cmd/server`.

The dynamic metadata path is a third type-rendering path, separate from the ordinary
code generator, and it never learned that `Chan`, `SendChan`, and `RecvChan` are
compiler-internal types that map to `chan T`, `chan<- T`, and `<-chan T`. That same
path also emits a dynamic constructor for every implementation in imported metadata
regardless of whether the implementation can be dispatched through, so the concurrency
wrappers are dead code that still has to compile.

## What Changes

- Render the compiler channel types to Go channel types in the dynamic Go API: the
  generated declarations SHALL use `chan T`, `chan<- T`, and `<-chan T` for `Chan[T]`,
  `SendChan[T]`, and `RecvChan[T]` instead of a qualified named type.
- Keep every consumer round-trip consistent: the code that reads the generated dynamic
  API back into MyGO declarations, and the code that reconstructs imported declarations
  from metadata, SHALL understand the channel types in both directions.
- Stop emitting a dynamic constructor for an implementation whose interface offers no
  dispatchable operation and that the compiling package does not reference, so a
  package with no usable dynamic API emits nothing.
- Do not change the metadata protocol: the producer keeps encoding the channel types as
  it does today, and `CompilerABI`/`AdapterABI` stay at their current versions.
- Consumer-only rendering is chosen over adding a producer channel descriptor. See
  design.md for the rejected alternative and its consequences.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `language/dynamic-typeclass-values`: the dynamic Go API must render compiler channel
  types as Go channel types, and must emit dynamic constructors only for implementations
  that are usable or explicitly referenced.

## Impact

- `internal/mygo/codegen2/dynamic_metadata.go` - renders the dynamic Go API and drives
  the emission loop
- `internal/mygo/codegen2/dynamic_api_decls.go` - converts the generated dynamic Go API
  back into MyGO declarations for inference
- `internal/mygo/metadata/ast.go` - reconstructs MyGO declarations from package metadata
- `lib/concurrency` and every project importing it (regenerated
  `zz_mygo_dynamic.gen.go`)
- `tutorial-video-generation` `go build ./...` is the end-to-end regression target
