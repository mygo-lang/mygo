## Why

The self-hosted type-inference stage now costs more than the parse stage when
the compiler syncs its own packages. For `internal/mygo/typeinference2`, the
current checkout measures about 4.6 s in `typeinference2` versus 2.4 s in
`parser2`, and a profile attributes most of the sample time to repeated import
environment construction and allocation-heavy helper tables. The dominant
cost is structural rather than the Hindley-Milner algorithm itself.

## What Changes

- Add reusable inference-stage benchmarks for declaration-heavy packages,
  local-binding-heavy functions, Go FFI import surfaces, MyGO import surfaces,
  and a pinned real package, reporting time, bytes, and allocations.
- Build each source file's import overlay once and reuse it across declaration
  predeclaration, symbol-table construction, inference, and package output.
- Cache both Go and MyGO import batches together with their lookup indexes and
  free-variable unions, so cache hits do not rebuild the same environment data.
- Replace the per-call Go primitive-pair table allocation with a zero-allocation
  lookup path.
- Cache each persistent `Env` layer's free-variable union so generalization of
  long local-binding sequences does not rescan the whole scope chain.
- Reduce repeated declaration predeclaration passes while preserving forward
  aliases and shadowing behavior.
- Remove the quadratic typed-declaration prepend in `inferDeclsAt`, and build
  the package symbol index once per inference setup instead of once per source
  file.
- Keep all existing diagnostics, inferred types, typed AST shapes, environment
  ordering, and generated Go output unchanged.

## Capabilities

### New Capabilities

- `inference-stage-performance`: requirements for linear, allocation-conscious
  type inference as package size and import surface grow, with reusable
  benchmarks and preserved observable inference results.

### Modified Capabilities

None.

## Impact

- `internal/mygo/typeinference2/infer.mygo`
- `internal/mygo/typeinference2/types.mygo`
- `internal/mygo/typeinference2/env.mygo`
- `internal/mygo/typeinference2/utils.mygo`
- New MyGO benchmarks under `internal/mygo/typeinference2`
- Regenerated `internal/mygo/typeinference2/zz_*.gen.go` files
- Bootstrap validation of `typeinference2`, `parser2`, `compiler`, and the
  generated-Go fixed point

