## 1. Anonymous type identity registry

- [ ] 1.1 Add a Go-side registry for anonymous FFI types that stores the actual `types.Type`, a deterministic full-path key, and a human-readable Go spelling; verify with a Go unit test that equivalent types share a key and structurally different types do not.
- [ ] 1.2 Extend the FFI loader to register anonymous types found in parameters, results, fields, embeds, method signatures, and nested composites before signatures are consumed; verify with a loader test using a fixture package that returns an anonymous interface.
- [ ] 1.3 Encode anonymous identities in a reserved internal namespace that cannot collide with named package types, while leaving `interface{}` mapped to `Any`; verify the parser never treats the namespace as a package name.

## 2. Signature parsing and type identity

- [ ] 2.1 Add early recognition for balanced `interface{...}` and `struct{...}` forms before package-qualified-name splitting; verify `GoTypeNameWithPackage` accepts a non-empty anonymous interface and anonymous struct without producing a fabricated `go:interface...` path.
- [ ] 2.2 Make repeated occurrences of the same anonymous type unify consistently and keep distinct method sets distinct; verify a test with two bindings of the same anonymous interface type and a negative test with different method sets.
- [ ] 2.3 Preserve a readable anonymous type in diagnostics and never expose the internal registry key as a package name; verify the mismatch message contains the original `interface{...}` spelling and identifies the argument.

## 3. Go FFI assignability

- [ ] 3.1 Extend `goBoundaryArgumentAssignable` to resolve an anonymous source identity and a named target type through the Go service, using `types.AssignableTo`; verify a positive test for an anonymous interface embedding the target interface.
- [ ] 3.2 Verify pointer method-set assignability still works when a `Ref[T]` value is passed to a target interface, and non-assignable named types remain rejected.
- [ ] 3.3 Add negative coverage for an anonymous interface missing a required method or carrying a mismatched method signature; verify the call is rejected with argument context.

## 4. Goldmark-shaped integration coverage

- [ ] 4.1 Add a compiler-level fixture with named interfaces `renderer.Option` and `html.Option`, a function returning `interface { renderer.Option; html.Option }`, and a variadic function taking `...renderer.Option`; verify a MyGO source call compiles without a source workaround.
- [ ] 4.2 Verify generated Go passes the anonymous interface value directly to the variadic call and contains no conversion, helper call, or internal anonymous-type key; compile the generated fixture with the Go toolchain.
- [ ] 4.3 Add nested coverage for anonymous interfaces inside slices, maps, pointers, channels, and function signatures, plus an anonymous-struct case that fails with a clear diagnostic instead of a qualified-name error.

## 5. Validation

- [ ] 5.1 Run the focused Go tests for the FFI loader and typeinference2 with a task-local writable `GOCACHE`; verify all new positive and negative tests pass.
- [ ] 5.2 Rebuild the bootstrap compiler and run the affected self-host validation in the established order; verify generated sources remain clean and focused bootstrap tests pass.
- [ ] 5.3 Rebuild `mygo` and compile a temporary copy of the `commune-backend` `internal/content` fixture, including `rich.mygo:84`, without changing the consumer source; verify the original Goldmark call now type-checks.
- [ ] 5.4 Run `openspec validate recognize-go-anonymous-types --strict` and the relevant compiler test suite; verify the change validates and existing named Go type behavior has not regressed.
