## 1. Production compiler commit

- [ ] 1.1 On the old production-compiler branch, trace qualified named-variant literal resolution and make it retain enum ownership instead of consulting a colliding bare variant symbol; verify the focused production inference/compiler test passes.
- [ ] 1.2 Add production regression fixtures for a named `AgentState.Running { Loop: ... }` and a zero-field `ToolExecutionStatus.Running` in one package, plus distinct same-name named payload fields; verify compilation and generated Go validity where the old branch supports it.
- [ ] 1.3 Commit only production compiler sources and production tests as `fix(typeinference): resolve qualified enum variant name collisions`; verify `git show --stat HEAD` contains no bootstrap source or generated bootstrap files, so the commit can be cherry-picked independently.

## 2. Bootstrap compiler commit

- [ ] 2.1 Update bootstrap enum constructor registration and qualified named-variant literal inference so `EnumName.VariantName` resolves without falling back to a colliding bare variant; verify the focused `typeinference2` regression test passes.
- [ ] 2.2 Regenerate every affected checked-in bootstrap `zz_*.gen.go` artifact using the project workflow; verify generated files match the MyGO source change and the bootstrap compiler remains self-hostable.
- [ ] 2.3 Add bootstrap compiler coverage for the same collision matrix and verify `go test ./internal/mygo/typeinference2 ./internal/mygo/compiler` plus `./mygo --bootstrap build /media/btr/data/xyh/code/go/mygo-projects/qlm-harness/internal/agent` pass.
- [ ] 2.4 Commit only bootstrap inference/codegen sources, their regenerated artifacts, and bootstrap tests as `fix(typeinference2): resolve qualified enum variant name collisions`; verify the commit has no production-compiler files intended for the old branch.

## 3. Integration verification

- [ ] 3.1 Verify both commits independently on their target branches, then verify the current branch with both compiler paths accepts the collision fixture and ordinary named-variant construction; record the exact commands and results in the change review.
