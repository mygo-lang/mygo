package codegen

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"regexp"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

var goPlaceholderRE = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)
var goTupleErrorRE = regexp.MustCompile(`func\(\)\s*\([^)]+,\s*error\s*\)`)

// translateGoExpr handles inline Go expressions.
func (g *gen) translateGoExpr(n *GoExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	// Build substitution map from operands
	operands := map[string]string{}
	for _, op := range n.Operands {
		code, _, _ := g.translateExpr(op.Value, ctx, "")
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
		return ast.NewIdent("_"), "", common.ErrorAtPos(g.currentFile, n.Line, n.Column, "go code references unknown operand %q", missing)
	}

	resultType := g.goType(n.Result, ctx.typeParams)
	// DEBUG: resultType=%q expected=%q

	// Parse the substituted Go expression
	expr, err := goparser.ParseExpr(substituted)
	if err != nil {
		return ast.NewIdent(substituted), "", common.ErrorAtPos(g.currentFile, n.Line, n.Column, "invalid go expression: %v", err)
	}

	if resultType == "" || resultType == "struct{}" {
		return expr, "", nil
	}

	// Auto-wrapping: handle Result/Option type mismatches between
	// the Go expression's native return type and the declared result type.

	// Handle go[((), error)] → Result[T, error] when expected is Result[T, error].
	// The go expression returns a Go tuple (T, error) but the enclosing function
	// expects Result[T, error].
	if goTupleErrorRE.MatchString(substituted) && strings.HasPrefix(expected, "Result[") && strings.HasSuffix(expected, "]") {
		innerParts := splitGenericArgs(expected[7 : len(expected)-1])
		if len(innerParts) == 2 {
			okType := strings.TrimSpace(innerParts[0])
			errType := strings.TrimSpace(innerParts[1])
			return g.goTupleResultToResult(substituted, okType, errType), expected, nil
		}
	}

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
		if strings.Contains(substituted, "Some") || strings.Contains(substituted, "None") {
			return expr, resultType, nil
		}
		innerType := resultType[7 : len(resultType)-1]
		if strings.HasPrefix(innerType, "*") {
			return g.goRefToOption(expr, innerType), resultType, nil
		}
		return g.goExprToOption(expr, innerType), resultType, nil
	}

	return expr, resultType, nil
}

func (g *gen) translateGoUnitStmts(n *GoExpr, ctx *egCtx) ([]ast.Stmt, error) {
	substituted, err := g.substituteGoCode(n, ctx)
	if err != nil {
		return nil, err
	}
	return parseInlineGoUnitStmts(g.currentFile, substituted)
}

func (g *gen) substituteGoCode(n *GoExpr, ctx *egCtx) (string, error) {
	operands := map[string]string{}
	for _, op := range n.Operands {
		code, _, _ := g.translateExpr(op.Value, ctx, "")
		operands[op.Name] = exprToGoString(code)
	}
	for _, tp := range n.TypeOperands {
		operands[tp.Name] = g.goType(tp.Type, ctx.typeParams)
	}
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
		return "", common.ErrorAtPos(g.currentFile, n.Line, n.Column, "go code references unknown operand %q", missing)
	}
	return substituted, nil
}

func parseInlineGoUnitStmts(srcFile, code string) ([]ast.Stmt, error) {
	pfile, err := goparser.ParseFile(token.NewFileSet(), "inline.go", "package inline\nfunc _(){\n"+code+"\n}", 0)
	if err != nil {
		return nil, common.ErrorAtPos(srcFile, 0, 0, "invalid go statement: %v", err)
	}
	if len(pfile.Decls) != 1 {
		return nil, common.ErrorAtPos(srcFile, 0, 0, "invalid go statement")
	}
	fn, ok := pfile.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil || len(fn.Body.List) == 0 {
		return nil, common.ErrorAtPos(srcFile, 0, 0, "invalid go statement")
	}
	return fn.Body.List, nil
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
						Rhs: []ast.Expr{&ast.ParenExpr{X: g.parseGoExprOrIdent(substituted)}},
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

// parseGoExprOrIdent tries to parse a string as a Go expression. Falls back to ast.NewIdent.
func (g *gen) parseGoExprOrIdent(s string) ast.Expr {
	expr, err := goparser.ParseExpr(s)
	if err != nil {
		return ast.NewIdent(s)
	}
	return expr
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
