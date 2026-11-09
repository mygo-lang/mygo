## Why

`.github/workflows/selfhost-codegen3.yml` is rejected by GitHub Actions as an
invalid workflow file. Line 17 sets `MYGO_CACHE: ${{ runner.temp }}/mygo-codegen3-cache`
in the job-level `env` block, but the `runner` context has no value when
job-level environment variables are evaluated, so the expression is reported as
an unrecognized named value. Because the definition never parses, the codegen3
self-host build, full test suite, and repository-cleanliness checks have never
run in CI.

## What Changes

- Stop referencing the `runner` context from the job-level `env` block. Keep
  `GOPROXY` at job scope and declare `MYGO_CACHE` in the two steps that
  actually invoke the codegen3 compiler, still resolving to
  `${{ runner.temp }}/mygo-codegen3-cache`. The cache root therefore stays
  outside the checkout, as the codegen3 cache contract requires.
- Make the generated-source snapshots tolerate an empty result set by passing
  `-r` to `xargs`, so the absence of `*.gen.go` files cannot make
  `sha256sum` wait on stdin.
- Leave the trigger rules, Go toolchain selection, timeout, permissions, build
  order, and verification commands otherwise unchanged.

## Capabilities

### New Capabilities

- `selfhost-ci`: the CI contract for the self-hosted codegen3 workflow, covering
  workflow validity, cache placement outside the checkout, and the end-to-end
  build, test, and generated-source verification sequence.

### Modified Capabilities

None. The compiler behavior described by `bootstrap-codegen3` and
`bootstrap-ir3-pipeline` is unchanged; this change makes the existing
verification workflow executable.

## Impact

- `.github/workflows/selfhost-codegen3.yml` is the only project file expected to
  change.
- No compiler, CLI, LSP, or runtime behavior changes.
- `MYGO_CACHE` semantics are unchanged: an explicit root still selects the cache
  location, and the default remains the operating system user cache directory.
- The workflow starts running on every branch push and pull request after the
  fix, which may surface pre-existing codegen3 failures that were previously
  hidden by the invalid definition.
