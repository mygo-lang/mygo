## 1. Type inference

- [x] 1.1 Add the array-to-slice branch to `inferTypeAs` so `Int[32] as Slice[Int]` infers as `Slice[Int]`; verify with a bootstrap test that the binding type is `Slice[Int]`
- [x] 1.2 Keep the array-aware diagnostic for the rejected directions; verify `Int[32] as Slice[String]` and `Slice[Int] as Int[32]` still fail with source-located messages

## 2. Code generation

- [x] 2.1 Add a slice-expression builder to `internal/mygo/codegen2/goast/goast.go` and emit `value[:]` from `translateTypeAsAst` for array-to-slice ascriptions; verify the generated Go contains `[:]`
- [x] 2.2 Lower non-addressable operands through a copy-then-slice IIFE; verify `f() as Slice[Int]` compiles and runs

## 3. Bootstrap sync

- [x] 3.1 Re-sync `typeinference2`, `codegen2`, then `compiler` in that order with a writable `GOCACHE`; verify the CLI rebuilds
- [x] 3.2 Verify the prelude and the rest of the repository still sync and compile after the change (`go test ./...`)

## 4. Tests and docs

- [x] 4.1 Add end-to-end bootstrap test cases covering variable, nested, call-result, and return-position ascriptions; verify they pass
- [x] 4.2 Update `docs/compiler/collections.md` with the ascription and verify the documented example compiles

## 5. Validation and archive

- [x] 5.1 Run `openspec validate allow-array-to-slice-ascription --strict` and the compiler test suite; verify both pass
- [x] 5.2 Sync the delta into `openspec/specs/bootstrap-go-arrays/spec.md`, archive the change, and verify `openspec list --json` shows no active change

