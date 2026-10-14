## 1. 回归基线

- [x] 1.1 添加从 MyGO 源到生成 Go 类型检查的后缀回归用例，覆盖局部 `let/var` 的全部后缀；确认修复前 `let count = 0i64` 的实际 Go 类型错误被测试捕获。
- [x] 1.2 检查并记录负数的实际 AST 表示，添加有符号最小值、最大值与 UInt64 大值用例；以正确数值和对应 Go 类型作为断言。

## 2. 类型保持实现

- [x] 2.1 在 codegen2 数值表达式入口为显式后缀生成有类型常量，复用现有类型映射并集中后缀处理；验证全部后缀的局部绑定回归通过，新增手写文件不超过 500 行。
- [x] 2.2 处理一元负号与转换的顺序，验证各有符号类型最小值及同后缀算术可编译且结果正确，并确认超范围输入不被静默截断。
- [x] 2.3 补充直接泛型调用、Go `any` 参数动态类型观察、十六/八/二进制及下划线字面量测试；验证生成 Go 的实际类型和值与后缀一致。

## 3. 兼容性与自举验证

- [x] 3.1 添加显式注解、包级绑定、类型化返回、Slice 元素及无后缀默认与上下文用例；验证定向测试均通过且无后缀行为不变。
- [x] 3.2 使用自举工具刷新修改源对应的生成 Go 并构建更新后的编译器；用该编译器生成后缀探针，确认 Go 类型检查或执行通过，差异仅涉及本次修复。
- [x] 3.3 使用任务级可写 GOCACHE 运行 parser2、typeinference2、codegen2 相关回归及项目要求的自举检查和标准示例构建；记录命令、结果与任何已有失败。

## 验证记录

- 修复前 Go 类型检查确认 `let count = 0i64` 生成 `int`；泛型回归产生 `int` 无法作为 `int64` 返回。
- 解析器 `lowerSyntaxUnaryExpr` 将负数构造为 `UnaryExpr("-", NumberExpr(...))`；生成器现将带后缀的完整负数转换为对应类型，最小值回归通过。
- 以下命令均使用 `GOCACHE=/tmp/mygo-explore-gocache`。
- `go test ./internal/mygo/codegen2 -run '^TestNumericSuffix' -count=1`：通过，包含全部后缀、边界、溢出拒绝、泛型、真实 `fmt.Sprintf` 动态类型观察及兼容性用例。
- `go build -o /tmp/mygo-numeric-base ./cmd/mygo` 后同步 `internal/mygo/codegen2`，再构建 `/tmp/mygo-numeric-current` 并再次同步：通过，生成 Go 均由编译器刷新。
- `go test ./internal/mygo/parser2 ./internal/mygo/typeinference2 ./internal/mygo/codegen2 ./internal/mygo/compiler ./cmd/mygo -count=1`：全部通过，包含现有自举集成检查。
- 构建 `/tmp/mygo-numeric-final`，在独立临时模块中运行 CLI `build`：后缀探针通过，输出 `int64 int64 int8 -128`；复制的标准 `examples/main/main.mygo` 构建通过。临时模块构建设置 `GOFLAGS=-buildvcs=false`，避免 VCS 元数据检测错误。
- `git diff --check` 与 `openspec validate fix-bootstrap-numeric-suffix-types --strict`：通过。
