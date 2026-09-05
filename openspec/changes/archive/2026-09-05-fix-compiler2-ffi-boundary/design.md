## Context

See proposal.md (Why) for motivation. Current FFI bounds for compiler2 (tested in HEAD + reproduction
Verification):

- `goSymbolsFromTypes` only registers Go type methods (`GoMethod`), never fields → chaining 
`r.Header.Set(...)` reports `unknown field http.Request.Header` during inference.
- `GoSignatureRawResultType` parses the result string using `goSignatureTypes` without package context 
→ The type name `CancelFunc` in the bare package of `context.WithTimeout` is reported 
`unresolved Go type name: CancelFunc`, tuple let degenerates into `__tuple` path.
- `goPackageTypeForNameWith` parses the named type (`CancelFunc`) with the underlying `func()` into 
Nominal `TQualifiedName(...TCon("CancelFunc"))` → `cancel()` report 
`cannot unify CancelFunc with func() -> t2`.
- The qualified name structure literal (`http.Client { }`) has produced `http.Client{}` in HEAD, only missing 
Regression testing. Direct method calls (`h.Set(...)`) already produce direct Go selectors normally.

Designed to only work with compiler2 (`typeinference2` / `codegen2` / bootstrap FFI loader),
The production compiler is not affected.

## Goals / Non-Goals

**Goals:**
- Let chained `ref.Field.Method(...)` (fields from go:` types) be inferred and generated directly 
selector call.
- Let tuple let signatures containing local type names within the package maintain native Go assignments.
- Make Go named function type values ​​inferable and callable.
- Create formal tests for the above behavior + qualified name struct literals.

**Non-Goals:**
- Do not change the production compiler (`internal/mygo/codegen` and other Go implementations).
- Do not change the existing bounds of `(T, bool)` → `Option[T]`, `(T, error)` → `Result[T, E]` 
Convention; tuple let is still the only way to explicitly select native multi-value returns.
- No other FFI capabilities such as Go constants/unnamed structure fields are introduced.
- Qualified path normalization of cross-package field references is not resolved (see Risks).

## Decisions

### D1: FFI loader extracts both exported fields and underlying types
`bootstrapGoPackageInfoFromTypes` (`internal/mygo/compiler/go_ffi_import.go`)
For each exported named type:
- If `named.Underlying()` is `*types.Struct`, collect exported named fields 
`{Name, TypeString}` (skips unexported/embedded anonymous), added `GoTypeSignature.Fields`.
- Document `GoTypeSignature.Underlying = TypeString(named.Underlying())`.

Alternative: hardcode common types using reflection/magic tables inside typeinference2 - give up, loader already has it
`go/types` information, extending the structure can cover all packages.

### D2: The field symbols follow the existing SymbolIndex, and the methods and fields are in the same table.
`goSymbolsFromTypes` registers `Symbol.StructField(typeName, fieldName,
mono)`, same keyspace as `GoMethod` (`typeName::field`). Field type string is used
`goTypeNameWith(fieldType, Some(pkg), receiverTypeParams)` parsing (reuse signature parsing
device, guaranteed to be isomorphic with parameter/result types); fields that fail to parse are skipped (when accessing this field, press
`unknown field` error is reported, the error is located at the access point, which is more precise than the overall failure during the loading period).

Alternative: Directly store MonoType for `GoTypeSignature` - No, `GoTypeSignature` is FFI
Boundary data type (passed on both sides by Go/mygo), can only store strings.

### D3: Original multiple return value parsing plus package context
Added `GoSignatureRawResultTypeWithPackage(sig, pkg)` (used
`goSignatureTypesWithPackage`), old packageless versions remain as thin packages (existing tests/calls are not affected by
influence). `GoSignatureForCallee` adds a variant that returns `(sig, pkg)`, like this:
- `ffiRawTupleResultType` of `infer.mygo` and
- `translateFFITupleLetStmt` of `codegen2/translate_ast.mygo`

can get the corresponding package and correctly parse the type name in the bare package (such as `CancelFunc`).

### D4: Go named function type → TFunc
When `goPackageTypeForNameWith` hits `goPackageHasType`, check `Underlying`:
- Begins with `func(` → `goTypeNameWith(underlying, Some(pkg), typeParams)` resolves to 
`TFunc` (parameters/results are still package-aware, such as `func(w http.ResponseWriter, r *http.Request)`);
- Starting with `chan`/`<-chan`/`chan<-` → corresponds to Chan type;
- other (struct/int/slice/map/...) → Keep existing nominal `TQualifiedName` behavior.

At the same time, replace `GoSignatureType(sig)` at `infer.mygo` GoMethod instantiation with the package-aware version
(Find the package in `state.GoPackages` according to the type name of the method) to avoid the appearance of the package in the method signature.
Local type names are also not parsable.

Alternative: Convert `TQualifiedName(...TCon("CancelFunc"))` to a function at the call point - too brittle,
There is no way to know the signature; resolution from the underlying type is the only reliable source of information.

### D5: codegen does not change method dispatch
`translateReceiverImplMethodCall` has no candidates for FFI receiver (inherent/typeclass candidates
Only from MyGO impl), the chain of calls naturally falls into the plain path → directly `v.F0.Header.Set(...)`.
Verified (`h.Set(...)` reproduces successfully). So #1 only changes the inference side (field symbols), codegen is zero
Change.

## Risks / Trade-offs

- [If field type parsing fails, the field is unavailable] → Fields that fail to parse are skipped registration; obtained when user code accesses 
Explicit `unknown field` inference errors can be gradually added to the parser shape.
- [`Underlying` may not be supported by the parser when it is an anonymous struct/unexported combination] → only for `func(`/ 
The `chan` prefix is special, the rest remain nominal types, and the parser does not add complex struct grammars.
- [Normalize cross-package field reference paths (such as `*url.URL` → `go:url`, if the user imports with `go:net/url` 
It is not unified)] → This quirk also exists in existing signature parameters/results and will not be fixed in this change; the same package 
Reference (`http.Header`) is not affected, and gateway class usage scenarios are covered.
- [`GoTypeSignature` After adding fields, all construction points need to be synchronized] → Construction points only go_ffi_import.go and 
Test literals (keyed literals), backwards compatible, non-breaking.
- [bootstrap bootstrap and regenerate `zz_*.gen.go`] → use after modifying .mygo 
`go run ./cmd/mygo --bootstrap sync <pkg>` Regenerate and submit via git. The threshold for the package is to be fully green.

## Migration Plan

1. Change the .mygo source of Go side loader (go_ffi_import.go) and typeinference2/codegen2.
2. Regenerate `zz_*.gen.go` using the current bootstrap compiler (the new code in this change only uses 
It already has MyGO syntax structure and can be bootstrapped and compiled).
3. Add/update Go tests (codegen2, typeinference2, compiler bootstrap e2e), 
`go test ./internal/mygo/...` is all green.
4. Rollback strategy: Functions are mutually exclusive and simple. Rollback means revert corresponds to commit and retains the old `zz_*.gen.go`.

## Open Questions

- Do you need to add a scene level for `http.HandlerFunc` (named function type with parameters/results) 
Request? The current spec only promises callability itself, and the signature shape is guaranteed by the parser, which can be supplemented after implementation.
- The Go FFI loader only exports functions and types; package-level constants and
  variables (`*types.Const` / `*types.Var`) fall through the switch in
  `bootstrapGoPackageInfoFromTypes` and are dropped silently. The real
  `time.Second` (a `time.Duration` constant) is therefore not usable from MyGO
  today. A follow-up enhancement could seed typed constants (or untyped ones
  after `types.Default`) as env value bindings so
  `http.Client { Timeout: time.Second }` works directly. The tests in this
  change use the func form (`time.Second()`) to express the equivalent
  scenario.
