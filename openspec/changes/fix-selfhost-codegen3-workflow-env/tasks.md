## 1. Fix the job-scoped environment

- [ ] 1.1 Remove the `MYGO_CACHE` entry from the job-level `env` block, leaving
  `GOPROXY` in place, and verify `.github/workflows/selfhost-codegen3.yml` no
  longer contains a `runner` context expression between `env:` and `steps:`
- [ ] 1.2 Add `MYGO_CACHE: ${{ runner.temp }}/mygo-codegen3-cache` to the
  `Build the self-hosted codegen3 compiler` step and verify the step still
  writes both binaries under `$RUNNER_TEMP`
- [ ] 1.3 Add the same `MYGO_CACHE` entry to the `Run the self-hosted compiler
  test suite` step and verify `rg -n 'mygo-codegen3-cache'
  .github/workflows/selfhost-codegen3.yml` reports exactly those two
  step-level declarations

## 2. Make the generated-source snapshots empty-set safe

- [ ] 2.1 Pass `-r` to both `xargs` invocations in the snapshot and comparison
  steps, and verify an empty match set completes locally, for example against an
  empty directory, with a zero exit status and an empty output file
- [ ] 2.2 Re-read both `find` pipelines and verify they still prune `.git`,
  sort with `-z`, and hash the same file set as before

## 3. Validate the workflow definition

- [ ] 3.1 Verify the file parses as YAML and that every `${{ runner.* }}`
  expression now sits inside a step, using
  `rg -n 'runner\.|^    env:' .github/workflows/selfhost-codegen3.yml` and
  reading the surrounding blocks
- [ ] 3.2 Confirm the trigger, permissions, timeout, checkout, Go setup, and
  verification commands are unchanged by diffing the file against `HEAD`
- [ ] 3.3 Push the branch and verify GitHub accepts the definition, queues the
  `self-host` job, and reaches the codegen3 build step; record any pre-existing
  codegen3 failure as a separate follow-up rather than reopening this change
