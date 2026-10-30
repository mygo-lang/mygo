# compiler3 Go backend

`codegen3` emits Go from verified `ir3`; it does not choose representations or
reconstruct source semantics from an untyped tree. The backend owns per-file
grouping, imports, line directives, structured control-flow reconstruction, Go
construction/matching syntax, FFI adapters, and cache integration.

## Per-file emission

Each IR `Decl.Path` maps back to its originating `.mygo` file. `GenerateFiles`
groups declarations by that path and emits one `zz_<source>.gen.go` (or
`zz_<source>.gen_test.go` for a test source) per input. Shared package helpers are
emitted once, in deterministic source order.

Generated line directives use package-relative source names and the original
source position. The absolute module checkout path never enters generated text,
so two roots with identical sources produce byte-identical output.

## Imports

The backend collects referenced package identities while emitting code and adds
only the imports needed by that generated file. Prelude is imported for a file
that references prelude declarations, except when generating the prelude package
itself. Go FFI imports preserve the source alias and `go:` resolution boundary.

## Values and constructors

Canonical representations are described in `representation.md`. The backend
uses the `Repr` already attached to IR types:

- tagged enums emit variant constructors and direct tag/field access;
- indirect enum payloads preserve source-level constructor arguments while
  storing pointers;
- `Option[Ref[T]]` uses `nil`/pointer operations rather than tagged fields;
- lists and maps retain their native linked/slice/map layouts;
- generic functions keep real Go type parameters;
- dynamic typeclass values retain the `MygoDynamic*` adapter ABI.

Legacy constructor spellings used by the established backend are emitted as
aliases where required, without changing the canonical field layout.

## Control-flow mapping

| IR form | Go form |
| --- | --- |
| `LetExpr` | `:=` in a scoped block, or assignment to a hoisted local. |
| `IfExpr` | `if`/`else`. |
| `WhileExpr` | `for condition`. |
| `SwitchExpr` | `switch tag`/decision `switch true` with one subject evaluation. |
| `BlockExpr` join | Ordinary structured block/if/switch when reducible. |
| `JumpExpr` | Direct statement-list emission, or `goto label` across irreducible joins. |
| `RaiseExpr` | Fallback jump/condition to the next match arm. |
| `ReturnExpr` | Go `return`, including multi-return unboxing. |
| `BreakExpr` / `ContinueExpr` | `break`/`continue`, labeled when inside reconstructed joins. |

When a local must cross a label/goto boundary, the backend hoists a typed
declaration to function scope. Verification rejects missing types before this
stage, so every hoisted variable has a valid Go declaration.

## Pattern matches

The backend emits the optimizer's match tree rather than re-testing the whole
subject for each arm. Variant matches dispatch on `Tag`; bindings select fields
from the active variant. Tuple, literal, nested, and guarded patterns use the
same subject value. A failed guard transfers to the next applicable arm.

## Tail calls

Self-tail calls can emit a loop directly. Mutual tail-call SCCs lower to a
state-machine loop with one state per target. Exported functions keep their
source names/signatures and dispatch into the loop; unknown closure/interface
tail calls remain ordinary calls.

## FFI and inline Go

- Go `(T, error)` is wrapped as `Result[T, error]`, with `Err` carrying a non-
  nil error.
- `Option[Ref[T]]` and `Ref[T]` cross the boundary as pointer/nil shapes.
- FFI calls are `CallKind.Foreign`; unknown or unadaptable signatures fail with
  a located diagnostic.
- Inline Go operands use explicit IR values/types and canonical spelling. A
  snippet that expects the previous interface ADT shape is rejected.
- Dynamic boxing emits dispatch adapters selected by the `adapterABI` table.

## Output boundary

An ordinary build/test does not write generated Go beside the source tree. The
pipeline stores generated files in the content-addressed user cache and runs
`go build`/`go test` with a temporary overlay. `--codegen3-emit=<dir>` is the
explicit debug materialization path. No metadata artifact is generated.

The cache key covers compiler identity, representation/adapter versions,
optimization level, normalized sources, dependency keys, and relevant Go module/
toolchain inputs. Removing the cache changes only execution time; the next cold
build regenerates the same generated text.

## Error policy

Backend lowering returns a diagnostic instead of emitting guessed Go when:

- a required type or representation is absent;
- dictionary/instance evidence is unresolved;
- a Go FFI or inline-Go shape cannot be adapted;
- control-flow lowering would enter an invalid variable scope;
- an unsupported IR node reaches expression emission.

Diagnostics retain the originating source path, line, and column.
