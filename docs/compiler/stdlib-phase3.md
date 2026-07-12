# Standard Library — Phase 3: Sorting and Searching (`lib/sort/`)

> Implements the third phase of `docs/plan/standard-library.md`. A new standalone package at `lib/sort/sort.mygo`.

---

## Overview

The `lib/sort/` package provides generic sorting and searching algorithms for `Slice[A]`. Since `Ord[A]` constraint resolution is not available across packages at the MyGo compiler level, all functions accept an explicit comparison/less function parameter rather than relying on typeclass dispatch.

All implementations use `go[T]{}` inline Go embeddings with hand-written algorithms (no dependency on Go's `sort` package).

## Functions

### `Sort[A](slc: Slice[A], less: func(A, A) -> Bool) -> ()`

In-place quicksort implementation.

- Uses Hoare-style partitioning: last element as pivot, two-pointer scan
- Recursive sorting of left and right partitions
- Mutates the input slice in place; returns `()`

### `IsSorted[A](slc: Slice[A], less: func(A, A) -> Bool) -> Bool`

Checks whether a slice is sorted in ascending order according to the `less` predicate.

- Scans adjacent pairs; returns `false` if any `less(slc[i+1], slc[i])` is true
- Returns `true` for empty or single-element slices

### `BinarySearch[A](slc: Slice[A], target: A, compare: func(A, A) -> Int) -> Option[Int]`

Standard binary search on a sorted slice.

- Uses a three-way `compare` function (returns `<0`, `==0`, `>0`)
- Returns `Some(index)` if found, `None` if not present
- Assumes the input slice is already sorted

### `Min[A](a: A, b: A, less: func(A, A) -> Bool) -> A`

Returns the smaller of two values.

### `Max[A](a: A, b: A, greater: func(A, A) -> Bool) -> A`

Returns the larger of two values.

## Implementation Details

All four functions are implemented as standalone generic functions (not methods on any type), using `go[T]{}` inline Go embeddings:

```mygo
func Sort[A](slc: Slice[A], less: func(A, A) -> Bool) -> ()
  go[()] {
    code: """
      var sortFn func([]A, int, int)
      sortFn = func(arr []A, lo, hi int) {
        if lo >= hi { return }
        pivot := arr[hi]
        i := lo
        for j := lo; j < hi; j++ {
          if less(arr[j], pivot) {
            arr[i], arr[j] = arr[j], arr[i]
            i++
          }
        }
        arr[i], arr[hi] = arr[hi], arr[i]
        sortFn(arr, lo, i-1)
        sortFn(arr, i+1, hi)
      }
      sortFn(slc, 0, len(slc)-1)
    """
    in slc = slc
    in less = less
  }
end
```

Key points:
- The closure function `less` is captured from MyGo scope and passed into the Go inline block via `in less = less`, so Go's generic `A` is resolved through the MyGo compile-time type binding
- Recursive helper closures (like `sortFn`) are defined as local variables inside the `code:` string to avoid polluting the package namespace
- `BinarySearch` returns `Option[Int]` — uses `Some[int]()` / `None[int]()` constructors with explicit type arguments

## Files

| File | Role |
|------|------|
| `lib/sort/sort.mygo` | MyGo source — function definitions with `go[T]{}` inline Go |
| `lib/sort/zz_sort.gen.go` | Generated Go — compiler output from `sort.mygo` |

## Build Verification

```bash
mygo sync lib/
go build ./lib/sort/
go build ./cmd/mygo/
```
