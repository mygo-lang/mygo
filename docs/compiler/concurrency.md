# concurrency.md

> Notes for the concurrency library and the compiler support added around it.

## Summary

- `lib/concurrency/` introduces channel-oriented helpers built on top of inline Go embedding.
- The compiler now understands Go channel type strings in both directions:
  - `chan T`
  - `chan<- T`
  - `<-chan T`
- The compiler also maps the language types `Chan[T]`, `SendChan[T]`, and `RecvChan[T]` to valid Go channel syntax during:
  - inline Go translation
  - HM/type-string rendering
  - generated Go type emission

## Library Surface

- `MakeChan[T](buffer: Int) -> Chan[T]`
- `MakeChanUnbuffered[T]() -> Chan[T]`
- `AsSend[T](ch: Chan[T]) -> SendChan[T]`
- `AsRecv[T](ch: Chan[T]) -> RecvChan[T]`
- `Spawn(fn: func() -> ()) -> ()`

## Channel Capabilities

- `Chan[T]` supports both readable and writable channel interfaces.
- `SendChan[T]` supports send-only operations and channel metadata.
- `RecvChan[T]` supports receive-only operations and channel metadata.

## Compiler Notes

- `generate.go` now keeps interface-only HKT emission scoped to prelude output so package generation stays valid for non-prelude packages.
- `generate.go` also inserts the prelude import block even when a generated file has no existing import section.
- Inline Go tests now cover:
  - plain `chan T`
  - directional channel types
  - generated Go parsing correctness
