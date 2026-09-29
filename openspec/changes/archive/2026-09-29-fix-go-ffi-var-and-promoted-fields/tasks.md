## 1. Package-level `var` in the bootstrap FFI collector

- [x] 1.1 In `bootstrapGoPackageInfoFromTypes` (`internal/mygo/compiler/go_ffi_import.go`), extend the third pass to match `*types.Var` alongside `*types.Const` and emit it as a `GoConstSignature` with `typeString(v.Type())`; verify `go build ./...` succeeds and an exported `var` still renders identically to a `const` of the same type
- [x] 1.2 Add a collector test asserting an exported package-level `var` (for example a GORM-style `ErrRecordNotFound error`) appears in the returned `Constants` with its declared type, and that an unexported `var` does not; verify the test passes

## 2. Package-level `var` in the hand-written loader (parity)

- [x] 2.1 In `loadGoPackageInfo` (`internal/mygo/typeinference/go_imports.go`), surface exported `*types.Var` objects into `info.Constants` the same way constants are surfaced; verify `go test ./internal/mygo/typeinference/...` passes
- [x] 2.2 Confirm both loaders agree on a package exporting both a `const` and a `var`: run the same fixture through the bootstrap and hand-written paths and verify neither rejects a selector the other accepts

## 3. Promoted-field resolution for MyGO structs with `embed` (superseded one-level base)

- [x] 3.1 Extend `structSymbolsInEnvAt` (`internal/mygo/typeinference2/env.mygo`) to detect an `embed` field (legacy AST stores the literal `"embed"` in the field-name slot) whose type resolves to an imported Go named struct, and register a `StructField` on the embedding MyGO type for each exported field of that Go type, reusing the `GoTypeSignature` field table already collected by the loader; verify with a targeted test that `u.ID` on a `User` embedding `gorm.Model` resolves at the embedded field's type instead of failing with `unknown field User.ID`
- [x] 3.2 Ensure own declared fields win over promoted fields of the same name (Go's outer-declaration rule) by controlling registration order in `structSymbolsInEnvAt`; verify with a regression test where the MyGO struct declares a field whose name collides with a promoted field
- [x] 3.3 Verify a bare imported Go struct value's own fields still resolve (e.g. `m.ID` on a `gorm.Model`-typed binding), confirming the change did not regress the pre-existing path; verify via the typeinference2 test suite

## 4. Regenerate and end-to-end validation

- [x] 4.1 Sync the `typeinference2` package with the existing `./mygo` binary first (`GOCACHE=<writable> ./mygo --bootstrap sync internal/mygo/typeinference2`), then build consumers with `go run ./cmd/mygo`; verify the regenerated `.gen.go` files are consistent and `go build ./...` is clean
- [x] 4.2 Run the real GORM fixture end to end: compile a program with `import gorm "go:gorm.io/gorm"` that selects `gorm.ErrRecordNotFound` and reads `u.ID` off a `struct User` containing `embed gorm.Model`, then confirm `go vet`/`go build` on the generated Go succeeds and the generated Go contains a direct `gorm.ErrRecordNotFound` reference and a direct `u.ID` selector
- [x] 4.3 Run the full `bootstrap-ffi-boundary` regression suite plus the typeinference2 and compiler test packages; verify all previously passing scenarios still pass unchanged
- [x] 4.4 Record the single-level promotion limitation in `KNOWN_ISSUES.md` (or the FFI docs) so deeper embedding is a known, discoverable gap rather than a silent failure; verify the note is present and accurate (removed in 5.6 now that multi-level promotion lands)

## 5. Go promotion semantics (transitive, fields + methods, depth-resolved)

- [x] 5.1 Record embedded identity on the Go FFI surface: add an embedded-type list to `GoTypeSignature`, populate it from `f.Anonymous()` in both `bootstrapGoPackageInfoFromTypes` (`internal/mygo/compiler/go_ffi_import.go`) and the hand-written loader (`internal/mygo/typeinference/go_imports.go`), and update every `GoTypeSignature` literal construction site; verify both pipelines still build and `go test ./internal/...` passes
- [x] 5.2 Build the selector-time promotion resolver in `internal/mygo/typeinference2`: on a `findSymbol` miss in `inferOrdinaryField`, walk the receiver's embedded types breadth-first, collecting candidate members annotated with `(depth, originType)`, guarded by a visited set keyed by type name; verify a self-referential embedding terminates rather than looping
- [x] 5.3 Implement Go's resolution rules in the resolver: declared members win first, then minimum depth, then a unique origin wins, then report an ambiguous selector naming the member; verify own-declaration-wins, shallowest-wins, and the ambiguous case are each covered by a regression test
- [x] 5.4 Cover both access paths: register the embedded field under the embedded type's own name so `b.A` / `u.Model` resolve (recovering the type name from the field's type expression, since the legacy AST stores the literal `"embed"` marker in the name slot) and `b.A.F1` / `u.Model.F1` type-check; verify the qualified path is never subject to the ambiguity check
- [x] 5.5 Cover both embedding sides and both member kinds: transitive promotion for MyGO-embed-MyGO, Go-embed-Go, and mixed chains in either order, plus promoted methods on a MyGO receiver; verify the bare imported Go struct field and method paths still resolve unchanged
- [x] 5.6 Remove the now-obsolete single-level promotion note from `KNOWN_ISSUES.md`, and update `docs/compiler/ffi.md` with the supported promotion semantics (transitive, fields and methods, shallowest-wins, ambiguous-selector error, qualified path as disambiguation); verify the docs match the implemented behaviour
- [x] 5.7 Remove the superseded one-level flattening in `internal/mygo/typeinference2/env.mygo` (`withPromotedFields`, `embeddedTConName`, `qualifiedTConName`, `promoteGoTypeFields`) now that 5.2-5.4 replace it, keeping only the embedded-field-under-its-own-name registration; verify the full test suite passes and no dead helper remains
- [x] 5.8 Run end-to-end validation: sync `typeinference2`, then re-run the GORM fixture plus a multi-level fixture through both pipelines, and run the full `bootstrap-ffi-boundary` regression suite, the typeinference2 tests, and the compiler tests
