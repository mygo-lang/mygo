// Package goast exposes go/ast constructors that are convenient to call from
// MyGO's Go FFI.  It deliberately contains no MyGO lowering policy.
package goast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"strconv"
)

// These aliases are the FFI-facing node categories used by MyGO codegen2.
type Expr = ast.Expr
type Stmt = ast.Stmt
type Decl = ast.Decl

// Import is the FFI representation of a Go import declaration.
type Import struct {
	Alias string
	Path  string
}

// StructField is a serializable field description for the MyGO FFI.  It avoids
// exposing go/ast's pointer-heavy FieldList representation to the lowering.
type StructField struct {
	Name string
	Type string
	Tag  string
}

// StructDecl creates a generic or non-generic struct declaration without
// parsing generated declaration text.
func StructDecl(name string, typeParams []string, fields []StructField) ast.Decl {
	astFields := make([]*ast.Field, 0, len(fields))
	for _, field := range fields {
		typ, err := parser.ParseExpr(field.Type)
		if err != nil {
			panic(fmt.Sprintf("invalid generated field type %q: %v", field.Type, err))
		}
		astField := &ast.Field{Names: []*ast.Ident{ast.NewIdent(field.Name)}, Type: typ}
		if field.Tag != "" {
			astField.Tag = &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(field.Tag)}
		}
		astFields = append(astFields, astField)
	}

	spec := &ast.TypeSpec{
		Name: ast.NewIdent(name),
		Type: &ast.StructType{Fields: &ast.FieldList{List: astFields}},
	}
	if len(typeParams) != 0 {
		params := make([]*ast.Field, 0, len(typeParams))
		for _, param := range typeParams {
			params = append(params, &ast.Field{Names: []*ast.Ident{ast.NewIdent(param)}, Type: ast.NewIdent("any")})
		}
		spec.TypeParams = &ast.FieldList{List: params}
	}
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

// StructDeclFromParts is the MyGO-facing form of StructDecl.  MyGO's FFI can
// pass slices of primitive values reliably, whereas Go struct values are not
// yet available as MyGO type annotations.
func StructDeclFromParts(name string, typeParams, fieldNames, fieldTypes, fieldTags []string) ast.Decl {
	if len(fieldNames) != len(fieldTypes) || len(fieldNames) != len(fieldTags) {
		panic("mismatched struct field metadata")
	}
	fields := make([]StructField, len(fieldNames))
	for i := range fields {
		fields[i] = StructField{Name: fieldNames[i], Type: fieldTypes[i], Tag: fieldTags[i]}
	}
	return StructDecl(name, typeParams, fields)
}

// InterfaceDeclFromParts builds an interface declaration from method names and
// function-type suffixes such as "(value int) string".  The suffixes describe
// only types; no Go declaration text is assembled or reparsed.
func InterfaceDeclFromParts(name string, typeParams, methodNames, signatures []string) ast.Decl {
	if len(methodNames) != len(signatures) {
		panic("mismatched interface method metadata")
	}
	methods := make([]*ast.Field, 0, len(methodNames))
	for i, signature := range signatures {
		expr, err := parser.ParseExpr("func" + signature)
		if err != nil {
			panic(fmt.Sprintf("invalid generated method signature %q: %v", signature, err))
		}
		funcType, ok := expr.(*ast.FuncType)
		if !ok {
			panic(fmt.Sprintf("generated method signature %q is not a function type", signature))
		}
		methods = append(methods, &ast.Field{Names: []*ast.Ident{ast.NewIdent(methodNames[i])}, Type: funcType})
	}
	spec := &ast.TypeSpec{
		Name: ast.NewIdent(name),
		Type: &ast.InterfaceType{Methods: &ast.FieldList{List: methods}},
	}
	if len(typeParams) != 0 {
		params := make([]*ast.Field, 0, len(typeParams))
		for _, param := range typeParams {
			params = append(params, &ast.Field{Names: []*ast.Ident{ast.NewIdent(param)}, Type: ast.NewIdent("any")})
		}
		spec.TypeParams = &ast.FieldList{List: params}
	}
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

// AppendDecls is an FFI-friendly slice append for declaration groups.
func AppendDecls(dst, more []ast.Decl) []ast.Decl { return append(dst, more...) }

// EnumDeclsFromParts lowers a MyGO enum to its interface, variant structs,
// marker methods, and constructors as individual Go AST declarations.
func EnumDeclsFromParts(enumName string, typeParams, variantNames []string, variantFields [][]string, constructorNames []string) []ast.Decl {
	if len(variantNames) != len(variantFields) || len(variantNames) != len(constructorNames) {
		panic("mismatched enum variant metadata")
	}
	decls := []ast.Decl{enumInterfaceDecl(enumName, typeParams)}
	for i, variantName := range variantNames {
		decls = append(decls, enumVariantDecls(enumName, typeParams, variantName, variantFields[i], constructorNames[i])...)
	}
	return decls
}

// FuncDeclFromParts constructs a function declaration structurally.  body is
// currently a statement-list migration boundary; it is parsed as a block, not
// as a complete generated declaration.
func FuncDeclFromParts(name string, typeParams, paramNames, paramTypes []string, returnType, body string, loop bool) (ast.Decl, error) {
	if len(paramNames) != len(paramTypes) {
		return nil, fmt.Errorf("mismatched function parameter metadata")
	}
	params := make([]*ast.Field, 0, len(paramNames))
	for i, sourceType := range paramTypes {
		typ, err := parser.ParseExpr(sourceType)
		if err != nil {
			return nil, fmt.Errorf("parse parameter type %q: %w", sourceType, err)
		}
		params = append(params, &ast.Field{Names: []*ast.Ident{ast.NewIdent(paramNames[i])}, Type: typ})
	}
	results := &ast.FieldList{}
	if returnType != "" {
		typ, err := parser.ParseExpr(returnType)
		if err != nil {
			return nil, fmt.Errorf("parse return type %q: %w", returnType, err)
		}
		results.List = []*ast.Field{{Type: typ}}
	}
	block, err := parseStatementBlock(body)
	if err != nil {
		return nil, err
	}
	if loop {
		block = &ast.BlockStmt{List: []ast.Stmt{&ast.ForStmt{Body: block}}}
	}
	return &ast.FuncDecl{
		Name: ast.NewIdent(name),
		Type: &ast.FuncType{TypeParams: typeParamsFieldList(typeParams), Params: &ast.FieldList{List: params}, Results: results},
		Body: block,
	}, nil
}

// MustFuncDeclFromParts is the single-result form required by MyGO's current
// Go FFI. Metadata originates from checked MyGO declarations, so failure is a
// compiler bug rather than a recoverable source-program error.
func MustFuncDeclFromParts(name string, typeParams, paramNames, paramTypes []string, returnType, body string, loop bool) ast.Decl {
	decl, err := FuncDeclFromParts(name, typeParams, paramNames, paramTypes, returnType, body, loop)
	if err != nil {
		panic(err)
	}
	return decl
}

// MustFuncDeclFromStmts constructs a function from already-lowered AST
// statements.  This is the preferred path for codegen2.
func MustFuncDeclFromStmts(name string, typeParams, paramNames, paramTypes []string, returnType string, body []ast.Stmt, loop bool) ast.Decl {
	results := []string{}
	if returnType != "" {
		results = []string{returnType}
	}
	return MustFuncDeclFromStmtsMulti(name, typeParams, paramNames, paramTypes, results, body, loop)
}

func MustFuncDeclFromStmtsMulti(name string, typeParams, paramNames, paramTypes, returnTypes []string, body []ast.Stmt, loop bool) ast.Decl {
	if len(paramNames) != len(paramTypes) {
		panic("mismatched function parameter metadata")
	}
	params := make([]*ast.Field, 0, len(paramNames))
	for i, sourceType := range paramTypes {
		typ, err := parser.ParseExpr(sourceType)
		if err != nil {
			panic(err)
		}
		params = append(params, &ast.Field{Names: []*ast.Ident{ast.NewIdent(paramNames[i])}, Type: typ})
	}
	results := &ast.FieldList{}
	for _, returnType := range returnTypes {
		typ, err := parser.ParseExpr(returnType)
		if err != nil {
			panic(err)
		}
		results.List = append(results.List, &ast.Field{Type: typ})
	}
	block := &ast.BlockStmt{List: body}
	if loop {
		block = &ast.BlockStmt{List: []ast.Stmt{&ast.ForStmt{Body: block}}}
	}
	return &ast.FuncDecl{Name: ast.NewIdent(name), Type: &ast.FuncType{TypeParams: typeParamsFieldList(typeParams), Params: &ast.FieldList{List: params}, Results: results}, Body: block}
}

func Unit() ast.Expr { return &ast.CompositeLit{Type: &ast.StructType{Fields: &ast.FieldList{}}} }

func AppendStmts(dst, more []ast.Stmt) []ast.Stmt { return append(dst, more...) }

// LocalFromParts creates either a short declaration or a typed var
// declaration.  The type string is limited to generated Go type syntax.
func LocalFromParts(name, typ string, value ast.Expr) ast.Stmt {
	if typ == "" {
		return &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(name)}, Tok: token.DEFINE, Rhs: []ast.Expr{value}}
	}
	parsed, err := parser.ParseExpr(typ)
	if err != nil {
		panic(err)
	}
	return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(name)}, Type: parsed, Values: []ast.Expr{value}}}}}
}

func DeclareFromType(name, typ string) ast.Stmt {
	parsed, err := parser.ParseExpr(typ)
	if err != nil {
		panic(err)
	}
	return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(name)}, Type: parsed}}}}
}

func parseStatementBlock(body string) (*ast.BlockStmt, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "body.go", "package p\nfunc _() {\n"+body+"\n}", 0)
	if err != nil {
		return nil, fmt.Errorf("parse function body: %w", err)
	}
	decl, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok {
		return nil, fmt.Errorf("function body parse produced no function")
	}
	return decl.Body, nil
}

// MustInlineGoStatements parses the user-authored Go body of an InlineGoExpr
// into statements. Inline Go is the deliberate raw-Go boundary; parsed nodes
// are inserted directly into the surrounding generated AST.
func MustInlineGoStatements(body string) []ast.Stmt {
	block, err := parseStatementBlock(body)
	if err != nil {
		panic(fmt.Errorf("parse inline Go body: %w", err))
	}
	return block.List
}

func MustExprSource(expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		panic(err)
	}
	return buf.String()
}

// MustInlineGoExprWithOperands parses an inline-Go expression after replacing
// compiler operand placeholders such as {value} and {Type}.
func MustInlineGoExprWithOperands(body string, valueNames, valueSources, typeNames, typeSources []string) ast.Expr {
	if len(valueNames) != len(valueSources) || len(typeNames) != len(typeSources) {
		panic("mismatched inline Go operand metadata")
	}
	for i, name := range valueNames {
		body = replaceInlineOperand(body, name, valueSources[i])
	}
	for i, name := range typeNames {
		body = replaceInlineOperand(body, name, typeSources[i])
	}
	expr, err := parser.ParseExpr(body)
	if err != nil {
		panic(fmt.Errorf("parse inline Go expression: %w", err))
	}
	return expr
}

// MustInlineGoStatementsWithOperands substitutes {name} placeholders before
// parsing the inline body. Replacement is identifier-boundary aware so a
// binding named "x" does not alter "x1".
func MustInlineGoStatementsWithOperands(body string, valueNames, valueSources, typeNames, typeSources []string) []ast.Stmt {
	if len(valueNames) != len(valueSources) || len(typeNames) != len(typeSources) {
		panic("mismatched inline Go operand metadata")
	}
	for i, name := range valueNames {
		body = replaceInlineOperand(body, name, valueSources[i])
	}
	for i, name := range typeNames {
		body = replaceInlineOperand(body, name, typeSources[i])
	}
	return MustInlineGoStatements(body)
}

// MustInlineGoFinalStatementsWithOperands lowers the final inline-Go form of
// a non-unit MyGO function. Expression snippets become a Go return value,
// while statement snippets (notably an explicit "return ...") remain intact.
func MustInlineGoFinalStatementsWithOperands(body string, valueNames, valueSources, typeNames, typeSources []string) []ast.Stmt {
	if len(valueNames) != len(valueSources) || len(typeNames) != len(typeSources) {
		panic("mismatched inline Go operand metadata")
	}
	for i, name := range valueNames {
		body = replaceInlineOperand(body, name, valueSources[i])
	}
	for i, name := range typeNames {
		body = replaceInlineOperand(body, name, typeSources[i])
	}
	if expr, err := parser.ParseExpr(body); err == nil {
		return []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{expr}}}
	}
	return MustInlineGoStatements(body)
}

func replaceInlineOperand(body, name, value string) string {
	re := regexp.MustCompile(`\{` + regexp.QuoteMeta(name) + `\}`)
	return re.ReplaceAllString(body, value)
}

// MustTypeExpr converts generated Go type syntax into an AST type node.  Type
// normalization remains in codegen2; this package owns the Go parser boundary.
func MustTypeExpr(source string) ast.Expr {
	expr, err := parser.ParseExpr(source)
	if err != nil {
		panic(fmt.Errorf("parse generated Go type %q: %w", source, err))
	}
	return expr
}

func enumInterfaceDecl(enumName string, typeParams []string) ast.Decl {
	marker := &ast.Field{Names: []*ast.Ident{ast.NewIdent("is" + enumName)}, Type: &ast.FuncType{Params: &ast.FieldList{}}}
	spec := &ast.TypeSpec{Name: ast.NewIdent(enumName), Type: &ast.InterfaceType{Methods: &ast.FieldList{List: []*ast.Field{marker}}}}
	spec.TypeParams = typeParamsFieldList(typeParams)
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
}

func enumVariantDecls(enumName string, typeParams []string, variantName string, fieldTypes []string, constructorName string) []ast.Decl {
	fields := make([]*ast.Field, 0, len(fieldTypes))
	params := make([]*ast.Field, 0, len(fieldTypes))
	values := make([]ast.Expr, 0, len(fieldTypes))
	for i, sourceType := range fieldTypes {
		typ, err := parser.ParseExpr(sourceType)
		if err != nil {
			panic(fmt.Sprintf("invalid generated enum field type %q: %v", sourceType, err))
		}
		fieldName := fmt.Sprintf("F%d", i)
		paramName := fmt.Sprintf("v%d", i)
		fields = append(fields, &ast.Field{Names: []*ast.Ident{ast.NewIdent(fieldName)}, Type: typ})
		params = append(params, &ast.Field{Names: []*ast.Ident{ast.NewIdent(paramName)}, Type: typ})
		values = append(values, &ast.KeyValueExpr{Key: ast.NewIdent(fieldName), Value: ast.NewIdent(paramName)})
	}
	variantSpec := &ast.TypeSpec{Name: ast.NewIdent(variantName), Type: &ast.StructType{Fields: &ast.FieldList{List: fields}}}
	variantSpec.TypeParams = typeParamsFieldList(typeParams)
	variantDecl := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{variantSpec}}

	receiver := &ast.Field{Type: namedWithTypeParams(variantName, typeParams)}
	markerDecl := &ast.FuncDecl{
		Recv: &ast.FieldList{List: []*ast.Field{receiver}},
		Name: ast.NewIdent("is" + enumName),
		Type: &ast.FuncType{Params: &ast.FieldList{}},
		Body: &ast.BlockStmt{},
	}
	constructor := &ast.FuncDecl{
		Name: ast.NewIdent(constructorName),
		Type: &ast.FuncType{
			TypeParams: typeParamsFieldList(typeParams),
			Params:     &ast.FieldList{List: params},
			Results:    &ast.FieldList{List: []*ast.Field{{Type: namedWithTypeParams(enumName, typeParams)}}},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{&ast.CompositeLit{Type: namedWithTypeParams(variantName, typeParams), Elts: values}}}}},
	}
	return []ast.Decl{variantDecl, markerDecl, constructor}
}

func typeParamsFieldList(typeParams []string) *ast.FieldList {
	if len(typeParams) == 0 {
		return nil
	}
	fields := make([]*ast.Field, 0, len(typeParams))
	for _, param := range typeParams {
		fields = append(fields, &ast.Field{Names: []*ast.Ident{ast.NewIdent(param)}, Type: ast.NewIdent("any")})
	}
	return &ast.FieldList{List: fields}
}

func namedWithTypeParams(name string, typeParams []string) ast.Expr {
	if len(typeParams) == 0 {
		return ast.NewIdent(name)
	}
	indices := make([]ast.Expr, 0, len(typeParams))
	for _, param := range typeParams {
		indices = append(indices, ast.NewIdent(param))
	}
	if len(indices) == 1 {
		return &ast.IndexExpr{X: ast.NewIdent(name), Index: indices[0]}
	}
	return &ast.IndexListExpr{X: ast.NewIdent(name), Indices: indices}
}

// ParseDecl is a temporary compatibility boundary for declarations that have
// not yet been lowered structurally.  Keeping it here prevents codegen2 from
// constructing and reparsing a synthetic Go file itself.
//
// New lowering code should prefer the constructors in this package and return
// ast.Decl values directly.
func ParseDecl(source string) (ast.Decl, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "decl.go", "package p\n"+source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(file.Decls) != 1 {
		return nil, fmt.Errorf("expected one declaration, got %d", len(file.Decls))
	}
	return file.Decls[0], nil
}

// RenderFile formats a complete Go file from structural declarations.
func RenderFile(packageName string, imports []Import, decls []ast.Decl) (string, error) {
	f := &ast.File{Name: ast.NewIdent(packageName)}
	for _, im := range imports {
		spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(im.Path)}}
		if im.Alias != "" {
			spec.Name = ast.NewIdent(im.Alias)
		}
		f.Decls = append(f.Decls, &ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}})
	}
	f.Decls = append(f.Decls, decls...)

	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), f); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderSources is the migration adapter for the legacy string-based
// lowering.  It is intentionally isolated in goast so codegen2 has one
// rendering path while declarations are converted to structural nodes.
func RenderSources(packageName string, imports []Import, sources []string) (string, error) {
	decls := make([]ast.Decl, 0, len(sources))
	for _, source := range sources {
		decl, err := ParseDecl(source)
		if err != nil {
			return "", fmt.Errorf("parse declaration: %w", err)
		}
		decls = append(decls, decl)
	}
	return RenderFile(packageName, imports, decls)
}

// RenderWithLegacy is the FFI bridge used while codegen2 is migrated
// declaration by declaration.  Structural declarations never re-enter the
// parser; only legacy declaration sources are parsed here.
func RenderWithLegacy(packageName string, imports []Import, decls []ast.Decl, sources []string) (string, error) {
	decls = append([]ast.Decl(nil), decls...)
	for _, source := range sources {
		decl, err := ParseDecl(source)
		if err != nil {
			return "", fmt.Errorf("parse declaration: %w", err)
		}
		decls = append(decls, decl)
	}
	return RenderFile(packageName, imports, decls)
}

func Ident(name string) ast.Expr { return ast.NewIdent(name) }

func String(value string) ast.Expr {
	return &ast.BasicLit{Kind: token.STRING, Value: value}
}

func Number(value string) ast.Expr {
	return &ast.BasicLit{Kind: token.INT, Value: value}
}

func Selector(x ast.Expr, sel string) ast.Expr {
	return &ast.SelectorExpr{X: x, Sel: ast.NewIdent(sel)}
}

func Call(fun ast.Expr, args []ast.Expr) ast.Expr {
	return &ast.CallExpr{Fun: fun, Args: args}
}

func Unary(op string, x ast.Expr) ast.Expr {
	return &ast.UnaryExpr{Op: operator(op), X: x}
}

func Binary(left ast.Expr, op string, right ast.Expr) ast.Expr {
	return &ast.BinaryExpr{X: left, Op: operator(op), Y: right}
}

func KeyValue(key, value ast.Expr) ast.Expr {
	return &ast.KeyValueExpr{Key: key, Value: value}
}

func Composite(typ ast.Expr, elts []ast.Expr) ast.Expr {
	return &ast.CompositeLit{Type: typ, Elts: elts}
}

func ExpressionStmt(expr ast.Expr) ast.Stmt { return &ast.ExprStmt{X: expr} }

func Assign(left []ast.Expr, op string, right []ast.Expr) ast.Stmt {
	return &ast.AssignStmt{Lhs: left, Tok: operator(op), Rhs: right}
}

func Define(name string, value ast.Expr) ast.Stmt {
	return Assign([]ast.Expr{Ident(name)}, ":=", []ast.Expr{value})
}

func Return(values []ast.Expr) ast.Stmt { return &ast.ReturnStmt{Results: values} }

func Continue() ast.Stmt { return &ast.BranchStmt{Tok: token.CONTINUE} }

func Block(stmts []ast.Stmt) *ast.BlockStmt { return &ast.BlockStmt{List: stmts} }

func If(cond ast.Expr, body []ast.Stmt, elseBody []ast.Stmt) ast.Stmt {
	n := &ast.IfStmt{Cond: cond, Body: Block(body)}
	if elseBody != nil {
		n.Else = Block(elseBody)
	}
	return n
}

func For(cond ast.Expr, body []ast.Stmt) ast.Stmt {
	return &ast.ForStmt{Cond: cond, Body: Block(body)}
}

func Field(names []string, typ ast.Expr, tag string) *ast.Field {
	field := &ast.Field{Type: typ}
	for _, name := range names {
		field.Names = append(field.Names, ast.NewIdent(name))
	}
	if tag != "" {
		field.Tag = &ast.BasicLit{Kind: token.STRING, Value: tag}
	}
	return field
}

func Struct(fields []*ast.Field) ast.Expr {
	return &ast.StructType{Fields: &ast.FieldList{List: fields}}
}

func Interface(methods []*ast.Field) ast.Expr {
	return &ast.InterfaceType{Methods: &ast.FieldList{List: methods}}
}

func FuncType(params []*ast.Field, results []*ast.Field) *ast.FuncType {
	typ := &ast.FuncType{Params: &ast.FieldList{List: params}}
	if len(results) != 0 {
		typ.Results = &ast.FieldList{List: results}
	}
	return typ
}

func Func(name string, params, results []*ast.Field, body []ast.Stmt) ast.Decl {
	return &ast.FuncDecl{Name: ast.NewIdent(name), Type: FuncType(params, results), Body: Block(body)}
}

func Type(name string, typ ast.Expr) ast.Decl {
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: ast.NewIdent(name), Type: typ}}}
}

func Var(name string, typ, value ast.Expr) ast.Stmt {
	spec := &ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(name)}, Type: typ}
	if value != nil {
		spec.Values = []ast.Expr{value}
	}
	return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}}
}

func operator(op string) token.Token {
	if tok, ok := map[string]token.Token{
		"=": token.ASSIGN, ":=": token.DEFINE, "+": token.ADD, "-": token.SUB,
		"*": token.MUL, "/": token.QUO, "%": token.REM, "!": token.NOT,
		"==": token.EQL, "!=": token.NEQ, "<": token.LSS, "<=": token.LEQ,
		">": token.GTR, ">=": token.GEQ, "&&": token.LAND, "||": token.LOR,
	}[op]; ok {
		return tok
	}
	return token.ILLEGAL
}
