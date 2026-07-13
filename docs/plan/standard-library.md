# Standard Library Enrichment Plan

> Building upon the existing `prelude` and `concurrency` modules, this plan adds standard library components—such as type classes, math, strings, I/O, time, JSON, sorting, and testing—to the MyGo language.
> All components are implemented by bridging the Go standard library via `go[T]{}` inline Go embeddings. **Proceed sequentially, starting with Phase 1.**

---

## General Principles

- All new code is implemented using `go[T] { code: "..."; in x = expr; type T = ... }` inline Go embeddings, following the established patterns in `lib/concurrency/`.
- Independent library packages are placed in `lib/<name>/`, with one `.mygo` file per functional module → generating `zz_<basename>.gen.go`.
- Use `import x "go:<pkg>"` to bridge the Go standard library.
- Prioritize `Option[A]` and `Result[A, E]` over native Go errors to ensure MyGo type safety.
- Use `using` constraints for all generic operations.
- All new `.mygo` files reside under `lib/` and do not intrude upon the `prelude` (unless they involve fundamental type classes or operations on basic types).
- String utility functions are placed directly in `prelude/string.mygo` (since `String` is a primitive type).
- Compilation verification is required after each phase: `go build ./cmd/mygo/` + test generation.

---

## Phase 1: Basic Type Class Extensions (Prelude Enhancement)

**Goal**: Add core type classes to `prelude/prelude.mygo` to provide a foundation of constraints for all subsequent generic libraries. ### 1.1 `Ord[A]` (Ordering/Comparison)

```mygo
interface Ord[A]
func Compare(left: A, right: A) -> Int   # <0, ==0, >0
func Less(left: A, right: A) -> Bool     # left < right (default based on Compare)
func Greater(left: A, right: A) -> Bool  # left > right (default based on Compare)
end
```

* Implemented for all 12 numeric types + String + Bool (all using inline Go comparison operators)
* Bool comparison: `false < true`
* `Less`/`Greater` provide default implementations based on `Compare` (must be explicitly provided for each type in the standard library, or implemented via inlining)

### 1.2 Default[A] (Zero Value/Default Value)
```mygo
interface Default[A]
func Default() -> A
end
```

* Implemented for all numeric types (returns 0/0.0)
* Bool → `false`, String → `""`
* Option[A] → `None`

### 1.3 Into[A, B] / From[A, B] (Type Conversion)
```mygo
interface From[A, B]
func From(value: A) -> B
end

interface Into[A, B]
func Into(value: A) -> B
end
```

* Mutual conversion between all numeric types (Int ↔ Int64 ↔ Float64, etc.)
* Conversion from String to numeric types (bridged via `strconv` + returns `Result`)
* `Into` and `From` are reciprocal; implementing `From` automatically provides `Into` (via blanket impl)

### 1.4 Completion of Option/Result Helper Functions
* OptionToResult[A, E](Option[A], errVal: E) -> Result[A, E]
* ResultToOption[A, E](Result[A, E]) -> Option[A]
* ResultFlatten[A, E](Result[Result[A, E], E]) -> Result[A, E]
* OptionFilter[A](Option[A], fn: func(A) -> Bool) -> Option[A]

### 1.5 `panic` / `assert` Utility Functions
```mygo
func Panic(msg: String) -> ()
go[()] { code: `panic(msg)` }
end

func Assert(cond: Bool, msg: String) -> ()
if !cond then Panic(msg)
end
```

Files involved:

* prelude/prelude.mygo — Adding new interfaces, implementations for `Ord`/`Default`/`Into`/`From`, and helper functions
* prelude/string.mygo — String-specific `From` implementations (if applicable)

## Phase 2: String Utility Functions (Enhancements to Prelude)

Goal: Add comprehensive string manipulation functions to `prelude/string.mygo` that wrap the Go `strings` package. | Function Name | Signature |
|------|-----|
| HasPrefix | (s: String, prefix: String) -> Bool |
| HasSuffix | (s: String, suffix: String) -> Bool |
| Trim | (s: String, cutset: String) -> String |
| TrimSpace | (s: String) -> String |
| TrimPrefix | (s: String, prefix: String) -> String |
| TrimSuffix | (s: String, suffix: String) -> String |
| Split | (s: String, sep: String) -> Slice[String] |
| SplitN | (s: String, sep: String, n: Int) -> Slice[String] |
| Join | (elems: Slice[String], sep: String) -> String |
| Replace | (s: String, old: String, new: String, n: Int) -> String |
| ReplaceAll | (s: String, old: String, new: String) -> String |
| ToUpper | (s: String) -> String |
| ToLower | (s: String) -> String |
| Repeat | (s: String, count: Int) -> String |
| Index | (s: String, substr: String) -> Int |
| LastIndex | (s: String, substr: String) -> Int |
| Fields | (s: String) -> Slice[String] |

Note: The first parameter of `strings.Join` is `Slice[String]`; it is necessary to verify that the `Slice` bridges correctly when passed as an argument to `go[T]{}`.

Files involved:

prelude/string.mygo — Append all new functions (the `strings` import is already present)

## Phase 3: lib/sort/ — Sorting and Searching
Goal: Provide generic sorting and binary search. Depends on Phase 1.1 `Ord[A]`.

Implementation approach: Use `go[T]{}` to inline a call to `sort.Slice` or manually implement quicksort.

Files involved:

lib/sort/sort.mygo (new file)
