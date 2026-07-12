package codegen

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// isUnitType returns true if t represents the unit type.
func isUnitType(t TypeExpr) bool {
	if nt, ok := t.(*NamedType); ok && nt.Name == "Unit" && len(nt.Args) == 0 {
		return true
	}
	if tt, ok := t.(*TupleType); ok && len(tt.Elems) == 0 {
		return true
	}
	return false
}

// genFuncDecl generates a Go function declaration from a MyGO FuncDecl.
func (g *gen) genFuncDecl(d *FuncDecl) (ast.Decl, error) {
	retTypes := g.goReturnTypes(d.Ret, typeParamSet(d.TypeParams))
	retType := ""
	if len(retTypes) == 1 {
		retType = retTypes[0]
	}
	ctx := &egCtx{
		locals:           map[string]string{},
		bindings:         map[string]string{},
		sourceTypes:      map[string]string{},
		mutable:          map[string]bool{},
		typeParams:       typeParamSet(d.TypeParams),
		constraintFuncs:  map[string]string{},
		typeclassMethods: map[string][]egTcBinding{},
		retType:          retType,
		retTypes:         retTypes,
	}
	for _, p := range d.Params {
		gt := g.goType(p.Type, typeParamSet(d.TypeParams))
		ctx.locals[p.Name] = gt
		ctx.sourceTypes[p.Name] = gt
		ctx.bindings[p.Name] = p.Name
	}

	// Process using constraints — add constraint function parameters
	var constraintParamFields []*ast.Field
	for _, c := range d.Using {
		namedImpl, ifc, ok := resolveConstraint(c, g.pkg)
		if !ok || ifc == nil {
			continue
		}
		typeArgs := append([]TypeExpr(nil), c.Args...)
		// For named impls found via BindName, use the impl's InterfaceArgs.
		implSubst := map[string]string{}
		if namedImpl != nil && c.BindName != "" {
			typeArgs = append([]TypeExpr(nil), namedImpl.InterfaceArgs...)
			if len(typeArgs) == 0 {
				typeArgs = append([]TypeExpr(nil), namedImpl.TypeArgs...)
			}
			if len(namedImpl.TypeParams) > 0 {
				for i, tp := range namedImpl.TypeParams {
					if i < len(c.Args) {
						implSubst[tp] = g.goType(c.Args[i], typeParamSet(d.TypeParams))
					}
				}
			}
			for i, arg := range typeArgs {
				typeArgs[i] = substituteTypeExpr(arg, implSubst)
			}
		}
		subst := map[string]string{}
		for i, tp := range ifc.TypeParams {
			if i < len(typeArgs) {
				subst[tp] = g.goType(typeArgs[i], typeParamSet(d.TypeParams))
			}
		}
		namedImplTypeKey := ""
		if namedImpl != nil {
			implTypeArgs := append([]TypeExpr(nil), namedImpl.InterfaceArgs...)
			if len(implTypeArgs) == 0 {
				implTypeArgs = append([]TypeExpr(nil), namedImpl.TypeArgs...)
			}
			for i, arg := range implTypeArgs {
				implTypeArgs[i] = substituteTypeExpr(arg, implSubst)
			}
			namedImplTypeKey = g.implHelperKey(namedImpl, implTypeArgs)
		}
		for _, m := range ifc.Methods {
			paramName := m.Name + "Fn"
			var paramTypes []string
			for _, p := range m.Params {
				paramTypes = append(paramTypes, typeString(p.Type, subst))
			}
			retTypeStr := typeStringReturn(m.Ret, subst)

			if namedImplTypeKey != "" {
				// Named impl: use the helper function name directly.
				// The caller auto-injects the helper function reference.
				fnName := helperFuncName(m.Name, namedImplTypeKey)
				funcTypeStr := typeclassFuncType(paramTypes, retTypeStr)
				constraintParamFields = append(constraintParamFields, &ast.Field{
					Names: []*ast.Ident{ast.NewIdent(paramName)},
					Type:  ast.NewIdent(funcTypeStr),
				})
				ctx.constraintFuncs[m.Name] = paramName
				ctx.typeclassMethods[m.Name] = append(ctx.typeclassMethods[m.Name], egTcBinding{
					Interface:  c.Name,
					TargetType: firstTypeArgString(typeArgs, subst),
					ParamTypes: paramTypes,
					RetType:    retTypeStr,
					DictExpr:   fnName,
				})
			} else {
				funcTypeStr := typeclassFuncType(paramTypes, retTypeStr)
				constraintParamFields = append(constraintParamFields, &ast.Field{
					Names: []*ast.Ident{ast.NewIdent(paramName)},
					Type:  ast.NewIdent(funcTypeStr),
				})
				if _, ok := ctx.constraintFuncs[m.Name]; !ok {
					ctx.constraintFuncs[m.Name] = paramName
				}
				ctx.typeclassMethods[m.Name] = append(ctx.typeclassMethods[m.Name], egTcBinding{
					Interface:  c.Name,
					TargetType: firstTypeArgString(typeArgs, subst),
					ParamTypes: paramTypes,
					RetType:    retTypeStr,
					DictExpr:   paramName,
				})
			}
		}
	}

	// Params
	allParams := make([]*ast.Field, 0, len(d.Params)+len(constraintParamFields))
	for _, p := range d.Params {
		allParams = append(allParams, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(p.Name))},
			Type:  goastTypeExpr(p.Type),
		})
	}
	allParams = append(allParams, constraintParamFields...)
	params := allParams

	// Results
	var results []*ast.Field
	if len(retTypes) == 1 {
		results = []*ast.Field{{Type: goastTypeExpr(d.Ret)}}
	} else if len(retTypes) > 1 {
		if tt, ok := d.Ret.(*TupleType); ok {
			for _, e := range tt.Elems {
				results = append(results, &ast.Field{Type: goastTypeExpr(e)})
			}
		} else {
			for _, rt := range retTypes {
				results = append(results, &ast.Field{Type: ast.NewIdent(rt)})
			}
		}
	}

	// Type params
	tp := typeParamFields(d.TypeParams)

	// Body
	var bodyStmts []ast.Stmt
	if block, ok := d.Body.(*BlockExpr); ok {
		var err error
		bodyStmts, err = g.translateBlockStmts(block, ctx, ctx.retType, ctx.retTypes)
		if err != nil {
			return nil, err
		}
	} else if len(retTypes) == 0 {
		code, _, err := g.translateExpr(d.Body, ctx, ctx.retType)
		if err != nil {
			return nil, err
		}
		bodyStmts = append(bodyStmts, &ast.ExprStmt{X: code})
		bodyStmts = append(bodyStmts, &ast.ReturnStmt{})
	} else if len(retTypes) > 1 {
		if tuple, ok := d.Body.(*TupleLitExpr); ok {
			values := make([]ast.Expr, len(tuple.Elems))
			for i, elem := range tuple.Elems {
				v, _, err := g.translateExpr(elem, ctx, retTypes[i])
				if err != nil {
					return nil, err
				}
				values[i] = v
			}
			bodyStmts = append(bodyStmts, &ast.ReturnStmt{Results: values})
		} else {
			code, _, err := g.translateExpr(d.Body, ctx, ctx.retType)
			if err != nil {
				return nil, err
			}
			bodyStmts = append(bodyStmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
		}
	} else {
		code, _, err := g.translateExpr(d.Body, ctx, ctx.retType)
		if err != nil {
			return nil, err
		}
		bodyStmts = append(bodyStmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
	}

	return astFuncDecl(d.Name, nil, tp, params, results, &ast.BlockStmt{List: bodyStmts}), nil
}

// exprToStmt wraps an ast.Expr as an ast.Stmt for use in statement lists.
func exprToStmt(x ast.Expr) ast.Stmt {
	if x == nil {
		return &ast.ExprStmt{X: ast.NewIdent("_")}
	}
	if s, ok := x.(ast.Stmt); ok {
		return s
	}
	return &ast.ExprStmt{X: x}
}

// tokenFromOp maps MyGO operator strings to Go tokens.
func tokenFromOp(op string) token.Token {
	switch op {
	case "+":
		return token.ADD
	case "-":
		return token.SUB
	case "*":
		return token.MUL
	case "/":
		return token.QUO
	case "==":
		return token.EQL
	case "!=":
		return token.NEQ
	case "<":
		return token.LSS
	case ">":
		return token.GTR
	case "<=":
		return token.LEQ
	case ">=":
		return token.GEQ
	case "&&":
		return token.LAND
	case "||":
		return token.LOR
	default:
		return token.ILLEGAL
	}
}

// astStringLit creates a string literal AST node.
// The value must be a Go string literal (including quotes).
func astStringLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

// astStringLitRaw creates a string literal AST node from a pre-quoted string.
func astStringLitRaw(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: s}
}

// astBoolLit creates a boolean literal AST node.
func astBoolLit(v bool) *ast.Ident {
	if v {
		return ast.NewIdent("true")
	}
	return ast.NewIdent("false")
}

// firstTypeArgString returns the type string of the first type argument.
func firstTypeArgString(args []TypeExpr, subst map[string]string) string {
	if len(args) == 0 {
		return ""
	}
	return typeString(args[0], subst)
}

// substituteTypeExpr substitutes type parameters with concrete types.
func substituteTypeExpr(t TypeExpr, subst map[string]string) TypeExpr {
	switch tt := t.(type) {
	case *NamedType:
		if subst != nil {
			if repl, ok := subst[tt.Name]; ok && len(tt.Args) == 0 {
				return &NamedType{Name: repl}
			}
		}
		if len(tt.Args) == 0 {
			return tt
		}
		args := make([]TypeExpr, len(tt.Args))
		for i, a := range tt.Args {
			args[i] = substituteTypeExpr(a, subst)
		}
		return &NamedType{Name: tt.Name, Args: args}
	case *FuncType:
		params := make([]TypeExpr, len(tt.Params))
		for i, p := range tt.Params {
			params[i] = substituteTypeExpr(p, subst)
		}
		ret := substituteTypeExpr(tt.Ret, subst)
		return &FuncType{Params: params, Ret: ret}
	default:
		return t
	}
}

// findNamedImpl finds an impl declaration by bindName (type name) and interface name.
// If bindName is non-empty, it matches by the impl's type name (e.g. FastEq).
// Otherwise it matches by InterfaceName + typeArgs.
func (g *gen) findNamedImpl(bindName, ifaceName string, typeArgs []TypeExpr) *ImplDecl {
	for _, impl := range g.pkg.Impls {
		if bindName != "" {
			// Named lookup: match by the impl's type Name (e.g. FastEq in "impl FastEq: Eq[Int]").
			implTypeName := implDisplayTypeName(impl.Type)
			if implTypeName != "" && implTypeName == bindName {
				return impl
			}
			// Also check impl.Name (which the parser sets to the interface name).
			if impl.Name == bindName {
				return impl
			}
			continue
		}
		iname := impl.InterfaceName
		if iname == "" {
			iname = impl.Name
		}
		if iname != ifaceName {
			continue
		}
		if len(impl.InterfaceArgs) != len(typeArgs) {
			continue
		}
		return impl
	}
	return nil
}

// implTypeKey computes the base type key from type arguments.
func (g *gen) implTypeKey(args []TypeExpr) string {
	if len(args) == 0 {
		return ""
	}
	var out []string
	for _, a := range args {
		out = append(out, typeKeyFromType(g.goType(a, nil)))
	}
	return "_" + strings.Join(out, "_")
}

// implDisplayTypeName returns the type name of an impl's Type field.
func implDisplayTypeName(t TypeExpr) string {
	if nt, ok := t.(*NamedType); ok {
		return nt.Name
	}
	return ""
}

// hasDuplicateImplForTypeKey checks if there are multiple impls with the same
// typeKey for the same interface (which requires disambiguation).
func (g *gen) hasDuplicateImplForTypeKey(target *ImplDecl, args []TypeExpr) bool {
	if target == nil {
		return false
	}
	ifaceName := target.InterfaceName
	if ifaceName == "" {
		ifaceName = target.Name
	}
	typeKey := g.implTypeKey(args)
	count := 0
	for _, impl := range g.pkg.Impls {
		otherIface := impl.InterfaceName
		if otherIface == "" {
			otherIface = impl.Name
		}
		if otherIface != ifaceName {
			continue
		}
		otherArgs := impl.InterfaceArgs
		if len(otherArgs) == 0 {
			otherArgs = impl.TypeArgs
		}
		if g.implTypeKey(otherArgs) == typeKey {
			count++
		}
	}
	return count > 1
}

// implHelperKey generates a unique key for an impl's type arguments.
// When there are duplicate impls with the same type key (e.g., two impls of
// Eq[Int]), it prefixes the type name to disambiguate.
func (g *gen) implHelperKey(d *ImplDecl, args []TypeExpr) string {
	typeKey := g.implTypeKey(args)
	if d == nil || d.Type == nil || !g.hasDuplicateImplForTypeKey(d, args) {
		return typeKey
	}
	name := implDisplayTypeName(d.Type)
	if name == "" {
		return typeKey
	}
	// If the type name matches the first arg, no prefix needed.
	if len(args) > 0 {
		if nt, ok := args[0].(*NamedType); ok && nt.Name == name {
			return typeKey
		}
	}
	return "_" + typeKeyFromType(name) + typeKey
}

// genFuncDecl exports genFunc for test usage
func (g *gen) genFunc(d *FuncDecl) (ast.Decl, error) {
	return g.genFuncDecl(d)
}

// astNewIdent is an alias for ast.NewIdent for compatibility.
func astNewIdent(name string) *ast.Ident {
	return ast.NewIdent(name)
}
