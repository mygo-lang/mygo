# inference.md — HM type inference, unary ops, empty set inference

## HM Type Inference (`internal/mygo/typeinference/`)

A Hindley-Milner (Algorithm W) type inference pass implementing Haskell 98 core HM + typeclass constraints, added as a pre-pass before Go code generation.

### Internal Type Representation
- `MonoType`: sum type with `TVar{ID}`, `TCon{Name, Args}`, `TFunc{Args, Ret, Variadic}`, `TGoPackage{Alias}`, `TUnit`
- `Scheme{Bound []int, Body QualifiedType}` — polymorphic type with optional typeclass predicates
- `Subst map[int]MonoType` — type variable substitution with `Compose`/`ApplyMT`
- `InferState{FreshVarID int}` — fresh variable supply (starts at 1), package metadata, Go import table, and current `TypedInfo`
- `TypeEnv map[string]*Scheme` — variable-to-scheme environment

### Key Files
- `internal/mygo/typeinference/types.go` — core type definitions, `Instantiate`, `Generalize`, substitution application, free variable computation
- `internal/mygo/typeinference/unify.go` — `Unify` with occurs check, `bindVar`, structural equality for all MonoType variants
- `internal/mygo/typeinference/infer.go` — `inferExpr` (Algorithm W), `inferDecl`, `inferFuncDecl`, `inferLetDecl`, full expression coverage

### Expression Coverage
- Literals (Int/Float64/String/Bool) — class-defaulted numeric types; all supported types (`Int`, `Int8`, `UInt8`, `Int16`, `UInt16`, `Int32`, `UInt32`, `Int64`, `UInt`, `UInt64`, `Float32`, `Float64`) resolve as `TCon` in HM
- Ident lookup with let-polymorphism (instantiate scheme → fresh vars per use site)
- Function calls — callee type unified with `(arg_types) -> fresh_ret`
- `if`/`switch` — branch types unified, `while` returns `()`
- Function literals — explicit param/return types registered in body env
- Pipe operators `|>` / `<|` — unified as function application
- Arithmetic (`+`, `-`, `*`, `/`), logical (`&&`, `||`), comparison (`==` etc.)
- Slice/Map/Set literals — element types unified, empty ones accept context
- `None` — resolved as `Option[?a]` with fresh type variable

### Typeclass Constraints
- `==` / `!=` / `<` / `>` / `<=` / `>=` each generate `Eq[operand_type]` predicates
- Predicates collected during inference and stored in `TypedInfo`

### Integration into Compiler Pipeline
- Called from `compiler/generate.go` `Generate()` before codegen
- Produces `TypedInfo` with `ExprTypes`, `BindingSchemes`, and `Predicates`
- Blocking/default path: inference errors stop code generation instead of being silently ignored
- Generator struct uses `typedInfo *typeinference.TypedInfo` during expression lowering to obtain expected and result types
- Go package imports are loaded in `typeinference` so package selectors and function values such as `fmt.Sprint` participate in HM inference

### Key Semantics
- `let`: generalizes inferred type to scheme; subsequent references instantiate fresh vars
- `var`: no generalization, monomorphic mutable binding
- `let _ = ...`: discard form, no binding added to env
- Explicit type annotations unify with inferred type; error on mismatch
- Occurs check prevents infinite types (e.g. `func(x) x(x)`)

### Tests (`internal/mygo/typeinference/infer_test.go`)
- 37 tests covering: literals, ident lookup, let binding, let-polymorphism, occurs check, None inference, if/if-mismatch, function calls, blocks, slice/map/set literals, function literals, comparison with Eq predicate, unification (simple/var/mismatch/function/compose), substitution, generalization, instantiation, free vars, full package inference, type equality

## Unary Operators (`!` and `-`) & Multi-line Strings

### Unary Operators
- **`NOT` keyword removed**, replaced with `!` prefix operator. Syntax: `!expr`.
  - `parser_lex.l`: removed `not` keyword rule; added `!` to single-char token set `[-+*/!<>_=]`.
  - `parser.y`: `NOT postfix_expr` now emits `Op: "!"`; added `'-' postfix_expr %prec NOT` for unary negation; removed `case "not"` keyword mapping; added `case "!": return int(NOT)` symbol mapping. The `NOT` token constant is kept as the yacc token name but now maps to `!` instead of the `not` keyword.
  - `compiler/translate_expr.go`: `switch n.Op` handles `"!"` → `!expr` and `"-"` → `-expr`.
  - `typeinference/infer.go`: `inferPrefix` handles `case "!"` requiring Bool operand (was `case "not"`). Unary `-` already worked.
- `prelude/prelude.mygo` and `examples/data-structure/data-structure.mygo`: all `while not done` → `while !done`.

### Multi-line Strings
- **Python-style `"""..."""`** triple-quoted string syntax added.
  - `parser_lex.l`: added `\"\"\"` lex rule that delegates to `scanMultilineString()` helper.
  - `parser_lexer.go`: added `scanMultilineString()` method that reads until closing `"""` (or EOF); `nextToken()` tokenizer marks triple-quoted input as `tokString` and strips the `"""` delimiters.
  - Standard single-line strings (using `strconv.Unquote`) are unchanged.

## Empty Collection Literal Type Inference Fix

### Parser Bug: `binding_stmt` omits `Type` field
- All three `binding_stmt` alternatives (`LET ident`, `LET bind_pattern`, `VAR ident`) in `parser.y` omitted `Type: p.currentType` from `LetStmt` struct initialization. The top-level `let_decl`/`var_decl` had it correctly.
- This caused `let x: Slice[Int] = []` inside function bodies to lose the type annotation entirely -> `s.Type == nil` in codegen -> empty slice literal got no expected type context -> `translateSliceLit` emitted "could not infer slice element type".
- **Fix**: Added `Type: p.currentType` to all three `binding_stmt` alternatives.

### Stale `currentType` Fix
- `opt_type_annot` empty alternative (`/* empty */`) did not clear `p.currentType`, causing type annotations from enclosing contexts (e.g., function return type `-> String`) to leak into untyped `let` bindings inside the body.
- **Fix**: Added `p.currentType = nil` action to the empty `opt_type_annot` alternative.

### Collection Literal Codegen Fix (Jennifer API)
- `translateMapLit`, `translateSetLit`, `translateEmptyMapLit` used `jen.Lit(jen.Dict{...})` which is invalid — `jen.Dict` implements `Code` via its `render()` method and must be passed to `Values()`, not `Lit()`.
- `translateSliceLit` used `jen.Lit(jen.DictFunc(...)).IndexFunc(...)` for the same reason.
- **Fix**:
  - **Map literals**: `jen.Map(jen.Id(keyType)).Add(jen.Id(valType)).Values(dict)`
  - **Set literals**: `jen.Map(jen.Id(elemType)).Struct().Values(dict)` with `jen.Struct().Values()` for `struct{}{}` values
  - **Empty map**: `jen.Map(jen.Id(keyType)).Add(jen.Id(valType)).Values()`
  - **Slice literals**: `jen.Index().Add(jenTypeExpr(...)).Values(parts...)`

### Key Files Changed
- `internal/mygo/parser/parser.y` — `binding_stmt` + `opt_type_annot` rules; regenerated `parser.go` via `goyacc`.
- `internal/mygo/parser/parser_lex.l` / `lex.yy.go`: unchanged (fix is grammar-only).
- `internal/mygo/compiler/translate_expr.go` — `translateSliceLit`, `translateMapLit`, `translateSetLit`, `translateEmptyMapLit`
