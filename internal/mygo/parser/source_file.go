package parser

import "github.com/mygo-lang/mygo/internal/mygo/ast"

func setASTSourceFile(f *ast.File, filename string) {
	if f == nil || filename == "" {
		return
	}
	for _, d := range f.Decls {
		setDeclSourceFile(d, filename)
	}
}

func setDeclSourceFile(d ast.Decl, filename string) {
	switch n := d.(type) {
	case *ast.ImportDecl:
		n.SourceFile = filename
	case *ast.EnumDecl:
		n.SourceFile = filename
		for i := range n.Variants {
			n.Variants[i].SourceFile = filename
			setFieldsSourceFile(n.Variants[i].Fields, filename)
		}
	case *ast.StructDecl:
		n.SourceFile = filename
		setFieldsSourceFile(n.Fields, filename)
	case *ast.InterfaceDecl:
		n.SourceFile = filename
		for _, m := range n.Methods {
			setFuncDeclSourceFile(m, filename)
		}
	case *ast.ImplDecl:
		n.SourceFile = filename
		for _, m := range n.Methods {
			setFuncDeclSourceFile(m, filename)
		}
	case *ast.FuncDecl:
		setFuncDeclSourceFile(n, filename)
	case *ast.LetStmt:
		setLetStmtSourceFile(n, filename)
	}
}

func setFuncDeclSourceFile(n *ast.FuncDecl, filename string) {
	if n == nil {
		return
	}
	n.SourceFile = filename
	for i := range n.Params {
		n.Params[i].SourceFile = filename
	}
	for i := range n.Using {
		n.Using[i].SourceFile = filename
	}
	setExprSourceFile(n.Body, filename)
}

func setFieldsSourceFile(fields []ast.Field, filename string) {
	for i := range fields {
		fields[i].SourceFile = filename
	}
}

func setLetStmtSourceFile(n *ast.LetStmt, filename string) {
	if n == nil {
		return
	}
	n.SourceFile = filename
	setBindPatternSourceFile(n.Bind, filename)
	setExprSourceFile(n.Value, filename)
}

func setStmtSourceFile(s ast.Stmt, filename string) {
	switch n := s.(type) {
	case *ast.ExprStmt:
		n.SourceFile = filename
		setExprSourceFile(n.Expr, filename)
	case *ast.LetStmt:
		setLetStmtSourceFile(n, filename)
	case *ast.ReturnStmt:
		n.SourceFile = filename
		setExprSourceFile(n.Value, filename)
	case *ast.AssignStmt:
		n.SourceFile = filename
		setExprSourceFile(n.Value, filename)
	}
}

func setExprSourceFile(e ast.Expr, filename string) {
	switch n := e.(type) {
	case *ast.IdentExpr:
		n.SourceFile = filename
	case *ast.LiteralExpr:
		n.SourceFile = filename
	case *ast.CallExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Callee, filename)
		for _, arg := range n.Args {
			setExprSourceFile(arg, filename)
		}
	case *ast.StructLitExpr:
		n.SourceFile = filename
		for i := range n.Fields {
			n.Fields[i].SourceFile = filename
			setExprSourceFile(n.Fields[i].Value, filename)
		}
	case *ast.BinaryExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Left, filename)
		setExprSourceFile(n.Right, filename)
	case *ast.PrefixExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Expr, filename)
	case *ast.CastExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Expr, filename)
	case *ast.FieldExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Expr, filename)
	case *ast.FuncLitExpr:
		n.SourceFile = filename
		for i := range n.Params {
			n.Params[i].SourceFile = filename
		}
		setExprSourceFile(n.Body, filename)
	case *ast.IfExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Cond, filename)
		setExprSourceFile(n.Then, filename)
		setExprSourceFile(n.Else, filename)
	case *ast.SwitchExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Target, filename)
		for i := range n.Cases {
			n.Cases[i].SourceFile = filename
			setPatternSourceFile(n.Cases[i].Pattern, filename)
			setExprSourceFile(n.Cases[i].Body, filename)
		}
	case *ast.WhileExpr:
		n.SourceFile = filename
		setExprSourceFile(n.Cond, filename)
		setExprSourceFile(n.Body, filename)
	case *ast.SliceLitExpr:
		n.SourceFile = filename
		for _, elem := range n.Elems {
			setExprSourceFile(elem, filename)
		}
	case *ast.MapLitExpr:
		n.SourceFile = filename
		for i := range n.Pairs {
			n.Pairs[i].SourceFile = filename
			setExprSourceFile(n.Pairs[i].Key, filename)
			setExprSourceFile(n.Pairs[i].Value, filename)
		}
	case *ast.SetLitExpr:
		n.SourceFile = filename
		for _, elem := range n.Elems {
			setExprSourceFile(elem, filename)
		}
	case *ast.TupleLitExpr:
		n.SourceFile = filename
		for _, elem := range n.Elems {
			setExprSourceFile(elem, filename)
		}
	case *ast.UnitLitExpr:
		n.SourceFile = filename
	case *ast.GoExpr:
		n.SourceFile = filename
		for i := range n.Operands {
			n.Operands[i].SourceFile = filename
			setExprSourceFile(n.Operands[i].Value, filename)
		}
		for i := range n.TypeOperands {
			n.TypeOperands[i].SourceFile = filename
		}
	case *ast.BlockExpr:
		n.SourceFile = filename
		for _, st := range n.Stmts {
			setStmtSourceFile(st, filename)
		}
	}
}

func setPatternSourceFile(p ast.Pattern, filename string) {
	switch n := p.(type) {
	case *ast.VariantPattern:
		n.SourceFile = filename
	case *ast.LiteralPattern:
		n.SourceFile = filename
	case *ast.TuplePattern:
		n.SourceFile = filename
		for _, elem := range n.Elems {
			setPatternSourceFile(elem, filename)
		}
	case *ast.WildcardPattern:
		n.SourceFile = filename
	}
}

func setBindPatternSourceFile(p ast.BindPattern, filename string) {
	switch n := p.(type) {
	case *ast.BindNamePattern:
		n.SourceFile = filename
	case *ast.BindTuplePattern:
		n.SourceFile = filename
		for _, elem := range n.Elems {
			setBindPatternSourceFile(elem, filename)
		}
	}
}
