package compiler

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

// expandTypeAliases makes aliases transparent to HM inference.  Generation
// still receives the original alias declarations, so Go retains their public
// spelling; only type checking sees their target type.
func expandTypeAliases(p *Package) {
	if p == nil || len(p.TypeAliases) == 0 {
		return
	}
	var substitute func(ast.TypeExpr, map[string]ast.TypeExpr) ast.TypeExpr
	substitute = func(t ast.TypeExpr, params map[string]ast.TypeExpr) ast.TypeExpr {
		switch n := t.(type) {
		case *ast.NamedType:
			if len(n.Args) == 0 {
				if replacement := params[n.Name]; replacement != nil {
					return substitute(replacement, nil)
				}
			}
			out := &ast.NamedType{Line: n.Line, Column: n.Column, Name: n.Name, Args: make([]ast.TypeExpr, len(n.Args))}
			for i, arg := range n.Args {
				out.Args[i] = substitute(arg, params)
			}
			return out
		case *ast.FuncType:
			out := &ast.FuncType{Line: n.Line, Column: n.Column, Params: make([]ast.TypeExpr, len(n.Params))}
			for i, param := range n.Params {
				out.Params[i] = substitute(param, params)
			}
			out.Ret = substitute(n.Ret, params)
			return out
		case *ast.TupleType:
			out := &ast.TupleType{Line: n.Line, Column: n.Column, Elems: make([]ast.TypeExpr, len(n.Elems))}
			for i, elem := range n.Elems {
				out.Elems[i] = substitute(elem, params)
			}
			return out
		default:
			return t
		}
	}
	var expand func(ast.TypeExpr, map[string]bool) ast.TypeExpr
	expand = func(t ast.TypeExpr, visiting map[string]bool) ast.TypeExpr {
		switch n := t.(type) {
		case *ast.NamedType:
			for i, arg := range n.Args {
				n.Args[i] = expand(arg, visiting)
			}
			if alias := p.TypeAliases[n.Name]; alias != nil && len(alias.TypeParams) == len(n.Args) && !visiting[n.Name] {
				visiting[n.Name] = true
				params := make(map[string]ast.TypeExpr, len(alias.TypeParams))
				for i, name := range alias.TypeParams {
					params[name] = n.Args[i]
				}
				result := expand(substitute(alias.Type, params), visiting)
				delete(visiting, n.Name)
				return result
			}
		case *ast.FuncType:
			for i, param := range n.Params {
				n.Params[i] = expand(param, visiting)
			}
			n.Ret = expand(n.Ret, visiting)
		case *ast.TupleType:
			for i, elem := range n.Elems {
				n.Elems[i] = expand(elem, visiting)
			}
		}
		return t
	}
	applyType := func(t *ast.TypeExpr) {
		if *t != nil {
			*t = expand(*t, map[string]bool{})
		}
	}
	var applyExpr func(ast.Expr)
	applyExpr = func(expr ast.Expr) {
		switch e := expr.(type) {
		case *ast.CallExpr:
			applyExpr(e.Callee)
			for i := range e.TypeArgs {
				applyType(&e.TypeArgs[i])
			}
			for _, arg := range e.Args {
				applyExpr(arg)
			}
		case *ast.StructLitExpr:
			if alias := p.TypeAliases[e.TypeName]; alias != nil {
				if target, ok := alias.Type.(*ast.NamedType); ok && len(target.Args) == 0 {
					e.TypeName = target.Name
				}
			}
			for i := range e.TypeArgs {
				applyType(&e.TypeArgs[i])
			}
			for _, field := range e.Fields {
				applyExpr(field.Value)
			}
		case *ast.BinaryExpr:
			applyExpr(e.Left)
			applyExpr(e.Right)
		case *ast.PrefixExpr:
			applyExpr(e.Expr)
		case *ast.CastExpr:
			applyExpr(e.Expr)
			applyType(&e.Type)
		case *ast.FieldExpr:
			applyExpr(e.Expr)
		case *ast.FuncLitExpr:
			for i := range e.Params {
				applyType(&e.Params[i].Type)
			}
			applyType(&e.Ret)
			applyExpr(e.Body)
		case *ast.IfExpr:
			applyExpr(e.Cond)
			applyExpr(e.Then)
			applyExpr(e.Else)
		case *ast.SwitchExpr:
			applyExpr(e.Target)
			for _, c := range e.Cases {
				applyExpr(c.Body)
			}
		case *ast.WhileExpr:
			applyExpr(e.Cond)
			applyExpr(e.Body)
		case *ast.SliceLitExpr:
			applyType(&e.Elem)
			for _, elem := range e.Elems {
				applyExpr(elem)
			}
		case *ast.MapLitExpr:
			applyType(&e.Key)
			applyType(&e.Val)
			for _, pair := range e.Pairs {
				applyExpr(pair.Key)
				applyExpr(pair.Value)
			}
		case *ast.SetLitExpr:
			applyType(&e.Elem)
			for _, elem := range e.Elems {
				applyExpr(elem)
			}
		case *ast.TupleLitExpr:
			for _, elem := range e.Elems {
				applyExpr(elem)
			}
		case *ast.GoExpr:
			applyType(&e.Result)
			for i := range e.TypeOperands {
				applyType(&e.TypeOperands[i].Type)
			}
			for _, operand := range e.Operands {
				applyExpr(operand.Value)
			}
		case *ast.BlockExpr:
			for _, stmt := range e.Stmts {
				switch s := stmt.(type) {
				case *ast.ExprStmt:
					applyExpr(s.Expr)
				case *ast.LetStmt:
					applyType(&s.Type)
					applyExpr(s.Value)
				case *ast.LetRecStmt:
					for i := range s.Bindings {
						applyType(&s.Bindings[i].Type)
						applyExpr(s.Bindings[i].Value)
					}
				case *ast.ReturnStmt:
					applyExpr(s.Value)
				case *ast.AssignStmt:
					applyExpr(s.Target)
					applyExpr(s.Value)
				}
			}
		}
	}
	for _, decl := range p.Decls {
		switch d := decl.(type) {
		case *ast.EnumDecl:
			for vi := range d.Variants {
				for fi := range d.Variants[vi].Fields {
					applyType(&d.Variants[vi].Fields[fi].Type)
				}
			}
		case *ast.StructDecl:
			for i := range d.Fields {
				applyType(&d.Fields[i].Type)
			}
		case *ast.InterfaceDecl:
			for _, method := range d.Methods {
				for i := range method.Params {
					applyType(&method.Params[i].Type)
				}
				applyType(&method.Ret)
			}
		case *ast.ImplDecl:
			applyType(&d.Type)
			for i := range d.TypeArgs {
				applyType(&d.TypeArgs[i])
			}
			for i := range d.InterfaceArgs {
				applyType(&d.InterfaceArgs[i])
			}
			for _, method := range d.Methods {
				for i := range method.Params {
					applyType(&method.Params[i].Type)
				}
				applyType(&method.Ret)
				applyExpr(method.Body)
			}
		case *ast.FuncDecl:
			for i := range d.Params {
				applyType(&d.Params[i].Type)
			}
			applyType(&d.Ret)
			applyExpr(d.Body)
		}
	}
}

// InferTyped runs type inference on a package and returns the TypedInfo.
func InferTyped(p *Package) (*typeinference.TypedInfo, error) {
	expandTypeAliases(p)
	pkgInfo := &typeinference.PkgInfo{
		Dir:            p.Dir,
		WorkspaceRoot:  p.WorkspaceRoot,
		Name:           p.Name,
		Decls:          p.Decls,
		TypeAliases:    p.TypeAliases,
		Types:          p.Types,
		Enums:          p.Enums,
		Structs:        p.Structs,
		Interfaces:     p.Interfaces,
		Funcs:          p.Funcs,
		Impls:          p.Impls,
		DotImportTypes: p.DotImportTypes,
		DotImportEnums: map[string]*ast.EnumDecl{},
	}
	state := typeinference.NewInferState()
	return typeinference.InferPackage(pkgInfo, state)
}
