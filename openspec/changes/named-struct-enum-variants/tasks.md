## 1. Production Compiler (commit 1)

- [x] 1.1 Extend `parser.y` to parse `Name { field: Type, ... }` enum variant declarations and add parser tests covering named fields, mixing tuple/named variants, and invalid mixed payload syntax
- [x] 1.2 Add `StructVariantPattern` (or equivalent) AST node for `Variant { field }`, `Variant { field: bind }`, and `Variant { _, ... }` patterns; parse it in `parser.y` and add parser tests for shorthand, explicit binding, partial fields, and `_`
- [x] 1.3 Update type inference to type-check named-struct variant constructors (`Enum.Variant { ... }`) and struct patterns with field-name binding, including generic enum type substitution; add type-inference tests
- [x] 1.4 Update `generate.go` so named-struct variant Go struct fields use declared MyGO field names and `genEnumDecl` emits them correctly
- [x] 1.5 Update `translate_literal.go` to lower `Enum.Variant { field: expr }` construction into the variant Go struct literal, checking unknown/duplicate fields
- [x] 1.6 Update `translate_control.go` to lower `Variant { ... }` switch patterns into type assertion + named field bindings, including `_` discard
- [x] 1.7 Add validators in `compiler/validate.go` for duplicate/unknown struct-pattern fields and duplicate variant declaration field names
- [x] 1.8 Add end-to-end tests (parser, type inference, codegen, runtime) exercising the full named-struct variant flow from declaration to switch matching
- [x] 1.9 Run `go test ./internal/mygo/...` (or the project's standard test command) and verify all tests pass, then commit with a `feat(compiler):` message

## 2. Bootstrap Compiler (commit 2)

- [x] 2.1 Port parser changes to `parser2` (or `parser2/..` equivalent) for named-struct enum variant declarations and struct patterns
- [x] 2.2 Port type-inference changes to `typeinference2` for constructor and pattern typing
- [x] 2.3 Port codegen changes to `codegen2` for construction and switch-pattern lowering
- [x] 2.4 Port validation logic for duplicate/unknown field names to the bootstrap compiler paths
- [x] 2.5 Add or update bootstrap compiler tests covering declaration, construction, and pattern matching
- [x] 2.6 Run the full bootstrap compiler test suite and verify parity with the production compiler behavior, then commit with a `feat(compiler2):` message
