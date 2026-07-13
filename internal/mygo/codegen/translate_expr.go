package codegen

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

// translateBlockStmts translates statements within a BlockExpr.
func (g *gen) translateBlockStmts(n *BlockExpr, ctx *egCtx, returnExpected string, retTypes []string) ([]ast.Stmt, error) {
	child := ctx.child()
	var stmts []ast.Stmt
	for i, stmt := range n.Stmts {
		isLast := i == len(n.Stmts)-1
		switch s := stmt.(type) {
		case *ExprStmt:
			if branch := branchStmtForExpr(s.Expr); branch != nil {
				stmts = append(stmts, branch)
				continue
			}
			if goExpr, ok := s.Expr.(*GoExpr); ok && g.isUnitGoExpr(goExpr, child) {
				goStmts, err := g.translateGoUnitStmts(goExpr, child)
				if err == nil {
					if isLast && returnExpected != "" {
						stmts = append(stmts, goStmts...)
						stmts = append(stmts, &ast.ReturnStmt{})
					} else {
						stmts = append(stmts, goStmts...)
					}
					continue
				}
			}
			expectedType := ""
			if isLast && returnExpected != "" {
				expectedType = returnExpected
			}
			code, typ, err := g.translateExpr(s.Expr, child, expectedType)
			if err != nil {
				return stmts, err
			}
			if isLast && returnExpected != "" {
				stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
			} else if branch := branchStmtForExpr(s.Expr); branch != nil {
				stmts = append(stmts, branch)
			} else if typ == "" {
				stmts = append(stmts, &ast.ExprStmt{X: code})
			} else {
				stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("_")}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
			}
		case *ReturnStmt:
			if s.Value != nil {
				code, _, err := g.translateExpr(s.Value, child, returnExpected)
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
					code, _, err := g.translateExpr(s.Value, child, "")
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
			code, valType, err := g.translateExpr(s.Value, child, expectedType)
			if err != nil {
				return stmts, err
			}
			if s.Name == "_" {
				// Use ExprStmt to discard the result — handles multi-return Go calls safely.
				stmts = append(stmts, &ast.ExprStmt{X: code})
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
			code, _, err := g.translateExpr(s.Value, child, child.locals[s.Name])
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
		}
	}
	return stmts, nil
}

func (g *gen) isUnitGoExpr(n *GoExpr, ctx *egCtx) bool {
	resultType := g.goType(n.Result, ctx.typeParams)
	return resultType == "" || resultType == "struct{}"
}

func branchStmtForExpr(e Expr) *ast.BranchStmt {
	id, ok := e.(*IdentExpr)
	if !ok {
		return nil
	}
	switch id.Name {
	case "break":
		return &ast.BranchStmt{Tok: token.BREAK}
	case "continue":
		return &ast.BranchStmt{Tok: token.CONTINUE}
	}
	return nil
}

func stmtForExpr(src Expr, code ast.Expr, typ string) ast.Stmt {
	if branch := branchStmtForExpr(src); branch != nil {
		return branch
	}
	if typ != "" {
		return &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("_")}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN}
	}
	return &ast.ExprStmt{X: code}
}

// translateExpr is the main expression translator.
func (g *gen) translateExpr(e Expr, ctx *egCtx, expected string) (ast.Expr, string, error) {
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
				callee := &ast.IndexExpr{X: ast.NewIdent("None"), Index: goTypeExprFromString(tas[0])}
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
		case "rune":
			return &ast.BasicLit{Kind: token.CHAR, Value: strconv.QuoteRune([]rune(n.Value)[0])}, "rune", nil
		case "bool":
			if n.Value == "true" {
				return ast.NewIdent("true"), "bool", nil
			}
			return ast.NewIdent("false"), "bool", nil
		}
	case *BinaryExpr:
		left, lt, err := g.translateExpr(n.Left, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if left == nil {
			line, col := common.NodePos(n.Left)
			return nil, "", common.ErrorAtPos(line, col, "binary left operand produced nil Go AST")
		}
		right, rt, err := g.translateExpr(n.Right, ctx, lt)
		if err != nil {
			return nil, "", err
		}
		if right == nil {
			line, col := common.NodePos(n.Right)
			return nil, "", common.ErrorAtPos(line, col, "binary right operand produced nil Go AST")
		}
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
				callee, _, _ := g.translateExpr(call.Callee, ctx, "")
				args := make([]ast.Expr, 0, len(call.Args)+1)
				for _, a := range call.Args {
					ac, _, _ := g.translateExpr(a, ctx, "")
					args = append(args, ac)
				}
				args = append(args, left)
				return &ast.CallExpr{Fun: callee, Args: args}, lt, nil
			}
			return &ast.CallExpr{Fun: right, Args: []ast.Expr{left}}, lt, nil
		case "<|":
			if call, ok := n.Left.(*CallExpr); ok {
				callee, _, _ := g.translateExpr(call.Callee, ctx, "")
				args := make([]ast.Expr, 0, len(call.Args)+1)
				for _, a := range call.Args {
					ac, _, _ := g.translateExpr(a, ctx, "")
					args = append(args, ac)
				}
				args = append(args, right)
				return &ast.CallExpr{Fun: callee, Args: args}, lt, nil
			}
			return &ast.CallExpr{Fun: left, Args: []ast.Expr{right}}, lt, nil
		}
	case *PrefixExpr:
		expr, typ, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if expr == nil {
			line, col := common.NodePos(n.Expr)
			return nil, "", common.ErrorAtPos(line, col, "prefix operand produced nil Go AST")
		}
		switch n.Op {
		case "!":
			return &ast.UnaryExpr{Op: token.NOT, X: expr}, "bool", nil
		case "-":
			return &ast.UnaryExpr{Op: token.SUB, X: expr}, typ, nil
		}
	case *CastExpr:
		code, _, err := g.translateExpr(n.Expr, ctx, g.goType(n.Type, ctx.typeParams))
		if err != nil {
			return nil, "", err
		}
		if code == nil {
			line, col := common.NodePos(n.Expr)
			return nil, "", common.ErrorAtPos(line, col, "cast operand produced nil Go AST")
		}
		target := g.goType(n.Type, ctx.typeParams)
		return &ast.CallExpr{Fun: ast.NewIdent(target), Args: []ast.Expr{code}}, target, nil
	case *FieldExpr:
		base, bt, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if base == nil {
			line, col := common.NodePos(n.Expr)
			return nil, "", common.ErrorAtPos(line, col, "field receiver produced nil Go AST")
		}
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
		return g.translateCall(n, ctx, expected)
	case *IfExpr:
		return g.translateIf(n, ctx, expected)
	case *SwitchExpr:
		return g.translateSwitch(n, ctx, expected)
	case *WhileExpr:
		return g.translateWhile(n, ctx)
	case *BlockExpr:
		stmts, err := g.translateBlockStmts(n, ctx, expected, nil)
		if err != nil {
			return nil, "", err
		}
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
		return g.translateStructLit(n, ctx, expected)
	case *SliceLitExpr:
		return g.translateSliceLit(n, ctx, expected)
	case *MapLitExpr:
		return g.translateMapLit(n, ctx, expected)
	case *SetLitExpr:
		return g.translateSetLit(n, ctx, expected)
	case *TupleLitExpr:
		return g.translateTupleLit(n, ctx, expected)
	case *UnitLitExpr:
		return &ast.CompositeLit{Type: &ast.StructType{Fields: &ast.FieldList{}}}, "Unit", nil
	case *FuncLitExpr:
		return g.translateFuncLit(n, ctx)
	case *GoExpr:
		return g.translateGoExpr(n, ctx, expected)
	}
	line, col := common.NodePos(e)
	return nil, "", common.ErrorAtPos(line, col, "unsupported expression %T", e)
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
