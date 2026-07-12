package codegen

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

var goPlaceholderRE = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
var goTupleErrorRE = regexp.MustCompile(`func\(\)\s*\([^)]+,\s*error\s*\)`)

// splitGenericArgs splits a generic type argument list into individual arguments.
func splitGenericArgs(s string) []string {
	var args []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start <= len(s) {
		args = append(args, strings.TrimSpace(s[start:]))
	}
	return args
}

// translateBlockStmts2 translates statements within a BlockExpr.
func (g *gen) translateBlockStmts2(n *BlockExpr, ctx *egCtx, returnExpected string, retTypes []string) ([]ast.Stmt, error) {
	child := ctx.child()
	var stmts []ast.Stmt
	for i, stmt := range n.Stmts {
		isLast := i == len(n.Stmts)-1
		switch s := stmt.(type) {
		case *ExprStmt:
			code, typ, err := g.translateExpr2(s.Expr, child, "")
			if err != nil {
				return stmts, err
			}
			if isLast && returnExpected != "" {
				stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
			} else if typ == "" {
				stmts = append(stmts, &ast.ExprStmt{X: code})
			} else {
				stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("_")}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
			}
		case *ReturnStmt:
			if s.Value != nil {
				code, _, err := g.translateExpr2(s.Value, child, returnExpected)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
			} else {
				stmts = append(stmts, &ast.ReturnStmt{})
			}
		case *LetStmt:
			if s.Bind != nil {
				if bind, ok := s.Bind.(*BindTuplePattern); ok {
					code, _, err := g.translateExpr2(s.Value, child, "")
					if err != nil {
						return stmts, err
					}
					stmts = g.emitBindDestructure(stmts, child, code, bind)
					continue
				}
			}
			expectedType := ""
			if s.Type != nil {
				expectedType = g.goType(s.Type, child.typeParams)
			}
			code, valType, err := g.translateExpr2(s.Value, child, expectedType)
			if err != nil {
				return stmts, err
			}
			if s.Name == "_" {
				stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("_")}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
			} else {
				g.localSeq++
				base := sanitizeIdent(s.Name)
				if base == "" || base == "_" {
					base = "tmp"
				}
				lbType := valType
				if lbType == "" && s.Type != nil {
					lbType = g.goType(s.Type, child.typeParams)
				}
				actual := base + "_" + strconv.Itoa(g.localSeq)
				child.bindings[s.Name] = actual
				child.locals[s.Name] = lbType
				child.mutable[actual] = s.Mutable
				if s.Type != nil {
					typeExpr := goastTypeExpr(s.Type)
					stmts = append(stmts, &ast.DeclStmt{
						Decl: &ast.GenDecl{
							Tok: token.VAR,
							Specs: []ast.Spec{
								&ast.ValueSpec{
									Names:  []*ast.Ident{ast.NewIdent(actual)},
									Type:   typeExpr,
									Values: []ast.Expr{code},
								},
							},
						},
					})
				} else {
					stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.DEFINE})
				}
			}
		case *AssignStmt:
			actual, ok := child.bindings[s.Name]
			if !ok {
				return nil, common.ErrorAtPos(s.Line, s.Column, "unknown binding %q", s.Name)
			}
			if !child.mutable[actual] {
				return nil, common.ErrorAtPos(s.Line, s.Column, "cannot assign to immutable binding %q", s.Name)
			}
			code, _, err := g.translateExpr2(s.Value, child, child.locals[s.Name])
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
		}
	}
	return stmts, nil
}

// translateExpr2 is a complete rewrite of translateExpr.
func (g *gen) translateExpr2(e Expr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	switch n := e.(type) {
	case *IdentExpr:
		// Check bindings for renamed identifiers
		if bound, ok := ctx.bindings[n.Name]; ok {
			return ast.NewIdent(bound), ctx.locals[n.Name], nil
		}
		// Handle enum variant constructors with no args (e.g., bare `None` as IdentExpr)
		if n.Name == "None" {
			useExpected := expected
			if useExpected == "" {
				useExpected = ctx.retType
			}
			if base, tas := splitTypeArgs(useExpected); base == "Option" && len(tas) > 0 {
				callee := &ast.IndexExpr{X: ast.NewIdent("None"), Index: ast.NewIdent(tas[0])}
				return &ast.CallExpr{Fun: callee}, useExpected, nil
			}
		}
		return ast.NewIdent(n.Name), ctx.locals[n.Name], nil
	case *LiteralExpr:
		switch n.Kind {
		case "number":
			if strings.Contains(n.Value, ".") {
				return ast.NewIdent(n.Value), "float64", nil
			}
			return ast.NewIdent(n.Value), "int", nil
		case "string":
			return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(n.Value)}, "string", nil
		case "bool":
			if n.Value == "true" {
				return ast.NewIdent("true"), "bool", nil
			}
			return ast.NewIdent("false"), "bool", nil
		}
	case *BinaryExpr:
		left, lt, _ := g.translateExpr2(n.Left, ctx, "")
		right, rt, _ := g.translateExpr2(n.Right, ctx, lt)
		switch n.Op {
		case "+":
			return &ast.BinaryExpr{X: left, Op: token.ADD, Y: right}, chooseType(lt, rt), nil
		case "-":
			return &ast.BinaryExpr{X: left, Op: token.SUB, Y: right}, chooseType(lt, rt), nil
		case "*":
			return &ast.BinaryExpr{X: left, Op: token.MUL, Y: right}, chooseType(lt, rt), nil
		case "/":
			return &ast.BinaryExpr{X: left, Op: token.QUO, Y: right}, chooseType(lt, rt), nil
		case "&&":
			return &ast.BinaryExpr{X: left, Op: token.LAND, Y: right}, "bool", nil
		case "||":
			return &ast.BinaryExpr{X: left, Op: token.LOR, Y: right}, "bool", nil
		case "==", "!=", "<", ">", "<=", ">=":
			if err := g.ensureRelationAllowed(n, lt, rt); err != nil {
				return nil, "", err
			}
			tok := token.EQL
			switch n.Op {
			case "==":
				tok = token.EQL
			case "!=":
				tok = token.NEQ
			case "<":
				tok = token.LSS
			case ">":
				tok = token.GTR
			case "<=":
				tok = token.LEQ
			case ">=":
				tok = token.GEQ
			}
			return &ast.BinaryExpr{X: left, Op: tok, Y: right}, "bool", nil
		case "|>":
			if call, ok := n.Right.(*CallExpr); ok {
				callee, _, _ := g.translateExpr2(call.Callee, ctx, "")
				args := make([]ast.Expr, 0, len(call.Args)+1)
				for _, a := range call.Args {
					ac, _, _ := g.translateExpr2(a, ctx, "")
					args = append(args, ac)
				}
				args = append(args, left)
				return &ast.CallExpr{Fun: callee, Args: args}, lt, nil
			}
			return &ast.CallExpr{Fun: right, Args: []ast.Expr{left}}, lt, nil
		case "<|":
			if call, ok := n.Left.(*CallExpr); ok {
				callee, _, _ := g.translateExpr2(call.Callee, ctx, "")
				args := make([]ast.Expr, 0, len(call.Args)+1)
				for _, a := range call.Args {
					ac, _, _ := g.translateExpr2(a, ctx, "")
					args = append(args, ac)
				}
				args = append(args, right)
				return &ast.CallExpr{Fun: callee, Args: args}, lt, nil
			}
			return &ast.CallExpr{Fun: left, Args: []ast.Expr{right}}, lt, nil
		}
	case *PrefixExpr:
		expr, typ, _ := g.translateExpr2(n.Expr, ctx, "")
		switch n.Op {
		case "!":
			return &ast.UnaryExpr{Op: token.NOT, X: expr}, "bool", nil
		case "-":
			return &ast.UnaryExpr{Op: token.SUB, X: expr}, typ, nil
		}
	case *CastExpr:
		code, _, _ := g.translateExpr2(n.Expr, ctx, g.goType(n.Type, ctx.typeParams))
		target := g.goType(n.Type, ctx.typeParams)
		return &ast.CallExpr{Fun: ast.NewIdent(target), Args: []ast.Expr{code}}, target, nil
	case *FieldExpr:
		base, bt, _ := g.translateExpr2(n.Expr, ctx, "")
		// Handle Ref.value — dereference pointer
		if n.Field == "value" {
			btNorm := strings.TrimSpace(bt)
			if strings.HasPrefix(btNorm, "Ref[") && strings.HasSuffix(btNorm, "]") {
				inner := btNorm[4 : len(btNorm)-1]
				return &ast.UnaryExpr{Op: token.MUL, X: base}, inner, nil
			}
			if strings.HasPrefix(btNorm, "*") {
				return &ast.UnaryExpr{Op: token.MUL, X: base}, btNorm[1:], nil
			}
		}
		// Try to look up the field type from the base type
		ft := g.fieldType(bt, n.Field)
		if ft == "" {
			ft = lookupMyGoFieldType(n.Expr, n.Field, g)
		}
		if ft != "" {
			return &ast.SelectorExpr{X: base, Sel: ast.NewIdent(n.Field)}, ft, nil
		}
		return &ast.SelectorExpr{X: base, Sel: ast.NewIdent(n.Field)}, bt, nil
	case *CallExpr:
		return g.translateCall2(n, ctx, expected)
	case *IfExpr:
		return g.translateIf2(n, ctx, expected)
	case *SwitchExpr:
		return g.translateSwitch2(n, ctx, expected)
	case *WhileExpr:
		return g.translateWhile2(n, ctx)
	case *BlockExpr:
		stmts, _ := g.translateBlockStmts2(n, ctx, expected, nil)
		if expected == "" && len(stmts) > 0 {
			// Statement-only block
			if last, ok := stmts[len(stmts)-1].(*ast.ReturnStmt); ok && len(last.Results) > 0 {
				fn := astFuncLit(nil, nil, &ast.BlockStmt{List: stmts})
				return &ast.CallExpr{Fun: fn}, "", nil
			}
			fn := astFuncLit(nil, nil, &ast.BlockStmt{List: stmts})
			return &ast.CallExpr{Fun: fn}, "", nil
		}
		fn := &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: fieldListForReturn(expected),
			},
			Body: &ast.BlockStmt{List: stmts},
		}
		return &ast.CallExpr{Fun: fn}, expected, nil
	case *StructLitExpr:
		return g.translateStructLit2(n, ctx, expected)
	case *SliceLitExpr:
		return g.translateSliceLit2(n, ctx, expected)
	case *MapLitExpr:
		return g.translateMapLit2(n, ctx, expected)
	case *SetLitExpr:
		return g.translateSetLit2(n, ctx, expected)
	case *TupleLitExpr:
		return g.translateTupleLit2(n, ctx, expected)
	case *UnitLitExpr:
		return &ast.CompositeLit{Type: &ast.StructType{Fields: &ast.FieldList{}}}, "Unit", nil
	case *FuncLitExpr:
		return g.translateFuncLit2(n, ctx)
	case *GoExpr:
		return g.translateGoExpr2(n, ctx, expected)
	}
	line, col := common.NodePos(e)
	return nil, "", common.ErrorAtPos(line, col, "unsupported expression %T", e)
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

func baseNamedType2(typeName string) string {
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

func fieldListForReturn(expected string) *ast.FieldList {
	if expected == "" {
		return nil
	}
	return &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent(expected)}}}
}

func fieldListIfNonEmptyGoast(fields []*ast.Field) *ast.FieldList {
	if len(fields) == 0 {
		return nil
	}
	return &ast.FieldList{List: fields}
}

// translateCall2 handles function/method calls.
func (g *gen) translateCall2(n *CallExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	// Check for IdentExpr callee — handles Some, None, Ok, Err, func calls
	if id, ok := n.Callee.(*IdentExpr); ok {
		switch id.Name {
		case "Some", "None", "Ok", "Err":
			args := make([]ast.Expr, len(n.Args))
			for i, a := range n.Args {
				ac, _, _ := g.translateExpr2(a, ctx, "")
				args[i] = ac
			}
			typeArgExprs := typeArgExprsFromExpected(expected)
			var fun ast.Expr = ast.NewIdent(id.Name)
			if len(typeArgExprs) > 0 {
				if len(typeArgExprs) == 1 {
					fun = &ast.IndexExpr{X: fun, Index: typeArgExprs[0]}
				} else {
					fun = &ast.IndexListExpr{X: fun, Indices: typeArgExprs}
				}
			}
			return &ast.CallExpr{Fun: fun, Args: args}, expected, nil
		}
		// Auto-inject constraint function args for functions with using clauses.
		// E.g., same(1, 2) → same(1, 2, Equals_fasteq_int) when same has using.
		if fnDecl, ok := g.pkg.Funcs[id.Name]; ok && len(fnDecl.Using) > 0 {
			args := make([]ast.Expr, len(n.Args))
			for i, a := range n.Args {
				ac, _, _ := g.translateExpr2(a, ctx, "")
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
					for i, arg := range implTypeArgs {
						implTypeArgs[i] = substituteTypeExpr(arg, implSubst)
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
				ac, _, _ := g.translateExpr2(a, ctx, "")
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
			ac, _, _ := g.translateExpr2(a, ctx, "")
			args[i] = ac
		}
		retType := ctx.retType
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
					ta[i] = ast.NewIdent(a)
				}
				if len(ta) == 1 {
					callee = &ast.IndexExpr{X: ast.NewIdent(id.Name), Index: ta[0]}
				}
			}
		case "Ok", "Err":
			if base, tas := splitTypeArgs(useExpected); base == "Result" && len(tas) == 2 {
				ta := make([]ast.Expr, len(tas))
				for i, a := range tas {
					ta[i] = ast.NewIdent(a)
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
			baseExpr, baseType, _ := g.translateExpr2(field.Expr, ctx, "")
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
				arg, argType, _ := g.translateExpr2(n.Args[0], ctx, "")
				ptrType := "*" + argType
				return &ast.UnaryExpr{Op: token.AND, X: arg}, ptrType, nil
			}
		}
		if id, ok := field.Expr.(*IdentExpr); ok {
			// Check for inherent static method call: Type.method(args)
			if methods, ok := g.inherentMethods[id.Name]; ok {
				if method, ok := methods[field.Field]; ok && !method.HasReceiver {
					args := make([]ast.Expr, len(n.Args))
					for i, a := range n.Args {
						ac, _, _ := g.translateExpr2(a, ctx, "")
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
					ac, _, _ := g.translateExpr2(a, ctx, "")
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
						}
					}
				}
				callee := ast.NewIdent(id.Name + "." + field.Field)
				args := make([]ast.Expr, len(n.Args))
				for i, a := range n.Args {
					ac, _, _ := g.translateExpr2(a, ctx, "")
					args[i] = ac
				}
				return &ast.CallExpr{Fun: callee, Args: args}, expected, nil
			}
		}
		base, bt, _ := g.translateExpr2(field.Expr, ctx, "")
		args := make([]ast.Expr, len(n.Args))
		for i, a := range n.Args {
			ac, _, _ := g.translateExpr2(a, ctx, "")
			args[i] = ac
		}
		// Check for inherent method call: receiverType.method(args...) → receiverType_method(args..., receiver)
		recvTypeName := baseNamedType2(bt)
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
		// Check for typeclass method call: value.show() → show_type() or showFn()
		if ifaceName, ok := g.interfaceByMethod[field.Field]; ok {
			if iface := g.pkg.Interfaces[ifaceName]; iface != nil {
				// First check if there's a constraint function in scope (from `using`)
				if fnName, ok := ctx.constraintFuncForMethod(field.Field); ok {
					allArgs := append([]ast.Expr{base}, args...)
					return &ast.CallExpr{Fun: ast.NewIdent(fnName), Args: allArgs}, "string", nil
				}
				// Otherwise use the impl helper function
				typeKey := typeKeyFromType(bt)
				helperName := helperFuncName(field.Field, typeKey)
				allArgs := append([]ast.Expr{base}, args...)
				return &ast.CallExpr{Fun: ast.NewIdent(helperName), Args: allArgs}, "string", nil
			}
		}
		return &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: base, Sel: ast.NewIdent(field.Field)},
			Args: args,
		}, bt, nil
	}
	// Fallback
	callee, ct, _ := g.translateExpr2(n.Callee, ctx, "")
	args := make([]ast.Expr, len(n.Args))
	for i, a := range n.Args {
		ac, _, _ := g.translateExpr2(a, ctx, "")
		args[i] = ac
	}
	return &ast.CallExpr{Fun: callee, Args: args}, ct, nil
}

func typeArgExprsFromExpected(expected string) []ast.Expr {
	_, args := splitTypeArgs(expected)
	if len(args) == 0 {
		return nil
	}
	out := make([]ast.Expr, len(args))
	for i, a := range args {
		out[i] = ast.NewIdent(a)
	}
	return out
}

// translateIf2 handles if expressions.
func (g *gen) translateIf2(n *IfExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	cond, _, _ := g.translateExpr2(n.Cond, ctx, "bool")
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenCode, thenType, _ := g.translateExpr2(n.Then, thenCtx, expected)
	elseCode, elseType, _ := g.translateExpr2(n.Else, elseCtx, expected)

	resultType := expected
	if resultType == "" {
		if thenType != "" {
			resultType = thenType
		} else {
			resultType = elseType
		}
	}
	if resultType == "" || resultType == "any" {
		// Statement form: wrap in IIFE so both branches are expressions
		ifStmt := &ast.IfStmt{
			Cond: cond,
			Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: thenCode}}},
		}
		if elseCode != nil {
			ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: elseCode}}}
		}
		fn := astFuncLit(nil, nil, &ast.BlockStmt{List: []ast.Stmt{ifStmt}})
		return &ast.CallExpr{Fun: fn}, "", nil
	}
	// Expression form: wrap in IIFE returning resultType
	fn := &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent(resultType)}}},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: cond,
					Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{thenCode}}}},
					Else: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{elseCode}}}},
				},
			},
		},
	}
	return &ast.CallExpr{Fun: fn}, resultType, nil
}

// translateSwitch2 handles switch expressions.
func (g *gen) translateSwitch2(n *SwitchExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	target, ttype, _ := g.translateExpr2(n.Target, ctx, "")
	_, _ = target, ttype

	lastIsWildcard := false
	if len(n.Cases) > 0 {
		if _, ok := n.Cases[len(n.Cases)-1].Pattern.(*WildcardPattern); ok {
			lastIsWildcard = true
		}
	}

	var tail ast.Stmt
	for i := len(n.Cases) - 1; i >= 0; i-- {
		c := n.Cases[i]
		if _, ok := c.Pattern.(*WildcardPattern); ok {
			code, _, _ := g.translateExpr2(c.Body, ctx.child(), expected)
			if expected == "" {
				tail = &ast.ExprStmt{X: code}
			} else {
				tail = &ast.ReturnStmt{Results: []ast.Expr{code}}
			}
			continue
		}
		if lit, ok := c.Pattern.(*LiteralPattern); ok {
			patExpr := litToExpr(lit)
			child := ctx.child()
			code, _, _ := g.translateExpr2(c.Body, child, expected)
			var bodyBlock *ast.BlockStmt
			if expected == "" {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: code}}}
			} else {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{code}}}}
			}
			cond := &ast.BinaryExpr{X: target, Op: token.EQL, Y: patExpr}
			ifStmt := &ast.IfStmt{Cond: cond, Body: bodyBlock}
			if tail != nil {
				ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{tail}}
			}
			tail = ifStmt
			continue
		}
		if vp, ok := c.Pattern.(*VariantPattern); ok {
			g.switchVarSeq++
			varName := "v_" + strconv.Itoa(g.switchVarSeq)

			// Construct type assertion name from enum info
			assertTypeName := vp.Name
			enumName, found := g.variantByName[vp.Name]
			if !found {
				if baseName, _ := splitTypeArgs(ttype); baseName != "" && baseName != vp.Name {
					enumName = baseName
					found = true
				}
			}
			if found {
				assertTypeName = variantNameForEnum(enumName, vp.Name)
			}
			var assertType ast.Expr = ast.NewIdent(assertTypeName)
			if found {
				if _, typeArgs := splitTypeArgs(ttype); len(typeArgs) > 0 {
					taExprs := make([]ast.Expr, len(typeArgs))
					for i, ta := range typeArgs {
						taExprs[i] = ast.NewIdent(ta)
					}
					assertType = genericIdent(assertTypeName, taExprs...)
				}
			}
			// Check if any pattern arg is used in the body
			hasBindings := false
			for _, arg := range vp.Args {
				if arg != "_" && exprUsesIdent(c.Body, arg) {
					hasBindings = true
					break
				}
			}
			child := ctx.child()
			varNameOrBlank := ast.NewIdent("_")
			if hasBindings {
				varNameOrBlank = ast.NewIdent(varName)
				for i, arg := range vp.Args {
					if arg != "_" {
						child.bindings[arg] = fmt.Sprintf("%s.F%d", varName, i)
						child.locals[arg] = ""
					}
				}
			}
			code, _, _ := g.translateExpr2(c.Body, child, expected)
			var bodyBlock *ast.BlockStmt
			if expected == "" {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: code}}}
			} else {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{code}}}}
			}
			ifStmt := &ast.IfStmt{
				Init: &ast.AssignStmt{
					Lhs: []ast.Expr{varNameOrBlank, ast.NewIdent("ok")},
					Rhs: []ast.Expr{&ast.TypeAssertExpr{X: target, Type: assertType}},
					Tok: token.DEFINE,
				},
				Cond: ast.NewIdent("ok"),
				Body: bodyBlock,
			}
			if tail != nil {
				ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{tail}}
			} else if expected != "" && !lastIsWildcard {
				ifStmt.Else = &ast.BlockStmt{
					List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic"), Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote("unreachable")}}}}},
				}
			}
			tail = ifStmt
		}
	}
	_ = lastIsWildcard
	if tail == nil {
		return ast.NewIdent("_"), "", nil
	}
	if expected == "" {
		// Wrap in IIFE since Stmt can't be returned as Expr
		fn := astFuncLit(nil, nil, &ast.BlockStmt{List: []ast.Stmt{tail}})
		return &ast.CallExpr{Fun: fn}, "", nil
	}
	// Wrap in IIFE for expression form
	fn := &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent(expected)}}},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{tail}},
	}
	return &ast.CallExpr{Fun: fn}, expected, nil
}

func litToExpr(l *LiteralPattern) ast.Expr {
	switch l.Kind {
	case "string":
		return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(l.Value)}
	case "number":
		return &ast.BasicLit{Kind: token.INT, Value: l.Value}
	default:
		return ast.NewIdent(l.Value)
	}
}

// translateWhile2 handles while loops.
func (g *gen) translateWhile2(n *WhileExpr, ctx *egCtx) (ast.Expr, string, error) {
	cond, _, _ := g.translateExpr2(n.Cond, ctx, "bool")
	body := &ast.BlockStmt{}
	switch b := n.Body.(type) {
	case *BlockExpr:
		for _, stmt := range b.Stmts {
			g.translateWhileStmt(stmt, ctx, body)
		}
	default:
		code, _, _ := g.translateExpr2(n.Body, ctx, "")
		body.List = append(body.List, &ast.ExprStmt{X: code})
	}
	forStmt := &ast.ForStmt{Cond: cond, Body: body}
	fn := astFuncLit(nil, nil, &ast.BlockStmt{List: []ast.Stmt{forStmt}})
	return &ast.CallExpr{Fun: fn}, "", nil
}

func (g *gen) translateWhileStmt(stmt Stmt, ctx *egCtx, body *ast.BlockStmt) {
	switch s := stmt.(type) {
	case *ExprStmt:
		code, _, _ := g.translateExpr2(s.Expr, ctx, "")
		body.List = append(body.List, &ast.ExprStmt{X: code})
	case *LetStmt:
		code, _, _ := g.translateExpr2(s.Value, ctx, "")
		actual := sanitizeIdent(s.Name)
		ctx.bindings[s.Name] = actual
		body.List = append(body.List, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.DEFINE})
	case *AssignStmt:
		code, _, _ := g.translateExpr2(s.Value, ctx, "")
		actual := ctx.bindings[s.Name]
		body.List = append(body.List, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
	}
}

// translateFuncLit2 handles function literals.
func (g *gen) translateFuncLit2(n *FuncLitExpr, ctx *egCtx) (ast.Expr, string, error) {
	retType := g.goReturnType(n.Ret, ctx.typeParams)
	params := make([]*ast.Field, len(n.Params))
	for i, p := range n.Params {
		params[i] = &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(p.Name))},
			Type:  goastTypeExpr(p.Type),
		}
	}
	var results []*ast.Field
	if retType != "" {
		results = []*ast.Field{{Type: ast.NewIdent(retType)}}
	}
	child := ctx.child()
	child.retType = retType
	for _, p := range n.Params {
		child.locals[p.Name] = g.goType(p.Type, ctx.typeParams)
		child.bindings[p.Name] = p.Name
	}
	if block, ok := n.Body.(*BlockExpr); ok {
		stmts, _ := g.translateBlockStmts2(block, child, retType, nil)
		return &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{List: params},
				Results: fieldListIfNonEmptyGoast(results),
			},
			Body: &ast.BlockStmt{List: stmts},
		}, retType, nil
	}
	bodyCode, _, _ := g.translateExpr2(n.Body, child, retType)
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: fieldListIfNonEmptyGoast(results),
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{bodyCode}}}},
	}, retType, nil
}

// translateStructLit2 handles struct literal construction.
func (g *gen) translateStructLit2(n *StructLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	typeName := sanitizeIdent(n.TypeName)
	st := g.pkg.Structs[n.TypeName]
	if st == nil {
		return ast.NewIdent(typeName), typeName, nil
	}
	subst := map[string]string{}
	if len(n.TypeArgs) > 0 {
		for i, tp := range st.TypeParams {
			if i < len(n.TypeArgs) {
				subst[tp] = g.goType(n.TypeArgs[i], ctx.typeParams)
			}
		}
	}
	elts := make([]ast.Expr, len(n.Fields))
	for i, f := range n.Fields {
		code, _, _ := g.translateExpr2(f.Value, ctx, "")
		fieldName := goastFieldName(f.Name)
		if fieldName == "" {
			elts[i] = code
		} else {
			elts[i] = &ast.KeyValueExpr{Key: ast.NewIdent(fieldName), Value: code}
		}
		_ = i
	}
	var typeExpr ast.Expr = ast.NewIdent(typeName)
	if len(n.TypeArgs) > 0 {
		typeArgs := make([]ast.Expr, len(n.TypeArgs))
		for i, a := range n.TypeArgs {
			typeArgs[i] = ast.NewIdent(g.goType(a, ctx.typeParams))
		}
		typeExpr = genericIdent(typeName, typeArgs...)
	}
	return &ast.CompositeLit{Type: typeExpr, Elts: elts}, typeName, nil
}

// translateSliceLit2 handles slice literals.
func (g *gen) translateSliceLit2(n *SliceLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	var elemType ast.Expr = ast.NewIdent("int")
	elemTypeStr := "int"
	if n.Elem != nil {
		elemType = goastTypeExpr(n.Elem)
		elemTypeStr = g.goType(n.Elem, ctx.typeParams)
	} else {
		elemTypeStr = elemTypeFromExpected(expected)
		if elemTypeStr == "any" {
			elemTypeStr = "int"
		}
		if expr, err := goast.TypeExprToGo(elemTypeStr); err == nil {
			elemType = expr
		} else {
			elemType = ast.NewIdent(elemTypeStr)
		}
	}
	var elts []ast.Expr
	for _, elem := range n.Elems {
		ac, _, _ := g.translateExpr2(elem, ctx, elemTypeStr)
		elts = append(elts, ac)
	}
	arrType := &ast.ArrayType{Elt: elemType}
	return &ast.CompositeLit{Type: arrType, Elts: elts}, "[]" + elemTypeStr, nil
}

func elemTypeFromExpected(expected string) string {
	expected = strings.TrimSpace(expected)
	if strings.HasPrefix(expected, "[]") {
		return expected[2:]
	}
	if strings.HasPrefix(expected, "Slice[") && strings.HasSuffix(expected, "]") {
		return expected[6 : len(expected)-1]
	}
	if strings.HasPrefix(expected, "Set[") && strings.HasSuffix(expected, "]") {
		return expected[4 : len(expected)-1]
	}
	return "any"
}

// translateMapLit2 handles map literals.
func (g *gen) translateMapLit2(n *MapLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	kt, vt := mapKeyValFromExpected(expected)
	if kt == "" {
		kt = "any"
	}
	if vt == "" {
		vt = "any"
	}
	if n.Key != nil {
		kt = g.goType(n.Key, ctx.typeParams)
	}
	if n.Val != nil {
		vt = g.goType(n.Val, ctx.typeParams)
	}
	var elts []ast.Expr
	for _, pair := range n.Pairs {
		k, _, _ := g.translateExpr2(pair.Key, ctx, kt)
		v, _, _ := g.translateExpr2(pair.Value, ctx, vt)
		elts = append(elts, &ast.KeyValueExpr{Key: k, Value: v})
	}
	mapType := &ast.MapType{Key: ast.NewIdent(kt), Value: ast.NewIdent(vt)}
	return &ast.CompositeLit{Type: mapType, Elts: elts}, "map[" + kt + "]" + vt, nil
}

func mapKeyValFromExpected(expected string) (string, string) {
	expected = strings.TrimSpace(expected)
	if strings.HasPrefix(expected, "map[") {
		end := strings.Index(expected, "]")
		if end > 0 {
			return expected[4:end], expected[end+1:]
		}
	}
	if strings.HasPrefix(expected, "Map[") && strings.HasSuffix(expected, "]") {
		inner := expected[4 : len(expected)-1]
		parts := splitTopLevel(inner, ',')
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	return "", ""
}

// translateSetLit2 handles set literals.
func (g *gen) translateSetLit2(n *SetLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	et := elemTypeFromExpected(expected)
	if et == "any" || et == "" {
		et = "any"
	}
	if n.Elem != nil {
		et = g.goType(n.Elem, ctx.typeParams)
	}
	var elts []ast.Expr
	for _, elem := range n.Elems {
		ac, _, _ := g.translateExpr2(elem, ctx, et)
		elts = append(elts, &ast.KeyValueExpr{
			Key:   ac,
			Value: &ast.CompositeLit{Type: ast.NewIdent("struct{}")},
		})
	}
	mapType := &ast.MapType{
		Key:   ast.NewIdent(et),
		Value: ast.NewIdent("struct{}"),
	}
	return &ast.CompositeLit{Type: mapType, Elts: elts}, "map[" + et + "]struct{}", nil
}

// translateTupleLit2 handles tuple literals.
func (g *gen) translateTupleLit2(n *TupleLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	fields := make([]*ast.Field, len(n.Elems))
	elts := make([]ast.Expr, len(n.Elems))
	fieldTypes := make([]string, len(n.Elems))
	for i, elem := range n.Elems {
		code, typ, _ := g.translateExpr2(elem, ctx, "")
		elts[i] = &ast.KeyValueExpr{Key: ast.NewIdent("F" + strconv.Itoa(i)), Value: code}
		fieldTypes[i] = typ
		if typ == "" {
			typ = "any"
		}
		fields[i] = &ast.Field{
			Names: []*ast.Ident{ast.NewIdent("F" + strconv.Itoa(i))},
			Type:  ast.NewIdent(typ),
		}
	}
	structType := &ast.StructType{Fields: &ast.FieldList{List: fields}}
	parts := make([]string, len(fieldTypes))
	for i, ft := range fieldTypes {
		if ft == "" {
			ft = "any"
		}
		parts[i] = "F" + strconv.Itoa(i) + " " + ft
	}
	return &ast.CompositeLit{Type: structType, Elts: elts}, "struct { " + strings.Join(parts, "; ") + " }", nil
}

// translateGoExpr2 handles inline Go expressions.
func (g *gen) translateGoExpr2(n *GoExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	// Build substitution map from operands
	operands := map[string]string{}
	for _, op := range n.Operands {
		code, _, _ := g.translateExpr2(op.Value, ctx, "")
		operands[op.Name] = exprToGoString(code)
	}
	for _, tp := range n.TypeOperands {
		operands[tp.Name] = g.goType(tp.Type, ctx.typeParams)
	}
	// Substitute {name} placeholders in the Go code
	substituted := n.Code
	missing := ""
	substituted = goPlaceholderRE.ReplaceAllStringFunc(substituted, func(match string) string {
		name := match[1 : len(match)-1]
		code, ok := operands[name]
		if !ok {
			missing = name
			return match
		}
		return code
	})
	if missing != "" {
		return ast.NewIdent("_"), "", common.ErrorAtPos(n.Line, n.Column, "go code references unknown operand %q", missing)
	}

	resultType := g.goType(n.Result, ctx.typeParams)
	// DEBUG: resultType=%q expected=%q

	// Parse the substituted Go expression
	expr, err := goparser.ParseExpr(substituted)
	if err != nil {
		return ast.NewIdent(substituted), "", fmt.Errorf("invalid go expression: %v", err)
	}

	if resultType == "" || resultType == "struct{}" {
		return expr, "", nil
	}

	// Auto-wrapping: handle Result/Option type mismatches between
	// the Go expression's native return type and the declared result type.

	// Always wrap Result types — the Go expression may return error or a plain value.
	if strings.HasPrefix(resultType, "Result[") && strings.HasSuffix(resultType, "]") {
		innerParts := splitGenericArgs(resultType[7 : len(resultType)-1])
		if len(innerParts) == 2 {
			okType := strings.TrimSpace(innerParts[0])
			errType := strings.TrimSpace(innerParts[1])
			if goTupleErrorRE.MatchString(substituted) {
				return g.goTupleResultToResult(substituted, okType, errType), resultType, nil
			}
			return g.goExprToResult(expr, okType, errType), resultType, nil
		}
		return expr, resultType, nil
	}

	// Option wrapping: *T → Option[*T] (when go[T] declares Ref[T] but expected is Option[Ref[T]])
	if strings.HasPrefix(resultType, "*") && strings.HasPrefix(expected, "Option[") && strings.HasSuffix(expected, "]") {
		expInner := expected[7 : len(expected)-1]
		goInner := goast.TypeStringToGo(expInner)
		if resultType == goInner || strings.TrimPrefix(resultType, "*") == strings.TrimPrefix(goInner, "*") {
			return g.goRefToOption(expr, expInner), expected, nil
		}
	}

	// Direct Option wrapping: go[Option[T]] — the declared result IS Option.
	if strings.HasPrefix(resultType, "Option[") && strings.HasSuffix(resultType, "]") {
		innerType := resultType[7 : len(resultType)-1]
		if strings.HasPrefix(innerType, "*") {
			return g.goRefToOption(expr, innerType), resultType, nil
		}
		return g.goExprToOption(expr, innerType), resultType, nil
	}

	return expr, resultType, nil
}

// goRefToOption wraps a *T expression into Option[T] with nil checking.
func (g *gen) goRefToOption(expr ast.Expr, innerType string) ast.Expr {
	innerExpr := strToType(innerType)
	return &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: &ast.FieldList{List: []*ast.Field{{Type: &ast.IndexExpr{X: ast.NewIdent("Option"), Index: innerExpr}}}},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent("__mygo_go_ref")},
						Rhs: []ast.Expr{expr},
						Tok: token.DEFINE,
					},
					&ast.IfStmt{
						Cond: &ast.BinaryExpr{X: ast.NewIdent("__mygo_go_ref"), Op: token.EQL, Y: ast.NewIdent("nil")},
						Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: &ast.IndexExpr{X: ast.NewIdent("None"), Index: innerExpr}}}}}},
					},
					&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: &ast.IndexExpr{X: ast.NewIdent("Some"), Index: innerExpr}, Args: []ast.Expr{ast.NewIdent("__mygo_go_ref")}}}},
				},
			},
		},
	}
}

// goExprToOption wraps an expression into Option[T].
func (g *gen) goExprToOption(expr ast.Expr, innerType string) ast.Expr {
	innerExpr := strToType(innerType)
	return &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: &ast.FieldList{List: []*ast.Field{{Type: &ast.IndexExpr{X: ast.NewIdent("Option"), Index: innerExpr}}}},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: &ast.IndexExpr{X: ast.NewIdent("Some"), Index: innerExpr}, Args: []ast.Expr{expr}}}},
				},
			},
		},
	}
}

// goTupleResultToResult wraps a Go (T, error) call into Result[T, error].
func (g *gen) goTupleResultToResult(substituted, okType, errType string) ast.Expr {
	okTypeExpr := strToType(okType)
	errTypeExpr := strToType(errType)
	resultType := &ast.IndexListExpr{X: ast.NewIdent("Result"), Indices: []ast.Expr{okTypeExpr, errTypeExpr}}
	errCall := &ast.IndexListExpr{X: ast.NewIdent("Err"), Indices: []ast.Expr{okTypeExpr, errTypeExpr}}
	okCall := &ast.IndexListExpr{X: ast.NewIdent("Ok"), Indices: []ast.Expr{okTypeExpr, errTypeExpr}}
	return &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: &ast.FieldList{List: []*ast.Field{{Type: resultType}}},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent("__mygo_result_val"), ast.NewIdent("__mygo_result_err")},
						Rhs: []ast.Expr{ast.NewIdent("(" + substituted + ")")},
						Tok: token.DEFINE,
					},
					&ast.IfStmt{
						Cond: &ast.BinaryExpr{X: ast.NewIdent("__mygo_result_err"), Op: token.NEQ, Y: ast.NewIdent("nil")},
						Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: errCall, Args: []ast.Expr{ast.NewIdent("__mygo_result_err")}}}}}},
					},
					&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: okCall, Args: []ast.Expr{ast.NewIdent("__mygo_result_val")}}}},
				},
			},
		},
	}
}

// goExprToResult wraps an expression into Result[T, E].
// Executes the expression as statement, then returns Ok[T, E](zero(T)).
func (g *gen) goExprToResult(expr ast.Expr, okType, errType string) ast.Expr {
	errIdent := strToType(errType)
	okTypeExpr := strToType(okType)
	resultType := &ast.IndexListExpr{X: ast.NewIdent("Result"), Indices: []ast.Expr{okTypeExpr, errIdent}}
	okCall := &ast.IndexListExpr{X: ast.NewIdent("Ok"), Indices: []ast.Expr{okTypeExpr, errIdent}}
	zeroValue := strToZero(okType)
	return &ast.CallExpr{
		Fun: &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: &ast.FieldList{List: []*ast.Field{{Type: resultType}}},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{ast.NewIdent("_")},
						Rhs: []ast.Expr{expr},
						Tok: token.ASSIGN,
					},
					&ast.ReturnStmt{Results: []ast.Expr{&ast.CallExpr{Fun: okCall, Args: []ast.Expr{zeroValue}}}},
				},
			},
		},
	}
}

// strToType converts a string like "struct{}", "string", or "int" to an ast.Expr.
func strToType(s string) ast.Expr {
	s = strings.TrimSpace(s)
	switch s {
	case "struct{}", "Unit", "()":
		return ast.NewIdent("struct{}")
	case "string":
		return ast.NewIdent("string")
	case "int":
		return ast.NewIdent("int")
	case "bool":
		return ast.NewIdent("bool")
	case "any":
		return ast.NewIdent("any")
	case "error":
		return ast.NewIdent("error")
	default:
		if strings.HasPrefix(s, "*") {
			return &ast.StarExpr{X: strToType(s[1:])}
		}
		return ast.NewIdent(s)
	}
}

// strToZero creates a zero value expression for the given type string.
func strToZero(s string) ast.Expr {
	s = strings.TrimSpace(s)
	switch s {
	case "struct{}", "Unit", "()":
		return &ast.CompositeLit{Type: ast.NewIdent("struct{}")}
	case "string":
		return &ast.BasicLit{Kind: token.STRING, Value: `""`}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
		return &ast.BasicLit{Kind: token.INT, Value: "0"}
	case "bool":
		return ast.NewIdent("false")
	default:
		return ast.NewIdent("nil")
	}
}

// exprToGoString converts an AST expression back to a Go string.
// This is used for inline Go template substitution.
func exprToGoString(e ast.Expr) string {
	if e == nil {
		return ""
	}
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.SelectorExpr:
		return exprToGoString(v.X) + "." + v.Sel.Name
	case *ast.CallExpr:
		return exprToGoString(v.Fun) + "(...)"
	case *ast.UnaryExpr:
		return opToString(v.Op) + exprToGoString(v.X)
	case *ast.StarExpr:
		return "*" + exprToGoString(v.X)
	default:
		return "_"
	}
}

func opToString(op token.Token) string {
	switch op {
	case token.ADD:
		return "+"
	case token.SUB:
		return "-"
	case token.MUL:
		return "*"
	case token.QUO:
		return "/"
	case token.AND:
		return "&"
	case token.NOT:
		return "!"
	default:
		return "?"
	}
}

// fieldType looks up the Go type string for a field of the given base type.
func (g *gen) fieldType(baseType, field string) string {
	baseType = strings.TrimSpace(baseType)
	baseName, typeArgs := splitTypeArgs(baseType)
	st := g.pkg.Structs[baseName]
	if st == nil {
		// Check in interface methods
		iface := g.pkg.Interfaces[baseName]
		if iface != nil {
			for _, m := range iface.Methods {
				if m.Name == field {
					return g.goReturnType(m.Ret, nil)
				}
			}
		}
		return ""
	}
	subst := map[string]string{}
	for i, tp := range st.TypeParams {
		if i < len(typeArgs) {
			subst[tp] = typeArgs[i]
		}
	}
	for _, f := range st.Fields {
		if f.Name == field {
			return g.goTypeStringSubst(f.Type, subst)
		}
	}
	return ""
}

// goTypeStringSubst renders a TypeExpr as a Go type string with type param substitution.
func (g *gen) goTypeStringSubst(t TypeExpr, subst map[string]string) string {
	switch tt := t.(type) {
	case *NamedType:
		if subst != nil {
			if repl, ok := subst[tt.Name]; ok && len(tt.Args) == 0 {
				return repl
			}
		}
		result := g.goType(tt, nil)
		if len(tt.Args) > 0 {
			args := make([]string, len(tt.Args))
			for i, a := range tt.Args {
				args[i] = g.goTypeStringSubst(a, subst)
			}
			baseName := tt.Name
			switch baseName {
			case "Ref":
				return "*" + args[0]
			case "Slice":
				return "[]" + args[0]
			case "Map":
				return "map[" + args[0] + "]" + args[1]
			case "Set":
				return "map[" + args[0] + "]struct{}"
			default:
				return baseName + "[" + strings.Join(args, ", ") + "]"
			}
		}
		return result
	case *FuncType:
		params := make([]string, len(tt.Params))
		for i, p := range tt.Params {
			params[i] = g.goTypeStringSubst(p, subst)
		}
		ret := g.goTypeStringSubst(tt.Ret, subst)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	default:
		return "any"
	}
}

// lookupMyGoFieldType extracts the type of a field expression from the AST.
func lookupMyGoFieldType(expr Expr, field string, g *gen) string {
	switch n := expr.(type) {
	case *IdentExpr:
		// Look up from struct field by type name
		if st := g.pkg.Structs[n.Name]; st != nil {
			for _, f := range st.Fields {
				if f.Name == field {
					return g.goType(f.Type, nil)
				}
			}
		}
	}
	return ""
}

// exprUsesIdent checks if an identifier with the given name is used in an expression.
func exprUsesIdent(e Expr, name string) bool {
	switch n := e.(type) {
	case *IdentExpr:
		return n.Name == name
	case *CallExpr:
		if exprUsesIdent(n.Callee, name) {
			return true
		}
		for _, arg := range n.Args {
			if exprUsesIdent(arg, name) {
				return true
			}
		}
	case *FieldExpr:
		return exprUsesIdent(n.Expr, name)
	case *BinaryExpr:
		return exprUsesIdent(n.Left, name) || exprUsesIdent(n.Right, name)
	case *PrefixExpr:
		return exprUsesIdent(n.Expr, name)
	case *CastExpr:
		return exprUsesIdent(n.Expr, name)
	case *IfExpr:
		return exprUsesIdent(n.Cond, name) || exprUsesIdent(n.Then, name) || exprUsesIdent(n.Else, name)
	case *SwitchExpr:
		if exprUsesIdent(n.Target, name) {
			return true
		}
		for _, c := range n.Cases {
			if exprUsesIdent(c.Body, name) {
				return true
			}
		}
	case *BlockExpr:
		for _, stmt := range n.Stmts {
			switch s := stmt.(type) {
			case *ExprStmt:
				if exprUsesIdent(s.Expr, name) {
					return true
				}
			case *LetStmt:
				if exprUsesIdent(s.Value, name) {
					return true
				}
			case *AssignStmt:
				if exprUsesIdent(s.Value, name) {
					return true
				}
			}
		}
	case *StructLitExpr:
		for _, f := range n.Fields {
			if exprUsesIdent(f.Value, name) {
				return true
			}
		}
	case *GoExpr:
		for _, op := range n.Operands {
			if exprUsesIdent(op.Value, name) {
				return true
			}
		}
	}
	return false
}

// emitBindDestructure handles tuple pattern destructuring.
// Returns the updated statements slice.
func (g *gen) emitBindDestructure(stmts []ast.Stmt, ctx *egCtx, rhs ast.Expr, pat *BindTuplePattern) []ast.Stmt {
	// Check for nested tuple patterns
	hasNested := false
	for _, elem := range pat.Elems {
		if _, ok := elem.(*BindTuplePattern); ok {
			hasNested = true
			break
		}
	}

	if hasNested {
		// Use temp variable for nested destructuring
		g.localSeq++
		tmpName := "__tuple_" + strconv.Itoa(g.localSeq)
		targets := make([]ast.Expr, len(pat.Elems))
		for i, elem := range pat.Elems {
			if name, ok := elem.(*BindNamePattern); ok && name.Name != "_" {
				g.localSeq++
				actual := sanitizeIdent(name.Name) + "_" + strconv.Itoa(g.localSeq)
				ctx.bindings[name.Name] = actual
				targets[i] = ast.NewIdent(actual)
			} else if _, ok := elem.(*BindTuplePattern); ok {
				targets[i] = ast.NewIdent(tmpName)
			} else {
				targets[i] = ast.NewIdent("_")
			}
		}
		stmts = append(stmts, &ast.AssignStmt{Lhs: targets, Rhs: []ast.Expr{rhs}, Tok: token.DEFINE})

		// Now destructure the temp variable for nested patterns
		for i, elem := range pat.Elems {
			if tuple, ok := elem.(*BindTuplePattern); ok {
				stmts = g.emitBindDestructureFromField(stmts, ctx, tmpName, i, tuple)
			}
		}
	} else {
		// Simple flat tuple - direct destructuring
		targets := make([]ast.Expr, len(pat.Elems))
		for i, elem := range pat.Elems {
			if name, ok := elem.(*BindNamePattern); ok && name.Name != "_" {
				g.localSeq++
				actual := sanitizeIdent(name.Name) + "_" + strconv.Itoa(g.localSeq)
				ctx.bindings[name.Name] = actual
				targets[i] = ast.NewIdent(actual)
			} else {
				targets[i] = ast.NewIdent("_")
			}
		}
		stmts = append(stmts, &ast.AssignStmt{Lhs: targets, Rhs: []ast.Expr{rhs}, Tok: token.DEFINE})
	}
	return stmts
}

// emitBindDestructureFromField destructures a tuple pattern from a field of a temp variable.
func (g *gen) emitBindDestructureFromField(stmts []ast.Stmt, ctx *egCtx, tmpName string, fieldIdx int, pat *BindTuplePattern) []ast.Stmt {
	fieldExpr := &ast.SelectorExpr{
		X:   ast.NewIdent(tmpName),
		Sel: ast.NewIdent("F" + strconv.Itoa(fieldIdx)),
	}

	hasNested := false
	for _, elem := range pat.Elems {
		if _, ok := elem.(*BindTuplePattern); ok {
			hasNested = true
			break
		}
	}

	if hasNested {
		g.localSeq++
		innerTmp := "__tuple_" + strconv.Itoa(g.localSeq)
		targets := make([]ast.Expr, len(pat.Elems))
		for i, elem := range pat.Elems {
			if name, ok := elem.(*BindNamePattern); ok && name.Name != "_" {
				g.localSeq++
				actual := sanitizeIdent(name.Name) + "_" + strconv.Itoa(g.localSeq)
				ctx.bindings[name.Name] = actual
				targets[i] = ast.NewIdent(actual)
			} else if _, ok := elem.(*BindTuplePattern); ok {
				targets[i] = ast.NewIdent(innerTmp)
			} else {
				targets[i] = ast.NewIdent("_")
			}
		}
		stmts = append(stmts, &ast.AssignStmt{Lhs: targets, Rhs: []ast.Expr{fieldExpr}, Tok: token.DEFINE})

		for i, elem := range pat.Elems {
			if tuple, ok := elem.(*BindTuplePattern); ok {
				stmts = g.emitBindDestructureFromField(stmts, ctx, innerTmp, i, tuple)
			}
		}
	} else {
		targets := make([]ast.Expr, len(pat.Elems))
		for i, elem := range pat.Elems {
			if name, ok := elem.(*BindNamePattern); ok && name.Name != "_" {
				g.localSeq++
				actual := sanitizeIdent(name.Name) + "_" + strconv.Itoa(g.localSeq)
				ctx.bindings[name.Name] = actual
				targets[i] = ast.NewIdent(actual)
			} else {
				targets[i] = ast.NewIdent("_")
			}
		}
		stmts = append(stmts, &ast.AssignStmt{Lhs: targets, Rhs: []ast.Expr{fieldExpr}, Tok: token.DEFINE})
	}
	return stmts
}

// goTypeFromExpr extracts the Go type string from an expression, given context.
func (g *gen) goTypeFromExpr(e Expr, ctx *egCtx) string {
	switch n := e.(type) {
	case *IdentExpr:
		return ctx.locals[n.Name]
	case *LiteralExpr:
		if n.Kind == "number" {
			return "int"
		}
		return n.Kind
	case *CallExpr:
		if id, ok := n.Callee.(*IdentExpr); ok {
			return ctx.locals[id.Name]
		}
		return "any"
	default:
		return "any"
	}
}
