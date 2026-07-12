// Package goast provides a builder API for constructing Go AST nodes.
// It replaces the jennifer library with standard go/ast-based code generation.
package goast

import (
	"go/ast"
	"go/token"
	"strconv"
)

// --- Expression Builders ---

// Id creates an identifier expression.
func Id(name string) *ast.Ident {
	return ast.NewIdent(name)
}

// Lit creates a basic literal (string, int, float).
func Lit(value string) *ast.BasicLit {
	kind := token.STRING
	if isNumeric(value) {
		if isInt(value) {
			kind = token.INT
		} else {
			kind = token.FLOAT
		}
	}
	return &ast.BasicLit{Kind: kind, Value: value}
}

// BoolLit creates a boolean literal.
func BoolLit(v bool) *ast.Ident {
	if v {
		return ast.NewIdent("true")
	}
	return ast.NewIdent("false")
}

// Nil creates a nil identifier.
func Nil() *ast.Ident {
	return ast.NewIdent("nil")
}

// Op creates an operator expression (unary or binary).
// For binary ops, use BinaryExpr; for unary, use UnaryExpr.
func Op(op string) *ast.Ident {
	return ast.NewIdent(op)
}

// BinaryExpr creates a binary expression: left op right.
func BinaryExpr(left ast.Expr, op token.Token, right ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{X: left, Op: op, Y: right}
}

// UnaryExpr creates a unary expression: op x.
func UnaryExpr(op token.Token, x ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: op, X: x}
}

// CallExpr creates a function call: fun(args...).
func CallExpr(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fun, Args: args}
}

// SelectorExpr creates a selector expression: x.name.
func SelectorExpr(x ast.Expr, name string) *ast.SelectorExpr {
	return &ast.SelectorExpr{X: x, Sel: ast.NewIdent(name)}
}

// IndexExpr creates an index expression: x[index].
func IndexExpr(x ast.Expr, index ast.Expr) *ast.IndexExpr {
	return &ast.IndexExpr{X: x, Index: index}
}

// IndexListExpr creates an index list expression: x[i, j] (Go 1.18+ generics).
func IndexListExpr(x ast.Expr, indices ...ast.Expr) *ast.IndexListExpr {
	return &ast.IndexListExpr{X: x, Indices: indices}
}

// SliceExpr creates a slice expression: x[low:high] or x[low:].
func SliceExpr(x ast.Expr, low, high ast.Expr) *ast.SliceExpr {
	return &ast.SliceExpr{X: x, Low: low, High: high}
}

// TypeAssertExpr creates a type assertion: x.(type).
func TypeAssertExpr(x ast.Expr, typ ast.Expr) *ast.TypeAssertExpr {
	return &ast.TypeAssertExpr{X: x, Type: typ}
}

// ParenExpr creates a parenthesized expression: (x).
func ParenExpr(x ast.Expr) *ast.ParenExpr {
	return &ast.ParenExpr{X: x}
}

// KeyValueExpr creates a key-value expression for composite literals.
func KeyValueExpr(key ast.Expr, value ast.Expr) *ast.KeyValueExpr {
	return &ast.KeyValueExpr{Key: key, Value: value}
}

// CompositeLitExpr creates a composite literal: Type{elts...}.
func CompositeLitExpr(typ ast.Expr, elts ...ast.Expr) *ast.CompositeLit {
	return &ast.CompositeLit{Type: typ, Elts: elts}
}

// Ellipsis creates an ellipsis expression: ...x.
func Ellipsis(x ast.Expr) *ast.Ellipsis {
	return &ast.Ellipsis{Elt: x}
}

// StarExpr creates a pointer expression: *x.
func StarExpr(x ast.Expr) *ast.StarExpr {
	return &ast.StarExpr{X: x}
}

// ArrayType creates an array/slice type: []elem or [n]elem.
func ArrayType(len ast.Expr, elem ast.Expr) *ast.ArrayType {
	return &ast.ArrayType{Len: len, Elt: elem}
}

// MapType creates a map type: map[key]value.
func MapType(key, value ast.Expr) *ast.MapType {
	return &ast.MapType{Key: key, Value: value}
}

// StructType creates a struct type: struct { fields }.
func StructType(fields []*ast.Field) *ast.StructType {
	return &ast.StructType{Fields: &ast.FieldList{List: fields}}
}

// InterfaceType creates an interface type: interface { methods }.
func InterfaceType(methods []*ast.Field) *ast.InterfaceType {
	return &ast.InterfaceType{Methods: &ast.FieldList{List: methods}}
}

// FuncType creates a function type: func(params) results.
func FuncType(params, results []*ast.Field) *ast.FuncType {
	ft := &ast.FuncType{}
	if len(params) > 0 {
		ft.Params = &ast.FieldList{List: params}
	}
	if len(results) > 0 {
		ft.Results = &ast.FieldList{List: results}
	}
	return ft
}

// GenericType creates a generic instantiation: Name[T1, T2, ...].
// Uses IndexListExpr for Go 1.18+ generics.
func GenericType(name string, typeArgs ...ast.Expr) ast.Expr {
	if len(typeArgs) == 0 {
		return ast.NewIdent(name)
	}
	if len(typeArgs) == 1 {
		return &ast.IndexExpr{X: ast.NewIdent(name), Index: typeArgs[0]}
	}
	return &ast.IndexListExpr{X: ast.NewIdent(name), Indices: typeArgs}
}

// --- Type Expression Builders ---

// PrimitiveType returns a Go primitive type expression.
func PrimitiveType(name string) ast.Expr {
	switch name {
	case "int":
		return ast.NewIdent("int")
	case "int8":
		return ast.NewIdent("int8")
	case "int16":
		return ast.NewIdent("int16")
	case "int32":
		return ast.NewIdent("int32")
	case "int64":
		return ast.NewIdent("int64")
	case "uint":
		return ast.NewIdent("uint")
	case "uint8":
		return ast.NewIdent("uint8")
	case "uint16":
		return ast.NewIdent("uint16")
	case "uint32":
		return ast.NewIdent("uint32")
	case "uint64":
		return ast.NewIdent("uint64")
	case "float32":
		return ast.NewIdent("float32")
	case "float64":
		return ast.NewIdent("float64")
	case "string":
		return ast.NewIdent("string")
	case "bool":
		return ast.NewIdent("bool")
	case "any":
		return ast.NewIdent("any")
	case "byte":
		return ast.NewIdent("byte")
	case "rune":
		return ast.NewIdent("rune")
	default:
		return ast.NewIdent(name)
	}
}

// --- Statement Builders ---

// BlockStmt creates a block statement: { stmts }.
func BlockStmt(stmts ...ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{List: stmts}
}

// ReturnStmt creates a return statement: return exprs.
func ReturnStmt(exprs ...ast.Expr) *ast.ReturnStmt {
	return &ast.ReturnStmt{Results: exprs}
}

// IfStmt creates an if statement: if cond { body } else { elseBody }.
func IfStmt(cond ast.Expr, body *ast.BlockStmt, elseBody *ast.BlockStmt) *ast.IfStmt {
	stmt := &ast.IfStmt{Cond: cond, Body: body}
	if elseBody != nil {
		stmt.Else = elseBody
	}
	return stmt
}

// IfElseStmt creates an if-else statement where the else branch is another
// statement (for chaining: if ... else if ... else ...).
func IfElseStmt(cond ast.Expr, body *ast.BlockStmt, elseStmt ast.Stmt) *ast.IfStmt {
	return &ast.IfStmt{Cond: cond, Body: body, Else: elseStmt}
}

// ForStmt creates a for statement: for cond { body }.
func ForStmt(cond ast.Expr, body *ast.BlockStmt) *ast.ForStmt {
	return &ast.ForStmt{Cond: cond, Body: body}
}

// AssignStmt creates an assignment: lhs = rhs.
func AssignStmt(lhs []ast.Expr, rhs []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Rhs: rhs, Tok: token.ASSIGN}
}

// DefineStmt creates a short variable declaration: lhs := rhs.
func DefineStmt(lhs []ast.Expr, rhs []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Rhs: rhs, Tok: token.DEFINE}
}

// ExprStmt wraps an expression as a statement.
func ExprStmt(expr ast.Expr) *ast.ExprStmt {
	return &ast.ExprStmt{X: expr}
}

// DeclStmt wraps a declaration as a statement.
func DeclStmt(decl ast.Decl) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: decl}
}

// VarDecl creates a var declaration: var name type = value.
// Pass nil for value to omit initialization.
func VarDecl(name string, typ ast.Expr, value ast.Expr) *ast.GenDecl {
	spec := &ast.ValueSpec{
		Names:  []*ast.Ident{ast.NewIdent(name)},
		Type:   typ,
		Values: nil,
	}
	if value != nil {
		spec.Values = []ast.Expr{value}
	}
	return &ast.GenDecl{
		Tok:   token.VAR,
		Specs: []ast.Spec{spec},
	}
}

// VarDeclList creates a var declaration with multiple names: var name1, name2 type = values.
func VarDeclList(names []string, typ ast.Expr, values []ast.Expr) *ast.GenDecl {
	idents := make([]*ast.Ident, len(names))
	for i, n := range names {
		idents[i] = ast.NewIdent(n)
	}
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  idents,
				Type:   typ,
				Values: values,
			},
		},
	}
}

// --- Declaration Builders ---

// FuncDecl creates a function declaration:
// func name(typeParams) (params) results { body }.
func FuncDecl(name string, recv *ast.FieldList, typeParams *ast.FieldList, params, results []*ast.Field, body *ast.BlockStmt) *ast.FuncDecl {
	fd := &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: fieldListIfNonEmpty(results),
		},
		Body: body,
	}
	if recv != nil {
		fd.Recv = recv
	}
	if typeParams != nil && len(typeParams.List) > 0 {
		fd.Type.TypeParams = typeParams
	}
	return fd
}

// MethodDecl creates a method declaration on a receiver type.
func MethodDecl(name string, recvName, recvType string, typeParams *ast.FieldList, params, results []*ast.Field, body *ast.BlockStmt) *ast.FuncDecl {
	recv := &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{ast.NewIdent(recvName)}, Type: ast.NewIdent(recvType)},
		},
	}
	return FuncDecl(name, recv, typeParams, params, results, body)
}

// TypeDecl creates a type declaration: type name typeParams = typ.
func TypeDecl(name string, typeParams *ast.FieldList, typ ast.Expr) *ast.GenDecl {
	spec := &ast.TypeSpec{
		Name: ast.NewIdent(name),
		Type: typ,
	}
	if typeParams != nil && len(typeParams.List) > 0 {
		spec.TypeParams = typeParams
	}
	return &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{spec},
	}
}

// ImportDecl creates an import declaration: import ( specs ).
func ImportDecl(specs []*ast.ImportSpec) *ast.GenDecl {
	return &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: toSpecSlice(specs),
	}
}

// ImportSpec creates a single import spec.
func ImportSpec(path string, alias string) *ast.ImportSpec {
	spec := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)},
	}
	if alias != "" {
		spec.Name = ast.NewIdent(alias)
	}
	return spec
}

// --- Field Builders ---

// Field creates a struct/method field: names type.
// If tag is not empty, it sets the field tag.
func Field(names []string, typ ast.Expr, tag string) *ast.Field {
	f := &ast.Field{
		Names: identSlice(names),
		Type:  typ,
	}
	if tag != "" {
		f.Tag = &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(tag)}
	}
	return f
}

// FieldList creates a list of fields.
func FieldList(fields ...*ast.Field) *ast.FieldList {
	return &ast.FieldList{List: fields}
}

// ParamField creates a parameter field: name type.
func ParamField(name string, typ ast.Expr) *ast.Field {
	return &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type:  typ,
	}
}

// --- File Builder ---

// FileDecl creates a Go source file with the given package name and declarations.
func FileDecl(pkgName string, decls []ast.Decl) *ast.File {
	return &ast.File{
		Name:  ast.NewIdent(pkgName),
		Decls: decls,
	}
}

// --- Utility Functions ---

func fieldListIfNonEmpty(fields []*ast.Field) *ast.FieldList {
	if len(fields) == 0 {
		return nil
	}
	return &ast.FieldList{List: fields}
}

func toSpecSlice(specs []*ast.ImportSpec) []ast.Spec {
	result := make([]ast.Spec, len(specs))
	for i, s := range specs {
		result[i] = s
	}
	return result
}

func identSlice(names []string) []*ast.Ident {
	result := make([]*ast.Ident, len(names))
	for i, n := range names {
		result[i] = ast.NewIdent(n)
	}
	return result
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
		if c == '.' || c == 'e' || c == 'E' {
			return true
		}
	}
	return false
}

func isInt(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return len(s) > 0
}

// Add appends stmts to an existing block and returns it.
func Add(block *ast.BlockStmt, stmts ...ast.Stmt) *ast.BlockStmt {
	block.List = append(block.List, stmts...)
	return block
}

// Println creates a call to fmt.Println with the given args.
func Println(args ...ast.Expr) *ast.ExprStmt {
	return ExprStmt(CallExpr(SelectorExpr(ast.NewIdent("fmt"), "Println"), args...))
}

// Panic creates a call to panic with the given arg.
func Panic(arg ast.Expr) *ast.ExprStmt {
	return ExprStmt(CallExpr(ast.NewIdent("panic"), arg))
}

// Len creates a call to len().
func Len(x ast.Expr) *ast.CallExpr {
	return CallExpr(ast.NewIdent("len"), x)
}

// Append creates a call to append().
func Append(slice, elem ast.Expr) *ast.CallExpr {
	return CallExpr(ast.NewIdent("append"), slice, elem)
}

// Delete creates a call to delete().
func Delete(m, key ast.Expr) *ast.CallExpr {
	return CallExpr(ast.NewIdent("delete"), m, key)
}

// Make creates a call to make().
func Make(typ ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return CallExpr(ast.NewIdent("make"), append([]ast.Expr{typ}, args...)...)
}

// New creates a call to new().
func New(typ ast.Expr) *ast.CallExpr {
	return CallExpr(ast.NewIdent("new"), typ)
}

// StringLit creates a string literal expression.
func StringLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

// IntLit creates an integer literal expression.
func IntLit(n int) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)}
}

// AssignId creates an assignment to an identifier: ident = rhs.
func AssignId(name string, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(name)},
		Rhs: []ast.Expr{rhs},
		Tok: token.ASSIGN,
	}
}

// DefineId creates a short variable declaration: ident := rhs.
func DefineId(name string, rhs ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent(name)},
		Rhs: []ast.Expr{rhs},
		Tok: token.DEFINE,
	}
}

// Underscore creates a blank identifier `_`.
func Underscore() *ast.Ident {
	return ast.NewIdent("_")
}

// EmptyBlock creates an empty block statement.
func EmptyBlock() *ast.BlockStmt {
	return &ast.BlockStmt{}
}

// FuncLit creates a function literal: func(params) results { body }.
func FuncLit(params, results []*ast.Field, body *ast.BlockStmt) *ast.FuncLit {
	return &ast.FuncLit{
		Type: FuncType(params, results),
		Body: body,
	}
}

// GoStmt creates a go statement: go f().
func GoStmt(call *ast.CallExpr) *ast.GoStmt {
	return &ast.GoStmt{Call: call}
}

// IncDecStmt creates an increment/decrement statement: x++ / x--.
func IncDecStmt(x ast.Expr, tok token.Token) *ast.IncDecStmt {
	return &ast.IncDecStmt{X: x, Tok: tok}
}

// BadStmt creates a bad statement placeholder.
func BadStmt() *ast.BadStmt {
	return &ast.BadStmt{}
}

// CommClause creates a communication clause for select statements.
func CommClause(comm ast.Stmt, body []ast.Stmt) *ast.CommClause {
	return &ast.CommClause{Comm: comm, Body: body}
}

// CaseClause creates a case clause for switch statements.
func CaseClause(list []ast.Expr, body []ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{List: list, Body: body}
}
