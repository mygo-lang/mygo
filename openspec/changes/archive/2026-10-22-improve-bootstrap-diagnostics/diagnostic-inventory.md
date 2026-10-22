# Bootstrap diagnostic exit inventory

This inventory classifies failure exits by their owning source module and
operation. A row covers both direct constructors and helpers reached from that
module. Propagation exits retain the existing diagnostic unchanged, except for
deduplicated source, function, package, or import context. Regression cases in
this document are acceptance obligations, not claims that migration is complete.

## Classification rules

| Exit family | Category | Required provenance | Regression obligation |
| --- | --- | --- | --- |
| Syntax recovery diagnostic | source | Recovery token, missing-token insertion point, or actual EOF | Named input, Unicode, tabs, CRLF, EOF |
| Invalid source construct | source | Smallest responsible expression, annotation, pattern, field, or declaration | Exact primary coordinates |
| Deferred semantic failure | source | Instantiated use or constraint origin | Two uses of one constrained scheme |
| Conflicting declarations or branch types | source | Current construct plus previous origin | Cross-file conflict and branch mismatch |
| Import or FFI resolution failure | source | Triggering import or FFI use; dependency failure keeps its own primary | Missing import, cycle, foreign type |
| Invalid compiler index or impossible AST/CST shape | internal | Nearest known original construct, if available | Injected malformed tree/index |
| Compiler-created support construct | generated-support | Explicit support origin and known source triggers | Dynamic metadata/dispatch failure |
| Filesystem, environment, or process failure | operation | Actual target and operation; related source trigger when known | Read, write, rename, and tool-start failure |
| Error propagation or compatibility conversion | inherited | Preserve category, code, primary, related locations and causes | Library/CLI consistency and colon-bearing explanation |
| Runtime panic deliberately emitted into user code | runtime behavior | Not a compiler failure exit | Existing non-exhaustive/runtime semantics remain unchanged |

An ordinary source error without provenance is an implementation defect. It
cannot be reclassified as an operation or support error to pass acceptance.

## Source ownership

Paths below are relative to `internal/mygo/` unless stated otherwise.

| Module / files | Exit coverage | Provenance provider | Regression obligation |
| --- | --- | --- | --- |
| `parser2/parser.mygo` | Syntax rejection, missing diagnostic index, public rendering adapters | SyntaxDiagnostic span and source identity | Structured syntax diagnostic and missing-index invariant |
| `parser2/syntax_lower_declarations.mygo` | Package/import/type/struct/enum/interface/impl/function/binding lowering; unsupported or incomplete CST | Current declaration CST and component spans | Declaration lowering and malformed CST |
| `parser2/syntax_lower_expressions.mygo` | Every expression/statement lowering family; literal and escape decoding; inline Go operands | Expression/statement CST, literal token span | Invalid escape, guard/body, inline Go |
| `parser2/syntax_lower_types_patterns.mygo` | Pattern constructors, literal patterns and recursive fields | Pattern CST and containing case | Nested literal/variant/tuple patterns |
| `parser2/syntax_tree_helpers.mygo` | Result predicates inspect failures without creating new ones | Inherited | Parse/lower public compatibility |
| `ast2/ast2_ids.mygo` | Loop-control validation; failed indexed access during traversal/ID assignment | Loop-control statement position, traversed original node | Break/continue outside loop; malformed AST |
| `ast2/array_length.mygo` | Integer conversion failure used as validation sentinel | Containing array annotation or literal | Invalid/overflowed length |
| `typeinference2/types.mygo` | File/package setup, source import conflicts, symbols, external packages, Go signatures/types and source iteration | PkgDeclSource declaration positions, annotation and import origins | Multi-file/external-test/prelude setup, imported type errors |
| `typeinference2/infer.mygo` | Declaration checks; functions/methods; expressions/statements; patterns; guards; calls; fields; bindings; collections | Declaration component, expression/span, argument or field value | Nested/type/arity errors, duplicate fields/elements, branch conflicts |
| `typeinference2/unify.mygo` | Type incompatibility, occurs check, tuple/function/application/array shape and indexed-list invariants | Calling semantic operation or constraint origin | Both resolved types and offending use |
| `typeinference2/solver.mygo` | Instance lookup/constraint failures and malformed predicate/instance lists | Predicate use origin and known instance declaration | Deferred constraints and malformed lists |
| `typeinference2/arrays.mygo`, `returns.mygo` | Contextual array element/length/return checks | Element, return expression, annotation | Array size/element/return mismatch |
| `typeinference2/dynamic_boxing.mygo`, `promotion.mygo` | Dynamic boxing and promoted selector failures | Source ascription, selector, condition or branch | Missing/ambiguous implementation or selector |
| `typeinference2/utils.mygo` | Impossible MonoType panic paths | Current inference context; explicitly internal | Malformed internal type |
| `codegen2/codegen2.mygo` | File iteration, missing output/index, dynamic append/metadata, unresolved foreign qualification | Source file/declaration origin or explicit support origin | Missing input, unresolved foreign type, support output |
| `codegen2/decls.mygo` | Declaration/method/enum/interface/dictionary lowering and malformed metadata | Declaration/component origin | Invalid signature, missing metadata |
| `codegen2/translate_ast.mygo`, `translate_expr.mygo` | Expression, statement, FFI, function, pattern and collection lowering | Current typed expression and nested operands | Nested failure retains deepest source |
| `codegen2/guarded_switch.mygo`, `tailcall.mygo` | Guarded branch plans, tail-call plans, transformed returns and malformed plan indices | Original case/body/function; synthetic origin | Guard and tail-call provenance |
| `codegen2/numeric.mygo` | Numeric conversion checks | Original numeric literal | Invalid numeric spelling or range |
| `codegen2/inferred_imports.mygo` | Foreign qualification error stored in generator state | Foreign type reference and defining package | Qualification failure with source |
| `codegen2/gofile.mygo` | Go rendering errors and source-name helpers | Go AST origin metadata | Source-backed render failure |
| `codegen2/goast/*.go`, `goast/provenance.go` | Parse/render errors, constructor panics, Go AST traversal/structural keys and line-directive injection for generated fields, signatures, types, inline Go and metadata shape | Origin at the FFI construction boundary; Go-owned provenance registry with final formatted ranges | Inline Go/type construction failure, line mapping and support classification |
| `compiler/bootstrap.mygo` | Source reads/parsing/ID assignment, prelude resolution, dependency cycles/traversal, inference/generation, output writes and missing indices | Source input, retained import chain, stage and operation target | Full sync/build and named-source entry points |
| `compiler/go_ffi_import.mygo`, `go_ffi_import.go` | Go module/import/package loading and signature conversion | Triggering import plus tool/package cause | Missing Go package and invalid signatures |
| `compiler/bootstrap_entry.go` | Result/error compatibility adapters | Existing structured value; invalid result is internal | Go library API compatibility |
| `compiler/api.go` helpers used by bootstrap | Directory discovery and module/import path resolution | Actual filesystem operation/path, triggering import | Directory/module read failure |
| `cmd/mygo/main.go` | Compilation dispatch, Go tool launch/run and exit presentation | Structured compile error or Go line directives embedded in generated files | CLI sync/build flags, mapped Go errors, startup failures |

## Boundaries and audit commands

`compiler/editor_*.mygo` are editor-only entry points, not additional bootstrap
stages. They consume compatibility interfaces and must still compile after
migration. LSP protocol behavior remains unchanged; its position adapter must
continue translating compiler byte columns into UTF-16 positions.

Generated `zz_*` code is audited through its authored source. Test fixtures and
the legacy parser/compiler are excluded. Searches also include `None` results
that callers convert into failures, Go FFI `err` conversions, errors cached in
generator state, and constructor panics rather than counting only `Err` text.

```sh
rg -n 'Err\(|Panic\(|panic\(|Error\(\)|\$mygo\$error' internal/mygo/{parser2,ast2,typeinference2,codegen2,compiler} --glob '*.mygo' --glob '!**/*test*'
rg -n 'panic\(|return .*err|exec.Command|must\(' internal/mygo/codegen2/goast internal/mygo/compiler/{bootstrap_entry,go_ffi_import,api}.go cmd/mygo/main.go --glob '*.go' --glob '!**/*test*'
```

Completion requires matching each family above to passing regression evidence
and checking that the active pipeline never renders and reparses diagnostics.

## Implemented diagnostics

The active pipeline now uses the following communication paths:

- `ast2.Diagnostic` is the shared value. It carries stage, code, category,
  an optional primary span, labeled related spans and deduplicated context.
- Parser errors use `ParseFileAtDiagnostic` and `LowerSyntaxDiagnostic`;
  string-returning parser APIs are rendering adapters at the public boundary.
- Package inference preserves declaration positions, attaches expression and
  constraint origins, and reports duplicate declarations or import aliases with
  the previous declaration as a related location.
- Bootstrap imports retain `ImportDecl` positions. A dependency keeps its own
  primary location while the importing declaration becomes related context;
  cycle detection covers both directory traversal and imported-package cache
  construction.
- `BootstrapDiagnosticError` crosses the Go API boundary without reparsing its
  rendered text. MyGO owns classification and enrichment; the small Go file
  supplies the native `Error` and `Unwrap` method sets.
- Code generation records expression and declaration origins in a Go AST node
  registry. Final formatted output is reparsed to obtain final byte ranges, and
  formatting-only nodes are classified as generated support.
- `codegen2/goast/provenance.go` injects one inline `/*line file:line:col*/` at the
  start of each generated line with a known origin. Support-only lines point to
  the generated file rather than inventing a MyGO position. Earlier pipeline
  diagnostics retain exact UTF-8 byte columns; final build columns are
  best-effort and may follow generated formatting.
- Bootstrap writes generated Go files through temporary files and rename.
  Package completion removes obsolete compiler-generated outputs while
  retaining unrelated files.
- `cmd/mygo build` forwards Go tool stderr directly. The compiler reads the
  embedded line directives and reports the MyGO file and line; no sidecar map
  lookup or remapping remains in the CLI.

## Coordinate and line-directive lifecycle

Diagnostics produced before final Go compilation use the original source
identity and one-based line and UTF-8 byte column. Tabs count as one byte, CRLF
is one line break, and EOF uses the actual end position. LSP converts compiler
byte columns to UTF-16 at its protocol boundary.

Line directives are inserted after final Go formatting, so the generated text
and its provenance are written together and regenerated together. Final build
diagnostics identify the MyGO file and source line; column precision may follow
the generated token layout. No persistent sidecar, fingerprint, stale-map
lookup or map cleanup path exists.

## Validation evidence

- Bootstrap regeneration completed for `ast2`, `parser2`, `typeinference2`,
  `codegen2`, `codegen2/goast`, `compiler`, `lsp` and `lib/text/parsec` using a
  task-local writable `GOCACHE=/tmp/mygo-diag-gocache`.
- A fresh CLI was built with
  `go build -buildvcs=false -o /tmp/mygo-line-final3 ./cmd/mygo`; the rebuilt
  compiler successfully regenerated the changed packages with line directives.
- Focused diagnostic, output lifecycle, dependency/cycle, external-test,
  parser coordinate, inference provenance, FFI and final build line-mapping
  tests passed. No `.mygo-map.json` artifacts remain.
- `go test ./...` passed all 30 listed packages. The run was bounded by
  `ulimit -v 8388608` and `timeout 600`; the compiler package was the longest
  at approximately 263 seconds, followed by formatter, LSP and parser2.
- An end-to-end CLI fixture with invalid generated Go exited with status 1 and
  reported a `main.mygo:4` location from the embedded directive. The column is
  intentionally best-effort for this final compiler-emitted case.
  Unmapped dependency output is preserved verbatim.
- `openspec validate improve-bootstrap-diagnostics --strict` succeeds.
