# compiler3 baseline (codegen2)

Recorded on 2026-10-25 from the current bootstrap pipeline (`parser2 → ast2 →
typeinference2 → codegen2`). These numbers are the reference point for
`optimizer3`/`codegen3` work; they are not performance guarantees.

Environment:

- Linux x86-64
- Go `go1.27.1 linux/amd64`
- AMD Ryzen 9 PRO 7940HS

Command:

```bash
go test ./internal/mygo/compiler \
  -run '^$' \
  -bench '^BenchmarkBootstrapIR3Baseline$' \
  -benchtime=1x -count=1
```

Result (one compile per benchmark iteration):

| Workload | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `option-list` | 587,858,120 | 216,418,072 | 3,333,425 |
| `dictionary` | 596,114,649 | 215,252,944 | 3,326,262 |
| `tail-recursion` | 595,505,325 | 215,770,912 | 3,331,271 |

Each iteration performs a full package compile plus prelude resolution. The
`benchtime=1x` setting keeps the check fast; use `-benchtime=5x` or higher when
comparing optimization changes.

Stack-depth baseline:

```bash
go test ./internal/mygo/compiler \
  -run '^TestBootstrapIR3BaselineStackDepth$' \
  -count=1
```

The test compiles self and mutual tail recursion and executes both at depth
`200000`. It passes on the current AST-level tail-call lowering.
