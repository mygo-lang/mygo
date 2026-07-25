# Bootstrap compiler
## Paths
* `internal/mygo/parser` -> `internal/mygo/parser2`
* `internal/mygo/ast` -> `internal/mygo/ast2`
* `internal/mygo/typeinference` -> `internal/mygo/typeinference2`
* `internal/mygo/codegen` -> `internal/mygo/codegen2`

## Commands
* To build a MyGO package with compiler2 `go run ./cmd/mygo --bootstrap sync <package path>`
* To build a MyGO package with compiler1 `./mygo sync <package path>`
