## 1. Library API and generated adapters

- [ ] 1.1 Define ordinary library construction and invocation APIs for dynamic values; verify callers need no new syntax or changed `switch/case` behavior
- [ ] 1.2 Generate an adapter for each supported existing `impl` method; verify adapters preserve concrete implementation selection and compile as Go
- [ ] 1.3 Define the erased payload and implementation identity representation; verify one wrapper type stores values from distinct concrete implementations of the same interface
- [ ] 1.4 Diagnose missing or ambiguous implementations and interface methods that cannot be represented by the dynamic adapter ABI

## 2. Metadata schema and emission

- [ ] 2.1 Define a versioned JSON schema for package identity, exported MyGO signatures, type descriptors, implementations, and method adapters; verify deterministic serialization fixtures
- [ ] 2.2 Define canonical type encoding and stable interface/implementation identities; verify generic targets and imported named types round-trip without absolute paths
- [ ] 2.3 Emit adapters and a comment-only `zz_mygo_metadata.gen.go` file with schema/ABI versions and generated-source digests; verify Go parsing/build accepts the file and metadata describes emitted symbols
- [ ] 2.4 Write generated metadata atomically and diagnose duplicate blocks, malformed JSON, stale digests, and incompatible versions

## 3. Caller-side metadata loading and dispatch

- [ ] 3.1 Read metadata blocks from generated Go files and reconstruct exported package declarations without dependency `.mygo` sources; verify a caller compiles when those sources are unavailable
- [ ] 3.2 Resolve implementation descriptors from the caller's dependency graph and emit direct adapter references; verify the Go linker includes producer adapters
- [ ] 3.3 Emit generic method trampolines from adapter metadata; verify caller-selected method type arguments reach the dependency adapter
- [ ] 3.4 Report package-specific diagnostics for missing metadata, unsupported methods, missing producer dependencies, and incompatible adapter ABI

## 4. Integration and compatibility

- [ ] 4.1 Compile and run a heterogeneous dynamic collection example using only ordinary library APIs and generated adapters
- [ ] 4.2 Verify a caller can adopt an already compiled implementation package by recompiling only itself while unchanged dependencies remain untouched
- [ ] 4.3 Verify existing static `using` calls retain their generated signatures and behavior
- [ ] 4.4 Verify dynamic metadata is found in generated Go source comments and document that binary-only archives without generated sources are unsupported
