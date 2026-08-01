package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
)

// mutualTailPlan is a private, immutable code-generation plan for one SCC.
type mutualTailPlan struct {
	name    string
	owner   string
	members []*FuncDecl
	state   map[*FuncDecl]int
}

func (p *mutualTailPlan) member(name string) *FuncDecl {
	for _, f := range p.members {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func (g *gen) buildMutualTailPlans() {
	funcs := make([]*FuncDecl, 0, len(g.pkg.Funcs))
	for _, f := range g.pkg.Funcs {
		funcs = append(funcs, f)
	}
	sort.Slice(funcs, func(i, j int) bool { return mtOrder(funcs[i], funcs[j]) })
	byName := map[string]*FuncDecl{}
	for _, f := range funcs {
		byName[f.Name] = f
	}
	edges := map[*FuncDecl][]*FuncDecl{}
	for _, f := range funcs {
		mtCollectExpr(f.Body, true, byName, func(to *FuncDecl) { edges[f] = append(edges[f], to) })
	}

	indices, low, onStack := map[*FuncDecl]int{}, map[*FuncDecl]int{}, map[*FuncDecl]bool{}
	var stack []*FuncDecl
	next := 0
	var visit func(*FuncDecl)
	visit = func(v *FuncDecl) {
		indices[v], low[v] = next, next
		next++
		stack, onStack[v] = append(stack, v), true
		for _, w := range edges[v] {
			if _, seen := indices[w]; !seen {
				visit(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && indices[w] < low[v] {
				low[v] = indices[w]
			}
		}
		if low[v] != indices[v] {
			return
		}
		var members []*FuncDecl
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			members = append(members, w)
			if w == v {
				break
			}
		}
		g.addMutualTailPlan(members, byName)
	}
	for _, f := range funcs {
		if _, seen := indices[f]; !seen {
			visit(f)
		}
	}
}

func (g *gen) addMutualTailPlan(members []*FuncDecl, byName map[string]*FuncDecl) {
	// A one-member SCC is eligible only when it has a tail edge to itself.
	// Tarjan also produces singleton SCCs for non-recursive functions, which
	// must retain their ordinary direct lowering.
	if len(members) == 1 && !mtHasTailSelfEdge(members[0], byName) {
		return
	}
	if !mtSameABI(g, members) {
		return
	}
	inGroup := map[*FuncDecl]bool{}
	for _, f := range members {
		inGroup[f] = true
	}
	// A direct intra-group call must be in tail position; otherwise rewriting
	// only part of a cycle would still leave a stack-growing path.
	for _, f := range members {
		ok := true
		mtCollectDirectCalls(f.Body, true, byName, func(to *FuncDecl, tail bool) {
			if inGroup[to] && !tail {
				ok = false
			}
		})
		if !ok {
			return
		}
	}
	sort.Slice(members, func(i, j int) bool { return mtOrder(members[i], members[j]) })
	owner := sourceFileOf(members[0])
	parts := make([]string, len(members))
	for i, f := range members {
		if sourceFileOf(f) < owner {
			owner = sourceFileOf(f)
		}
		parts[i] = strings.ToLower(sanitizeIdent(f.Name))
	}
	p := &mutualTailPlan{name: "__mygo_mt_" + sanitizeIdent(g.pkg.Name) + "_" + strings.Join(parts, "_"), owner: owner, members: members, state: map[*FuncDecl]int{}}
	for i, f := range members {
		p.state[f] = i
		g.mutualTail[f] = p
	}
}

func mtHasTailSelfEdge(f *FuncDecl, funcs map[string]*FuncDecl) bool {
	found := false
	mtCollectExpr(f.Body, true, funcs, func(to *FuncDecl) {
		if to == f {
			found = true
		}
	})
	return found
}

func mtCollectDirectCalls(e Expr, tail bool, funcs map[string]*FuncDecl, add func(*FuncDecl, bool)) {
	if e == nil {
		return
	}
	if c, ok := e.(*CallExpr); ok {
		if id, ok := c.Callee.(*IdentExpr); ok && funcs[id.Name] != nil {
			add(funcs[id.Name], tail)
		}
		mtCollectDirectCalls(c.Callee, false, funcs, add)
		for _, arg := range c.Args {
			mtCollectDirectCalls(arg, false, funcs, add)
		}
		return
	}
	switch x := e.(type) {
	case *IfExpr:
		mtCollectDirectCalls(x.Cond, false, funcs, add)
		mtCollectDirectCalls(x.Then, tail, funcs, add)
		mtCollectDirectCalls(x.Else, tail, funcs, add)
	case *SwitchExpr:
		mtCollectDirectCalls(x.Target, false, funcs, add)
		for _, c := range x.Cases {
			mtCollectDirectCalls(c.Body, tail, funcs, add)
		}
	case *BlockExpr:
		for i, s := range x.Stmts {
			stmtTail := tail && i == len(x.Stmts)-1
			switch st := s.(type) {
			case *ExprStmt:
				mtCollectDirectCalls(st.Expr, stmtTail, funcs, add)
			case *ReturnStmt:
				mtCollectDirectCalls(st.Value, true, funcs, add)
			case *LetStmt:
				mtCollectDirectCalls(st.Value, false, funcs, add)
			case *AssignStmt:
				mtCollectDirectCalls(st.Value, false, funcs, add)
			}
		}
	case *BinaryExpr:
		mtCollectDirectCalls(x.Left, false, funcs, add)
		mtCollectDirectCalls(x.Right, false, funcs, add)
	case *PrefixExpr:
		mtCollectDirectCalls(x.Expr, false, funcs, add)
	case *CastExpr:
		mtCollectDirectCalls(x.Expr, false, funcs, add)
	case *FieldExpr:
		mtCollectDirectCalls(x.Expr, false, funcs, add)
	}
}

func mtOrder(a, b *FuncDecl) bool {
	if sourceFileOf(a) != sourceFileOf(b) {
		return sourceFileOf(a) < sourceFileOf(b)
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.Column != b.Column {
		return a.Column < b.Column
	}
	return a.Name < b.Name
}

func mtSameABI(g *gen, fs []*FuncDecl) bool {
	a := fs[0]
	for _, b := range fs[1:] {
		if len(a.Params) != len(b.Params) || len(a.TypeParams) != len(b.TypeParams) || len(a.Using) != len(b.Using) {
			return false
		}
		for i := range a.TypeParams {
			if a.TypeParams[i] != b.TypeParams[i] {
				return false
			}
		}
		for i := range a.Params {
			if g.goType(a.Params[i].Type, typeParamSet(a.TypeParams)) != g.goType(b.Params[i].Type, typeParamSet(b.TypeParams)) {
				return false
			}
		}
		ar, br := g.goReturnTypes(a.Ret, typeParamSet(a.TypeParams)), g.goReturnTypes(b.Ret, typeParamSet(b.TypeParams))
		if len(ar) != len(br) {
			return false
		}
		for i := range ar {
			if ar[i] != br[i] {
				return false
			}
		}
		for i := range a.Using {
			if a.Using[i].Name != b.Using[i].Name || len(a.Using[i].Args) != len(b.Using[i].Args) {
				return false
			}
			for j := range a.Using[i].Args {
				if typeString(a.Using[i].Args[j], nil) != typeString(b.Using[i].Args[j], nil) {
					return false
				}
			}
		}
	}
	return true
}

func mtCollectExpr(e Expr, tail bool, funcs map[string]*FuncDecl, add func(*FuncDecl)) {
	if e == nil {
		return
	}
	if c, ok := e.(*CallExpr); ok {
		if id, ok := c.Callee.(*IdentExpr); ok && tail && funcs[id.Name] != nil {
			add(funcs[id.Name])
		}
		mtCollectExpr(c.Callee, false, funcs, add)
		for _, a := range c.Args {
			mtCollectExpr(a, false, funcs, add)
		}
		return
	}
	switch x := e.(type) {
	case *IfExpr:
		mtCollectExpr(x.Cond, false, funcs, add)
		mtCollectExpr(x.Then, tail, funcs, add)
		mtCollectExpr(x.Else, tail, funcs, add)
	case *SwitchExpr:
		mtCollectExpr(x.Target, false, funcs, add)
		for _, c := range x.Cases {
			mtCollectExpr(c.Body, tail, funcs, add)
		}
	case *BlockExpr:
		for i, s := range x.Stmts {
			mtCollectStmt(s, tail && i == len(x.Stmts)-1, funcs, add)
		}
	case *BinaryExpr:
		mtCollectExpr(x.Left, false, funcs, add)
		mtCollectExpr(x.Right, false, funcs, add)
	case *PrefixExpr:
		mtCollectExpr(x.Expr, false, funcs, add)
	case *CastExpr:
		mtCollectExpr(x.Expr, false, funcs, add)
	case *FieldExpr:
		mtCollectExpr(x.Expr, false, funcs, add)
	}
}

func mtCollectStmt(s Stmt, tail bool, funcs map[string]*FuncDecl, add func(*FuncDecl)) {
	switch x := s.(type) {
	case *ExprStmt:
		mtCollectExpr(x.Expr, tail, funcs, add)
	case *ReturnStmt:
		mtCollectExpr(x.Value, true, funcs, add)
	case *LetStmt:
		mtCollectExpr(x.Value, false, funcs, add)
	case *AssignStmt:
		mtCollectExpr(x.Value, false, funcs, add)
	}
}

func (g *gen) addMutualTailTrampolines(sf *goast.SourceFile, owner string) error {
	seen := map[*mutualTailPlan]bool{}
	var plans []*mutualTailPlan
	for _, p := range g.mutualTail {
		if p.owner != owner || seen[p] {
			continue
		}
		seen[p] = true
		plans = append(plans, p)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].name < plans[j].name })
	for _, p := range plans {
		d, err := g.genMutualTailTrampoline(p)
		if err != nil {
			return err
		}
		sf.AddDecl(d)
	}
	return nil
}

func (g *gen) genMutualTailWrapper(d *FuncDecl, p *mutualTailPlan) (ast.Decl, error) {
	g.generatingMutualTail = true
	decl, err := g.genFuncDecl(d)
	g.generatingMutualTail = false
	if err != nil {
		return nil, err
	}
	fd := decl.(*ast.FuncDecl)
	args := mtFieldNames(fd.Type.Params)
	args = append(args, &ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", p.state[d])})
	call := &ast.CallExpr{Fun: mtGenericIdent(p.name, d.TypeParams), Args: args}
	if fd.Type.Results == nil || len(fd.Type.Results.List) == 0 {
		fd.Body = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: call}, &ast.ReturnStmt{}}}
	} else {
		fd.Body = &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{call}}}}
	}
	return fd, nil
}

func mtGenericIdent(name string, typeParams []string) ast.Expr {
	if len(typeParams) == 0 {
		return ast.NewIdent(name)
	}
	if len(typeParams) == 1 {
		return &ast.IndexExpr{X: ast.NewIdent(name), Index: ast.NewIdent(typeParams[0])}
	}
	indices := make([]ast.Expr, len(typeParams))
	for i, p := range typeParams {
		indices[i] = ast.NewIdent(p)
	}
	return &ast.IndexListExpr{X: ast.NewIdent(name), Indices: indices}
}

func mtFieldNames(fields *ast.FieldList) []ast.Expr {
	var out []ast.Expr
	if fields == nil {
		return out
	}
	for _, f := range fields.List {
		for _, n := range f.Names {
			out = append(out, ast.NewIdent(n.Name))
		}
	}
	return out
}

func (g *gen) genMutualTailTrampoline(p *mutualTailPlan) (ast.Decl, error) {
	g.generatingMutualTail = true
	protoDecl, err := g.genFuncDecl(p.members[0])
	g.generatingMutualTail = false
	if err != nil {
		return nil, err
	}
	proto := protoDecl.(*ast.FuncDecl)
	params := append([]*ast.Field(nil), proto.Type.Params.List...)
	params = append(params, &ast.Field{Names: []*ast.Ident{ast.NewIdent("__mygo_state")}, Type: ast.NewIdent("int")})
	shared := mtFieldNames(proto.Type.Params)
	clauses := []ast.Stmt{}
	for _, member := range p.members {
		g.generatingMutualTail = true
		d, err := g.genFuncDecl(member)
		g.generatingMutualTail = false
		if err != nil {
			return nil, err
		}
		fd := d.(*ast.FuncDecl)
		body := append([]ast.Stmt(nil), fd.Body.List...)
		locals := mtFieldNames(fd.Type.Params)
		for i := len(locals) - 1; i >= 0; i-- {
			if locals[i].(*ast.Ident).Name != shared[i].(*ast.Ident).Name {
				body = append([]ast.Stmt{&ast.AssignStmt{Lhs: []ast.Expr{locals[i]}, Tok: token.DEFINE, Rhs: []ast.Expr{shared[i]}}}, body...)
			}
		}
		body = mtRewriteStmts(body, p, shared)
		clauses = append(clauses, &ast.CaseClause{List: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", p.state[member])}}, Body: body})
	}
	clauses = append(clauses, &ast.CaseClause{Body: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic"), Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: "\"mygo: invalid mutual-tailcall state\""}}}}}})
	loop := &ast.ForStmt{Body: &ast.BlockStmt{List: []ast.Stmt{&ast.SwitchStmt{Tag: ast.NewIdent("__mygo_state"), Body: &ast.BlockStmt{List: clauses}}}}}
	return astFuncDecl(p.name, nil, proto.Type.TypeParams, params, proto.Type.Results.List, &ast.BlockStmt{List: []ast.Stmt{loop}}), nil
}

func mtRewriteStmts(in []ast.Stmt, p *mutualTailPlan, shared []ast.Expr) []ast.Stmt {
	var out []ast.Stmt
	for _, s := range in {
		switch x := s.(type) {
		case *ast.IfStmt:
			x.Body.List = mtRewriteStmts(x.Body.List, p, shared)
			if b, ok := x.Else.(*ast.BlockStmt); ok {
				b.List = mtRewriteStmts(b.List, p, shared)
			}
			out = append(out, x)
		case *ast.BlockStmt:
			x.List = mtRewriteStmts(x.List, p, shared)
			out = append(out, x)
		case *ast.ReturnStmt:
			if len(x.Results) == 1 {
				if jump := mtJump(x.Results[0], p, shared); jump != nil {
					out = append(out, jump...)
					continue
				}
			}
			out = append(out, x)
		case *ast.AssignStmt:
			if len(x.Rhs) == 1 {
				if jump := mtJump(x.Rhs[0], p, shared); jump != nil {
					out = append(out, jump...)
					continue
				}
			}
			out = append(out, x)
		case *ast.ExprStmt:
			if jump := mtJump(x.X, p, shared); jump != nil {
				out = append(out, jump...)
				continue
			}
			out = append(out, x)
		default:
			out = append(out, x)
		}
	}
	return out
}

func mtJump(e ast.Expr, p *mutualTailPlan, shared []ast.Expr) []ast.Stmt {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return nil
	}
	target := p.member(mtCalledName(c.Fun))
	if target == nil || len(c.Args) != len(shared) {
		return nil
	}
	out := []ast.Stmt{}
	temps := make([]ast.Expr, len(c.Args))
	for i, arg := range c.Args {
		tmp := ast.NewIdent(fmt.Sprintf("__mygo_next_%d", i))
		temps[i] = tmp
		out = append(out, &ast.AssignStmt{Lhs: []ast.Expr{tmp}, Tok: token.DEFINE, Rhs: []ast.Expr{arg}})
	}
	for i, tmp := range temps {
		out = append(out, &ast.AssignStmt{Lhs: []ast.Expr{shared[i]}, Tok: token.ASSIGN, Rhs: []ast.Expr{tmp}})
	}
	out = append(out, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("__mygo_state")}, Tok: token.ASSIGN, Rhs: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", p.state[target])}}}, &ast.BranchStmt{Tok: token.CONTINUE})
	return out
}

func mtCalledName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return mtCalledName(x.X)
	case *ast.IndexListExpr:
		return mtCalledName(x.X)
	}
	return ""
}
