package codegen

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

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
