# Standard Library — Phase 1: Basic Type Class Extensions

> Implements the first phase of `docs/plan/standard-library.md`. All additions are in `lib/prelude/prelude.mygo`.

---

## Added Interfaces

### `Ord[A]` — Ordering/Comparison

```mygo
interface Ord[A]
  func Compare(left: A, right: A) -> Int   # <0, ==0, >0
  func Less(left: A, right: A) -> Bool     # left < right
  func Greater(left: A, right: A) -> Bool  # left > right
end
```

- **14 concrete impls**: `Int`, `Int8`, `UInt8`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Int64`, `UInt`, `UInt64`, `Float32`, `Float64`, `String`, `Bool`
- All impls use inline Go (`go[T]{}`) with direct comparison operators (`<`, `>`, `==`)
- `Bool` special case: `false < true` → `!left && right` (Go does not support `<` on bools)
- `Compare` returns `-1` / `0` / `1` via an IIFE in Go

### `Default[A]` — Zero Value

```mygo
interface Default[A]
  func Default() -> A
end
```

- **15 impls**: 12 numeric types → `0` / `0.0`, `Bool` → `false`, `String` → `""`, `Option[A]` → `None`
- Numeric/Bool/String impls return plain MyGO literals (no `go[T]{}` needed)
- `Option[A]` is generic: `impl[A] OptionADefault[A]: Default[Option[A]]`

### `From[A, B]` / `Into[A, B]` — Type Conversion

```mygo
interface From[A, B]
  func From(value: A) -> B
end

interface Into[A, B]
  func Into(value: A) -> B
end
```

- **12 concrete impls** (all `From`; `Into` is separately implemented for each pair):

| Source | Target | Mechanism |
|--------|--------|-----------|
| `Int` | `Int64` | `int64(value)` |
| `Int64` | `Int` | `int(value)` |
| `Int64` | `Float64` | `float64(value)` |
| `Float64` | `Int64` | `int64(value)` |
| `Float32` | `Float64` | `float64(value)` |
| `Float64` | `Float32` | `float32(value)` |
| `Int` | `Float64` | `float64(value)` |
| `Float64` | `Int` | `int(value)` |
| `Int` | `String` | `fmt.Sprint(value)` |
| `String` | `Result[Int, String]` | `strconv.Atoi` → Ok/Err |
| `String` | `Result[Int64, String]` | `strconv.ParseInt` → Ok/Err |
| `String` | `Result[Float64, String]` | `strconv.ParseFloat` → Ok/Err |

- String→numeric conversions use inline Go that wraps `strconv` errors into `Result`
- `import strconv "go:strconv"` added at the top of `prelude.mygo`

---

## Added Standalone Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `OptionToResult` | `(Option[A], errVal: E) -> Result[A, E]` | Convert `None` to `Err` |
| `ResultToOption` | `(Result[A, E]) -> Option[A]` | Discard error value |
| `ResultFlatten` | `(Result[Result[A, E], E]) -> Result[A, E]` | Flatten nested Result |
| `OptionFilter` | `(Option[A], fn: func(A) -> Bool) -> Option[A]` | Filter by predicate |
| `Panic` | `(msg: String) -> ()` | Runtime panic (via `go[()]{panic(msg)}`) |
| `Assert` | `(cond: Bool, msg: String) -> ()` | Assertion (via `go[()]{if !cond {panic(msg)}}`) |

---

## Existing Code Migrated

Two pre-existing `if ... then ... else` single-line expressions in `OptionIEnumerable` (Filter and Find) were migrated to the `=>` arrow form per current syntax convention:

```
# Before
if fn(v) then Some(v) else None

# After
if fn(v) => Some(v) else None
```

---

## Syntax Notes Discovered

During implementation, several syntax rules were clarified:

### `if` Expression

MyGO has no statement-if — only expression-if. **Both branches are mandatory.**

| Form | Syntax | Example |
|------|--------|---------|
| **Inline arrow** | `if cond => a else b` | `if x > 0 => x else 0` |
| **Inline `then`** | `if cond then a else b` | `if x > 0 then x else 0` |
| **Multi-line block** | `if cond NEWLINE ... else ... end` | See below |

```mygo
# Multi-line block form — condition followed by NEWLINE (no then keyword)
if !cond
  Panic(msg)
else
  ()
end
```

Key points:
- The block form uses a **newline** after the condition, NOT `then`
- The `then` keyword is only used in `switch case ... then ... end` block forms
- Single-line `=>` is the recommended/preferred inline form in this codebase

### `switch case` Block Form

```mygo
switch opt
  case Some(v) then       # <-- uses `then`, then newline + block + `end`
    if fn(v) => opt else None
  end
  case None => None       # <-- inline form stays on one line
end
```

### Unit Type `()`

- The `()` literal in generated code returns type `"Unit"` from the codegen
- Using `()` as an if-branch causes the compiler to wrap it in an IIFE with `return voidExpr()`, which produces invalid Go
- **Workaround**: Use `go[()]{}` blocks for void-only control flow that doesn't need to produce a value

---

## Build Verification

```bash
# Regenerate .gen.go from .mygo sources
mygo sync lib/

# Verify library packages compile
go build ./lib/prelude/
go build ./lib/concurrency/

# Verify the compiler itself still builds
go build ./cmd/mygo/

# All pass with no errors.
```
