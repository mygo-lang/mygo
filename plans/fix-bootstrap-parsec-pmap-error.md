# Fix Bootstrap Compilation Error: PMap return type mismatch

## Error

```
go run ./cmd/mygo --bootstrap sync lib
bootstrap infer /mnt/data-svr1-raid/xyh/code/go/mygo/lib/text/parsec: {function PMap return type mismatch: cannot unify B with Option}
exit status 1
```

## Trigger Code

`lib/text/parsec/parsec.mygo` — the `PMap` function:

```mygo
func PMap[A, B](p: Parser[A], f: func(A) -> B) -> Parser[B]
  Parser[B] {
    Run: func(state: State) -> Reply[B]
      let r = p.Run(state)
      if !r.Ok then
        Reply[B] {
          Ok: false,
          Consumed: r.Consumed,
          Value: Zero(),
          State: r.State,
          Error: r.Error,
        }
      else
        Reply[B] {
          Ok: true,
          Consumed: r.Consumed,
          Value: f(r.Value),
          State: r.State,
          Error: EmptyError(r.State.Position),
        }
      end
    end
  }
end
```

## Error Source

The error originates at `typeinference2/infer.mygo:76-80`:

```mygo
let checked = unify(inferredType, retType, r.Subst)
...
case Err(msg) => Err("function " + name + " return type mismatch: " + msg)
```

The inner unify error is `cannot unify B with Option`. In `monoString`:
- `B` = `TCon("B", [])` (a concrete type constructor)
- `Option` = `TCon("Option", [...])`

## Root Cause Analysis

### How `PMap` is predeclared

In `inferFuncDecl` (`infer.mygo:62-84`):
- `retType = typeFromASTWithParams(Parser[B], ["A","B"])` = `TCon("Parser", [TVar(-2)])` — B correctly becomes `TVar(-2)` (type param ID).
- `fnType = TFunc([Parser[TVar(-1)], func(TVar(-1)) -> TVar(-2)], Parser[TVar(-2)])`
- `envWithSig` stores `PMap` with `Bound: [-1, -2]`.

### How the body is inferred

The body is `Parser[B] { Run: func(state: State) -> Reply[B] ... end }`.

1. **Parser struct literal** — parsed as `GenericStructLitExpr("Parser", [NamedType("B", [])], [Run field])`.

2. **`inferGenericStructLit`** (`infer.mygo:223-229`) calls `inferStructLit` for field checking, then returns:
   ```
   TCon("Parser", typeArgsFromAST([NamedType("B", [])]))
   ```
   `typeArgsFromAST` uses `typeFromAST` (NOT `typeFromASTWithParams`), so `B` becomes `TCon("B", [])` — a **concrete type constructor**, not `TVar(-2)`.

3. The body inferred type is therefore `TCon("Parser", [TCon("B", [])])`.

### The unification

```
unify(TCon("Parser", [TCon("B", [])]),  // inferred body type
      TCon("Parser", [TVar(-2)]),        // declared return type
      r.Subst)
```

Unifying the args: `TCon("B", [])` vs `TVar(-2)`. `TVar(-2)` can be bound to anything, so this should succeed. **But the error says "cannot unify B with Option"**, meaning something in `r.Subst` has already bound `TVar(-2)` to `TCon("Option", ...)`.

### Where `TVar(-2)` gets incorrectly bound

During body inference, the inner function literal `func(state: State) -> Reply[B] ... end` is inferred. The function literal's return type annotation `Reply[B]` is converted via `typeFromAST` (line 207 of `inferFuncLit`), giving `TCon("Reply", [TCon("B", [])])`.

Inside the function body, struct literals like `Reply[B] { ..., Error: r.Error, ... }` are also `GenericStructLitExpr("Reply", [B], [...])`, returning `TCon("Reply", [TCon("B", [])])`.

The critical issue: `inferFuncLit` unifies the body type against the declared return type. Both are `TCon("Reply", [TCon("B", [])])` — so this succeeds with no substitution needed. The function literal type is `func(State) -> Reply[B]` where `B` is the concrete `TCon("B", [])`.

**However**, the `inferStructLitFields` function does NOT check field types against the struct definition. The `Error` field of `Reply[A]` has type `Option[ParseError]`. When `r.Error` (type `Option[ParseError]`) is assigned to the `Error` field, no type checking occurs — but the accumulated substitution from inferring `r.Error` is carried upward.

The substitution accumulated from inferring `r.Error` → `TCon("Option", [TCon("ParseError", [])])` is composed with the rest. When this substitution is later used to `applySubst(r.Subst, TVar(-2))`, if some intermediate binding connected `TVar(-2)` to `Option`-related types, the final unification fails.

### Likely mechanism

The struct field inference doesn't validate field types against the struct schema. But the `Value: Zero()` and `Value: f(r.Value)` fields have different inferred types (`TVar(fresh)` vs `TVar(-2)`). If the if-else branches unify the struct literal types including their field values (which they don't — `inferGenericStructLit` discards field types), or if some other path connects the type variables, the error occurs.

**Most likely cause**: The `inferGenericStructLit` returns `TCon(typeName, typeArgsFromAST(typeArgs))` — using `typeFromAST` which doesn't resolve type parameters. This means `B` in `Parser[B]` becomes `TCon("B", [])` (a free-standing type constructor) instead of being linked to the enclosing function's type parameter `TVar(-2)`. When the body's accumulated substitution inadvertently binds `TVar(-2)` through some other unification path (e.g., through field type inference leakage), the final return type check fails.

## Fix Options

### Option A: Make `inferGenericStructLit` resolve type params

Pass the enclosing type parameters into `inferGenericStructLit` so it uses `typeFromASTWithParams` instead of `typeFromAST` for the type arguments. This would produce `TCon("Parser", [TVar(-2)])` instead of `TCon("Parser", [TCon("B", [])])`, matching the declared return type exactly.

**Problem**: `inferGenericStructLit` is called from `inferExprInner` which doesn't have type parameter context. The type parameters are only available in `inferFuncDecl`.

### Option B: Make `inferFuncDecl` resolve type params in body check

After inferring the body, when checking against `retType`, also substitute the type param IDs with fresh variables before inference. This is essentially what `predeclareFunctions` already does for the scheme — the issue is that `inferFuncDecl` doesn't instantiate its own type params before inferring the body.

**Approach**: In `inferFuncDecl`, before calling `inferExpr(body, bodyEnv, state)`, add the type parameter mappings to the environment so that `B` in `typeFromAST` resolves to the correct `TVar`.

**Implementation**:
1. In `inferFuncDecl`, after building `bodyEnv`, add entries for each type parameter:
   ```
   For each tps[i] with ID -(i+1):
     envPut(bodyEnv, tps[i], Scheme { Bound: [], Body: TVar(-(i+1)) })
   ```
2. This way, when `typeFromAST` encounters `B` in `Reply[B]` or `Parser[B]` inside the function body, it resolves to `TVar(-2)` instead of `TCon("B", [])`.

**But wait**: `typeFromAST` doesn't look up names in the environment — it directly maps AST type names to `TCon`. So adding to the env won't help `typeFromAST`.

### Option C (Most Likely Fix): Fix `inferGenericStructLit` to propagate type arg types

The real issue is that `inferGenericStructLit` discards the inferred field types and uses `typeArgsFromAST(typeArgs)` which ignores type parameters. The fix should make the struct literal return type consistent with how the declared return type was constructed.

**Best approach**: When the body of `inferFuncDecl` is a struct literal (generic or not), unify the inferred struct literal type with `retType` — but first, ensure the struct literal type uses the same type variable representation.

Actually, the simplest fix is to **make `inferFuncDecl` not rely on the body's inferred type matching `retType` structurally**. Instead, when the body is a struct literal, check that the struct literal's fields are type-correct against the struct definition.

### Option D (Simplest Fix): Unify struct literal type args with retType args

In `inferGenericStructLit`, instead of using `typeArgsFromAST(typeArgs)` (which loses type parameter binding), the function should check that the type arguments are consistent with the struct's type parameters and produce the correct `TApp`/`TCon` representation.

But the fundamental issue is that `typeFromAST` in struct literal contexts doesn't know about the enclosing function's type parameters.

### Recommended Fix: Option B — Add type param bindings in bodyEnv

The cleanest fix is to make the type parameter names resolvable during body inference. Since `typeFromAST` doesn't use the env, we need to change how struct literals resolve their type arguments.

**Concrete change in `inferFuncDecl`** (`infer.mygo:62-84`):

Before calling `inferExpr(body, bodyEnv, state)`, extend `bodyEnv` with type parameter bindings so that identifiers like `B` are recognized as type variables:

```mygo
# Add type parameter bindings to bodyEnv so B resolves to TVar(-2) etc.
let bodyEnvWithTypeParams = addTypeParamBindings(bodyEnv, tps)
let inferred = inferExpr(body, bodyEnvWithTypeParams, state)
```

But since `typeFromAST` doesn't consult the env, this alone won't work. The actual fix needs to happen at the struct literal inference level.

**The real fix**: Change `inferGenericStructLit` (or add a variant) that accepts type parameter mappings and uses `typeFromASTWithParams` for the type arguments. Then in `inferFuncDecl`, when the body is a `GenericStructLitExpr`, pass the type parameters through.

Alternatively, and more simply: **make the return type check in `inferFuncDecl` more lenient by not requiring exact structural match on type arguments**. Since `TVar(-2)` is universally quantified and can be bound to anything, the unification should succeed. The fact that it doesn't means `TVar(-2)` is being constrained elsewhere.

### Option E (Debugging first): Add tracing to find exact substitution path

Before fixing, add debug logging to see:
1. What `r.Subst` contains after body inference
2. What `applySubst(r.Subst, TVar(-2))` resolves to
3. Which step in the body inference binds `TVar(-2)`

This would pinpoint the exact mechanism of the incorrect binding.

## Files Involved

| File | Role |
|------|------|
| `internal/mygo/typeinference2/infer.mygo:62-84` | `inferFuncDecl` — return type check |
| `internal/mygo/typeinference2/infer.mygo:204-221` | `inferFuncLit` — uses `typeFromAST` not `typeFromASTWithParams` |
| `internal/mygo/typeinference2/infer.mygo:223-229` | `inferGenericStructLit` — uses `typeArgsFromAST` |
| `internal/mygo/typeinference2/infer.mygo:952-970` | `inferStructLit`/`inferStructLitFields` — no field type checking |
| `internal/mygo/typeinference2/env.mygo:20-36` | `typeFromASTWithParams` — resolves type params to TVars |
| `internal/mygo/typeinference2/env.mygo:88-94` | `typeArgsFromAST` — does NOT resolve type params |
| `internal/mygo/typeinference2/unify.mygo` | Unification logic |
| `lib/text/parsec/parsec.mygo:84-107` | `PMap` function definition |

## Next Steps

1. Add debug tracing to `inferFuncDecl` to dump `r.Subst` and `applySubst(r.Subst, TVar(-2))` when the error occurs
2. Identify which step in body inference binds `TVar(-2)` to `Option`
3. Apply the appropriate fix based on findings
