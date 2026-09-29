## 1. Package-level `var` in the bootstrap FFI collector

- [ ] 1.1 In `bootstrapGoPackageInfoFromTypes` (`internal/mygo/compiler/go_ffi_import.go`), extend the third pass to match `*types.Var` alongside `*types.Const` and emit it as a `GoConstSignature` with `typeString(v.Type())`; verify `go build ./...` succeeds and an exported `var` still renders identically to a `const` of the same type
- [ ] 1.2 Add a collector test asserting an exported package-level `var` (for example a GORM-style `ErrRecordNotFound error`) appears in the returned `Constants` with its declared type, and that an unexported `var` does not; verify the test passes

## 2. Package-level `var` in the hand-written loader (parity)

- [ ] 2.1 In `loadGoPackageInfo` (`internal/mygo/typeinference/go_imports.go`), surface exported `*types.Var` objects into `info.Constants` the same way constants are surfaced; verify `go test ./internal/mygo/typeinference/...` passes
- [ ] 2.2 Confirm both loaders agree on a package exporting both a `const` and a `var`: run the same fixture through the bootstrap and hand-written paths and verify neither rejects a selector the other accepts

## 3. Promoted-field resolution for MyGO structs with `embed`

- [ ] 3.1 Extend `structSymbolsInEnvAt` (`internal/mygo/typeinference2/env.mygo`) to detect an `embed` field (legacy AST stores the literal `"embed"` in the field-name slot) whose type resolves to an imported Go named struct, and register a `StructField` on the embedding MyGO type for each exported field of that Go type, reusing the `GoTypeSignature` field table already collected by the loader; verify with a targeted test that `u.ID` on a `User` embedding `gorm.Model` resolves at the embedded field's type instead of failing with `unknown field User.ID`
- [ ] 3.2 Ensure own declared fields win over promoted fields of the same name (Go's outer-declaration rule) by controlling registration order in `structSymbolsInEnvAt`; verify with a regression test where the MyGO struct declares a field whose name collides with a promoted field
- [ ] 3.3 Verify a bare imported Go struct value's own fields still resolve (e.g. `m.ID` on a `gorm.Model`-typed binding), confirming the change did not regress the pre-existing path; verify via the typeinference2 test suite

## 4. Regenerate and end-to-end validation

- [ ] 4.1 Sync the `typeinference2` package with the existing `./mygo` binary first (`GOCACHE=<writable> ./mygo --bootstrap sync internal/mygo/typeinference2`), then build consumers with `go run ./cmd/mygo`; verify the regenerated `.gen.go` files are consistent and `go build ./...` is clean
- [ ] 4.2 Run the real GORM fixture end to end: compile a program with `import gorm "go:gorm.io/gorm"` that selects `gorm.ErrRecordNotFound` and reads `u.ID` off a `struct User` containing `embed gorm.Model`, then confirm `go vet`/`go build` on the generated Go succeeds and the generated Go contains a direct `gorm.ErrRecordNotFound` reference and a direct `u.ID` selector
- [ ] 4.3 Run the full `bootstrap-ffi-boundary` regression suite plus the typeinference2 and compiler test packages; verify all previously passing scenarios still pass unchanged
- [ ] 4.4 Record the single-level promotion limitation in `KNOWN_ISSUES.md` (or the FFI docs) so deeper embedding is a known, discoverable gap rather than a silent failure; verify the note is present and accurate
