# Verification notes

## End-to-end measurements (JSON-RPC client against `/tmp/mygo-lsp`)

Fixtures (same before/after):

- A: `app` (12 files) + `util` (4 files) + Prelude (28 files) = 98.5 KB.
- B: A + `bigdep` (6 files, 73.7 KB, never edited) = 172.2 KB.

Median time per `didChange` (10 edits, measured until `publishDiagnostics`):

| Scenario | Before | After | Improvement |
|---|---|---|---|
| A, 1 open document | 195 ms | 80 ms | 2.4× |
| A, 5 open documents | 971 ms | 83 ms | 11.7× |
| B, 1 open document | 423 ms | 170 ms | 2.5× |
| B, 5 open documents | 2203 ms | 179 ms | 12.3× |

Before the change, five open documents produced five sequential full analyses per edit
(each `publishDiagnostics` arrived ~190 ms / ~440 ms apart). After the change all five
publications complete at the same timestamp because one analysis is reused.

## Benchmark results (`go test ./lsp -run XXX -bench 'BenchmarkEditorAnalyzeEdit' -benchmem`)

```
BenchmarkEditorAnalyzeEditOneOpen-16      20  319760562 ns/op  1.000 analyses/edit  1.000 parses/edit
BenchmarkEditorAnalyzeEditFiveOpen-16     20  284710038 ns/op  1.000 analyses/edit  1.000 parses/edit
BenchmarkEditorAnalyzeEditOneOpenNoDep-16 20  173261087 ns/op  1.000 analyses/edit  1.000 parses/edit
```

- `parses/edit = 1.000` for every scenario: only the changed file is parsed per edit,
  independent of open-document count and unchanged dependency size.
- `analyses/edit = 1.000`: one package analysis per edited version.
- Five-open edit time is below the one-open time (within noise), satisfying the
  "≤1.2×" target.
- The remaining difference between the no-dependency and with-dependency fixtures
  (173 ms vs 320 ms) is type inference and Go-FFI collection over the unchanged
  dependency, which is explicitly out of scope for this change. Parse work is constant
  (`parses/edit = 1`), which is the behavior this change fixes.

## Commands used

- `go run ./cmd/mygo sync .` then `go build ./...`
- `go test ./lsp ./internal/mygo/compiler ./internal/mygo/parser2`
- `go test ./...`
- `go test ./lsp -run XXX -bench 'BenchmarkEditorAnalyzeEdit' -benchmem -benchtime 20x`
