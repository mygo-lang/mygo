# Tasks

## 1. Reproduce and characterize type identity

- [x] 1.1 Add a type inference regression using a Go named type in a callback parameter and the imported dynamic MyGO interface method signature; verify the same-type call infers successfully.
- [x] 1.2 Add a regression with two distinct named Go types that share an underlying representation; verify inference still reports a mismatch.

## 2. Normalize named Go type identity

- [x] 2.1 Update Go FFI type conversion and imported MyGO signature qualification to use consistent package identity recursively through function types; verify the callback regression passes.
- [x] 2.2 Cover source import aliases that differ from Go package names; verify both signatures resolve to one identity without changing generated selectors.
- [x] 2.3 Verify existing Go type alias behavior remains intact with the focused type inference suite.

## 3. Integration

- [x] 3.1 Run bootstrap inference on `internal/listener` in a temporary copy of mailgun-api-gateway and verify `RunLoop` no longer reports a mismatch between `Message` and `go:net/mail.Message`; inference proceeds to a separate Go interface-assignability error at `smtp.NewServer(backend)`.
- [x] 3.2 Run the compiler's relevant type inference and Go FFI regression suites; verify no new failures.
