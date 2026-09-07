## Why

`lib/concurrency` currently has no tests: `go test ./lib/concurrency` reports
`[no test files]`. The package's public surface (channel constructors, channel
wrappers, and the `IChannel` / `IReadableChan` / `IWritableChan` interfaces)
therefore has no executable guarantee that it builds and behaves as documented
in `docs/compiler/concurrency.md`. Adding tests also exercises the external
`_test` package pipeline (`<name>_test`), which the compiler supports but no
`lib/*` package has actually used yet.

## What Changes

- Add `lib/concurrency/channel_test.mygo` declaring `package concurrency_test`
  (an external test package) covering:
  - `MakeChan[T]` / `MakeChanUnbuffered[T]` construction and `Len` / `Cap`.
  - `AsSend[T]` / `AsRecv[T]` directional wrappers.
  - Send/receive round-trips and the `Option` results of `Receive` /
    `TryReceive` (including the closed-channel `None` case).
  - Interface-driven calls (`Len`, `Cap`, `Receive`, `Send`, `Close`) resolved
    through typeclass dispatch.
- Add `lib/concurrency/spawn_test.mygo` covering `Spawn` running a goroutine
  that sends to a channel, observed by the test via a receive.
- Regenerate the two test files through the **bootstrap** compiler pipeline
  (`./mygo --bootstrap build lib/concurrency`), emitting `zz_channel.gen_test.go`
  and `zz_spawn.gen_test.go` per the shared `sourceToGenName` convention.
- No production code in `lib/concurrency` changes.

## Capabilities

### New Capabilities

None. This change adds tests only and does not introduce new behavior, so it
sets `skip_specs: true` (see `.openspec.yaml`) rather than defining a new
capability.

### Modified Capabilities

None. No spec-level behavior of `lib/concurrency` or the compiler changes.

## Impact

- New files: `lib/concurrency/channel_test.mygo`,
  `lib/concurrency/spawn_test.mygo`, `lib/concurrency/zz_channel.gen_test.go`,
  `lib/concurrency/zz_spawn.gen_test.go`.
- Acceptance: `go test ./lib/concurrency` passes and `go test ./...` shows no
  regression. The external `_test` package path is thereby validated end to
  end for a real library package through the bootstrap (codegen2) pipeline.
