%{
package parser

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func tokLine(v any) int {
	if t, ok := v.(token); ok {
		return t.line
	}
	return 0
}

func tokCol(v any) int {
	if t, ok := v.(token); ok {
		return t.col
	}
	return 0
}

func tokLit(v any) string {
	if t, ok := v.(token); ok {
		return t.lit
	}
	return ""
}

func makeIdentExpr(t token) *ast.IdentExpr {
	return &ast.IdentExpr{Line: t.line, Column: t.col, Name: t.lit}
}
func makeLitExpr(t token) *ast.LiteralExpr {
	switch t.kind {
	case tokNumber:
		return &ast.LiteralExpr{Line: t.line, Column: t.col, Kind: "number", Value: t.lit}
	case tokString:
		return &ast.LiteralExpr{Line: t.line, Column: t.col, Kind: "string", Value: t.lit}
	default:
		return nil
	}
}

func bodyExprFromBlock(e ast.Expr) ast.Expr {
	block, ok := e.(*ast.BlockExpr)
	if !ok {
		return e
	}
	if len(block.Stmts) == 1 {
		if stmt, ok := block.Stmts[0].(*ast.ExprStmt); ok {
			return stmt.Expr
		}
	}
	return &ast.BlockExpr{Line: block.Line, Column: block.Column, Stmts: append([]ast.Stmt(nil), block.Stmts...)}
}
%}

%union {
	token token
	node  any
}

%token <token> IDENT NUMBER STRING
%token <token> PACKAGE IMPORT ENUM STRUCT INTERFACE IMPL FUNC IF THEN ELSE SWITCH CASE END USING NOT LET VAR EMBED WHILE RETURN GO IN TYPE
%token <token> NEWLINE
%token <token> ARROW EQEQ NEQ LTE GTE PIPEFWD PIPEBACK ANDAND OROR
%token <token> COLON COMMA DOT LPAREN RPAREN LBRACK RBRACK LBRACE RBRACE UNDER SLICE
%token <token> TYPELBRACK CONSTRLBRACK
%type <token> call_start
%left PIPEFWD PIPEBACK
%left OROR
%left ANDAND
%left EQEQ NEQ LTE GTE '<' '>'
%left '+' '-'
%left '*' '/'
%right NOT
%nonassoc POSTFIX

%%

file
	: program {
		p := yylex.(*parser)
		p.result = &ast.File{
			PackageName:   p.packageName,
			PackageLine:   p.packageLine,
			PackageColumn: p.packageColumn,
			Decls:         append([]Decl(nil), p.decls...),
		}
	}
	;

program
	: opt_package opt_newlines decl_list opt_newlines
	;

opt_package
	: /* empty */
	| PACKAGE IDENT {
		p := yylex.(*parser)
		p.packageName = $2.lit
		p.packageLine = $1.line
		p.packageColumn = $1.col
	}
	;

opt_newlines
	: /* empty */
	| opt_newlines NEWLINE
	;

decl_list
	: /* empty */
	| decl_list opt_newlines decl
	;

decl
	: import_decl
	| enum_decl
	| struct_decl
	| interface_decl
	| impl_decl
	| func_decl
	| let_decl
	| var_decl
	;

import_decl
	: IMPORT IDENT STRING {
		p := yylex.(*parser)
		p.decls = append(p.decls, &ast.ImportDecl{
			Line:  $3.line,
			Column: $3.col,
			Alias: $2.lit,
			Path:  $3.lit,
		})
	}
	| IMPORT STRING {
		p := yylex.(*parser)
		p.decls = append(p.decls, &ast.ImportDecl{
			Line:  $2.line,
			Column: $2.col,
			Path:  $2.lit,
		})
	}
	;

enum_decl
	: ENUM qualified_name {
		p := yylex.(*parser)
		p.savedDeclName = p.currentName
	}
	opt_type_params {
		p := yylex.(*parser)
		p.currentEnum = &ast.EnumDecl{
			Line: $1.line,
			Column: $1.col,
			Name: p.savedDeclName,
			TypeParams: append([]string(nil), p.currentTypeParams...),
		}
	}
	enum_body opt_newlines END {
		p := yylex.(*parser)
		if p.currentEnum != nil {
			p.decls = append(p.decls, p.currentEnum)
		}
		p.currentEnum = nil
		p.currentTypeParams = nil
	}
	;

enum_body
	: /* empty */
	| enum_body opt_newlines enum_variant
	;

enum_variant
	: IDENT {
		p := yylex.(*parser)
		if p.currentEnum != nil {
			p.currentEnum.Variants = append(p.currentEnum.Variants, ast.EnumVariant{
				Line: $1.line,
				Column: $1.col,
				Name: $1.lit,
			})
		}
	}
	| IDENT LPAREN enum_variant_fields RPAREN {
		p := yylex.(*parser)
		if p.currentEnum != nil {
			p.currentEnum.Variants = append(p.currentEnum.Variants, ast.EnumVariant{
				Line: $1.line,
				Column: $1.col,
				Name: $1.lit,
				Fields: append([]ast.Field(nil), p.currentEnumFields...),
			})
		}
		p.currentEnumFields = nil
	}
	;

enum_variant_fields
	: /* empty */
	| enum_variant_fields IDENT {
		p := yylex.(*parser)
		if p.currentEnum != nil {
			p.currentEnumFields = append(p.currentEnumFields, ast.Field{
				Line: $2.line,
				Column: $2.col,
				Type: &ast.NamedType{Line: $2.line, Column: $2.col, Name: $2.lit},
			})
		}
	}
	| enum_variant_fields type {
		p := yylex.(*parser)
		if p.currentEnum != nil {
			p.currentEnumFields = append(p.currentEnumFields, ast.Field{
				Line: p.currentTypeLine,
				Column: p.currentTypeCol,
				Type: p.currentType,
			})
		}
	}
	;

struct_decl
	: STRUCT qualified_name {
		p := yylex.(*parser)
		p.savedDeclName = p.currentName
	}
	opt_type_params {
		p := yylex.(*parser)
		p.currentStruct = &ast.StructDecl{
			Line: $1.line,
			Column: $1.col,
			Name: p.savedDeclName,
			TypeParams: append([]string(nil), p.currentTypeParams...),
		}
	}
	opt_newlines struct_body opt_newlines END {
		p := yylex.(*parser)
		if p.currentStruct != nil {
			p.decls = append(p.decls, p.currentStruct)
		}
		p.currentStruct = nil
		p.currentTypeParams = nil
	}
	;

struct_body
	: LPAREN {
		p := yylex.(*parser)
		p.expectStructTypeArgs = true
		p.currentStructTypeArgs = nil
	}
	maybe_type_list RPAREN {
		p := yylex.(*parser)
		if p.currentStruct != nil {
			for i, t := range p.currentStructTypeArgs {
				p.currentStruct.Fields = append(p.currentStruct.Fields, ast.Field{
					Line: $1.line,
					Column: $1.col,
					Name: __yyfmt__.Sprintf("F%d", i),
					Type: t,
				})
			}
		}
		p.currentStructTypeArgs = nil
		p.expectStructTypeArgs = false
	}
	| struct_fields
	;

struct_fields
	: /* empty */
	| struct_fields field opt_newlines
	;

field
	: IDENT COLON type {
		p := yylex.(*parser)
		if p.currentStruct != nil {
			p.currentStruct.Fields = append(p.currentStruct.Fields, ast.Field{
				Line: $1.line,
				Column: $1.col,
				Name: $1.lit,
				Type: p.currentType,
			})
		}
	}
	| EMBED type {
		p := yylex.(*parser)
		if p.currentStruct != nil {
			p.currentStruct.Fields = append(p.currentStruct.Fields, ast.Field{
				Line: $1.line,
				Column: $1.col,
				Name: $1.lit,
				Type: p.currentType,
			})
		}
	}
	;

interface_decl
	: INTERFACE qualified_name {
		p := yylex.(*parser)
		p.savedDeclName = p.currentName
	}
	opt_type_params {
		p := yylex.(*parser)
		p.currentInterface = &ast.InterfaceDecl{
			Line: $1.line,
			Column: $1.col,
			Name: p.savedDeclName,
			TypeParams: append([]string(nil), p.currentTypeParams...),
		}
		p.currentTypeParams = nil
	}
	func_sig_list opt_newlines END {
		p := yylex.(*parser)
		if p.currentInterface != nil {
			p.decls = append(p.decls, p.currentInterface)
		}
		p.currentInterface = nil
		p.currentTypeParams = nil
	}
	;

func_sig_list
	: /* empty */
	| func_sig_list opt_newlines func_sig
	;

func_sig
	: FUNC IDENT {
		p := yylex.(*parser)
		p.expectTypeSuffix = true
		p.currentTypeParams = nil
	}
	opt_type_params LPAREN maybe_param_list RPAREN ARROW type opt_using_clause {
		p := yylex.(*parser)
		if p.currentInterface != nil {
			p.currentInterface.Methods = append(p.currentInterface.Methods, &ast.FuncDecl{
				Line: $1.line,
				Column: $1.col,
				Name: $2.lit,
				TypeParams: append([]string(nil), p.currentTypeParams...),
				Params: append([]ast.Param(nil), p.currentParams...),
				Ret: p.currentType,
				Using: append([]ast.Constraint(nil), p.currentWhere...),
			})
		}
		p.currentParams = nil
		p.currentTypeParams = nil
		p.currentWhere = nil
		p.expectTypeSuffix = false
	}
	;

impl_decl
	: IMPL {
		p := yylex.(*parser)
		p.currentImplLine = $1.line
		p.currentImplCol = $1.col
	}
	opt_impl_type_params type {
		p := yylex.(*parser)
		p.currentImplType = p.currentType
	}
	COLON type {
		p := yylex.(*parser)
		// Named/generic form: "impl Type : Interface[Args]"
		// p.currentType holds the interface reference (e.g. "Show[Int]")
		if iface, ok := p.currentType.(*ast.NamedType); ok {
			p.currentName = iface.Name
			p.currentImplInterfaceArgs = append([]ast.TypeExpr(nil), iface.Args...)
		} else {
			p.currentName = ""
			p.currentImplInterfaceArgs = nil
		}
		p.currentImpl = &ast.ImplDecl{
			Line: p.currentImplLine,
			Column: p.currentImplCol,
			Name: p.currentName,
			InterfaceName: p.currentName,
			Type: p.currentImplType,
			TypeParams: append([]string(nil), p.currentImplTypeParams...),
		}
	}
	impl_body opt_newlines END {
		p := yylex.(*parser)
		if p.currentImpl != nil {
			p.currentImpl.InterfaceArgs = append([]ast.TypeExpr(nil), p.currentImplInterfaceArgs...)
			p.decls = append(p.decls, p.currentImpl)
		}
		p.currentImpl = nil
		p.currentImplTypeParams = nil
		p.parsingImplTypeParams = false
		p.currentImplType = nil
		p.currentImplInterfaceArgs = nil
	}
	;

opt_impl_type_params
	: /* empty */
	| LBRACK {
		p := yylex.(*parser)
		p.parsingImplTypeParams = true
		p.currentImplTypeParams = nil
	}
	maybe_name_list RBRACK {
		p := yylex.(*parser)
		p.parsingImplTypeParams = false
	}
	| TYPELBRACK {
		p := yylex.(*parser)
		p.parsingImplTypeParams = true
		p.currentImplTypeParams = nil
	}
	maybe_name_list RBRACK {
		p := yylex.(*parser)
		p.parsingImplTypeParams = false
	}
	;

impl_body
	: /* empty */
	| impl_body opt_newlines func_decl
	;

func_decl
	: FUNC IDENT {
		p := yylex.(*parser)
		p.expectTypeSuffix = true
		p.currentTypeParams = nil
	}
	opt_type_params LPAREN maybe_param_list RPAREN ARROW type opt_using_clause {
		p := yylex.(*parser)
		p.savedRetType = p.currentType
	}
	opt_newlines block_expr opt_newlines END {
		p := yylex.(*parser)
		body := bodyExprFromBlock(p.currentExpr)
		p.currentFunc = &ast.FuncDecl{
			Line: $1.line,
			Column: $1.col,
			Name: $2.lit,
			TypeParams: append([]string(nil), p.currentTypeParams...),
			Params: append([]ast.Param(nil), p.currentParams...),
			Ret: p.savedRetType,
			Using: append([]ast.Constraint(nil), p.currentWhere...),
			Body: body,
		}
		if p.currentImpl != nil {
			p.currentImpl.Methods = append(p.currentImpl.Methods, p.currentFunc)
		} else {
			p.decls = append(p.decls, p.currentFunc)
		}
		p.currentFunc = nil
		p.currentParams = nil
		p.currentTypeParams = nil
		p.currentWhere = nil
		p.currentBlock = nil
		p.savedRetType = nil
		p.expectTypeSuffix = false
	}
	;

let_decl
	: LET IDENT opt_type_annot '=' expr {
		p := yylex.(*parser)
		p.decls = append(p.decls, &ast.LetStmt{
			Line: $1.line,
			Column: $1.col,
			Name: $2.lit,
			Type: p.currentType,
			Value: p.currentExpr,
		})
	}
	;

var_decl
	: VAR IDENT opt_type_annot '=' expr {
		p := yylex.(*parser)
		p.decls = append(p.decls, &ast.LetStmt{
			Line: $1.line,
			Column: $1.col,
			Mutable: true,
			Name: $2.lit,
			Type: p.currentType,
			Value: p.currentExpr,
		})
	}
	;

opt_type_annot
	: /* empty */
	| COLON type
	;

opt_using_clause
	: /* empty */
	| USING constraint_list
	;

constraint_list
	: constraint
	| constraint_list COMMA constraint
	;

constraint
	: IDENT {
		p := yylex.(*parser)
		p.expectConstraintSuffix = true
		p.currentConstraintArgs = nil
		p.currentConstraintBindName = ""
	}
	constr_suffix {
		p := yylex.(*parser)
		name := $1.lit
		if p.currentConstraintBindName != "" {
			name = p.currentConstraintBindName
		}
		p.currentWhere = append(p.currentWhere, ast.Constraint{
			Line: $1.line,
			Column: $1.col,
			Name: name,
			Args: append([]ast.TypeExpr(nil), p.currentConstraintArgs...),
			BindName: p.currentConstraintBindName,
		})
		p.expectConstraintSuffix = false
		p.currentConstraintBindName = ""
		p.currentConstraintArgs = nil
	}
	;

constr_suffix
	: COLON IDENT constraint_suffix {
		p := yylex.(*parser)
		p.currentConstraintBindName = $2.lit
		p.currentConstraintArgs = nil
	}
	| constraint_suffix
	;

constraint_suffix
	: /* empty */
	| CONSTRLBRACK maybe_type_list RBRACK
	;

opt_type_params
	: /* empty */
	| LBRACK {
		p := yylex.(*parser)
		p.currentTypeParams = nil
	}
	| TYPELBRACK {
		p := yylex.(*parser)
		p.currentTypeParams = nil
	}
	maybe_name_list RBRACK
	;

maybe_name_list
	: /* empty */
	| name_list
	;

name_list
	: type {
		p := yylex.(*parser)
		name := ""
		if nt, ok := p.currentType.(*ast.NamedType); ok {
			name = nt.Name
		} else {
			p.err = common.ErrorAtPos(p.currentTypeLine, p.currentTypeCol, "expected type parameter name")
		}
		if p.parsingImplTypeParams {
			p.currentImplTypeParams = append(p.currentImplTypeParams, name)
		} else {
			p.currentTypeParams = append(p.currentTypeParams, name)
		}
	}
	| name_list COMMA type {
		p := yylex.(*parser)
		name := ""
		if nt, ok := p.currentType.(*ast.NamedType); ok {
			name = nt.Name
		} else {
			p.err = common.ErrorAtPos(p.currentTypeLine, p.currentTypeCol, "expected type parameter name")
		}
		if p.parsingImplTypeParams {
			p.currentImplTypeParams = append(p.currentImplTypeParams, name)
		} else {
			p.currentTypeParams = append(p.currentTypeParams, name)
		}
	}
	;

maybe_param_list
	: /* empty */
	| param_list
	;

param_list
	: param
	| param_list COMMA param
	;

param
	: IDENT COLON type {
		p := yylex.(*parser)
		p.currentParams = append(p.currentParams, ast.Param{
			Line: $1.line,
			Column: $1.col,
			Name: $1.lit,
			Type: p.currentType,
		})
	}
	;

maybe_type_list
	: /* empty */
	| type_list
	;

type_list
	: type {
		p := yylex.(*parser)
		p.currentStructTypeArgs = append(p.currentStructTypeArgs, p.currentType)
		if p.expectConstraintSuffix && p.funcTypeParamDepth == 0 {
			p.currentConstraintArgs = append(p.currentConstraintArgs, p.currentType)
		}
	}
	| type_list COMMA type {
		p := yylex.(*parser)
		p.currentStructTypeArgs = append(p.currentStructTypeArgs, p.currentType)
		if p.expectConstraintSuffix && p.funcTypeParamDepth == 0 {
			p.currentConstraintArgs = append(p.currentConstraintArgs, p.currentType)
		}
	}
	;

qualified_name
	: IDENT {
		p := yylex.(*parser)
		p.currentName = $1.lit
		p.currentNameLine = $1.line
		p.currentNameCol = $1.col
		p.expectTypeSuffix = true
	}
	| qualified_name DOT IDENT {
		p := yylex.(*parser)
		p.currentName += "." + $3.lit
		p.expectTypeSuffix = true
	}
	;

type
	: func_type {
		p := yylex.(*parser)
		yyVAL.node = p.currentType
	}
	| LPAREN type {
		p := yylex.(*parser)
		p.currentTupleTypeElems = append(p.currentTupleTypeElems, p.currentType)
	}
	tuple_type_tail
	| grouped_type {
		p := yylex.(*parser)
		yyVAL.node = p.currentType
	}
	| named_type {
		p := yylex.(*parser)
		yyVAL.node = p.currentType
	}
	;

func_type
	: FUNC func_type_params_start maybe_type_list RPAREN {
		p := yylex.(*parser)
		params := append([]ast.TypeExpr(nil), p.currentStructTypeArgs...)
		if len(p.currentTypeArgStack) > 0 {
			idx := len(p.currentTypeArgStack) - 1
			p.currentStructTypeArgs = p.currentTypeArgStack[idx]
			p.currentTypeArgStack = p.currentTypeArgStack[:idx]
		} else {
			p.currentStructTypeArgs = nil
		}
		if p.funcTypeParamDepth > 0 {
			p.funcTypeParamDepth--
		}
		p.currentFuncTypeParamStack = append(p.currentFuncTypeParamStack, params)
	}
	ARROW type {
		p := yylex.(*parser)
		ret := p.currentType
		var params []ast.TypeExpr
		if len(p.currentFuncTypeParamStack) > 0 {
			idx := len(p.currentFuncTypeParamStack) - 1
			params = p.currentFuncTypeParamStack[idx]
			p.currentFuncTypeParamStack = p.currentFuncTypeParamStack[:idx]
		}
		p.currentTypeLine = $1.line
		p.currentTypeCol = $1.col
		p.currentType = &ast.FuncType{
			Line:   p.currentTypeLine,
			Column: p.currentTypeCol,
			Params: params,
			Ret:    ret,
		}
	}
	;

tuple_type_tail
	: RPAREN {
		p := yylex.(*parser)
		p.currentType = &ast.TupleType{Line: $1.line, Column: $1.col, Elems: append([]ast.TypeExpr(nil), p.currentTupleTypeElems...)}
		p.currentTupleTypeElems = nil
		yyVAL.node = p.currentType
	}
	| COMMA type {
		p := yylex.(*parser)
		p.currentTupleTypeElems = append(p.currentTupleTypeElems, p.currentType)
	}
	tuple_type_items RPAREN {
		p := yylex.(*parser)
		elems := append([]ast.TypeExpr(nil), p.currentTupleTypeElems...)
		p.currentType = &ast.TupleType{Line: $1.line, Column: $1.col, Elems: elems}
		p.currentTupleTypeElems = nil
		yyVAL.node = p.currentType
	}
	;

tuple_type_items
	: /* empty */
	| COMMA type {
		p := yylex.(*parser)
		p.currentTupleTypeElems = append(p.currentTupleTypeElems, p.currentType)
	}
	tuple_type_items
	;

func_type_params_start
	: LPAREN {
		p := yylex.(*parser)
		p.currentTypeArgStack = append(p.currentTypeArgStack, p.currentStructTypeArgs)
		p.currentStructTypeArgs = nil
		p.funcTypeParamDepth++
	}
	;

grouped_type
	: LPAREN type RPAREN
	;

named_type
	: qualified_name named_type_suffix
	;

named_type_suffix
	: /* empty */ {
		p := yylex.(*parser)
		p.currentType = &ast.NamedType{
			Line: p.currentNameLine,
			Column: p.currentNameCol,
			Name: p.currentName,
		}
	}
	| TYPELBRACK {
		p := yylex.(*parser)
		p.expectStructTypeArgs = true
		// Push current name onto the stack before parsing inner types.
		p.savedTypeNameStack = append(p.savedTypeNameStack, typeNameEntry{
			name: p.currentName,
			line: p.currentNameLine,
			col:  p.currentNameCol,
			args: p.currentStructTypeArgs,
		})
		p.currentStructTypeArgs = nil
	}
	maybe_type_list RBRACK {
		p := yylex.(*parser)
		// Pop the saved name for this level.
		top := p.savedTypeNameStack[len(p.savedTypeNameStack)-1]
		p.savedTypeNameStack = p.savedTypeNameStack[:len(p.savedTypeNameStack)-1]
		args := append([]ast.TypeExpr(nil), p.currentStructTypeArgs...)
		p.currentType = &ast.NamedType{
			Line: top.line,
			Column: top.col,
			Name: top.name,
			Args: args,
		}
		// Restore outer struct type args context.
		p.currentStructTypeArgs = top.args
		p.expectStructTypeArgs = false
	}
	;

expr
	: pipe_expr
	;

pipe_expr
	: or_expr {
		p := yylex.(*parser)
		p.currentExpr = p.currentExpr
		p.currentLeftExpr = p.currentExpr
	}
	| pipe_expr PIPEFWD {
		p := yylex.(*parser)
		p.currentPipeLeftExpr = p.currentLeftExpr
	} or_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = &ast.BinaryExpr{Op: "|>", Left: p.currentPipeLeftExpr, Right: p.currentExpr}
		p.currentExpr = p.currentLeftExpr
	}
	| pipe_expr PIPEBACK {
		p := yylex.(*parser)
		p.currentPipeLeftExpr = p.currentLeftExpr
	} or_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = &ast.BinaryExpr{Op: "<|", Left: p.currentPipeLeftExpr, Right: p.currentExpr}
		p.currentExpr = p.currentLeftExpr
	}
	;

or_expr
	: and_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = p.currentExpr
	}
	| or_expr OROR {
		p := yylex.(*parser)
		p.currentOrSave = p.currentExpr
	} and_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "||", Left: p.currentOrSave, Right: p.currentExpr}
	}
	;

and_expr
	: compare_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = p.currentExpr
	}
	| and_expr ANDAND {
		p := yylex.(*parser)
		p.currentAndSave = p.currentExpr
	} compare_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "&&", Left: p.currentAndSave, Right: p.currentExpr}
	}
	;

compare_expr
	: add_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = p.currentExpr
	}
	| compare_expr EQEQ {
		p := yylex.(*parser)
		p.currentCompSave = p.currentExpr
	} add_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "==", Left: p.currentCompSave, Right: p.currentExpr}
	}
	| compare_expr NEQ {
		p := yylex.(*parser)
		p.currentCompSave = p.currentExpr
	} add_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "!=", Left: p.currentCompSave, Right: p.currentExpr}
	}
	| compare_expr LTE {
		p := yylex.(*parser)
		p.currentCompSave = p.currentExpr
	} add_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "<=", Left: p.currentCompSave, Right: p.currentExpr}
	}
	| compare_expr GTE {
		p := yylex.(*parser)
		p.currentCompSave = p.currentExpr
	} add_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: ">=", Left: p.currentCompSave, Right: p.currentExpr}
	}
	| compare_expr '<' {
		p := yylex.(*parser)
		p.currentCompSave = p.currentExpr
	} add_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "<", Left: p.currentCompSave, Right: p.currentExpr}
	}
	| compare_expr '>' {
		p := yylex.(*parser)
		p.currentCompSave = p.currentExpr
	} add_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: ">", Left: p.currentCompSave, Right: p.currentExpr}
	}
	;

add_expr
	: mul_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = p.currentExpr
	}
	| add_expr '+' {
		p := yylex.(*parser)
		p.currentAddSave = p.currentExpr
	} mul_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "+", Left: p.currentAddSave, Right: p.currentExpr}
	}
	| add_expr '-' {
		p := yylex.(*parser)
		p.currentAddSave = p.currentExpr
	} mul_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "-", Left: p.currentAddSave, Right: p.currentExpr}
	}
	;

mul_expr
	: prefix_expr {
		p := yylex.(*parser)
		p.currentLeftExpr = p.currentExpr
	}
	| mul_expr '*' {
		p := yylex.(*parser)
		p.currentMulSave = p.currentExpr
	} prefix_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "*", Left: p.currentMulSave, Right: p.currentExpr}
	}
	| mul_expr '/' {
		p := yylex.(*parser)
		p.currentMulSave = p.currentExpr
	} prefix_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.BinaryExpr{Op: "/", Left: p.currentMulSave, Right: p.currentExpr}
	}
	;

prefix_expr
	: postfix_expr %prec POSTFIX
	| NOT postfix_expr {
		p := yylex.(*parser)
		p.currentExpr = &ast.PrefixExpr{Line: $1.line, Column: $1.col, Op: "!", Expr: p.currentExpr}
	}
	| '-' postfix_expr %prec NOT {
		p := yylex.(*parser)
		p.currentExpr = &ast.PrefixExpr{Op: "-", Expr: p.currentExpr}
	}
	;

postfix_expr
	: primary
	| postfix_expr call_start maybe_expr_list RPAREN {
		p := yylex.(*parser)
		if len(p.currentCallCalleeStack) == 0 {
			p.currentExpr = &ast.CallExpr{Line: $2.line, Column: $2.col, Callee: p.currentExpr, Args: append([]ast.Expr(nil), p.currentArgs...)}
			p.currentArgs = nil
		} else {
			idx := len(p.currentCallCalleeStack) - 1
			callee := p.currentCallCalleeStack[idx]
			prevArgs := p.currentArgsStack[idx]
			prevSliceElems := p.currentSliceElemsStack[idx]
			args := append([]ast.Expr(nil), p.currentArgs...)
			p.currentCallCalleeStack = p.currentCallCalleeStack[:idx]
			p.currentArgsStack = p.currentArgsStack[:idx]
			p.currentSliceElemsStack = p.currentSliceElemsStack[:idx]
			p.currentArgs = prevArgs
			p.currentSliceElems = prevSliceElems
			p.currentExpr = &ast.CallExpr{Line: $2.line, Column: $2.col, Callee: callee, Args: args}
		}
	}
	| postfix_expr DOT IDENT {
		p := yylex.(*parser)
		p.currentExpr = &ast.FieldExpr{Line: $2.line, Column: $2.col, Expr: p.currentExpr, Field: $3.lit}
	}
	| postfix_expr TYPELBRACK maybe_type_list RBRACK LBRACE maybe_struct_fields RBRACE {
		p := yylex.(*parser)
		if id, ok := p.currentExpr.(*ast.IdentExpr); ok {
			p.currentExpr = &ast.StructLitExpr{Line: $2.line, Column: $2.col, TypeName: id.Name, TypeArgs: append([]ast.TypeExpr(nil), p.currentStructTypeArgs...), Fields: append([]ast.StructLitField(nil), p.currentStructFields...)}
		}
		p.currentStructTypeArgs = nil
		p.currentStructFields = nil
		p.expectStructTypeArgs = false
	}
	| postfix_expr LBRACK {
		p := yylex.(*parser)
		p.expectStructTypeArgs = true
		p.currentStructTypeArgs = nil
	}
	maybe_type_list RBRACK LBRACE maybe_struct_fields RBRACE {
		p := yylex.(*parser)
		if id, ok := p.currentExpr.(*ast.IdentExpr); ok {
			p.currentExpr = &ast.StructLitExpr{Line: $2.line, Column: $2.col, TypeName: id.Name, TypeArgs: append([]ast.TypeExpr(nil), p.currentStructTypeArgs...), Fields: append([]ast.StructLitField(nil), p.currentStructFields...)}
		}
		p.currentStructTypeArgs = nil
		p.currentStructFields = nil
		p.expectStructTypeArgs = false
	}
	| postfix_expr LBRACE maybe_struct_fields RBRACE {
		p := yylex.(*parser)
		if id, ok := p.currentExpr.(*ast.IdentExpr); ok {
			p.currentExpr = &ast.StructLitExpr{Line: $2.line, Column: $2.col, TypeName: id.Name, Fields: append([]ast.StructLitField(nil), p.currentStructFields...)}
		}
		p.currentStructFields = nil
		p.expectStructTypeArgs = false
	}
	;

primary
	: IDENT {
		p := yylex.(*parser)
		p.currentExpr = makeIdentExpr($1)
	}
	| NUMBER {
		p := yylex.(*parser)
		p.currentExpr = makeLitExpr($1)
	}
	| STRING {
		p := yylex.(*parser)
		p.currentExpr = makeLitExpr($1)
	}
	| LPAREN RPAREN {
		p := yylex.(*parser)
		p.currentExpr = &ast.UnitLitExpr{Line: $1.line, Column: $1.col}
	}
	| LPAREN expr {
		p := yylex.(*parser)
		p.currentTupleElems = append(p.currentTupleElems, p.currentExpr)
	}
	paren_expr_tail
	| slice_lit
	| collection_lit
	| if_expr
	| switch_expr
	| while_expr
	| func_lit
	| go_expr
	;

paren_expr_tail
	: RPAREN
	| COMMA expr {
		p := yylex.(*parser)
		p.currentTupleElems = append(p.currentTupleElems, p.currentExpr)
	}
	tuple_expr_elems RPAREN {
		p := yylex.(*parser)
		elems := append([]ast.Expr(nil), p.currentTupleElems...)
		p.currentExpr = &ast.TupleLitExpr{Line: $1.line, Column: $1.col, Elems: elems}
		p.currentTupleElems = nil
	}
	;

tuple_expr_elems
	: /* empty */
	| COMMA expr {
		p := yylex.(*parser)
		p.currentTupleElems = append(p.currentTupleElems, p.currentExpr)
	}
	tuple_expr_elems
	;

go_expr
	: GO LBRACK type RBRACK {
		p := yylex.(*parser)
		p.currentGoResult = p.currentType
		p.currentGoCode = ""
		p.currentGoOperands = nil
		p.currentGoTypeOperands = nil
	}
	LBRACE opt_newlines go_code go_field_list opt_newlines RBRACE {
		p := yylex.(*parser)
		p.currentExpr = &ast.GoExpr{
			Line: $1.line,
			Column: $1.col,
			Result: p.currentGoResult,
			Code: p.currentGoCode,
			Operands: append([]ast.GoOperand(nil), p.currentGoOperands...),
			TypeOperands: append([]ast.GoTypeOperand(nil), p.currentGoTypeOperands...),
		}
		p.currentGoResult = nil
		p.currentGoCode = ""
		p.currentGoOperands = nil
		p.currentGoTypeOperands = nil
	}
	;

go_code
	: IDENT COLON STRING opt_newlines {
		p := yylex.(*parser)
		if $1.lit != "code" {
			p.err = common.ErrorAtPos($1.line, $1.col, "expected go block field \"code\", got %q", $1.lit)
		}
		p.currentGoCode = $3.lit
	}
	;

go_field_list
	: /* empty */
	| go_field_list go_operand opt_newlines
	| go_field_list go_type_operand opt_newlines
	;

go_operand
	: IN IDENT '=' expr {
		p := yylex.(*parser)
		p.currentGoOperands = append(p.currentGoOperands, ast.GoOperand{
			Line: $2.line,
			Column: $2.col,
			Name: $2.lit,
			Value: p.currentExpr,
		})
	}
	;

go_type_operand
	: TYPE IDENT '=' type {
		p := yylex.(*parser)
		p.currentGoTypeOperands = append(p.currentGoTypeOperands, ast.GoTypeOperand{
			Line: $2.line,
			Column: $2.col,
			Name: $2.lit,
			Type: p.currentType,
		})
	}
	;

slice_lit
	: SLICE {
		p := yylex.(*parser)
		p.currentExpr = &ast.SliceLitExpr{Line: $1.line, Column: $1.col, Elems: nil}
		p.currentSliceElems = nil
	}
	| LBRACK maybe_expr_list RBRACK {
		p := yylex.(*parser)
		p.currentExpr = &ast.SliceLitExpr{Line: $1.line, Column: $1.col, Elems: append([]ast.Expr(nil), p.currentSliceElems...)}
		p.currentSliceElems = nil
	}
	;

call_start
	: LPAREN {
		p := yylex.(*parser)
		$$ = $1
		p.currentCallCalleeStack = append(p.currentCallCalleeStack, p.currentExpr)
		p.currentArgsStack = append(p.currentArgsStack, p.currentArgs)
		p.currentSliceElemsStack = append(p.currentSliceElemsStack, p.currentSliceElems)
		p.currentArgs = nil
		p.currentSliceElems = nil
	}
	;

maybe_expr_list
	: /* empty */
	| expr_list
	;

expr_list
	: expr {
		p := yylex.(*parser)
		p.currentArgs = append(p.currentArgs, p.currentExpr)
		p.currentSliceElems = append(p.currentSliceElems, p.currentExpr)
	}
	| expr_list COMMA expr {
		p := yylex.(*parser)
		p.currentArgs = append(p.currentArgs, p.currentExpr)
		p.currentSliceElems = append(p.currentSliceElems, p.currentExpr)
	}
	;

collection_lit
	: LBRACE maybe_collection_entries RBRACE {
		p := yylex.(*parser)
		if p.currentCollectionHasPair {
			p.currentExpr = &ast.MapLitExpr{Line: $1.line, Column: $1.col, Pairs: append([]ast.MapLitPair(nil), p.currentMapEntries...)}
		} else {
			p.currentExpr = &ast.SetLitExpr{Line: $1.line, Col: $1.col, Elems: append([]ast.Expr(nil), p.currentSetElems...)}
		}
		p.currentMapEntries = nil
		p.currentSetElems = nil
		p.currentCollectionHasPair = false
	}
	;

maybe_struct_fields
	: /* empty */
	| struct_field_list
	;

struct_field_list
	: struct_field
	| struct_field_list COMMA struct_field
	;

struct_field
	: IDENT COLON expr {
		p := yylex.(*parser)
		p.currentStructFields = append(p.currentStructFields, ast.StructLitField{
			Line: $1.line,
			Column: $1.col,
			Name: $1.lit,
			Value: p.currentExpr,
		})
	}
	| IDENT {
		p := yylex.(*parser)
		p.currentStructFields = append(p.currentStructFields, ast.StructLitField{
			Line: $1.line,
			Column: $1.col,
			Name: $1.lit,
		})
	}
	;

maybe_collection_entries
	: /* empty */
	| collection_entries
	;

collection_entries
	: collection_entry
	| collection_entries COMMA collection_entry
	;

collection_entry
	: expr {
		p := yylex.(*parser)
		p.currentMapKey = p.currentExpr
		p.currentSetElems = append(p.currentSetElems, p.currentExpr)
	}
	COLON expr {
		p := yylex.(*parser)
		p.currentCollectionHasPair = true
		p.currentMapValue = p.currentExpr
		p.currentMapEntries = append(p.currentMapEntries, ast.MapLitPair{
			Key: p.currentMapKey,
			Value: p.currentMapValue,
		})
		p.currentMapKey = nil
		p.currentMapValue = nil
	}
	| expr {
		p := yylex.(*parser)
		p.currentSetElems = append(p.currentSetElems, p.currentExpr)
	}
	;

if_expr
	: IF expr {
		p := yylex.(*parser)
		p.currentIfCond = p.currentExpr
	}
	THEN expr {
		p := yylex.(*parser)
		p.currentIfThen = p.currentExpr
	}
	ELSE expr {
		p := yylex.(*parser)
		p.currentIfElse = p.currentExpr
		p.currentExpr = &ast.IfExpr{Line: $1.line, Column: $1.col, Cond: p.currentIfCond, Then: p.currentIfThen, Else: p.currentIfElse}
		p.currentIfCond = nil
		p.currentIfThen = nil
		p.currentIfElse = nil
	}
	| IF expr {
		p := yylex.(*parser)
		p.currentIfCond = p.currentExpr
	}
	ARROW expr {
		p := yylex.(*parser)
		p.currentIfThen = p.currentExpr
	}
	ELSE expr {
		p := yylex.(*parser)
		p.currentIfElse = p.currentExpr
		p.currentExpr = &ast.IfExpr{Line: $1.line, Column: $1.col, Cond: p.currentIfCond, Then: p.currentIfThen, Else: p.currentIfElse}
		p.currentIfCond = nil
		p.currentIfThen = nil
		p.currentIfElse = nil
	}
	| IF expr NEWLINE {
		p := yylex.(*parser)
		p.currentIfCond = p.currentExpr
	}
	opt_newlines
	block_expr {
		p := yylex.(*parser)
		p.currentIfThen = p.currentExpr
	}
	ELSE opt_newlines block_expr opt_newlines END {
		p := yylex.(*parser)
		p.currentIfElse = p.currentExpr
		p.currentExpr = &ast.IfExpr{Line: $1.line, Column: $1.col, Cond: p.currentIfCond, Then: p.currentIfThen, Else: p.currentIfElse}
		p.currentIfCond = nil
		p.currentIfThen = nil
		p.currentIfElse = nil
	}
	;

while_expr
	: WHILE expr {
		p := yylex.(*parser)
		p.currentWhileCond = p.currentExpr
	}
	opt_newlines
	block_expr opt_newlines END {
		p := yylex.(*parser)
		p.currentWhileBody = p.currentExpr
		p.currentExpr = &ast.WhileExpr{Line: $1.line, Column: $1.col, Cond: p.currentWhileCond, Body: p.currentWhileBody}
		p.currentWhileCond = nil
		p.currentWhileBody = nil
	}
	;

switch_expr
	: SWITCH expr {
		p := yylex.(*parser)
		p.currentSwitchTarget = p.currentExpr
	}
	opt_newlines
	switch_case_list opt_newlines END {
		p := yylex.(*parser)
		p.currentExpr = &ast.SwitchExpr{Line: $1.line, Column: $1.col, Target: p.currentSwitchTarget, Cases: append([]ast.SwitchCase(nil), p.currentSwitchCases...)}
		p.currentSwitchTarget = nil
		p.currentSwitchCases = nil
	}
	;

switch_case_list
	: /* empty */
	| switch_case_list opt_newlines switch_case
	;

switch_case
	: CASE pattern ARROW opt_newlines block_expr {
		p := yylex.(*parser)
		p.currentSwitchCases = append(p.currentSwitchCases, ast.SwitchCase{
			Pattern: p.currentPattern,
			Body: p.currentExpr,
		})
		p.currentPattern = nil
	}
	| CASE pattern THEN opt_newlines block_expr opt_newlines END {
		p := yylex.(*parser)
		p.currentSwitchCases = append(p.currentSwitchCases, ast.SwitchCase{
			Pattern: p.currentPattern,
			Body: p.currentExpr,
		})
		p.currentPattern = nil
	}
	;

pattern
	: UNDER {
		p := yylex.(*parser)
		p.currentPattern = &ast.WildcardPattern{Line: $1.line, Column: $1.col}
	}
	| IDENT {
		p := yylex.(*parser)
		if $1.lit == "_" {
			p.currentPattern = &ast.WildcardPattern{Line: $1.line, Column: $1.col}
		} else {
			p.currentPattern = &ast.VariantPattern{Line: $1.line, Column: $1.col, Name: $1.lit}
		}
	}
	| IDENT LPAREN pattern_name_list RPAREN {
		p := yylex.(*parser)
		args := append([]string(nil), p.currentPatternArgs...)
		p.currentPattern = &ast.VariantPattern{Line: $1.line, Column: $1.col, Name: $1.lit, Args: args}
		p.currentPatternArgs = nil
	}
	;

pattern_name_list
	: /* empty */
	| pattern_name_list COMMA IDENT {
		p := yylex.(*parser)
		p.currentPatternArgs = append(p.currentPatternArgs, $3.lit)
	}
	| pattern_name_list COMMA UNDER {
		p := yylex.(*parser)
		p.currentPatternArgs = append(p.currentPatternArgs, "_")
	}
	| IDENT {
		p := yylex.(*parser)
		p.currentPatternArgs = append(p.currentPatternArgs, $1.lit)
	}
	| UNDER {
		p := yylex.(*parser)
		p.currentPatternArgs = append(p.currentPatternArgs, "_")
	}
	;

func_lit
	: FUNC LPAREN maybe_param_list RPAREN ARROW type opt_newlines block_expr opt_newlines END {
		p := yylex.(*parser)
		body := bodyExprFromBlock(p.currentExpr)
		p.currentExpr = &ast.FuncLitExpr{Line: $1.line, Column: $1.col, Params: append([]ast.Param(nil), p.currentParams...), Ret: p.currentType, Body: body}
		p.currentParams = nil
	}
	;

block_expr
	: block_start block_expr_items {
		p := yylex.(*parser)
		body := p.currentExpr
		if len(p.currentBlockStack) > 0 {
			idx := len(p.currentBlockStack) - 1
			p.currentBlock = p.currentBlockStack[idx]
			p.currentBlockStack = p.currentBlockStack[:idx]
		}
		p.currentExpr = body
	}
	;

block_start
	: /* empty */ {
		p := yylex.(*parser)
		p.currentBlockStack = append(p.currentBlockStack, p.currentBlock)
		p.currentBlock = nil
	}
	;

block_expr_items
	: stmt {
		p := yylex.(*parser)
		p.currentBlock = append(p.currentBlock, p.currentStmt)
		p.currentExpr = &ast.BlockExpr{Stmts: append([]ast.Stmt(nil), p.currentBlock...)}
	}
	| block_expr_items NEWLINE stmt {
		p := yylex.(*parser)
		p.currentBlock = append(p.currentBlock, p.currentStmt)
		p.currentExpr = &ast.BlockExpr{Stmts: append([]ast.Stmt(nil), p.currentBlock...)}
	}
	| block_expr_items NEWLINE
	{
		p := yylex.(*parser)
		p.currentExpr = &ast.BlockExpr{Stmts: append([]ast.Stmt(nil), p.currentBlock...)}
	}
	;

stmt
	: binding_stmt
	| assign_stmt
	| return_stmt
	| expr_stmt
	;

binding_stmt
	: LET IDENT opt_type_annot '=' expr {
		p := yylex.(*parser)
		p.currentStmt = &ast.LetStmt{Name: $2.lit, Mutable: false, Value: p.currentExpr}
	}
	| LET LPAREN tuple_bind_names RPAREN opt_type_annot '=' expr {
		p := yylex.(*parser)
		p.currentStmt = &ast.LetStmt{Names: append([]string(nil), p.currentPatternArgs...), Mutable: false, Value: p.currentExpr}
		p.currentPatternArgs = nil
	}
	| VAR IDENT opt_type_annot '=' expr {
		p := yylex.(*parser)
		p.currentStmt = &ast.LetStmt{Name: $2.lit, Mutable: true, Value: p.currentExpr}
	}
	;

tuple_bind_names
	: IDENT {
		p := yylex.(*parser)
		p.currentPatternArgs = append(p.currentPatternArgs, $1.lit)
	}
	| tuple_bind_names COMMA IDENT {
		p := yylex.(*parser)
		p.currentPatternArgs = append(p.currentPatternArgs, $3.lit)
	}
	;

assign_stmt
	: IDENT '=' expr {
		p := yylex.(*parser)
		p.currentStmt = &ast.AssignStmt{Name: $1.lit, Value: p.currentExpr}
	}
	;

return_stmt
	: RETURN {
		p := yylex.(*parser)
		p.currentStmt = &ast.ReturnStmt{}
	}
	| RETURN expr {
		p := yylex.(*parser)
		p.currentStmt = &ast.ReturnStmt{Value: p.currentExpr}
	}
	;

expr_stmt
	: expr {
		p := yylex.(*parser)
		p.currentStmt = &ast.ExprStmt{Expr: p.currentExpr}
	}
	;

%%

func (p *parser) Lex(lval *yySymType) int {
	tok := p.nextRaw()
	lval.setTok(tok)
	if tok.lit != "[" {
		p.expectTypeSuffix = false
		p.expectStructTypeArgs = false
	}
	switch tok.kind {
	case tokEOF:
		return 0
	case tokNewline:
		return int(NEWLINE)
	case tokIdent:
		return int(IDENT)
	case tokNumber:
		return int(NUMBER)
	case tokString:
		return int(STRING)
	case tokKeyword:
		switch tok.lit {
		case "package":
			return int(PACKAGE)
		case "import":
			return int(IMPORT)
		case "enum":
			return int(ENUM)
		case "struct":
			return int(STRUCT)
		case "interface":
			return int(INTERFACE)
		case "impl":
			return int(IMPL)
		case "func":
			return int(FUNC)
		case "if":
			return int(IF)
		case "then":
			return int(THEN)
		case "else":
			return int(ELSE)
		case "switch":
			return int(SWITCH)
		case "case":
			return int(CASE)
		case "end":
			return int(END)
		case "using":
			return int(USING)
		case "let":
			return int(LET)
		case "var":
			return int(VAR)
		case "embed":
			return int(EMBED)
		case "while":
			return int(WHILE)
		case "return":
			return int(RETURN)
		case "go":
			return int(GO)
		case "in":
			return int(IN)
		case "type":
			return int(TYPE)
		default:
			return int(IDENT)
		}
	default:
		switch tok.lit {
		case "[]":
			return int(SLICE)
		case "=>":
			return int(ARROW)
		case "->":
			return int(ARROW)
		case "==":
			return int(EQEQ)
		case "!=":
			return int(NEQ)
		case "<=":
			return int(LTE)
		case ">=":
			return int(GTE)
		case "<|":
			return int(PIPEBACK)
		case "|>":
			return int(PIPEFWD)
		case "&&":
			return int(ANDAND)
		case "||":
			return int(OROR)
		case ":":
			return int(COLON)
		case ",":
			return int(COMMA)
		case ".":
			return int(DOT)
		case "(":
			return int(LPAREN)
		case ")":
			return int(RPAREN)
		case "[":
			if p.expectStructTypeArgs {
				return int(TYPELBRACK)
			}
			if p.expectConstraintSuffix {
				return int(CONSTRLBRACK)
			}
			if p.expectTypeSuffix {
				return int(TYPELBRACK)
			}
			return int(LBRACK)
		case "]":
			return int(RBRACK)
		case "{":
			return int(LBRACE)
		case "}":
			return int(RBRACE)
		case "_":
			return int(UNDER)
		case "!":
			return int(NOT)
		case "=":
			return int('=')
		case "<":
			return int('<')
		case ">":
			return int('>')
		case "+":
			return int('+')
		case "-":
			return int('-')
		case "*":
			return int('*')
		case "/":
			return int('/')
		default:
			return int(tok.lit[0])
		}
	}
}
