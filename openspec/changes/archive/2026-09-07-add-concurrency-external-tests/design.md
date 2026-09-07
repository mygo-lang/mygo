## Context

See proposal.md for motivation. The relevant constraints:

- `lib/concurrency` exposes free functions (`MakeChan`, `MakeChanUnbuffered`,
  `AsSend`, `AsRecv`, `Spawn`) plus three interfaces (`IChannel`,
  `IReadableChan`, `IWritableChan`) implemented via `impl ... : Interface`
  blocks. Method calls such as `Len(ch)` are resolved at codegen time through
  typeclass dispatch (`matchTypeclassHelper`), so tests should call methods the
  way real users do, not the mangled `MygoIT...` symbols.
- The compiler supports two MyGO test-package styles:
  - **Internal**: a `*_test.mygo` file declaring the package's own name
    (e.g. `package parsec`), generated to `zz_<name>.gen_test.go` in the same
    Go package. This is what `prelude` and `lib/text/parsec` use.
  - **External**: a `*_test.mygo` file declaring `<package>_test` (e.g.
    `package concurrency_test`), auto-imported against the main package and
    generated to the same Go directory as a true external `_test` package.
    The `api.go` / `infer.go` / `generate.go` code paths for this style exist
    but have never been exercised by a real `lib/*` source.
- Go only compiles `_test.go` files under `go test`, so a malformed generated
  test file would not be caught by `go build ./...` — only by `go test`.

## Goals / Non-Goals

**Goals:**

- Provide a runnable, non-flaky test suite for the full public surface of
  `lib/concurrency` (constructors, wrappers, interface methods, `Spawn`).
- Validate the external `_test` package pipeline for a real library package.
- Keep tests in MyGO source (`.mygo`) so they are compiled by the same
  pipeline that compiles the library itself.

**Non-Goals:**

- No changes to `lib/concurrency` production code or to the compiler.
- No benchmarking, no stress/concurrency-safety testing, no unbuffered-
  channel timing tests.
- The `bootstrap` (`--bootstrap` / codegen2) pipeline is the **primary**
  generation path for this change (see D5). The default (legacy) pipeline is
  the fallback reference, not the target.

## Decisions

### D1 — Use an external test package (`package concurrency_test`)

The user explicitly asked for a `concurrency_test` external test. Beyond
matching the request, an external package is the stronger choice here: it
verifies that only *exported* symbols are reachable (the public API contract)
and it is the pipeline style with zero real-world coverage. The internal style
would be easier (no import wiring) but would not validate anything new.

### D2 — Drive every assertion through interface-dispatched calls

Test bodies call `Len(ch)`, `Receive(ch)`, `Send(ch, v)`, `Close(ch)`, etc.,
relying on typeclass dispatch rather than referencing mangled symbols or the
interfaces explicitly. This is how downstream users consume the package, so it
is the behavior that actually needs a guarantee.

### D3 — Only buffered channels + `close` for determinism

Every test uses `MakeChan[T](n)` with a buffer sized so the send cannot block,
and the "empty" / "closed" cases are produced by `close` rather than by racing
against an unsent value. The single `Spawn` test is the only cross-goroutine
hand-off: the spawned goroutine sends on a buffered channel and the test
receives once, which is a bounded block that cannot deadlock under the Go
memory model.

Alternatives considered: unbuffered channels (rejected — scheduling-
dependent) and `TryReceive`-only assertions for the empty case (kept for the
specific non-blocking semantics, but the closed case uses blocking `Receive`
so the `None`-on-close path is directly covered).

### D4 — Assert `Option` results with `switch` over `Some` / `None`

`Receive` / `TryReceive` return `Option[T]`. Tests destructure with the
standard `case Some(v) then ... / case None => t.Fatal(...)` pattern used by
the existing `parsec` and `prelude` tests, keeping the suite consistent and
avoiding a dependency on `Option` accessor methods that may not exist.

### D5 — Regenerate via the bootstrap pipeline (`--bootstrap`)

Per user direction, the new test files are generated with the **bootstrap**
pipeline (`./mygo --bootstrap build lib/concurrency`, i.e. parser2 /
typeinference2 / codegen2). This is the pipeline the project prioritizes going
forward, and the external `_test` typeclass-dispatch path in codegen2 is the
exact behavior this change is meant to validate end to end.

The bootstrap pipeline has been verified to reproduce `lib/concurrency`'s
existing `zz_channel.gen.go` / `zz_spawn.gen.go` byte-identically, so routing
the new test files through it introduces no churn in the non-test output. The
default (legacy) pipeline is kept only as a fallback reference in case the
bootstrap path exposes a compiler gap (see Risk 1).

Generated file naming follows the shared `sourceToGenName` convention, so
`channel_test.mygo` and `spawn_test.mygo` emit `zz_channel.gen_test.go` and
`zz_spawn.gen_test.go` (note: no `_test` segment in the stem — it is stripped,
matching the `prelude/zz_option.gen_test.go` convention).

## Risks / Trade-offs

- [External-test typeclass dispatch is unproven] The `infer.go` auto-import
  registers the main package's exported *functions* and *types* into the test
  package environment, but it is unverified that impl/typeclass method dispatch
  (`matchTypeclassHelper`) resolves the same mangled helpers from the
  test package. If it does not, this is a **compiler** gap (per project
  convention, diagnose and report before any compiler edit — it would become a
  separate change) rather than a test bug. With D5 now targeting the bootstrap
  pipeline, the unproven path is specifically codegen2's external-`_test`
  typeclass dispatch (`InferPackageWithExternalInfo` + codegen2 dictionary
  lowering). Mitigation: write the suite first, run it through `--bootstrap`,
  and triage any failure as test vs compiler before proceeding.
- [`_test.go` not covered by `go build`] A broken generated test file would not
  surface in `go build ./...`. Mitigation: acceptance explicitly runs `go test
  ./lib/concurrency` and `go test ./...`.
- [Subtle flakiness in the `Spawn` test] Mitigation: buffered channel + single
  send + single blocking receive; no timers, no `time.Sleep`.
- [Two source files vs one] `channel_test.mygo` and `spawn_test.mygo` are kept
  separate to mirror `channel.mygo` / `spawn.mygo` and to keep per-file
  generation mapping clean (`zz_channel_test.gen.go`, `zz_spawn_test.gen.go`).

## Migration Plan

Additive only. Steps: (1) add the two `*_test.mygo` sources, (2) run the
default `mygo` sync/build for `lib/concurrency` to emit the two generated test
files, (3) run `go test ./lib/concurrency` then `go test ./...`. Rollback is
deleting the four added files. No data, migration, or deploy steps.

## Open Questions

- **Resolved (bootstrap pipeline).** Running the suite through
  `./mygo --bootstrap build lib/concurrency` confirmed the external `_test`
  typeclass-dispatch path is known-good in codegen2: the generated
  `zz_channel.gen_test.go` / `zz_spawn.gen_test.go` reference the main
  package's mangled helpers (e.g.
  `MygoIT8IChannelFN4ChanGN1TEGN4ChanGN1TEN1TEM3Len`,
  `MygoIT13IWritableChanFN4ChanGN1TEGN4ChanGN1TEN1TEM4Send`) directly via the
  auto-generated `.` import of `github.com/mygo-lang/mygo/lib/concurrency`,
  and `Option` variant patterns lower to `Option__Some[any]` / `Option__None[any]`
  type-assertions via the `.` import of `prelude`. `go test ./lib/concurrency`
  passes all 7 tests (exit 0), including under `-race` for the `Spawn` test, and
  `go test ./...` shows no regression. No compiler gap was found on this path.
