## Why

Go FFI bounds for compiler2 (bootstrap pipeline: parser2 → typeinference2 → codegen2)
There are three type/code generation gaps that block the connection between gateway class code and standard libraries (net/http, context, etc.)
Interoperability: Go structure fields do not enter the symbol table, causing chained method calls to fail to type check; original multiple return values
The lack of package context in parsing causes package type names such as `context.CancelFunc` to be unresolved; Go named functions
The type is treated as a normal nominal type causing the function value to be uncallable.

## What Changes

- **Register Go FFI structure field symbol**: FFI loader extracts the exported field, `SymbolIndex` is 
`alias.TypeName::Field` registers `StructField`, making `ref.Field.Method(...)` 
Chained access can pass type checking and generate direct Go selectors (such as `v.F0.Header.Set(...)`).
- **`GoTypeSignature` extended metadata**: Added `Fields` (field name + type string) and 
`Underlying` (go/types underlying type string), used for package-aware type resolution.
- **Raw multiple return value parsing with package context**: Support for package-aware variants of `GoSignatureRawResultType` 
The local type name in the package (such as `CancelFunc`) appears in the signature, `let (ctx, cancel) = 
context.WithTimeout(...)` correctly generates `ctx, cancel := context.WithTimeout(...)`.
- **Go named function type is resolved to `TFunc`**: the underlying named type is `func(...)` (such as 
`context.CancelFunc`) is resolved to a function type, and the function value can be inferred to be callable (`cancel()`).
- **Regression Protection**: Fixed for qualified name struct literals (`http.Client { }` → 
`http.Client{}`) supplements the formal test to prevent rollback.

## Capabilities

### New Capabilities
- `bootstrap-ffi-boundary`: Compiler 2 Go FFI boundary behavioral requirements - structure field access, 
Multiple return values for type names within a package, call-by-value for named function types, and qualified name structure literals.

### Modified Capabilities
<!-- None: bootstrap-codegen-correctness only overrides MyGO tuple return/prelude imports, 
FFI boundary behavior is a new capability. -->

## Impact

- `internal/mygo/compiler/go_ffi_import.go`: field/underlying type extraction (Go side, driver 
`go/packages` + loader for `go/types`).
- `internal/mygo/typeinference2/types.mygo`: `GoTypeSignature` structure extension, field 
Symbol registration, `GoSignatureRawResultTypeWithPackage`, named function type resolution.
- `internal/mygo/typeinference2/infer.mygo`: `ffiRawTupleResultType` and GoMethod 
Instantiate the packet-aware path.
- `internal/mygo/codegen2/translate_ast.mygo`: `translateFFITupleLetStmt` used 
Package-aware primitive result type.
- Regenerate `zz_*.gen.go` (bootstrap product) and add/update tests 
(Go tests for codegen2, typeinference2).

Does not affect production compilers (`internal/mygo/codegen`, etc.), does not affect language semantics or
Openspec has existing capability requirements.
