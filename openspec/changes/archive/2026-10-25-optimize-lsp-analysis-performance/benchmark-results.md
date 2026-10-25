# LSP Performance Benchmark Results

Command (baseline and post-change):

```bash
GOCACHE=/tmp/mygo-lsp-bench go test ./lsp -run '^$' -bench . -benchmem -count 3 -benchtime=1x
```

Environment: linux/amd64, AMD Ryzen 9 PRO 7940HS, Go toolchain from the repository. `-benchtime=1x` keeps the 8000-document fingerprint case bounded; `-count 3` is reported with the median below.

## Baseline (before optimization)

| Benchmark | Median time/op | Bytes/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkAnalysisFingerprint/500` | 21.3 ms | 84,372,288 | 3,127 |
| `BenchmarkAnalysisFingerprint/2000` | 704.0 ms | 1,324,219,256 | 13,684 |
| `BenchmarkAnalysisFingerprint/8000` | 10.76 s | 21,066,999,256 | 56,913 |
| `BenchmarkWorkspacePaths` (200 dirs × 10 files) | 9.05 ms | 256,096 | 428 |
| `BenchmarkBuildSourceIndex` (500 files) | 123.2 ms | 89,182,080 | 324,662 |
| `BenchmarkAnalyzePackageWarm` | 1.74 ms | 1,578,896 | 669 |
| `BenchmarkPackageDiagnostics` | 1.56 ms | 1,606,400 | 1,271 |

Raw runs:

```text
BenchmarkAnalysisFingerprint/500-16    1  15.61 ms/op  84,376,480 B/op  3,149 allocs/op
BenchmarkAnalysisFingerprint/500-16    1  48.55 ms/op  84,363,288 B/op  3,127 allocs/op
BenchmarkAnalysisFingerprint/500-16    1  21.34 ms/op  84,372,288 B/op  3,127 allocs/op
BenchmarkAnalysisFingerprint/2000-16   1 682.81 ms/op 1,324,183,744 B/op 13,617 allocs/op
BenchmarkAnalysisFingerprint/2000-16   1 704.02 ms/op 1,324,219,256 B/op 13,684 allocs/op
BenchmarkAnalysisFingerprint/2000-16   1 774.30 ms/op 1,324,300,792 B/op 13,777 allocs/op
BenchmarkAnalysisFingerprint/8000-16   1  11.53 s/op  21,066,999,256 B/op 57,577 allocs/op
BenchmarkAnalysisFingerprint/8000-16   1  10.76 s/op  21,066,778,328 B/op 56,913 allocs/op
BenchmarkAnalysisFingerprint/8000-16   1  10.36 s/op  21,066,784,368 B/op 56,704 allocs/op
BenchmarkWorkspacePaths-16             1  10.09 ms/op     256,096 B/op     428 allocs/op
BenchmarkWorkspacePaths-16             1   8.50 ms/op     256,096 B/op     428 allocs/op
BenchmarkWorkspacePaths-16             1   9.05 ms/op     256,096 B/op     428 allocs/op
BenchmarkBuildSourceIndex-16           1 123.21 ms/op  89,182,080 B/op 324,662 allocs/op
BenchmarkBuildSourceIndex-16           1 106.24 ms/op  89,180,024 B/op 324,663 allocs/op
BenchmarkBuildSourceIndex-16           1 135.30 ms/op  89,176,192 B/op 324,646 allocs/op
BenchmarkAnalyzePackageWarm-16         1   2.05 ms/op   1,578,896 B/op     669 allocs/op
BenchmarkAnalyzePackageWarm-16         1   1.74 ms/op   1,578,896 B/op     669 allocs/op
BenchmarkAnalyzePackageWarm-16         1   0.90 ms/op   1,576,392 B/op     665 allocs/op
BenchmarkPackageDiagnostics-16         1   1.56 ms/op   1,603,896 B/op   1,267 allocs/op
BenchmarkPackageDiagnostics-16         1   7.32 ms/op   1,606,400 B/op   1,271 allocs/op
BenchmarkPackageDiagnostics-16         1   1.34 ms/op   1,606,400 B/op   1,271 allocs/op
```

Observations:

- Fingerprint cost is dominated by content bytes and grows super-linearly with file count: 2000 documents already allocate 1.3 GB per validation, and 8000 allocate 21 GB.
- `BenchmarkWorkspacePaths` allocates little but runs the full recursive stat walk plus a linear dedupe; it is the baseline for the listing cache change.
- `BenchmarkBuildSourceIndex` re-reads and re-parses all 500 files per call (89 MB, 324k allocations).
- `BenchmarkAnalyzePackageWarm` and `BenchmarkPackageDiagnostics` include the content-based fingerprint read on every warm call.

## Post-change (after optimization)

| Benchmark | Median time/op | Bytes/op | allocs/op | Time vs baseline |
|---|---:|---:|---:|---:|
| `BenchmarkAnalysisFingerprint/500` | 0.30 ms | 222,032 | 2,519 | ~70× faster |
| `BenchmarkAnalysisFingerprint/2000` | 0.77 ms | 973,200 | 10,025 | ~912× faster |
| `BenchmarkAnalysisFingerprint/8000` | 7.51 ms | 4,430,744 | 40,036 | ~1,433× faster |
| `BenchmarkWorkspacePaths` (200 dirs × 10 files) | 0.50 ms | 54,656 | 402 | ~18× faster |
| `BenchmarkBuildSourceIndex` (500 files) | 120.4 ms | 89,180,616 | 324,676 | unchanged (parse-bound) |
| `BenchmarkAnalyzePackageWarm` | 0.22 ms | 54,168 | 484 | ~7.8× faster |
| `BenchmarkPackageDiagnostics` | 0.44 ms | 81,976 | 1,088 | ~3.5× faster |

Allocation deltas versus baseline:

- Fingerprint: 84 MB → 0.22 MB at 500 documents; 1.32 GB → 0.97 MB at 2000; 21.07 GB → 4.43 MB at 8000. Fingerprint cost is now metadata-linear instead of content-linear.
- Workspace paths: 256 KB → 55 KB and 428 → 402 allocs, while time drops from 9.05 ms to 0.50 ms.
- Warm package analysis: 1.58 MB → 54 KB and 669 → 484 allocs.
- Package diagnostics: 1.61 MB → 82 KB and 1,271 → 1,088 allocs.
- Build-source-index is intentionally unchanged in this change: it remains parse-bound and is now reused through the workspace index cache rather than repeated per request.

Raw runs:

```text
BenchmarkAnalysisFingerprint/500-16    1 302.20 µs/op   222,032 B/op   2,519 allocs/op
BenchmarkAnalysisFingerprint/500-16    1 187.10 µs/op   222,032 B/op   2,519 allocs/op
BenchmarkAnalysisFingerprint/500-16    1 342.04 µs/op   222,032 B/op   2,519 allocs/op
BenchmarkAnalysisFingerprint/2000-16   1   1.28 ms/op    973,200 B/op  10,025 allocs/op
BenchmarkAnalysisFingerprint/2000-16   1 762.02 µs/op    973,200 B/op  10,025 allocs/op
BenchmarkAnalysisFingerprint/2000-16   1 772.04 µs/op    973,200 B/op  10,025 allocs/op
BenchmarkAnalysisFingerprint/8000-16   1   4.87 ms/op  4,431,048 B/op  40,039 allocs/op
BenchmarkAnalysisFingerprint/8000-16   1  17.36 ms/op  4,431,160 B/op  40,040 allocs/op
BenchmarkAnalysisFingerprint/8000-16   1   7.51 ms/op  4,430,936 B/op  40,036 allocs/op
BenchmarkWorkspacePaths-16             1   1.11 ms/op     54,656 B/op     402 allocs/op
BenchmarkWorkspacePaths-16             1 415.19 µs/op     54,656 B/op     402 allocs/op
BenchmarkWorkspacePaths-16             1 494.78 µs/op     54,656 B/op     402 allocs/op
BenchmarkBuildSourceIndex-16           1 120.38 ms/op  89,180,616 B/op 324,676 allocs/op
BenchmarkBuildSourceIndex-16           1 146.48 ms/op  89,167,256 B/op 324,659 allocs/op
BenchmarkBuildSourceIndex-16           1 102.39 ms/op  89,172,560 B/op 324,665 allocs/op
BenchmarkAnalyzePackageWarm-16         1 223.92 µs/op     54,168 B/op     484 allocs/op
BenchmarkAnalyzePackageWarm-16         1 223.67 µs/op     54,168 B/op     484 allocs/op
BenchmarkAnalyzePackageWarm-16         1 306.13 µs/op     54,168 B/op     484 allocs/op
BenchmarkPackageDiagnostics-16         1 444.90 µs/op     81,976 B/op   1,088 allocs/op
BenchmarkPackageDiagnostics-16         1 409.89 µs/op     81,976 B/op   1,088 allocs/op
BenchmarkPackageDiagnostics-16         1 652.72 µs/op     82,192 B/op   1,091 allocs/op
```
