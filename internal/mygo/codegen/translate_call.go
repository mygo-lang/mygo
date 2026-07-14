package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *gen) translateCallArgs(args []Expr, ctx *egCtx) ([]ast.Expr, error) {
	return g.translateCallArgsExpected(args, nil, ctx)
}

func (g *gen) translateCallArgsExpected(args []Expr, expected []string, ctx *egCtx) ([]ast.Expr, error) {
	out := make([]ast.Expr, len(args))
	for i, a := range args {
		argExpected := ""
		if i < len(expected) {
			argExpected = expected[i]
		}
		ac, _, err := g.translateExpr(a, ctx, argExpected)
		if err != nil {
			line, col := common.NodePos(a)
			return nil, common.ErrorAtPos(g.currentFile, line, col, "call argument %d: %s", i+1, err.Error())
		}
		if ac == nil {
			line, col := common.NodePos(a)
			return nil, common.ErrorAtPos(g.currentFile, line, col, "call argument %d produced nil Go AST", i+1)
		}
		out[i] = ac
	}
	return out, nil
}

func inferTypeSubstFromExpected(src TypeExpr, expected string) map[string]string {
	subst := map[string]string{}
	inferExpectedTypeSubst(src, expected, subst)
	return subst
}

func inferExpectedTypeSubst(src TypeExpr, expected string, subst map[string]string) {
	expected = strings.TrimSpace(expected)
	if expected == "" || src == nil {
		return
	}
	switch t := src.(type) {
	case *NamedType:
		if len(t.Args) == 0 {
			if _, ok := subst[t.Name]; !ok {
				subst[t.Name] = expected
			}
			return
		}
		base, args := splitTypeArgs(expected)
		if base != t.Name || len(args) != len(t.Args) {
			return
		}
		for i, arg := range t.Args {
			inferExpectedTypeSubst(arg, args[i], subst)
		}
	case *FuncType:
		base, _ := splitTypeArgs(expected)
		if base != "func" {
			return
		}
	}
}

func (g *gen) paramExpectedTypes(fn *FuncDecl, expected string, ctx *egCtx) []string {
	if fn == nil {
		return nil
	}
	subst := inferTypeSubstFromExpected(fn.Ret, expected)
	out := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		out[i] = g.goTypeStringSubst(p.Type, subst)
	}
	return out
}

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
	// Translate explicit type arguments if provided
	var typeArgExprs []ast.Expr
	if len(n.TypeArgs) > 0 {
		typeArgExprs = make([]ast.Expr, len(n.TypeArgs))
		for i, ta := range n.TypeArgs {
			typeArgExprs[i] = goastTypeExpr(ta)
		}
	}

	// Check for IdentExpr callee — handles Some, None, Ok, Err, func calls
	if id, ok := n.Callee.(*IdentExpr); ok {
		switch id.Name {
		case "Some", "None", "Ok", "Err":
			args, err := g.translateCallArgs(n.Args, ctx)
			if err != nil {
				return nil, "", err
			}
			useExpected := expected
			if useExpected == "" {
				useExpected = ctx.retType
			}
			// Use explicit type args if provided, otherwise infer from expected type
			if len(typeArgExprs) == 0 {
				typeArgExprs = typeArgExprsFromExpected(useExpected)
			}
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
			args, err := g.translateCallArgs(n.Args, ctx)
			if err != nil {
				return nil, "", err
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
		// If explicit type args are provided for a generic function, add them
		if len(typeArgExprs) > 0 && len(n.TypeArgs) > 0 {
			callee = &ast.IndexListExpr{X: ast.NewIdent(sanitizeIdent(id.Name)), Indices: typeArgExprs}
		}
		if id.Name == "Zero" && len(n.Args) == 0 && expected != "" {
			return &ast.CallExpr{
				Fun: &ast.IndexExpr{X: ast.NewIdent("Zero"), Index: goTypeExprFromString(expected)},
			}, expected, nil
		}
		// Check for constraint function call (e.g., show(value) → showFn(value))
		if fn, ok := ctx.constraintFuncs[id.Name]; ok && len(n.Args) > 0 {
			args, err := g.translateCallArgs(n.Args, ctx)
			if err != nil {
				return nil, "", err
			}
			retType := ctx.retType
			if expected != "" {
				retType = expected
			}
			return &ast.CallExpr{Fun: ast.NewIdent(fn), Args: args}, retType, nil
		}

		retType := ""
		var fnDecl *FuncDecl
		if decl, ok := g.pkg.Funcs[id.Name]; ok {
			fnDecl = decl
			retType = g.goReturnType(fnDecl.Ret, ctx.typeParams)
		}
		if expected != "" {
			retType = expected
		}
		args, err := g.translateCallArgsExpected(n.Args, g.paramExpectedTypes(fnDecl, retType, ctx), ctx)
		if err != nil {
			return nil, "", err
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
				arg, argType, err := g.translateExpr(n.Args[0], ctx, "")
				if err != nil {
					return nil, "", err
				}
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
					args, err := g.translateCallArgs(n.Args, ctx)
					if err != nil {
						return nil, "", err
					}
					fnName := inherentMethodName(id.Name, method.Func.Name)
					retType := g.goReturnType(method.Func.Ret, ctx.typeParams)
					return &ast.CallExpr{Fun: ast.NewIdent(fnName), Args: args}, retType, nil
				}
			}
			// Check if it's an enum constructor call (Enum.Variant)
			if g.variantByName[field.Field] != "" {
				args, err := g.translateCallArgs(n.Args, ctx)
				if err != nil {
					return nil, "", err
				}
				variantType := variantNameForEnum(id.Name, field.Field)
				return &ast.CallExpr{Fun: ast.NewIdent(variantType), Args: args}, variantType, nil
			}
			// Imported method call: pkg.Func()
			if g.importAliases[id.Name] != "" {
				path := g.importAliases[id.Name]
				// For MyGo imports (not prefixed with "go:"), check exported status
				if !strings.HasPrefix(path, "go:") && !isExportedIdent(field.Field) {
					return nil, "", common.ErrorAtPos(g.currentFile, field.Line, field.Column, "cannot refer to unexported symbol %s.%s", id.Name, field.Field)
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
								return nil, "", common.ErrorAtPos(g.currentFile, field.Line, field.Column, "call type mismatch for %s.%s: expected %d args, got %d", id.Name, field.Field, len(sig.params), len(n.Args))
							}
							callee := ast.NewIdent(id.Name + "." + field.Field)
							args, err := g.translateCallArgs(n.Args, ctx)
							if err != nil {
								return nil, "", err
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
				args, err := g.translateCallArgs(n.Args, ctx)
				if err != nil {
					return nil, "", err
				}
				return &ast.CallExpr{Fun: callee, Args: args}, expected, nil
			}
		}
		base, bt, err := g.translateExpr(field.Expr, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if base == nil {
			line, col := common.NodePos(field.Expr)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "method receiver produced nil Go AST")
		}
		args, err := g.translateCallArgs(n.Args, ctx)
		if err != nil {
			return nil, "", err
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
		// Also try resolving Go types to MyGO type names (e.g. []string → Slice, map[K]V → Map, map[A]struct{} → Set)
		// This handles cases where baseNamedType returns empty (e.g. "[]" from "[]string")
		if mygoName := goTypeToMyGoTypeName(bt); mygoName != "" {
			if methods, ok := g.inherentMethods[mygoName]; ok {
				if method, exists := methods[field.Field]; exists && method.HasReceiver {
					fnName := inherentMethodName(mygoName, method.Func.Name)
					allArgs := append([]ast.Expr{base}, args...)
					callee := ast.NewIdent(fnName)
					retType := g.goReturnType(method.Func.Ret, ctx.typeParams)
					return &ast.CallExpr{Fun: callee, Args: allArgs}, retType, nil
				}
			}
		}
		// Check for typeclass method call: value.show() → show_type() or showFn()
		var fallbackIface *InterfaceDecl
		for _, ifaceName := range g.interfaceNamesForMethod(field.Field) {
			if iface := g.pkg.Interfaces[ifaceName]; iface != nil {
				if fallbackIface == nil {
					fallbackIface = iface
				}
				// First check if there's a constraint function in scope (from `using`)
				if fnName, ok := ctx.constraintFuncForMethod(field.Field); ok {
					allArgs := append([]ast.Expr{base}, args...)
					retType := g.typeclassMethodReturnType(iface, field.Field, bt)
					return &ast.CallExpr{Fun: ast.NewIdent(fnName), Args: allArgs}, retType, nil
				}
				// Otherwise use the best matching impl helper function.
				helperName, retType, ok := g.matchTypeclassHelper(ifaceName, field.Field, bt)
				if !ok {
					continue
				}
				allArgs := append([]ast.Expr{base}, args...)
				return &ast.CallExpr{Fun: ast.NewIdent(helperName), Args: allArgs}, retType, nil
			}
		}
		if fallbackIface != nil {
			typeKey := typeKeyFromType(bt)
			helperName := helperFuncName(field.Field, typeKey)
			retType := g.typeclassMethodReturnType(fallbackIface, field.Field, bt)
			allArgs := append([]ast.Expr{base}, args...)
			return &ast.CallExpr{Fun: ast.NewIdent(helperName), Args: allArgs}, retType, nil
		}
		// For field calls where the field is a function type (e.g., parser.run(state)),
		// extract the return type from the field type
		ft := g.fieldType(bt, field.Field)
		if ft != "" && strings.HasPrefix(ft, "func(") {
			// Extract return type from func signature
			if ret := extractFuncReturnType(ft); ret != "" {
				return &ast.CallExpr{
					Fun:  &ast.SelectorExpr{X: base, Sel: ast.NewIdent(field.Field)},
					Args: args,
				}, ret, nil
			}
		}
		return &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: base, Sel: ast.NewIdent(field.Field)},
			Args: args,
		}, bt, nil
	}
	// Fallback
	callee, ct, err := g.translateExpr(n.Callee, ctx, "")
	if err != nil {
		return nil, "", err
	}
	if callee == nil {
		line, col := common.NodePos(n.Callee)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "call callee produced nil Go AST: %s", fmt.Sprintf("%T", n.Callee))
	}
	args, err := g.translateCallArgs(n.Args, ctx)
	if err != nil {
		return nil, "", err
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
	return common.ErrorAtPos(g.currentFile, n.Line, n.Column, "relation operator %q requires Eq[%s]", n.Op, typ)
}

func extractFuncReturnType(sig string) string {
	// Parse func(params) ret or func(params)
	// Example: "func(State) Reply[[]A]" or "func(a int, b string) bool"
	if !strings.HasPrefix(sig, "func(") {
		return ""
	}
	// Find the closing paren of the parameter list
	depth := 0
	end := -1
	for i := 0; i < len(sig); i++ {
		switch sig[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
				goto found
			}
		}
	}
found:
	if end == -1 {
		return ""
	}
	ret := strings.TrimSpace(sig[end+1:])
	if ret == "" {
		return ""
	}
	return ret
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
	case "Byte":
		return "byte"
	case "Rune":
		return "rune"
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
	case "Byte":
		return "byte"
	case "Rune":
		return "rune"
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
		"float32", "float64", "string", "bool", "byte", "rune", "any":
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

func goTypeToMyGoTypeName(goType string) string {
	goType = strings.TrimSpace(goType)
	// []T → Slice
	if strings.HasPrefix(goType, "[]") {
		return "Slice"
	}
	// map[K]V → Map, map[A]struct{} → Set
	if strings.HasPrefix(goType, "map[") {
		// Find the matching ']' for the opening '['
		depth := 0
		closeIdx := -1
		for i, r := range goType {
			switch r {
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					closeIdx = i
					break
				}
			}
		}
		if closeIdx > 0 {
			inner := goType[4:closeIdx] // content between "map[" and "]"
			// Split inner into key and value at the top level
			depth = 0
			splitIdx := -1
			for i, r := range inner {
				switch r {
				case '[', '(', '{':
					depth++
				case ']', ')', '}':
					depth--
				case ',':
					if depth == 0 {
						splitIdx = i
						break
					}
				}
				if splitIdx >= 0 {
					break
				}
			}
			if splitIdx >= 0 {
				val := strings.TrimSpace(inner[splitIdx+1:])
				if val == "struct{}" {
					return "Set"
				}
			}
		}
		return "Map"
	}
	// *T → Ref
	if strings.HasPrefix(goType, "*") {
		return "Ref"
	}
	// string → String (for inherent impls like String.FromRunes, String.PeekRune, etc.)
	if goType == "string" {
		return "String"
	}
	// int, bool, etc. → Int, Bool, etc.
	switch goType {
	case "int":
		return "Int"
	case "int8":
		return "Int8"
	case "int16":
		return "Int16"
	case "int32":
		return "Int32"
	case "int64":
		return "Int64"
	case "uint":
		return "UInt"
	case "uint8", "byte":
		return "UInt8"
	case "uint16":
		return "UInt16"
	case "uint32":
		return "UInt32"
	case "uint64":
		return "UInt64"
	case "float32":
		return "Float32"
	case "float64":
		return "Float64"
	case "rune":
		return "Int32"
	case "bool":
		return "Bool"
	}
	return ""
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

func (g *gen) interfaceNamesForMethod(methodName string) []string {
	seen := map[string]struct{}{}
	var names []string
	if ifaceName, ok := g.interfaceByMethod[methodName]; ok {
		if _, ok := seen[ifaceName]; !ok {
			seen[ifaceName] = struct{}{}
			names = append(names, ifaceName)
		}
	}
	for name, iface := range g.pkg.Interfaces {
		if _, ok := seen[name]; ok || iface == nil {
			continue
		}
		for _, method := range iface.Methods {
			if method.Name == methodName {
				seen[name] = struct{}{}
				names = append(names, name)
				break
			}
		}
	}
	return names
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
