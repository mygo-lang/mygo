package codegen

import (
	"go/ast"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
)

// translateStructLit handles struct literal construction.
func (g *gen) translateStructLit(n *StructLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
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
		code, _, _ := g.translateExpr(f.Value, ctx, "")
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

// translateSliceLit handles slice literals.
func (g *gen) translateSliceLit(n *SliceLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
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
		ac, _, _ := g.translateExpr(elem, ctx, elemTypeStr)
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

// translateMapLit handles map literals.
func (g *gen) translateMapLit(n *MapLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
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
		k, _, _ := g.translateExpr(pair.Key, ctx, kt)
		v, _, _ := g.translateExpr(pair.Value, ctx, vt)
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

// translateSetLit handles set literals.
func (g *gen) translateSetLit(n *SetLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	et := elemTypeFromExpected(expected)
	if et == "any" || et == "" {
		et = "any"
	}
	if n.Elem != nil {
		et = g.goType(n.Elem, ctx.typeParams)
	}
	var elts []ast.Expr
	for _, elem := range n.Elems {
		ac, _, _ := g.translateExpr(elem, ctx, et)
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

// translateTupleLit handles tuple literals.
func (g *gen) translateTupleLit(n *TupleLitExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	fields := make([]*ast.Field, len(n.Elems))
	elts := make([]ast.Expr, len(n.Elems))
	fieldTypes := make([]string, len(n.Elems))
	for i, elem := range n.Elems {
		code, typ, _ := g.translateExpr(elem, ctx, "")
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
