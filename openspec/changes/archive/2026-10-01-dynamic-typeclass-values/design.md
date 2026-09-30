## Context

See `proposal.md` for motivation and `specs/language/dynamic-typeclass-values/spec.md` for the behavior contract.

MyGO currently lowers `using I[T]` to explicit helper-function parameters and emits each `impl` method as a package-level Go function. `package_index.go` discovers MyGO packages by loading their `.mygo` sources. Generic interface methods lower to generic top-level helpers; Go cannot store an uninstantiated generic function in a vtable field. The feature adds no MyGO syntax and does not change existing syntax semantics.

## Goals / Non-Goals

**Goals:**

- Represent a dynamic value together with the exact typeclass implementation selected for its concrete type.
- Let values for one interface share a library-level runtime type and dispatch one value through its associated adapter.
- Let callers resolve exported MyGO declarations and implementations from generated Go packages without dependency `.mygo` sources.
- Support generic interface methods when the generated adapter ABI can express their signatures.
- Expose only dynamic operations that can use the package's stored hidden-type payload without a second caller-supplied value of that hidden type.
- Preserve current static typeclass call lowering.

**Non-Goals:**

- Adding syntax or changing existing syntax behavior, including `switch/case`.
- Opening multiple dynamic values or establishing equality between hidden types.
- Runtime plugin loading or reflective generic invocation.
- Reading metadata from compiled Go archives that do not include generated Go source files.

## Decisions

### Use an ordinary library API and compiler-generated adapters

For every exported typeclass interface, the compiler emits an ordinary wrapper type and ordinary functions for packaging and dynamically supported operations. The wrapper has one stable type for that interface, independent of its concrete implementation types. It stores an erased payload and a stable implementation identity. It exposes ordinary accessors needed by generated code, but does not expose a new MyGO operation or pattern-matching form.

Compiling an existing `impl` emits adapter functions that bind its concrete target type and call the already generated implementation helpers. These are ordinary package-level Go functions with explicit payload parameters; they do not become methods on the target struct and do not implement a Go interface. The importing caller reads the interface and implementation metadata, then emits ordinary construction and dispatch functions for the implementation set in its dependency graph. Those dispatch functions call the exported adapter symbols directly. This division lets a caller add an already compiled implementation package by recompiling only itself: the interface package's wrapper ABI remains stable, while the caller supplies direct references to the newly available adapters. No runtime registry or plugin loading is involved.

The adapter is generated as a consequence of compiling the existing `impl`; source declarations and their meaning do not change. An adapter receives the erased payload as its first argument, checks/converts it to the concrete target type, then calls the exact package-level helper emitted for that `impl` method. It returns the helper's result using the adapter ABI. No Go receiver dispatch or Go interface method set participates. Ordinary calls on statically known types continue to resolve through the current `using` mechanism. The dynamic API does not promise cross-package equality of hidden types or operations that would require such equality.

Generated public names use `MygoDynamic<Interface>` as the deterministic wrapper prefix and are recorded in metadata so callers do not reconstruct names. For example, `Show` gets the `MygoDynamicShow` wrapper, `NewMygoDynamicShow(payload, implementationID)`, `MygoDynamicShowPayload(value)`, and `MygoDynamicShowImplementation(value)`. The caller-side generated API consists of a typed constructor per concrete implementation and ordinary functions for dynamically supported methods. These functions use the wrapper's stored identity to select a direct adapter reference. Generated names are collision-checked against existing declarations; a collision is a compile-time diagnostic.

The dynamic façade treats the package's target value as its receiver-like operand, even though MyGO typeclass methods lower as package-level helpers. For a method such as `ToString[A](value: A)`, the façade supplies the packaged payload and returns the public result. A method such as `Equals[A](left: A, right: A)` is not exposed dynamically because one package cannot supply both operands. Calls such as `Map[B](c: C[A], fn: func(A) -> B)` are also not exposed in this version because the callback signature leaks the hidden type. They remain usable through ordinary static typeclass calls where the concrete types are known.

If a supported method returns the hidden type itself, the façade may package the result with the same implementation evidence. This is valid only when the result type still matches the implementation target and the adapter can preserve that evidence; otherwise that result shape is not dynamically exposed.

### Embed package metadata in generated Go source comments

Emit one dedicated comment-only Go source file per MyGO package, for example `zz_mygo_metadata.gen.go`. It contains the normal Go package clause and a delimited metadata block in comments. Go tooling compiles the file normally and ignores the comment payload. The MyGO compiler scans generated Go sources for the marker and decodes the block; no separate JSON sidecar is required.

The file has this shape:

```go
// Code generated by MyGO; DO NOT EDIT.
// mygo:metadata-begin
// {"format":"mygo-package","schemaVersion":1,
//  "compilerABI":"1","adapterABI":"1",
//  "package":{"name":"backend","importPath":"example.com/app/backend"},
//  "generatedSources":[],"exports":{},"implementations":[]}
// mygo:metadata-end
package backend
```

The marked payload is UTF-8 JSON. The reader accepts only one begin/end block per package, strips the comment prefix, then decodes and validates the JSON. JSON is emitted deterministically: fields follow the schema order, descriptor arrays are sorted by canonical identity, and the file is written atomically. Metadata is generated only after the package's declarations and adapter symbols are known. The metadata source file is excluded from its own generated-source digest to avoid a recursive hash.

The metadata is available when a dependency's generated `.go` files are distributed, even if its `.mygo` sources are absent. Comments are not part of Go compiled archive export data. Binary-only dependencies without generated source require a separately packaged metadata artifact and are outside this design.

### Define a versioned export schema

The top-level record contains:

- `format` and `schemaVersion`: metadata format identity and major schema version.
- `compilerABI` and `adapterABI`: compatibility versions for compiler lowering and generated adapter calling conventions.
- `package`: MyGO package name and canonical Go import path; no absolute filesystem paths are stored.
- `imports`: the package's declared import aliases and canonical paths. Go FFI imports are explicitly tagged, so a reader can rebuild qualified exported signatures and discover both MyGO and Go dependencies without producer `.mygo` files.
- `generatedSources`: sorted generated Go source paths and SHA-256 digests, excluding the metadata file itself. These detect stale metadata/adapters when source artifacts are present.
- `exports`: compiler-readable public declarations needed to compile callers without MyGO source: type aliases and named types, struct fields, enum variants, functions, and typeclass interfaces with full signatures.
- `implementations`: exported instance descriptors and their method adapter records.

An implementation record has this logical shape (omitted fields here are defined by the schema):

```json
{
  "implementationId": "sha256:<canonical-identity-digest>",
  "interface": {"package": "example.com/app/contracts", "name": "ToString", "arguments": [{"kind": "typeParam", "name": "A"}]},
  "target": {"kind": "named", "package": "example.com/app/backend", "name": "Widget", "arguments": []},
  "typeParameters": [],
  "constraints": [],
  "methods": [{
    "interfaceMethod": "ToString",
    "adapterSymbol": "MygoIDynamicToStringWidget",
    "methodTypeParameters": [],
    "logicalSignature": {"parameters": ["A"], "results": ["String"], "using": []},
    "goABISignature": {"parameters": ["any"], "results": ["string"]},
    "adapterABI": 1
  }]
}
```

The example's `logicalSignature` and `goABISignature` values are illustrative; the actual schema uses the structured type nodes below instead of strings.

Types are encoded as structured nodes rather than display strings. The schema includes nodes for built-in types, scoped type parameters, named types (with package import path, name, and type arguments), slice, map, tuple, function, reference, and higher-kinded application. A type-parameter node carries its lexical scope and ordinal as well as its display name, so nested or shadowed names remain distinct. Function and interface method signatures include parameter/result names and types, method type parameters, and `using` constraints. This keeps package identities and generic substitutions unambiguous and avoids depending on pretty-printer formatting.

An implementation descriptor contains:

- a stable `implementationId`;
- the canonical interface identity and its applied arguments;
- the normalized target type pattern, implementation type parameters, and constraints;
- method records keyed by canonical interface method identity;
- each method's exported adapter symbol, logical MyGO signature, lowered Go ABI signature, generic parameter list, and ABI version.

The stable implementation ID is derived from canonical package import path, interface identity, normalized target pattern, and the declared implementation name when present. For an unnamed implementation, the canonical interface/target identity is used; duplicate implementations with the same canonical key are rejected during compilation. The record also stores these inputs so the compiler can recompute and validate the ID. Source line numbers and machine-local paths are excluded. If two records resolve to the same identity but disagree in content, metadata loading fails rather than selecting one.

The schema describes all public MyGO declarations needed for type checking, not only interface and implementation names. It also describes only adapters that were actually emitted. A generic method adapter keeps its method-level type parameters; the ABI descriptor specifies which values are erased and how results are represented. If a method signature cannot be represented by the supported dynamic ABI, the producer omits that adapter with a diagnostic or marks it unsupported so callers receive a compile-time error rather than generating an invalid call.

### Load metadata for compiled dependencies

For a workspace package being compiled from source, the compiler continues to use its `.mygo` declarations directly. For an imported compiled package, it scans that package's generated Go files for exactly one metadata block, validates the package clause/import path, schema and ABI versions, and generated-source digests, then reconstructs its exported package model for type checking and typeclass resolution.

If metadata is absent, existing static import behavior may continue through the current source-loading path when `.mygo` source is available. Dynamic packaging/dispatch requires compatible metadata and adapters; if neither metadata nor source can provide the implementation, the compiler reports a package-specific diagnostic. Duplicate metadata blocks, malformed JSON, identity mismatches, stale digests, and unsupported schema majors fail with explicit errors naming the dependency.

### Generate call-site dispatch from dependency metadata

The caller compiler gathers implementation descriptors from its declared dependency graph. At a dynamic package construction site, it resolves the concrete type against those descriptors and records the selected implementation ID and adapter. It generates ordinary static references to exported adapters, so the Go linker includes the producer package. The caller's generated dispatcher branches on implementation ID only among implementations in that compile-time graph.

For generic methods, the caller emits a generic top-level trampoline whose method type parameters match the interface method. The trampoline calls the selected dependency adapter instantiated with the caller's type arguments. No uninstantiated generic function value, global init registry, runtime plugin, or reflection-based method invocation is used. Adding an already compiled producer package changes caller metadata inputs and requires recompiling the caller, but not unchanged dependencies.

An impl with hidden target parameters can still construct a wrapper. A method is dynamically callable only when its adapter can be formed without capturing an unknown target type argument in a stored generic function value. If a generic method requires that capture, metadata records the method as unsupported and the caller compiler reports it before emitting an invalid call.

### Keep ordinary static dispatch unchanged

Calls on statically known types continue to use current `using` dictionary injection and helper selection. Generated dynamic adapters are additional symbols and do not change those call signatures or add overhead to static calls.

## Risks / Trade-offs

- [Metadata schema and adapter ABI can drift] → Version them separately, validate before code generation, and provide dependency-specific compatibility diagnostics.
- [Generated Go source may be stripped or replaced by binary-only artifacts] → Require generated source metadata for this workflow; binary-only metadata packaging is a separate future extension.
- [Metadata may no longer describe its generated adapters] → Store deterministic digests for generated Go files excluding the metadata file and reject stale records.
- [Type encodings may become ambiguous across modules or aliases] → Encode canonical package import paths and normalized structured type nodes; test alias expansion and generic substitution.
- [Caller dispatch may omit a producer package] → Require producer packages to be declared dependencies and diagnose implementations absent from the compile-time graph.
- [Generic or hidden-type method signatures may exceed the adapter ABI] → Record representability per adapter and reject unsupported dynamic calls at compile time.
- [Large public APIs can make comment metadata sizable] → Keep one package-level metadata block, use compact deterministic JSON, and avoid repeating interface descriptors in every implementation record.

## Migration Plan

This is additive. Existing packages continue using static dispatch. Packages that publish dynamic implementations are rebuilt once to emit adapter functions and `zz_mygo_metadata.gen.go`. Callers can then compile against those generated Go artifacts without dependency `.mygo` files. If an implementation package changes, rebuild it to refresh adapters and metadata, then rebuild callers that use that change.
