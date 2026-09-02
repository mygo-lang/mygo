## 1. Package inference context

- [ ] 1.1 Initialize `InferState.PkgInfo` with the effective current-package declarations in `InferPackage`, and verify package inference can look up local named enum variants.
- [ ] 1.2 Initialize `InferState.PkgInfo` with the combined declaration set in `InferPackageWithExternal` (and matching package-level paths), and verify external declarations do not prevent local named-variant lookup.

## 2. Regression coverage

- [ ] 2.1 Add an `InferPackageWithExternal` regression test defining `Content`, `Message.Content: Slice[Content]`, and `Content.Text { Text: text }`; verify the inference result succeeds.
- [ ] 2.2 Run `go test ./internal/mygo/typeinference2` and verify existing and new inference tests pass.
- [ ] 2.3 Run the relevant bootstrap compilation test or `go test ./internal/mygo/compiler` and verify package-level bootstrap inference remains successful.
