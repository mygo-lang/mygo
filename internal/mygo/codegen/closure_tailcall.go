package codegen

import (
	"go/ast"
	"go/token"
)

// optimizeClosureTailcalls turns a returned closure's self-recursive factory
// invocation into an explicit continuation stack.  Unlike mutualTailPlan this
// also handles recursion modulo arbitrary work: the statements after the
// recursive invocation are captured as a continuation, not interpreted by
// this pass.  Consequently no library type, method, or function name is part
// of the recognition rule.
func optimizeClosureTailcalls(fd *ast.FuncDecl) {
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 || fd.Type.Params == nil {
		return
	}
	ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return
	}
	lit, ok := ret.Results[0].(*ast.FuncLit)
	if !ok || lit.Type.Params == nil || len(lit.Type.Params.List) != 1 || len(lit.Type.Params.List[0].Names) != 1 || lit.Type.Results == nil || len(lit.Type.Results.List) != 1 {
		return
	}
	// A single explicit final return gives every base path one well-defined
	// point at which to start unwinding continuations.
	final, ok := lit.Body.List[len(lit.Body.List)-1].(*ast.ReturnStmt)
	if !ok || len(final.Results) != 1 {
		return
	}

	candidate, path := closureRecursiveAssign(lit.Body.List, fd.Name.Name, mtFieldNames(fd.Type.Params))
	if candidate == nil || len(path) == 0 {
		return
	}
	call := candidate.Rhs[0].(*ast.CallExpr)
	factory := call.Fun.(*ast.CallExpr)
	next := call.Args[0]
	tail := candidate.Lhs[0].(*ast.Ident)
	state := lit.Type.Params.List[0].Names[0]
	resultType := lit.Type.Results.List[0].Type

	// The continuation is the suffix of every enclosing statement list from
	// the recursive assignment back to the closure's final return.  It is
	// generated as an ordinary Go closure, so all source-level locals and any
	// typeclass dictionaries remain captured with their normal semantics.
	continuation := []ast.Stmt{&ast.AssignStmt{Lhs: []ast.Expr{tail}, Tok: token.DEFINE, Rhs: []ast.Expr{ast.NewIdent("__mygo_tcmc_result")}}}
	for i := len(path) - 1; i >= 0; i-- {
		frame := path[i]
		continuation = append(continuation, frame.list[frame.index+1:]...)
	}
	if len(continuation) == 1 {
		return
	}
	cont := &ast.FuncLit{Type: &ast.FuncType{
		Params:  &ast.FieldList{List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("__mygo_tcmc_result")}, Type: resultType}}},
		Results: &ast.FieldList{List: []*ast.Field{{Type: resultType}}},
	}, Body: &ast.BlockStmt{List: continuation}}
	stack := ast.NewIdent("__mygo_tcmc_stack")
	stackType := &ast.ArrayType{Elt: &ast.FuncType{
		Params:  &ast.FieldList{List: []*ast.Field{{Type: resultType}}},
		Results: &ast.FieldList{List: []*ast.Field{{Type: resultType}}},
	}}
	push := &ast.AssignStmt{Lhs: []ast.Expr{stack}, Tok: token.ASSIGN, Rhs: []ast.Expr{&ast.CallExpr{Fun: ast.NewIdent("append"), Args: []ast.Expr{stack, cont}}}}
	replace := []ast.Stmt{
		// Keep the original binding in scope for the now-unreachable source
		// suffix. Go still type-checks unreachable statements.
		&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{tail}, Type: resultType}}}},
		push,
		&ast.AssignStmt{Lhs: []ast.Expr{state}, Tok: token.ASSIGN, Rhs: []ast.Expr{next}},
		&ast.BranchStmt{Tok: token.CONTINUE},
	}
	if !closureReplaceAssign(&lit.Body.List, candidate, replace) {
		return
	}

	// Replace the original final return with the generic continuation unwind.
	// The continuation itself contains the unmodified cloned-in-place suffix,
	// including its return statement, so each frame returns one value to the
	// next older frame.
	result := ast.NewIdent("__mygo_tcmc_result")
	last := ast.NewIdent("__mygo_tcmc_last")
	finalIndex := len(lit.Body.List) - 1
	lit.Body.List[finalIndex] = &ast.BlockStmt{List: []ast.Stmt{
		&ast.AssignStmt{Lhs: []ast.Expr{result}, Tok: token.DEFINE, Rhs: final.Results},
		&ast.ForStmt{Cond: &ast.BinaryExpr{X: &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{stack}}, Op: token.GTR, Y: &ast.BasicLit{Kind: token.INT, Value: "0"}}, Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.AssignStmt{Lhs: []ast.Expr{last}, Tok: token.DEFINE, Rhs: []ast.Expr{&ast.IndexExpr{X: stack, Index: &ast.BinaryExpr{X: &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{stack}}, Op: token.SUB, Y: &ast.BasicLit{Kind: token.INT, Value: "1"}}}}},
			&ast.AssignStmt{Lhs: []ast.Expr{stack}, Tok: token.ASSIGN, Rhs: []ast.Expr{&ast.SliceExpr{X: stack, High: &ast.BinaryExpr{X: &ast.CallExpr{Fun: ast.NewIdent("len"), Args: []ast.Expr{stack}}, Op: token.SUB, Y: &ast.BasicLit{Kind: token.INT, Value: "1"}}}}},
			&ast.AssignStmt{Lhs: []ast.Expr{result}, Tok: token.ASSIGN, Rhs: []ast.Expr{&ast.CallExpr{Fun: last, Args: []ast.Expr{result}}}},
		}}},
		&ast.ReturnStmt{Results: []ast.Expr{result}},
	}}
	lit.Body.List = []ast.Stmt{
		&ast.AssignStmt{Lhs: []ast.Expr{stack}, Tok: token.DEFINE, Rhs: []ast.Expr{&ast.CompositeLit{Type: stackType}}},
		&ast.ForStmt{Body: &ast.BlockStmt{List: lit.Body.List}},
	}
	_ = factory // documents that the factory call is deliberately eliminated.
}

type closureFrame struct {
	list  []ast.Stmt
	index int
}

func closureRecursiveAssign(list []ast.Stmt, name string, outer []ast.Expr) (*ast.AssignStmt, []closureFrame) {
	var found *ast.AssignStmt
	var foundPath []closureFrame
	var walk func([]ast.Stmt, []closureFrame)
	walk = func(stmts []ast.Stmt, parents []closureFrame) {
		for i, stmt := range stmts {
			if found != nil {
				return
			}
			frame := append(append([]closureFrame{}, parents...), closureFrame{list: stmts, index: i})
			if a, ok := stmt.(*ast.AssignStmt); ok && a.Tok == token.DEFINE && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
				if _, ok := a.Lhs[0].(*ast.Ident); ok {
					if call, ok := a.Rhs[0].(*ast.CallExpr); ok && len(call.Args) == 1 {
						if factory, ok := call.Fun.(*ast.CallExpr); ok && mtCalledName(factory.Fun) == name && len(factory.Args) == len(outer) {
							same := true
							for j, arg := range factory.Args {
								id, ok := arg.(*ast.Ident)
								if !ok || id.Name != outer[j].(*ast.Ident).Name {
									same = false
									break
								}
							}
							if same {
								found, foundPath = a, frame
								return
							}
						}
					}
				}
			}
			if ifs, ok := stmt.(*ast.IfStmt); ok {
				walk(ifs.Body.List, frame)
				if b, ok := ifs.Else.(*ast.BlockStmt); ok {
					walk(b.List, frame)
				}
			}
		}
	}
	walk(list, nil)
	return found, foundPath
}

func closureReplaceAssign(list *[]ast.Stmt, target *ast.AssignStmt, replacement []ast.Stmt) bool {
	for i, stmt := range *list {
		if stmt == target {
			out := append([]ast.Stmt{}, (*list)[:i]...)
			out = append(out, replacement...)
			*list = append(out, (*list)[i+1:]...)
			return true
		}
		if ifs, ok := stmt.(*ast.IfStmt); ok {
			if closureReplaceAssign(&ifs.Body.List, target, replacement) {
				return true
			}
			if b, ok := ifs.Else.(*ast.BlockStmt); ok && closureReplaceAssign(&b.List, target, replacement) {
				return true
			}
		}
	}
	return false
}
