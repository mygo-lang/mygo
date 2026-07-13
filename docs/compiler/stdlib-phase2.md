# Standard Library — Phase 2: String Utility Functions

> Implements the second phase of `docs/plan/standard-library.md`. All additions are in `prelude/string.mygo`.

---

## Added Functions

All functions are added to the existing `impl String` block in `prelude/string.mygo`, wrapping Go's `strings` package via inline Go (`go[T]{}`).

| Function | Signature | Go Bridge |
|----------|-----------|-----------|
| `HasPrefix` | `(s: String, prefix: String) -> Bool` | `strings.HasPrefix(s, prefix)` |
| `HasSuffix` | `(s: String, suffix: String) -> Bool` | `strings.HasSuffix(s, suffix)` |
| `Trim` | `(s: String, cutset: String) -> String` | `strings.Trim(s, cutset)` |
| `TrimSpace` | `(s: String) -> String` | `strings.TrimSpace(s)` |
| `TrimPrefix` | `(s: String, prefix: String) -> String` | `strings.TrimPrefix(s, prefix)` |
| `TrimSuffix` | `(s: String, suffix: String) -> String` | `strings.TrimSuffix(s, suffix)` |
| `Split` | `(s: String, sep: String) -> Slice[String]` | `strings.Split(s, sep)` |
| `SplitN` | `(s: String, sep: String, n: Int) -> Slice[String]` | `strings.SplitN(s, sep, n)` |
| `Join` | `(sep: String, elems: Slice[String]) -> String` | `strings.Join(elems, sep)` |
| `Replace` | `(s: String, old: String, new: String, n: Int) -> String` | `strings.Replace(s, old, new, n)` |
| `ReplaceAll` | `(s: String, old: String, new: String) -> String` | `strings.ReplaceAll(s, old, new)` |
| `ToUpper` | `(s: String) -> String` | `strings.ToUpper(s)` |
| `ToLower` | `(s: String) -> String` | `strings.ToLower(s)` |
| `Repeat` | `(s: String, count: Int) -> String` | `strings.Repeat(s, count)` |
| `Index` | `(s: String, substr: String) -> Int` | `strings.Index(s, substr)` |
| `LastIndex` | `(s: String, substr: String) -> Int` | `strings.LastIndex(s, substr)` |
| `Fields` | `(s: String) -> Slice[String]` | `strings.Fields(s)` |

### Implementation Notes

- `Join` parameter order differs from Go: `Join(sep, elems)` vs Go's `strings.Join(elems, sep)`. The MyGo signature places the separator first for consistency with the mental model: "Join with separator X, these elements."
- `Slice[String]` bridges correctly to `[]string` in Go when passed through `go[T]{}` inline blocks.
- All functions are methods on the `impl String` block, so they are called as `String.HasPrefix(s, "http")` or via method syntax `s.HasPrefix("http")` when typeclass dispatch supports it.

## Generated Code

The generated Go functions live in `prelude/zz_string.gen.go` with the standard `String_<FuncName>` mangling convention (e.g., `String_HasPrefix`, `String_Split`).

## Build Verification

```bash
go build ./cmd/mygo/
go build ./prelude/
```
