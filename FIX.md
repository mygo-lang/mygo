# FIX.md

## 现象

`lib/text/parsec/parsec_test.mygo` 生成的 Go 代码里仍然会出现未实例化的泛型参数 `A`，典型位置是：

```go
if v_6, ok := w0_135.(OptionSome[A]); ok {
```

这会导致 `go test ./lib/text/parsec` 失败。

## 已确认的来源

`A` 不是来自 `POptionalWithBetween` 那段测试，而是来自 `ParseCommaSeparatedWords` 这个测试中的 `Get` 调用链：

```mygo
let word = PMap(PMany1(PLetter()), func(rs: Slice[Rune]) -> String
  String.FromRunes(rs)
end)
let words = PSepBy(word, PChar(','))
let w0 = result.value.Get(0)
```

生成器在这里把 `Get` 的返回值保留成了 `Option[A]`，没有进一步实例化成具体类型。

## 已做的修复

- 让 `prelude` 的 dot import 不会被生成器的 unused-import 清理误删
- 修正了部分基础类型映射，使 `Int` 降成 Go 的 `int`
- 增强了部分高阶泛型反推逻辑，开始支持从容器实例里反推类型参数

## 仍需继续修的地方

重点检查并修复 typeclass / 方法分派里的泛型实例化链，尤其是：

- `typeclassSubstForRecv`
- `typeclassSubstForImpl`
- `matchTypeclassHelper`
- `typeclassMethodReturnType`

目标是让 `Slice[string].Get(0)`、`Option[string].Each(...)`、`Len()` 这类调用在生成 Go 时都能落到具体类型，而不是残留 `A`。

## 结论

`A` 的根因是 typeclass 实例化没有完全沿着具体接收者类型传播，而不是测试源码本身显式写出了 `A`。
