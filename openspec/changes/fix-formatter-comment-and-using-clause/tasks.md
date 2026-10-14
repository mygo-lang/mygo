## 1. 分隔结构中的分隔符与注释

- [ ] 1.1 让 item 区域区分首部注释、有效内容与尾部注释：注释独立成行并按 item 缩进渲染，分隔符只追加在有效内容之后，只有 trivia 的区域不追加分隔符；用「wrapped 列表尾随注释连续三次格式化不再增长」与「注释后的 item 保持缩进」的回归测试验证。
- [ ] 1.2 覆盖 struct literal 字段注释与 item 之间注释的用例，断言逐字节输出、二次格式化字节相等，并有意更新因此变化的既有 golden。

## 2. using 约束子句渲染

- [ ] 2.1 在 formatter 渲染中按父级前一个有效 token 计算首部 trivia 的分隔，使 `using ToString[Int]` 保留空格；用 func、impl method、interface 签名三种形态与多约束（`using A[Int], Show`）的 golden 验证。
- [ ] 2.2 断言单次 `FormatSyntaxBlocks` 的输出重新解析后仍存在 `Constraints` 节点、`using` 关键字未被合并，防止再次产生 `usingToString` 类破坏。

## 3. 结构保持校验

- [ ] 3.1 把「格式化前后 AST 等价（忽略位置）」从固定 fixtures 扩展到仓库范围：扫描全部 `.mygo`，对不满足的文件打印清单而不是静默跳过；用扫描输出验证。
- [ ] 3.2 修复后删除 `internal/mygo/typeinference2/env.mygo` 的非幂等例外，确认仓库级幂等扫描不再有例外。

## 4. 集成与交付验证

- [ ] 4.1 按 bootstrap 顺序同步 `internal/mygo/formatter` 生成 Go，使用任务专用可写 GOCACHE 构建；运行 `go test ./internal/mygo/parser2 ./internal/mygo/formatter`，确认既有与新增回归通过。
- [ ] 4.2 运行全量 `go test ./...`，并对仓库全部 `.mygo` 做两遍格式化字节相等扫描与单遍 AST 等价扫描，记录并报告剩余例外。
- [ ] 4.3 更新格式化说明（注释分隔与约束子句渲染），运行 `openspec validate fix-formatter-comment-and-using-clause --strict` 与 `git diff --check` 验证。

