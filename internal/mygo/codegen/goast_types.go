package codegen

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// ============================================================
// go/ast Type Expression Renderers
// Replaces jennifer_gen.go
// ============================================================

// goastTypeExpr converts a MyGO TypeExpr to a go/ast expression.
func goastTypeExpr(t TypeExpr) ast.Expr {
	switch tt := t.(type) {
	case *NamedType:
		switch tt.Name {
		case "Int":
			return ast.NewIdent("int")
		case "Int8":
			return ast.NewIdent("int8")
		case "Int16":
			return ast.NewIdent("int16")
		case "Int32":
			return ast.NewIdent("int32")
		case "Int64":
			return ast.NewIdent("int64")
		case "UInt":
			return ast.NewIdent("uint")
		case "UInt8":
			return ast.NewIdent("uint8")
		case "UInt16":
			return ast.NewIdent("uint16")
		case "UInt32":
			return ast.NewIdent("uint32")
		case "UInt64":
			return ast.NewIdent("uint64")
		case "Byte":
			return ast.NewIdent("byte")
		case "Rune":
			return ast.NewIdent("rune")
		case "Float32":
			return ast.NewIdent("float32")
		case "Float64":
			return ast.NewIdent("float64")
		case "String":
			return ast.NewIdent("string")
		case "Bool":
			return ast.NewIdent("bool")
		case "Unit":
			return ast.NewIdent("struct{}")
		case "Ref":
			if len(tt.Args) == 1 {
				return &ast.StarExpr{X: goastTypeExpr(tt.Args[0])}
			}
		case "Slice":
			if len(tt.Args) == 1 {
				return &ast.ArrayType{Elt: goastTypeExpr(tt.Args[0])}
			}
		case "Map":
			if len(tt.Args) == 2 {
				return &ast.MapType{Key: goastTypeExpr(tt.Args[0]), Value: goastTypeExpr(tt.Args[1])}
			}
		case "Set":
			if len(tt.Args) == 1 {
				return &ast.MapType{Key: goastTypeExpr(tt.Args[0]), Value: ast.NewIdent("struct{}")}
			}
		case "Any":
			return ast.NewIdent("any")
		case "Chan":
			if len(tt.Args) == 1 {
				return &ast.ChanType{Dir: ast.SEND | ast.RECV, Value: goastTypeExpr(tt.Args[0])}
			}
		case "SendChan":
			if len(tt.Args) == 1 {
				return &ast.ChanType{Dir: ast.SEND, Value: goastTypeExpr(tt.Args[0])}
			}
		case "RecvChan":
			if len(tt.Args) == 1 {
				return &ast.ChanType{Dir: ast.RECV, Value: goastTypeExpr(tt.Args[0])}
			}
		}
		if len(tt.Args) == 0 {
			return ast.NewIdent(tt.Name)
		}
		args := make([]ast.Expr, len(tt.Args))
		for i, a := range tt.Args {
			args[i] = goastTypeExpr(a)
		}
		return genericIdent(tt.Name, args...)
	case *FuncType:
		params := make([]*ast.Field, len(tt.Params))
		for i, p := range tt.Params {
			params[i] = &ast.Field{Type: goastTypeExpr(p)}
		}
		if isUnitType(tt.Ret) {
			return &ast.FuncType{Params: &ast.FieldList{List: params}}
		}
		ret := goastTypeExpr(tt.Ret)
		return &ast.FuncType{Params: &ast.FieldList{List: params}, Results: &ast.FieldList{List: []*ast.Field{{Type: ret}}}}
	case *TupleType:
		fields := make([]*ast.Field, len(tt.Elems))
		for i, e := range tt.Elems {
			fields[i] = &ast.Field{
				Names: []*ast.Ident{ast.NewIdent("F" + strconv.Itoa(i))},
				Type:  goastTypeExpr(e),
			}
		}
		return &ast.StructType{Fields: &ast.FieldList{List: fields}}
	default:
		return ast.NewIdent("any")
	}
}

// goastHKTTypeExpr renders a type expression, replacing HKT type parameters
// (e.g. C[A]) with HKT[C, A] encoding.
func goastHKTTypeExpr(t TypeExpr, hktSet map[string]struct{}) ast.Expr {
	switch tt := t.(type) {
	case *NamedType:
		if len(tt.Args) > 0 && hktSet != nil {
			if _, ok := hktSet[tt.Name]; ok {
				args := make([]ast.Expr, 0, len(tt.Args)+1)
				args = append(args, ast.NewIdent(tt.Name))
				for _, a := range tt.Args {
					args = append(args, goastHKTTypeExpr(a, hktSet))
				}
				return genericIdent("HKT", args...)
			}
		}
		return goastTypeExpr(tt)
	case *FuncType:
		params := make([]*ast.Field, len(tt.Params))
		for i, p := range tt.Params {
			params[i] = &ast.Field{Type: goastHKTTypeExpr(p, hktSet)}
		}
		if isUnitType(tt.Ret) {
			return &ast.FuncType{Params: &ast.FieldList{List: params}}
		}
		ret := goastHKTTypeExpr(tt.Ret, hktSet)
		return &ast.FuncType{Params: &ast.FieldList{List: params}, Results: &ast.FieldList{List: []*ast.Field{{Type: ret}}}}
	case *TupleType:
		fields := make([]*ast.Field, len(tt.Elems))
		for i, e := range tt.Elems {
			fields[i] = &ast.Field{
				Names: []*ast.Ident{ast.NewIdent("F" + strconv.Itoa(i))},
				Type:  goastHKTTypeExpr(e, hktSet),
			}
		}
		return &ast.StructType{Fields: &ast.FieldList{List: fields}}
	default:
		return ast.NewIdent("any")
	}
}

func genericIdent(name string, args ...ast.Expr) ast.Expr {
	if len(args) == 0 {
		return ast.NewIdent(name)
	}
	if len(args) == 1 {
		return &ast.IndexExpr{X: ast.NewIdent(name), Index: args[0]}
	}
	return &ast.IndexListExpr{X: ast.NewIdent(name), Indices: args}
}

func goTypeExprFromString(typ string) ast.Expr {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ast.NewIdent("any")
	}
	typ = mygoSigTypeToGo(typ)
	if expr := parseGoTypeString(typ); expr != nil {
		return expr
	}
	if expr, err := goparser.ParseExpr(typ); err == nil {
		return expr
	}
	return ast.NewIdent(typ)
}

func (g *gen) goTypeExprFromString(typ string) ast.Expr {
	return g.goTypeExprFromStringSeen(typ, map[string]bool{})
}

func (g *gen) goTypeExprFromStringSeen(typ string, seen map[string]bool) ast.Expr {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ast.NewIdent("any")
	}
	typ = mygoSigTypeToGo(typ)
	if expr := g.parseGoTypeString(typ, seen); expr != nil {
		return expr
	}
	if expr, err := goparser.ParseExpr(typ); err == nil {
		if id, ok := expr.(*ast.Ident); ok {
			if sel := g.importedTypeSelector(id.Name); sel != nil {
				return sel
			}
		}
		return expr
	}
	if sel := g.importedTypeSelector(typ); sel != nil {
		return sel
	}
	return ast.NewIdent(typ)
}

func (g *gen) goTypeExprForAssertion(typ string) ast.Expr {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ast.NewIdent("any")
	}
	typ = mygoSigTypeToGo(typ)
	switch typ {
	case "Any":
		return ast.NewIdent("any")
	}
	if expr := g.parseGoTypeString(typ, map[string]bool{}); expr != nil {
		return expr
	}
	if expr, err := goparser.ParseExpr(typ); err == nil {
		if id, ok := expr.(*ast.Ident); ok {
			if sel := g.importedTypeSelector(id.Name); sel != nil {
				return sel
			}
		}
		return expr
	}
	if sel := g.importedTypeSelector(typ); sel != nil {
		return sel
	}
	return ast.NewIdent(typ)
}

// goTypeExprForAssertion converts a mygo type name to a Go expression suitable
// for type assertions. It maps mygo built-in names (e.g. "Any") to their Go
// equivalents (e.g. "any").
func goTypeExprForAssertion(typ string) ast.Expr {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ast.NewIdent("any")
	}
	typ = mygoSigTypeToGo(typ)
	// Map mygo built-in type names to Go identifiers used in type assertions.
	switch typ {
	case "Any":
		return ast.NewIdent("any")
	}
	if expr := parseGoTypeString(typ); expr != nil {
		return expr
	}
	if expr, err := goparser.ParseExpr(typ); err == nil {
		return expr
	}
	return ast.NewIdent(typ)
}

func parseGoTypeString(typ string) ast.Expr {
	if strings.HasPrefix(typ, "map[") {
		end := matchingTypeArgEnd(typ, len("map"))
		if end > len("map") && end+1 < len(typ) {
			key := strings.TrimSpace(typ[len("map")+1 : end])
			value := strings.TrimSpace(typ[end+1:])
			return &ast.MapType{
				Key:   goTypeExprFromString(key),
				Value: goTypeExprFromString(value),
			}
		}
	}
	expr, err := goparser.ParseExpr("struct{ X " + typ + " }")
	if err != nil {
		return nil
	}
	st, ok := expr.(*ast.StructType)
	if !ok || st.Fields == nil || len(st.Fields.List) != 1 {
		return nil
	}
	return st.Fields.List[0].Type
}

func (g *gen) parseGoTypeString(typ string, seen map[string]bool) ast.Expr {
	if strings.HasPrefix(typ, "map[") {
		end := matchingTypeArgEnd(typ, len("map"))
		if end > len("map") && end+1 < len(typ) {
			key := strings.TrimSpace(typ[len("map")+1 : end])
			value := strings.TrimSpace(typ[end+1:])
			return &ast.MapType{
				Key:   g.goTypeExprFromStringSeen(key, seen),
				Value: g.goTypeExprFromStringSeen(value, seen),
			}
		}
	}
	expr, err := goparser.ParseExpr("struct{ X " + typ + " }")
	if err != nil {
		return nil
	}
	st, ok := expr.(*ast.StructType)
	if !ok || st.Fields == nil || len(st.Fields.List) != 1 {
		return nil
	}
	return g.qualifyImportedTypeExpr(st.Fields.List[0].Type, seen)
}

func (g *gen) qualifyImportedTypeExpr(expr ast.Expr, seen map[string]bool) ast.Expr {
	switch e := expr.(type) {
	case *ast.Ident:
		if sel := g.importedTypeSelector(e.Name); sel != nil {
			return sel
		}
		return e
	case *ast.ArrayType:
		e.Elt = g.qualifyImportedTypeExpr(e.Elt, seen)
		return e
	case *ast.StarExpr:
		e.X = g.qualifyImportedTypeExpr(e.X, seen)
		return e
	case *ast.MapType:
		e.Key = g.qualifyImportedTypeExpr(e.Key, seen)
		e.Value = g.qualifyImportedTypeExpr(e.Value, seen)
		return e
	case *ast.ChanType:
		e.Value = g.qualifyImportedTypeExpr(e.Value, seen)
		return e
	case *ast.IndexExpr:
		e.X = g.qualifyImportedTypeExpr(e.X, seen)
		e.Index = g.qualifyImportedTypeExpr(e.Index, seen)
		return e
	case *ast.IndexListExpr:
		e.X = g.qualifyImportedTypeExpr(e.X, seen)
		for i := range e.Indices {
			e.Indices[i] = g.qualifyImportedTypeExpr(e.Indices[i], seen)
		}
		return e
	case *ast.SelectorExpr:
		return e
	case *ast.FuncType:
		if e.Params != nil {
			for _, f := range e.Params.List {
				f.Type = g.qualifyImportedTypeExpr(f.Type, seen)
			}
		}
		if e.Results != nil {
			for _, f := range e.Results.List {
				f.Type = g.qualifyImportedTypeExpr(f.Type, seen)
			}
		}
		return e
	default:
		return e
	}
}

func (g *gen) importedTypeSelector(name string) ast.Expr {
	if name == "" || strings.Contains(name, ".") || g == nil || g.typedInfo == nil {
		return nil
	}
	for alias, pkg := range g.typedInfo.MyGoPackages {
		if pkg == nil {
			continue
		}
		if _, ok := pkg.Types[name]; ok {
			return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(name)}
		}
		if _, ok := pkg.Structs[name]; ok {
			return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(name)}
		}
		if _, ok := pkg.Enums[name]; ok {
			return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(name)}
		}
		if _, ok := pkg.Interfaces[name]; ok {
			return &ast.SelectorExpr{X: ast.NewIdent(alias), Sel: ast.NewIdent(name)}
		}
	}
	return nil
}

func typeParamFields(params []string) *ast.FieldList {
	return typeParamFieldsWithConstraints(params, nil)
}

func typeParamFieldsWithConstraints(params []string, constraints map[string]string) *ast.FieldList {
	if len(params) == 0 {
		return nil
	}
	fields := make([]*ast.Field, len(params))
	for i, p := range params {
		constraint := "any"
		if constraints != nil && constraints[p] != "" {
			constraint = constraints[p]
		}
		fields[i] = &ast.Field{Names: []*ast.Ident{ast.NewIdent(p)}, Type: ast.NewIdent(constraint)}
	}
	return &ast.FieldList{List: fields}
}

func mapKeyTypeParamConstraints(t TypeExpr) map[string]string {
	out := map[string]string{}
	collectMapKeyTypeParamConstraints(t, out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeMapKeyTypeParamConstraints(out map[string]string, types ...TypeExpr) map[string]string {
	for _, t := range types {
		if t == nil {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		collectMapKeyTypeParamConstraints(t, out)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectMapKeyTypeParamConstraints(t TypeExpr, out map[string]string) {
	switch tt := t.(type) {
	case *NamedType:
		if tt.Name == "Map" && len(tt.Args) == 2 {
			if key, ok := tt.Args[0].(*NamedType); ok && len(key.Args) == 0 {
				out[key.Name] = "comparable"
			}
		}
		if tt.Name == "Set" && len(tt.Args) == 1 {
			if key, ok := tt.Args[0].(*NamedType); ok && len(key.Args) == 0 {
				out[key.Name] = "comparable"
			}
		}
		for _, arg := range tt.Args {
			collectMapKeyTypeParamConstraints(arg, out)
		}
	case *FuncType:
		for _, p := range tt.Params {
			collectMapKeyTypeParamConstraints(p, out)
		}
		collectMapKeyTypeParamConstraints(tt.Ret, out)
	case *TupleType:
		for _, elem := range tt.Elems {
			collectMapKeyTypeParamConstraints(elem, out)
		}
	}
}

func typeArgIdents(params []string) []ast.Expr {
	if len(params) == 0 {
		return nil
	}
	out := make([]ast.Expr, len(params))
	for i, p := range params {
		out[i] = ast.NewIdent(p)
	}
	return out
}

func goastFieldName(name string) string {
	if name == "embed" {
		return ""
	}
	return sanitizeIdent(name)
}

func goastReceiverName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "x"
	}
	return sanitizeIdent(name)
}

// ============================================================
// go/ast Statement/Expression helpers
// ============================================================

func astReturn(x ast.Expr) *ast.ReturnStmt {
	if x == nil {
		return &ast.ReturnStmt{}
	}
	return &ast.ReturnStmt{Results: []ast.Expr{x}}
}

func astReturnMulti(xs []ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{Results: xs}
}

func astBlock(stmts []ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{List: stmts}
}

func astBlockFunc(fn func(*ast.BlockStmt)) *ast.BlockStmt {
	b := &ast.BlockStmt{}
	fn(b)
	return b
}

func astBlockAdd(b *ast.BlockStmt, stmts ...ast.Stmt) {
	b.List = append(b.List, stmts...)
}

func astId(name string) ast.Expr {
	return ast.NewIdent(name)
}

func astCall(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fun, Args: args}
}

func astSel(x ast.Expr, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: x, Sel: ast.NewIdent(name)}
}

func astIndex(x ast.Expr, index ast.Expr) *ast.IndexExpr {
	return &ast.IndexExpr{X: x, Index: index}
}

func astIndexList(x ast.Expr, indices ...ast.Expr) *ast.IndexListExpr {
	return &ast.IndexListExpr{X: x, Indices: indices}
}

func astAssert(x ast.Expr, t ast.Expr) *ast.TypeAssertExpr {
	return &ast.TypeAssertExpr{X: x, Type: t}
}

func astStar(x ast.Expr) *ast.StarExpr {
	return &ast.StarExpr{X: x}
}

func astIntLit(n int) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)}
}

func astNil() *ast.Ident {
	return ast.NewIdent("nil")
}

func astUnderscore() *ast.Ident {
	return ast.NewIdent("_")
}

func astAssign(lhs, rhs []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Rhs: rhs, Tok: token.ASSIGN}
}

func astDefine(lhs, rhs []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Rhs: rhs, Tok: token.DEFINE}
}

func astVarDecl(name string, typ ast.Expr, value ast.Expr) *ast.GenDecl {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(name)}}
	if typ != nil {
		spec.Type = typ
	}
	if value != nil {
		spec.Values = []ast.Expr{value}
	}
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}
}

func astTypeDecl(name string, typeParams *ast.FieldList, typ ast.Expr) *ast.GenDecl {
	spec := &ast.TypeSpec{Name: ast.NewIdent(name), Type: typ}
	if typeParams != nil && len(typeParams.List) > 0 {
		spec.TypeParams = typeParams
	}
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

func astFuncDecl(name string, recv *ast.FieldList, typeParams *ast.FieldList, params, results []*ast.Field, body *ast.BlockStmt) *ast.FuncDecl {
	fd := &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{Params: &ast.FieldList{List: params}},
		Body: body,
	}
	if recv != nil {
		fd.Recv = recv
	}
	if results != nil {
		fd.Type.Results = &ast.FieldList{List: results}
	}
	if typeParams != nil && len(typeParams.List) > 0 {
		fd.Type.TypeParams = typeParams
	}
	return fd
}

func astField(names []string, typ ast.Expr, tag string) *ast.Field {
	f := &ast.Field{Names: make([]*ast.Ident, len(names)), Type: typ}
	for i, n := range names {
		f.Names[i] = ast.NewIdent(n)
	}
	if tag != "" {
		f.Tag = &ast.BasicLit{Kind: token.STRING, Value: "`" + tag + "`"}
	}
	return f
}

func astParam(name string, typ ast.Expr) *ast.Field {
	return &ast.Field{Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(name))}, Type: typ}
}

func astIf(cond ast.Expr, body *ast.BlockStmt, elseStmt ast.Stmt) *ast.IfStmt {
	stmt := &ast.IfStmt{Cond: cond, Body: body}
	if elseStmt != nil {
		stmt.Else = elseStmt
	}
	return stmt
}

func astFor(cond ast.Expr, body *ast.BlockStmt) *ast.ForStmt {
	return &ast.ForStmt{Cond: cond, Body: body}
}

func astExprStmt(x ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{X: x}
}

func astDeclStmt(d ast.Decl) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: d}
}

func astKeyValue(key, value ast.Expr) *ast.KeyValueExpr {
	return &ast.KeyValueExpr{Key: key, Value: value}
}

func astCompositeLit(typ ast.Expr, elts []ast.Expr) *ast.CompositeLit {
	return &ast.CompositeLit{Type: typ, Elts: elts}
}

func astSliceLit(elemType ast.Expr, elts []ast.Expr) *ast.CompositeLit {
	return &ast.CompositeLit{Type: &ast.ArrayType{Elt: elemType}, Elts: elts}
}

func astMapLit(keyType, valType ast.Expr, elts []ast.Expr) *ast.CompositeLit {
	return &ast.CompositeLit{Type: &ast.MapType{Key: keyType, Value: valType}, Elts: elts}
}

func astBinaryOp(x ast.Expr, op token.Token, y ast.Expr) ast.Expr {
	return &ast.BinaryExpr{X: x, Op: op, Y: y}
}

func astUnaryOp(op token.Token, x ast.Expr) ast.Expr {
	return &ast.UnaryExpr{Op: op, X: x}
}

func astLen(x ast.Expr) *ast.CallExpr {
	return astCall(ast.NewIdent("len"), x)
}

func astAppend(slice, elem ast.Expr) *ast.CallExpr {
	return astCall(ast.NewIdent("append"), slice, elem)
}

func astDelete(m, key ast.Expr) *ast.CallExpr {
	return astCall(ast.NewIdent("delete"), m, key)
}

func astPanic(x ast.Expr) *ast.ExprStmt {
	return astExprStmt(astCall(ast.NewIdent("panic"), x))
}

func astAssignId(name string, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(name)}, Rhs: []ast.Expr{rhs}, Tok: token.ASSIGN}
}

func astDefineId(name string, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(name)}, Rhs: []ast.Expr{rhs}, Tok: token.DEFINE}
}

func astVarId(name string, value ast.Expr) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: astVarDecl(name, nil, value)}
}

// tokenFromOp maps MyGO operators to Go token operators.
