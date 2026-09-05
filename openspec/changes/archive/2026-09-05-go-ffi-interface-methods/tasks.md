## 1. Interface method collection

- [x] 1.1 In `goTypeMethods` (`internal/mygo/compiler/go_ffi_import.go`), branch
      on `named.Underlying()`: use `types.NewMethodSet(named)` when it is a
      `*types.Interface`, else keep `types.NewMethodSet(types.NewPointer(named))`; verify
      a throwaway probe reports `net/http.ResponseWriter` methods `Header`,
      `Write`, `WriteHeader` (not zero)
- [x] 1.2 Confirm the exported-alias pass surfaces interface methods without a
      separate alias path by loading a package that exports an alias to an
      interface and asserting the alias's `Methods` are non-empty (verification via
      the loader test below)

## 2. Loader tests

- [x] 2.1 Add a `TestBootstrapGoPackageInfoCollectsInterfaceMethods` assertion in
      `internal/mygo/compiler/go_ffi_import_test.go` that loads `net/http` and
      verifies `ResponseWriter`'s `Methods` contain `Write`, `WriteHeader`, and
      `Header`, and that `Fields` remains empty; verify `go test ./internal/mygo/compiler -run Interface`
- [x] 2.2 Add an alias-exposure test that loads a package exporting a type alias
      whose target is an interface and asserts the alias's `Methods` include the
      target's methods; verify the new test passes

## 3. Inference tests

- [x] 3.1 Add a `typeinference2` test driving `GoMethodSignatureInPackages` with an
      interface-typed receiver (e.g. `http.ResponseWriter` value) and asserting the
      resolved `Write` signature (params `[]byte`, result `(int, error)`) is found;
      verify `go test ./internal/mygo/typeinference2 -run GoMethod`
- [x] 3.2 Add a fixture/assertion that a `(T, error)` interface method resolves to
      the `Result`-wrapping boundary path (matching existing `client.Do` behavior);
      verify the new inference test passes

## 4. Codegen / end-to-end test

- [x] 4.1 Add a compile-and-lower test with a `.mygo` handler taking
      `w: http.ResponseWriter` that evaluates `w.Header()`, `w.WriteHeader(200)`,
      and `w.Write(bytes)`; assert compilation succeeds and the generated Go
      contains direct `w.Header()`, `w.WriteHeader(...)`, `w.Write(...)` calls
      (no `unknown field` error); verify the test passes
- [x] 4.2 Run the full suite and verify nothing regresses:
      `go test ./internal/mygo/...`
