## 1. Render channel types in the dynamic API

- [ ] 1.1 Map the reserved names `Chan`, `SendChan`, and `RecvChan` (arity 1) to `chan T`, `chan<- T`, and `<-chan T` in the dynamic descriptor-to-Go renderer (`dynamicGoType`); verify with a test that renders each of the three and asserts the output contains no qualified channel named type
- [ ] 1.2 Apply the same mapping in metadata-to-MyGO reconstruction (`metadata.astContext.typeExpr` in `internal/mygo/metadata/ast.go`); verify with a test that reconstructs a channel type from a `kind:"named"` descriptor and does not fall back to a package-qualified name
- [ ] 1.3 Add channel handling to the Go-source-to-MyGO conversion (`dynamicGoTypeExprToMygo`) so `chan T`, `chan<- T`, and `<-chan T` map back to `Chan`/`SendChan`/`RecvChan`; verify with a test that parses generated API source for each channel direction

## 2. Gate constructor emission

- [ ] 2.1 Emit a dynamic constructor only when the interface has a dispatchable method or the compiling package references that interface's dynamic API, extending the existing reference detection to dependency implementations; verify with a test where an unused, fully unsupported interface yields no constructor
- [ ] 2.2 Verify a referenced interface still emits its constructor and that unsupported explicit requests keep producing their diagnostic
- [ ] 2.3 Confirm the emitter produces no source when no constructor and no dispatch function qualifies, so no `zz_mygo_dynamic.gen.go` is written

## 3. End-to-end and regression validation

- [ ] 3.1 Regenerate the concurrency library and confirm its generated dynamic output compiles
- [ ] 3.2 Build `tutorial-video-generation` with `go build ./...` and confirm it succeeds with no `undefined: concurrency.SendChan`, `RecvChan`, or `Chan`
- [ ] 3.3 Run the codegen2 dynamic-metadata and metadata unit tests and confirm no regressions
- [ ] 3.4 Confirm `CompilerABI`/`AdapterABI` and implementation IDs are unchanged, so no protocol or ID churn was introduced
