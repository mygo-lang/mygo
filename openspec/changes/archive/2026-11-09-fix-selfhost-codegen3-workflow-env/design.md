## Context

See proposal.md for motivation. The workflow already exists and is committed;
only its expression scope is wrong. Two constraints shape the fix:

- The codegen3 cache contract in `bootstrap-codegen3` lets `MYGO_CACHE` select the
  cache root and defaults it to the operating system user cache directory. The
  root must stay outside the checkout.
- GitHub evaluates job-level `env` before a runner is assigned, so `runner.temp`
  is unavailable there. Inside a step, `${{ runner.temp }}` and `$RUNNER_TEMP`
  resolve to the same directory.

## Goals / Non-Goals

**Goals:**

- Make the workflow definition parse and run on its existing triggers.
- Preserve the original intent of placing the codegen3 cache under the runner
  temporary directory, outside the checkout.
- Keep the snapshot and comparison steps correct when no generated files exist.

**Non-Goals:**

- Changing which branches trigger the workflow, or deduplicating the push and
  pull request runs.
- Persisting the codegen3 cache across jobs with a cache action.
- Changing compiler behavior, `MYGO_CACHE` resolution, the Go version source,
  the timeout, or the permissions block.

## Decisions

**Declare `MYGO_CACHE` on the two codegen3 steps rather than deleting it.**
Deleting the line would also yield a valid workflow, because the default root
under the runner's home directory is outside the checkout. Keeping an explicit
step-scoped value preserves the intent that codegen3 output belongs in the
runner temporary area next to the binaries, and it keeps the value visible to a
reader of the step. The alternative of using `github.workspace` in the job-level
block is rejected: it would put the cache inside the checkout.

**Attach it only to the compiler-build and test steps.** The bootstrap build
compiles Go and does not consult `MYGO_CACHE`; the snapshot and comparison steps
only hash tracked files.

**Use `xargs -0 -r` for the snapshots.** GNU `xargs` is what `ubuntu-latest`
ships, so `-r` is available and prevents `sha256sum` from reading stdin when the
`find` pipeline produces nothing.

## Risks / Trade-offs

- The workflow has never run successfully, so the first valid run may expose
  unrelated codegen3 failures. Mitigation: treat those as separate findings
  rather than reworking this change; the workflow already reports the failing
  step.
- A duplicated run for branch pushes and their pull requests costs runner time.
  Mitigation: leave the trigger policy as it is and decide separately whether to
  narrow it.
