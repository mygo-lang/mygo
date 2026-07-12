package codegen

import (
	"go/ast"
	"go/token"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func typeArgExprsFromExpected(expected string) []ast.Expr {
	_, args := splitTypeArgs(expected)
	if len(args) == 0 {
		return nil
	}
	out := make([]ast.Expr, len(args))
	for i, a := range args {
		out[i] = goTypeExprFromString(a)
	}
	return out
}

// translateCall handles function/method calls.
func (g *gen) translateCall(n *CallExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	// Check for IdentExpr callee — handles Some, None, Ok, Err, func calls
	if id, ok := n.Callee.(*IdentExpr); ok {
		switch id.Name {
		case "Some", "None", "Ok", "Err":
			args := make([]ast.Expr, len(n.Args))
			for i, a := range n.Args {
				ac, _, _ := g.translateExpr(a, ctx, "")
				args[i] = ac
			}
			useExpected := expected
			if useExpected == "" {
				useExpected = ctx.retType
			}
			typeArgExprs := typeArgExprsFromExpected(useExpected)
			var fun ast.Expr = ast.NewIdent(id.Name)
			if len(typeArgExprs) > 0 {
				if len(typeArgExprs) == 1 {
					fun = &ast.IndexExpr{X: fun, Index: typeArgExprs[0]}
				} else {
					fun = &ast.IndexListExpr{X: fun, Indices: typeArgExprs}
				}
			}
			return &ast.CallExpr{Fun: fun, Args: args}, useExpected, nil
		}
		// Auto-inject constraint function args for functions with using clauses.
		// E.g., same(1, 2) → same(1, 2, Equals_fasteq_int) when same has using.
		if fnDecl, ok := g.pkg.Funcs[id.Name]; ok && len(fnDecl.Using) > 0 {
			args := make([]ast.Expr, len(n.Args))
			for i, a := range n.Args {
				ac, _, _ := g.translateExpr(a, ctx, "")
				args[i] = ac
			}
			for _, c := range fnDecl.Using {
				// If BindName is set, find the named impl directly.
				var namedImpl *ImplDecl
				var ifc *InterfaceDecl
				var ok bool
				if c.BindName != "" {
					namedImpl = g.findNamedImpl(c.BindName, c.Name, c.Args)
					if namedImpl != nil {
						ifaceName := namedImpl.InterfaceName
						if ifaceName == "" {
							ifaceName = namedImpl.Name
						}
						ifc = g.pkg.Interfaces[ifaceName]
					}
				} else {
					namedImpl, ifc, ok = resolveConstraint(c, g.pkg)
					if !ok {
						ifc = g.pkg.Interfaces[c.Name]
					}
				}
				if ifc == nil {
					continue
				}
				// Compute type substitution for the constraint's type args.
				implSubst := map[string]string{}
				typeArgs := append([]TypeExpr(nil), c.Args...)
				if namedImpl != nil && c.BindName != "" {
					typeArgs = append([]TypeExpr(nil), namedImpl.InterfaceArgs...)
					if len(typeArgs) == 0 {
						typeArgs = append([]TypeExpr(nil), namedImpl.TypeArgs...)
					}
					if len(namedImpl.TypeParams) > 0 {
						for i, tp := range namedImpl.TypeParams {
							if i < len(c.Args) {
								implSubst[tp] = typeString(c.Args[i], nil)
							}
						}
					}
					for i, arg := range typeArgs {
						typeArgs[i] = substituteTypeExpr(arg, implSubst)
					}
				}
				namedImplTypeKey := ""
				if namedImpl != nil {
					implTypeArgs := append([]TypeExpr(nil), namedImpl.InterfaceArgs...)
					if len(implTypeArgs) == 0 {
						implTypeArgs = append([]TypeExpr(nil), namedImpl.TypeArgs...)
					}
					namedImplTypeKey = g.implHelperKey(namedImpl, implTypeArgs)
				}
				for _, m := range ifc.Methods {
					if namedImplTypeKey != "" {
						// Named impl: inject the helper function directly.
						args = append(args, ast.NewIdent(helperFuncName(m.Name, namedImplTypeKey)))
					} else {
						// Anonymous impl: check caller context for constraint binding.
						if bindings, ok := ctx.typeclassMethods[m.Name]; ok && len(bindings) > 0 {
							args = append(args, ast.NewIdent(bindings[0].DictExpr))
						} else if helper, ok := ctx.constraintFuncForMethod(m.Name); ok {
							args = append(args, ast.NewIdent(helper))
						} else {
							args = append(args, ast.NewIdent(helperFuncName(m.Name, typeKeyFromType(""))))
						}
					}
				}
			}
			retType := g.goReturnType(fnDecl.Ret, ctx.typeParams)
			if expected != "" {
				retType = expected
			}
			return &ast.CallExpr{Fun: ast.NewIdent(sanitizeIdent(id.Name)), Args: args}, retType, nil
		}
		// Regular function call — check pkg.Funcs for return type
		var callee ast.Expr = ast.NewIdent(sanitizeIdent(id.Name))
		// Check for constraint function call (e.g., show(value) → showFn(value))
		if fn, ok := ctx.constraintFuncs[id.Name]; ok && len(n.Args) > 0 {
			args := make([]ast.Expr, len(n.Args))
			for i, a := range n.Args {
				ac, _, _ := g.translateExpr(a, ctx, "")
				args[i] = ac
			}
			retType := ctx.retType
			if expected != "" {
				retType = expected
			}
			return &ast.CallExpr{Fun: ast.NewIdent(fn), Args: args}, retType, nil
		}

		args := make([]ast.Expr, len(n.Args))
		for i, a := range n.Args {
			ac, _, _ := g.translateExpr(a, ctx, "")
			args[i] = ac
		}
		retType := ""
		if fnDecl, ok := g.pkg.Funcs[id.Name]; ok {
			retType = g.goReturnType(fnDecl.Ret, ctx.typeParams)
		}
		if expected != "" {
			retType = expected
		}
		// For Some/None/Ok/Err, add type args from expected or retType
		useExpected := expected
		if useExpected == "" {
			useExpected = ctx.retType
		}
		switch id.Name {
		case "Some", "None":
			if base, tas := splitTypeArgs(useExpected); base == "Option" && len(tas) > 0 {
				ta := make([]ast.Expr, len(tas))
				for i, a := range tas {
					ta[i] = goTypeExprFromString(a)
				}
				if len(ta) == 1 {
					callee = &ast.IndexExpr{X: ast.NewIdent(id.Name), Index: ta[0]}
				}
			}
		case "Ok", "Err":
			if base, tas := splitTypeArgs(useExpected); base == "Result" && len(tas) == 2 {
				ta := make([]ast.Expr, len(tas))
				for i, a := range tas {
					ta[i] = goTypeExprFromString(a)
				}
				callee = &ast.IndexListExpr{X: ast.NewIdent(id.Name), Indices: ta}
			}
		}
		return &ast.CallExpr{Fun: callee, Args: args}, retType, nil
	}
	// Field access call: x.method(args) or Enum.Variant(args)
	if field, ok := n.Callee.(*FieldExpr); ok {
		// Handle Ref.value() — dereference pointer in call context
		if field.Field == "value" && len(n.Args) == 0 {
			baseExpr, baseType, _ := g.translateExpr(field.Expr, ctx, "")
			bt := strings.TrimSpace(baseType)
			if strings.HasPrefix(bt, "Ref[") && strings.HasSuffix(bt, "]") {
				inner := bt[4 : len(bt)-1]
				return &ast.UnaryExpr{Op: token.MUL, X: baseExpr}, inner, nil
			}
			if strings.HasPrefix(bt, "*") {
				return &ast.UnaryExpr{Op: token.MUL, X: baseExpr}, bt[1:], nil
			}
		}
		// Check for Ref.new
		if id, ok := field.Expr.(*IdentExpr); ok && id.Name == "Ref" && field.Field == "new" {
			if len(n.Args) == 1 {
				arg, argType, _ := g.translateExpr(n.Args[0], ctx, "")
				ptrType := "*" + argType
				// If arg is a function call, wrap in IIFE: func() *T { v := expr; return &v }()
				if _, ok := n.Args[0].(*CallExpr); ok {
					fn := &ast.FuncLit{
						Type: &ast.FuncType{
							Params:  &ast.FieldList{},
							Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent(ptrType)}}},
						},
						Body: &ast.BlockStmt{
							List: []ast.Stmt{
								&ast.AssignStmt{
									Lhs: []ast.Expr{ast.NewIdent("__ref_tmp")},
									Rhs: []ast.Expr{arg},
									Tok: token.DEFINE,
								},
								&ast.ReturnStmt{Results: []ast.Expr{&ast.UnaryExpr{Op: token.AND, X: ast.NewIdent("__ref_tmp")}}},
							},
						},
					}
					return &ast.CallExpr{Fun: fn}, ptrType, nil
				}
				return &ast.UnaryExpr{Op: token.AND, X: arg}, ptrType, nil
			}
		}
		if id, ok := field.Expr.(*IdentExpr); ok {
			// Check for inherent static method call: Type.method(args)
			if methods, ok := g.inherentMethods[id.Name]; ok {
				if method, ok := methods[field.Field]; ok && !method.HasReceiver {
					args := make([]ast.Expr, len(n.Args))
					for i, a := range n.Args {
						ac, _, _ := g.translateExpr(a, ctx, "")
						args[i] = ac
					}
					fnName := inherentMethodName(id.Name, method.Func.Name)
					retType := g.goReturnType(method.Func.Ret, ctx.typeParams)
					return &ast.CallExpr{Fun: ast.NewIdent(fnName), Args: args}, retType, nil
				}
			}
			// Check if it's an enum constructor call (Enum.Variant)
			if g.variantByName[field.Field] != "" {
				args := make([]ast.Expr, len(n.Args))
				for i, a := range n.Args {
					ac, _, _ := g.translateExpr(a, ctx, "")
					args[i] = ac
				}
				variantType := variantNameForEnum(id.Name, field.Field)
				return &ast.CallExpr{Fun: ast.NewIdent(variantType), Args: args}, variantType, nil
			}
			// Imported method call: pkg.Func()
			if g.importAliases[id.Name] != "" {
				path := g.importAliases[id.Name]
				// For MyGo imports (not prefixed with "go:"), check exported status
				if !strings.HasPrefix(path, "go:") && !isExportedIdent(field.Field) {
					return nil, "", common.ErrorAtPos(field.Line, field.Column, "cannot refer to unexported symbol %s.%s", id.Name, field.Field)
				}
				// For Go imports, check function signature arity
				if strings.HasPrefix(path, "go:") {
					goPath := importPathForGo(path)
					sigs, err := loadGoPackageSigs(goPath)
					if err == nil && sigs != nil && sigs.funcs != nil {
						if sig, ok := sigs.funcs[field.Field]; ok {
							minArgs := len(sig.params)
							variadic := len(sig.params) > 0 && strings.HasPrefix(sig.params[len(sig.params)-1], "...")
							if variadic {
								minArgs--
							}
							if len(n.Args) < minArgs || (!variadic && len(n.Args) != len(sig.params)) {
								return nil, "", common.ErrorAtPos(field.Line, field.Column, "call type mismatch for %s.%s: expected %d args, got %d", id.Name, field.Field, len(sig.params), len(n.Args))
							}
							callee := ast.NewIdent(id.Name + "." + field.Field)
							args := make([]ast.Expr, len(n.Args))
							for i, a := range n.Args {
								ac, _, _ := g.translateExpr(a, ctx, "")
								args[i] = ac
							}
							retType := goSigReturnType(sig.ret)
							if expected != "" {
								retType = expected
							}
							return &ast.CallExpr{Fun: callee, Args: args}, retType, nil
						}
					}
				}
				callee := ast.NewIdent(id.Name + "." + field.Field)
				args := make([]ast.Expr, len(n.Args))
				for i, a := range n.Args {
					ac, _, _ := g.translateExpr(a, ctx, "")
					args[i] = ac
				}
				return &ast.CallExpr{Fun: callee, Args: args}, expected, nil
			}
		}
		base, bt, _ := g.translateExpr(field.Expr, ctx, "")
		args := make([]ast.Expr, len(n.Args))
		for i, a := range n.Args {
			ac, _, _ := g.translateExpr(a, ctx, "")
			args[i] = ac
		}
		// Check for inherent method call: receiverType.method(args...) → receiverType_method(args..., receiver)
		recvTypeName := baseNamedType(bt)
		if recvTypeName != "" {
			if methods, ok := g.inherentMethods[recvTypeName]; ok {
				if method, ok := methods[field.Field]; ok && method.HasReceiver {
					fnName := inherentMethodName(recvTypeName, method.Func.Name)
					allArgs := append([]ast.Expr{base}, args...)
					callee := ast.NewIdent(fnName)
					retType := g.goReturnType(method.Func.Ret, ctx.typeParams)
					return &ast.CallExpr{Fun: callee, Args: allArgs}, retType, nil
				}
			}
		}
		if helper, retType, ok := preludeCollectionMethod(field.Field, bt, args); ok {
			allArgs := append([]ast.Expr{base}, args...)
			return &ast.CallExpr{Fun: helper, Args: allArgs}, retType, nil
		}
		// Check for typeclass method call: value.show() → show_type() or showFn()
		if ifaceName, ok := g.interfaceByMethod[field.Field]; ok {
			if iface := g.pkg.Interfaces[ifaceName]; iface != nil {
				// First check if there's a constraint function in scope (from `using`)
				if fnName, ok := ctx.constraintFuncForMethod(field.Field); ok {
					allArgs := append([]ast.Expr{base}, args...)
					retType := g.typeclassMethodReturnType(iface, field.Field, bt)
					return &ast.CallExpr{Fun: ast.NewIdent(fnName), Args: allArgs}, retType, nil
				}
				// Otherwise use the best matching impl helper function.
				helperName, retType, ok := g.matchTypeclassHelper(ifaceName, field.Field, bt)
				if !ok {
					typeKey := typeKeyFromType(bt)
					helperName = helperFuncName(field.Field, typeKey)
					retType = g.typeclassMethodReturnType(iface, field.Field, bt)
				}
				allArgs := append([]ast.Expr{base}, args...)
				return &ast.CallExpr{Fun: ast.NewIdent(helperName), Args: allArgs}, retType, nil
			}
		}
		return &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: base, Sel: ast.NewIdent(field.Field)},
			Args: args,
		}, bt, nil
	}
	// Fallback
	callee, ct, _ := g.translateExpr(n.Callee, ctx, "")
	args := make([]ast.Expr, len(n.Args))
	for i, a := range n.Args {
		ac, _, _ := g.translateExpr(a, ctx, "")
		args[i] = ac
	}
	return &ast.CallExpr{Fun: callee, Args: args}, ct, nil
}

func (g *gen) ensureRelationAllowed(n *BinaryExpr, leftType, rightType string) error {
	typ := leftType
	if typ == "" || typ == "any" {
		typ = rightType
	}
	if typ == "" || typ == "any" {
		return nil
	}
	// Check if this type has Eq support
	baseName, _ := splitTypeArgs(typ)
	baseName = normalizeMyGoTypeName(baseName)
	if g.hasEqSupport(typ, baseName) {
		return nil
	}
	return common.ErrorAtPos(n.Line, n.Column, "relation operator %q requires Eq[%s]", n.Op, typ)
}

func goSigReturnType(results []string) string {
	if len(results) != 1 {
		return ""
	}
	return mygoSigTypeToGo(results[0])
}

func mygoSigTypeToGo(typ string) string {
	typ = strings.TrimSpace(typ)
	switch typ {
	case "":
		return ""
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Int":
		return "int"
	case "Int8":
		return "int8"
	case "Int16":
		return "int16"
	case "Int32":
		return "int32"
	case "Int64":
		return "int64"
	case "UInt":
		return "uint"
	case "UInt8":
		return "uint8"
	case "UInt16":
		return "uint16"
	case "UInt32":
		return "uint32"
	case "UInt64":
		return "uint64"
	case "Float32":
		return "float32"
	case "Float64":
		return "float64"
	case "Any":
		return "any"
	}
	base, args := splitTypeArgs(typ)
	switch base {
	case "Slice":
		if len(args) == 1 {
			return "[]" + mygoSigTypeToGo(args[0])
		}
	case "Ref":
		if len(args) == 1 {
			return "*" + mygoSigTypeToGo(args[0])
		}
	case "Map":
		if len(args) == 2 {
			return "map[" + mygoSigTypeToGo(args[0]) + "]" + mygoSigTypeToGo(args[1])
		}
	case "Set":
		if len(args) == 1 {
			return "map[" + mygoSigTypeToGo(args[0]) + "]struct{}"
		}
	}
	return typ
}

func normalizeMyGoTypeName(name string) string {
	switch name {
	case "Int":
		return "int"
	case "Int8":
		return "int8"
	case "Int16":
		return "int16"
	case "Int32":
		return "int32"
	case "Int64":
		return "int64"
	case "UInt":
		return "uint"
	case "UInt8":
		return "uint8"
	case "UInt16":
		return "uint16"
	case "UInt32":
		return "uint32"
	case "UInt64":
		return "uint64"
	case "Float32":
		return "float32"
	case "Float64":
		return "float64"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Any":
		return "any"
	}
	return name
}

func (g *gen) hasEqSupport(typ, baseName string) bool {
	if typ == "" {
		return false
	}
	// Primitive types always support Eq
	switch baseName {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "string", "bool", "any":
		return true
	}
	// Check for Eq[A] implementations in the package
	for _, impl := range g.pkg.Impls {
		if impl.Name != "Eq" {
			continue
		}
		if len(impl.TypeArgs) != 1 {
			continue
		}
		if g.goType(impl.TypeArgs[0], nil) == typ {
			return true
		}
	}
	return false
}

func chooseType(a, b string) string {
	if a != "" && a != "any" {
		return a
	}
	return b
}

func baseNamedType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if strings.HasPrefix(typeName, "*") {
		typeName = strings.TrimSpace(typeName[1:])
	}
	if idx := strings.Index(typeName, "["); idx >= 0 {
		typeName = typeName[:idx]
	}
	if strings.Contains(typeName, "{") {
		return ""
	}
	return typeName
}

func preludeCollectionMethod(method, recvType string, args []ast.Expr) (ast.Expr, string, bool) {
	recvType = strings.TrimSpace(recvType)
	// Len on native Go slices — handled via IEnumerable typeclass with merged prelude.
	if strings.HasPrefix(recvType, "[]") {
		elem := strings.TrimSpace(recvType[2:])
		elemExpr := ast.NewIdent(elem)
		if method == "Len" {
			return &ast.IndexExpr{X: ast.NewIdent("Len__t_t"), Index: elemExpr}, "int", true
		}
	}
	// Len on native Go maps — handled via IEnumerable typeclass with merged prelude.
	// Len on string — compiler-intrinsic, String -> string cannot be expressed in MyGO type system.
	if recvType == "string" && method == "Len" {
		return ast.NewIdent("Len_string_rune"), "int", true
	}
	return nil, "", false
}

func (g *gen) typeclassMethodReturnType(iface *InterfaceDecl, methodName, recvType string) string {
	if iface == nil {
		return ""
	}
	for _, m := range iface.Methods {
		if m.Name != methodName {
			continue
		}
		if subst := g.typeclassSubstForRecv(iface, recvType); len(subst) > 0 {
			return g.goType(substituteTypeExpr(m.Ret, subst), nil)
		}
		return g.goType(m.Ret, nil)
	}
	return ""
}

func (g *gen) matchTypeclassHelper(ifaceName, methodName, recvType string) (string, string, bool) {
	iface := g.pkg.Interfaces[ifaceName]
	if iface == nil {
		return "", "", false
	}
	for _, impl := range g.pkg.Impls {
		iname := impl.InterfaceName
		if iname == "" {
			iname = impl.Name
		}
		if iname != ifaceName {
			continue
		}
		typeArgs := impl.InterfaceArgs
		if len(typeArgs) == 0 {
			typeArgs = impl.TypeArgs
		}
		subst := g.typeclassSubstForImpl(iface, impl, recvType, typeArgs)
		if subst == nil {
			continue
		}
		helperKey := g.implHelperKey(impl, typeArgs)
		retType := ""
		for _, m := range iface.Methods {
			if m.Name != methodName {
				continue
			}
			retType = g.goType(substituteTypeExpr(m.Ret, subst), nil)
			break
		}
		return helperFuncName(methodName, helperKey), retType, true
	}
	return "", "", false
}

func (g *gen) typeclassSubstForRecv(iface *InterfaceDecl, recvType string) map[string]string {
	if iface == nil || len(iface.TypeParams) == 0 {
		return nil
	}
	for _, impl := range g.pkg.Impls {
		iname := impl.InterfaceName
		if iname == "" {
			iname = impl.Name
		}
		if iname != iface.Name {
			continue
		}
		typeArgs := impl.InterfaceArgs
		if len(typeArgs) == 0 {
			typeArgs = impl.TypeArgs
		}
		if subst := g.typeclassSubstForImpl(iface, impl, recvType, typeArgs); subst != nil {
			return subst
		}
	}
	return nil
}

func (g *gen) typeclassSubstForImpl(iface *InterfaceDecl, impl *ImplDecl, recvType string, typeArgs []TypeExpr) map[string]string {
	if iface == nil || impl == nil || len(typeArgs) == 0 {
		return nil
	}
	subst := map[string]string{}
	typeParamSet := map[string]struct{}{}
	for _, tp := range impl.TypeParams {
		typeParamSet[tp] = struct{}{}
	}
	for _, tp := range iface.TypeParams {
		typeParamSet[tp] = struct{}{}
	}
	pattern := typeArgs[0]
	if !inferTypeSubst(pattern, recvType, typeParamSet, subst) {
		return nil
	}
	return subst
}

func substitutedTypeArgs(args []TypeExpr, subst map[string]string) []TypeExpr {
	out := make([]TypeExpr, len(args))
	for i, a := range args {
		out[i] = substituteTypeExpr(a, subst)
	}
	return out
}

func inferTypeSubst(pattern TypeExpr, concrete string, typeParams map[string]struct{}, subst map[string]string) bool {
	concrete = strings.TrimSpace(concrete)
	switch pt := pattern.(type) {
	case *NamedType:
		if len(pt.Args) == 0 {
			if _, ok := typeParams[pt.Name]; ok {
				if existing, ok := subst[pt.Name]; ok {
					return existing == concrete
				}
				subst[pt.Name] = concrete
				return true
			}
			return typeString(pt, nil) == concrete
		}
		switch pt.Name {
		case "Ref":
			if !strings.HasPrefix(concrete, "*") {
				return false
			}
			return inferTypeSubst(pt.Args[0], concrete[1:], typeParams, subst)
		case "Slice":
			if !strings.HasPrefix(concrete, "[]") {
				return false
			}
			return inferTypeSubst(pt.Args[0], concrete[2:], typeParams, subst)
		case "Set":
			if !strings.HasPrefix(concrete, "map[") || !strings.HasSuffix(concrete, "struct{}") {
				return false
			}
			inner := strings.TrimSuffix(strings.TrimPrefix(concrete, "map["), "]struct{}")
			key, ok := splitMapConcrete(inner)
			if !ok {
				return false
			}
			return inferTypeSubst(pt.Args[0], key, typeParams, subst)
		case "Map":
			if !strings.HasPrefix(concrete, "map[") {
				return false
			}
			inner := strings.TrimPrefix(concrete, "map[")
			key, val, ok := splitMapKeyValue(inner)
			if !ok {
				return false
			}
			return inferTypeSubst(pt.Args[0], key, typeParams, subst) && inferTypeSubst(pt.Args[1], val, typeParams, subst)
		default:
			base, args := splitTypeArgs(concrete)
			if base != pt.Name || len(args) != len(pt.Args) {
				return false
			}
			for i := range pt.Args {
				if !inferTypeSubst(pt.Args[i], args[i], typeParams, subst) {
					return false
				}
			}
			return true
		}
	default:
		return typeString(pattern, nil) == concrete
	}
}

func splitMapConcrete(s string) (string, bool) {
	key, _, ok := splitMapKeyValue(s)
	return key, ok
}

func splitMapKeyValue(s string) (string, string, bool) {
	depth := 0
	for i, r := range s {
		switch r {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			if depth > 0 {
				depth--
			}
		}
		if r == ']' && depth == 0 {
			key := strings.TrimSpace(s[:i])
			val := strings.TrimSpace(s[i+1:])
			if key == "" || val == "" {
				return "", "", false
			}
			return key, val, true
		}
	}
	return "", "", false
}
