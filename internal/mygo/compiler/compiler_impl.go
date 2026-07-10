package compiler

import (
	"sort"
	"strconv"
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) genGlobals() ([]jen.Code, error) {
	ctx := &exprCtx{
		locals:          map[string]string{},
		bindings:        map[string]string{},
		sourceTypes:     map[string]string{},
		mutable:         map[string]bool{},
		typeParams:      map[string]struct{}{},
		constraintFuncs: map[string]string{},
	}
	var stmts []jen.Code
	for _, decl := range g.pkg.Decls {
		s, ok := decl.(*LetStmt)
		if !ok {
			continue
		}
		code, typ, err := g.translateExpr(s.Value, ctx, g.goType(s.Type, nil))
		if err != nil {
			return nil, common.ErrorAtPos(s.Line, s.Column, "global binding %q: %v", s.Name, err)
		}
		actual := sanitizeIdent(s.Name)
		if actual == "" || actual == "_" {
			actual = "tmp"
		}
		if s.Name == "_" {
			stmt := jen.Var().Id("_").Op("=").Add(code)
			stmts = append(stmts, stmt)
			continue
		}
		stmt := jen.Var().Id(actual)
		if s.Type != nil {
			stmt.Add(jenTypeExpr(s.Type))
		}
		stmt.Op("=").Add(code)
		stmts = append(stmts, stmt)
		ctx.bindings[s.Name] = actual
		if typ == "" && s.Type != nil {
			typ = g.goType(s.Type, nil)
		}
		ctx.locals[s.Name] = typ
		ctx.sourceTypes[s.Name] = typ
		ctx.mutable[actual] = s.Mutable
	}
	return stmts, nil
}

func (p *Package) sortedImports() []importSpec {
	imports := make([]importSpec, 0, len(p.ImportDecls))
	seen := map[string]struct{}{}
	seenPaths := map[string]struct{}{}
	for _, imp := range p.ImportDecls {
		alias := imp.Alias
		if alias == "" {
			alias = importAliasForPath(imp.Path)
		}
		path := importPathForGo(imp.Path)
		key := alias + "\x00" + path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		seenPaths[path] = struct{}{}
		imports = append(imports, importSpec{Alias: alias, Path: imp.Path})
	}
	for path := range p.Imports {
		rawPath := importPathForGo(path)
		if _, ok := seenPaths[rawPath]; ok {
			continue
		}
		imports = append(imports, importSpec{Path: path})
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias == imports[j].Alias {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	return imports
}

func (g *generator) genHKTType(file *jen.File) {
	needsHKT := false
	for _, iface := range g.pkg.Interfaces {
		hktSet := g.hktParams(iface)
		if len(hktSet) > 0 {
			needsHKT = true
			break
		}
	}
	if !needsHKT {
		return
	}
	file.Add(jen.Type().Id("HKTType").Interface())
	file.Line()
	file.Add(jen.Type().Id("HKT1").Index(jen.Id("F").Id("any")).Interface())
	file.Line()
	file.Add(jen.Type().Id("HKT2").Index(jen.Id("A").Id("any")).Interface())
	file.Line()
	file.Add(addTypeParams(jen.Type().Id("HKT"), []string{"F", "A"}).Interface())
}

func (g *generator) hktParams(iface *InterfaceDecl) map[string]struct{} {
	set := make(map[string]struct{})
	validParams := typeParamSet(iface.TypeParams)
	for _, m := range iface.Methods {
		for _, p := range m.Params {
			g.collectHKTTypeNames(p.Type, set, validParams)
		}
		g.collectHKTTypeNames(m.Ret, set, validParams)
	}
	return set
}

func (g *generator) collectHKTTypeNames(t TypeExpr, set map[string]struct{}, validParams map[string]struct{}) {
	switch tt := t.(type) {
	case *NamedType:
		if validParams != nil && len(tt.Args) > 0 {
			if _, ok := validParams[tt.Name]; ok {
				set[tt.Name] = struct{}{}
			}
		}
		for _, a := range tt.Args {
			g.collectHKTTypeNames(a, set, validParams)
		}
	case *FuncType:
		for _, p := range tt.Params {
			g.collectHKTTypeNames(p, set, validParams)
		}
		g.collectHKTTypeNames(tt.Ret, set, validParams)
	}
}

func (g *generator) genEnum(d *EnumDecl) []jen.Code {
	out := []jen.Code{
		addTypeParams(jen.Type().Id(d.Name), d.TypeParams).Interface(jen.Id("is" + d.Name).Params()),
	}
	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		fields := make([]jen.Code, 0, len(v.Fields))
		for i, f := range v.Fields {
			fields = append(fields, jen.Id("F"+strconv.Itoa(i)).Add(jenTypeExpr(f.Type)))
		}
		// Build receiver as: _ VariantName[TypeParams...]
		recvStmt := jen.Id("_").Id(tname)
		if len(d.TypeParams) > 0 {
			recvStmt = bracketArgs(recvStmt, genJenIds(d.TypeParams))
		}
		// Build constructor function: func VariantName[TypeParams...](a0 T0, ...) EnumType[TypeParams...]
		ctorParams := make([]jen.Code, 0, len(v.Fields))
		for i, f := range v.Fields {
			ctorParams = append(ctorParams, jen.Id("a"+strconv.Itoa(i)).Add(jenTypeExpr(f.Type)))
		}
		// Return type: EnumName[TypeParams...]
		ctorRet := jen.Id(d.Name)
		if len(d.TypeParams) > 0 {
			ctorRet = bracketArgs(ctorRet, genJenIds(d.TypeParams))
		}
		// Build body: return VariantName[TypeParams]{F0: a0, F1: a1, ...}
		litDict := jen.Dict{}
		for i := range v.Fields {
			litDict[jen.Id("F"+strconv.Itoa(i))] = jen.Id("a" + strconv.Itoa(i))
		}
		structLit := jen.Id(tname)
		if len(d.TypeParams) > 0 {
			structLit = bracketArgs(structLit, genJenIds(d.TypeParams))
		}
		ctorBody := jen.Return(structLit.Values(litDict))
		out = append(out,
			addTypeParams(jen.Type().Id(tname), d.TypeParams).Struct(fields...),
			jen.Func().Params(recvStmt).Id("is"+d.Name).Params().Block(),
			addTypeParams(jen.Func().Id(v.Name), d.TypeParams).Params(ctorParams...).Add(ctorRet).Block(ctorBody),
		)
	}
	return out
}

func (g *generator) genStruct(d *StructDecl) []jen.Code {
	fields := make([]jen.Code, 0, len(d.Fields))
	for _, f := range d.Fields {
		tagMap := parseStructTag(f.Tag)
		var field *jen.Statement
		if f.Name == "embed" {
			field = jenTypeExpr(f.Type).(*jen.Statement)
		} else {
			field = jen.Id(exportName(f.Name)).Add(jenTypeExpr(f.Type))
		}
		if len(tagMap) > 0 {
			field = field.Tag(tagMap)
		}
		fields = append(fields, field)
	}
	return []jen.Code{addTypeParams(jen.Type().Id(d.Name), d.TypeParams).Struct(fields...)}
}

func parseStructTag(tag string) map[string]string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}
	fields := strings.Fields(tag)
	out := make(map[string]string, len(fields))
	for _, field := range fields {
		parts := strings.SplitN(field, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		raw := strings.TrimSpace(parts[1])
		val, err := strconv.Unquote(raw)
		if err != nil {
			val = raw
		}
		if key != "" {
			out[key] = val
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (g *generator) genInterface(d *InterfaceDecl) []jen.Code {
	hktSet := g.hktParams(d)
	methods := make([]jen.Code, 0, len(d.Methods))
	for _, m := range d.Methods {
		if len(m.TypeParams) > 0 {
			continue
		}
		params := make([]jen.Code, 0, len(m.Params))
		for _, p := range m.Params {
			params = append(params, jen.Id(p.Name).Add(jenHKTTypeExpr(p.Type, hktSet)))
		}
		methods = append(methods, jen.Id(m.Name).Params(params...).Add(jenHKTTypeExpr(m.Ret, hktSet)))
	}
	return []jen.Code{addTypeParams(jen.Type().Id(d.Name), d.TypeParams).Interface(methods...)}
}

func (g *generator) genImpl(d *ImplDecl) ([]jen.Code, error) {
	ifaceName := d.InterfaceName
	if ifaceName == "" {
		ifaceName = d.Name
	}
	iface := g.pkg.Interfaces[ifaceName]
	if iface == nil {
		return nil, common.ErrorAtPos(d.Line, d.Column, "impl %s: missing interface declaration", ifaceName)
	}
	typeArgs := d.InterfaceArgs
	if len(typeArgs) == 0 {
		typeArgs = d.TypeArgs
	}
	if len(iface.TypeParams) != len(typeArgs) {
		return nil, common.ErrorAtPos(d.Line, d.Column, "impl %s: type arity mismatch", ifaceName)
	}
	subst := map[string]string{}
	for i, tp := range iface.TypeParams {
		subst[tp] = g.goType(typeArgs[i], nil)
	}
	typeKey := g.implTypeKey(typeArgs)
	methodBodies := map[string]*FuncDecl{}
	for _, m := range d.Methods {
		methodBodies[m.Name] = m
	}

	var allCode []jen.Code
	for _, sig := range iface.Methods {
		method := methodBodies[sig.Name]
		bodyExpr := sig.Body
		params := sig.Params
		ret := sig.Ret
		if method != nil {
			bodyExpr = method.Body
			params = method.Params
			ret = method.Ret
		}
		combinedTypeParams := typeParamSet(d.TypeParams)
		for tp := range typeParamSet(sig.TypeParams) {
			combinedTypeParams[tp] = struct{}{}
		}
		// Skip interface methods that have no impl body.
		if method == nil {
			continue
		}
		hktSet := g.hktParams(iface)
		retType := g.goHKTReturnType(ret, hktSet, combinedTypeParams)
		ctx := &exprCtx{
			locals:          map[string]string{},
			bindings:        map[string]string{},
			sourceTypes:     map[string]string{},
			mutable:         map[string]bool{},
			typeParams:      combinedTypeParams,
			constraintFuncs: map[string]string{},
			retType:         retType,
			currentImpl:     ifaceName,
			implTypeKey:     typeKey,
			implTypeParams:  d.TypeParams,
		}

		fnName := helperFuncName(sig.Name, typeKey)
		// Combined type params: impl-level (d.TypeParams) + method-level (sig.TypeParams)
		mergedTypeParams := make([]string, 0, len(d.TypeParams)+len(sig.TypeParams))
		seen := map[string]bool{}
		for _, tp := range d.TypeParams {
			if !seen[tp] {
				mergedTypeParams = append(mergedTypeParams, tp)
				seen[tp] = true
			}
		}
		for _, tp := range sig.TypeParams {
			if !seen[tp] {
				mergedTypeParams = append(mergedTypeParams, tp)
				seen[tp] = true
			}
		}
		var fn *jen.Statement
		if len(mergedTypeParams) > 0 {
			typeOpts := jen.Options{Open: fnName + "[", Close: "]", Separator: ", "}
			typeItems := make([]jen.Code, 0, len(mergedTypeParams))
			constraints := implTypeParamConstraints(typeArgs)
			for _, tp := range mergedTypeParams {
				constraint := "any"
				if constraints[tp] || (implUsesSet(typeArgs) && containsString(sig.TypeParams, tp)) {
					constraint = "comparable"
				}
				typeItems = append(typeItems, jen.Id(tp).Id(constraint))
			}
			fn = jen.Func().Custom(typeOpts, typeItems...)
		} else {
			fn = jen.Func().Id(fnName)
		}
		paramList := make([]jen.Code, 0, len(params))
		for _, p := range params {
			goType := g.goHKTType(p.Type, hktSet, combinedTypeParams)
			paramList = append(paramList, jen.Id(p.Name).Add(jen.Id(goType)))
			ctx.locals[p.Name] = goType
			ctx.sourceTypes[p.Name] = typeString(p.Type, subst)
			ctx.bindings[p.Name] = p.Name
			ctx.mutable[p.Name] = false
		}
		fn = fn.Params(paramList...)

		if retType != "" {
			fn = fn.Add(jen.Id(retType))
		}

		var bodyStmts []jen.Code
		if bodyExpr == nil {
			if retType == "" {
				bodyStmts = append(bodyStmts, jen.Return())
			} else {
				bodyStmts = append(bodyStmts, jen.Id("panic").Call(jen.Lit("unimplemented")))
			}
		} else {
			expr, _, err := g.translateExpr(bodyExpr, ctx, retType)
			if err != nil {
				bodyStmts = append(bodyStmts, jen.Id("panic").Call(jen.Lit("translate error")))
			} else if retType == "" {
				bodyStmts = append(bodyStmts, expr)
				bodyStmts = append(bodyStmts, jen.Return())
			} else {
				bodyStmts = append(bodyStmts, jen.Return().Add(expr))
			}
		}
		fn = fn.Block(bodyStmts...)
		allCode = append(allCode, fn)
	}
	return allCode, nil
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func implUsesSet(typeArgs []TypeExpr) bool {
	for _, arg := range typeArgs {
		if typeExprContainsName(arg, "Set") {
			return true
		}
	}
	return false
}

func typeExprContainsName(t TypeExpr, name string) bool {
	nt, ok := t.(*NamedType)
	if !ok {
		return false
	}
	if nt.Name == name {
		return true
	}
	for _, arg := range nt.Args {
		if typeExprContainsName(arg, name) {
			return true
		}
	}
	return false
}

func implTypeParamConstraints(typeArgs []TypeExpr) map[string]bool {
	constraints := map[string]bool{}
	for _, arg := range typeArgs {
		markComparableTypeParams(arg, constraints)
	}
	return constraints
}

func markComparableTypeParams(t TypeExpr, constraints map[string]bool) {
	nt, ok := t.(*NamedType)
	if !ok {
		return
	}
	switch nt.Name {
	case "Map":
		if len(nt.Args) > 0 {
			if key, ok := nt.Args[0].(*NamedType); ok && len(key.Args) == 0 {
				constraints[key.Name] = true
			}
		}
	case "Set":
		if len(nt.Args) > 0 {
			if elem, ok := nt.Args[0].(*NamedType); ok && len(elem.Args) == 0 {
				constraints[elem.Name] = true
			}
		}
	}
	for _, arg := range nt.Args {
		markComparableTypeParams(arg, constraints)
	}
}

func (g *generator) genFunc(d *FuncDecl) (jen.Code, error) {
	ctx := &exprCtx{
		locals:           map[string]string{},
		bindings:         map[string]string{},
		sourceTypes:      map[string]string{},
		mutable:          map[string]bool{},
		typeParams:       typeParamSet(d.TypeParams),
		constraintFuncs:  map[string]string{},
		typeclassMethods: map[string][]typeclassBinding{},
		retType:          g.goReturnType(d.Ret, typeParamSet(d.TypeParams)),
		retTypes:         g.goReturnTypes(d.Ret, typeParamSet(d.TypeParams)),
	}
	for _, p := range d.Params {
		goType := g.goType(p.Type, typeParamSet(d.TypeParams))
		ctx.locals[p.Name] = goType
		ctx.sourceTypes[p.Name] = goType
		ctx.bindings[p.Name] = p.Name
		ctx.mutable[p.Name] = false
	}
	type constraintParam struct {
		name string
		typ  string
	}
	constraintParams := map[string]constraintParam{}
	var constraintOrder []string
	for _, c := range d.Using {
		iface := g.pkg.Interfaces[c.Name]
		if iface == nil {
			return nil, common.ErrorAtPos(c.Line, c.Column, "function %s: missing interface %s", d.Name, c.Name)
		}
		if len(iface.TypeParams) != len(c.Args) {
			return nil, common.ErrorAtPos(c.Line, c.Column, "function %s: type arity mismatch for %s", d.Name, c.Name)
		}
		subst := map[string]string{}
		for i, tp := range iface.TypeParams {
			subst[tp] = g.goType(c.Args[i], typeParamSet(d.TypeParams))
		}
		for _, m := range iface.Methods {
			paramName := c.BindName
			if paramName == "" {
				paramName = m.Name + "Fn"
			}
			binding := typeclassBinding{
				Interface: c.Name,
				Score:     typeclassMatchScore(c.Args, typeParamSet(d.TypeParams)),
				ParamTypes: func() []string {
					out := make([]string, 0, len(m.Params))
					for _, p := range m.Params {
						out = append(out, typeString(p.Type, subst))
					}
					return out
				}(),
				RetType: typeStringReturn(m.Ret, subst),
			}
			ctx.typeclassMethods[m.Name] = append(ctx.typeclassMethods[m.Name], binding)
			if _, ok := constraintParams[m.Name]; !ok {
				ctx.constraintFuncs[m.Name] = paramName
				constraintParams[m.Name] = constraintParam{
					name: paramName,
					typ:  typeclassFuncType(binding.ParamTypes, binding.RetType),
				}
				constraintOrder = append(constraintOrder, m.Name)
			} else {
				if betterMatch(binding.Score, typeclassBindingBest(ctx.typeclassMethods[m.Name]).Score) {
					ctx.constraintFuncs[m.Name] = paramName
					constraintParams[m.Name] = constraintParam{name: paramName, typ: typeclassFuncType(binding.ParamTypes, binding.RetType)}
				}
			}
		}
	}
	retType := ""
	retTypes := g.goReturnTypes(d.Ret, typeParamSet(d.TypeParams))
	if len(retTypes) == 1 {
		retType = retTypes[0]
	}
	var fn *jen.Statement
	if len(d.TypeParams) > 0 {
		typeOpts := jen.Options{Open: d.Name + "[", Close: "]", Separator: ", "}
		typeItems := make([]jen.Code, 0, len(d.TypeParams))
		for _, tp := range d.TypeParams {
			typeItems = append(typeItems, jen.Id(tp).Id("any"))
		}
		fn = jen.Func().Custom(typeOpts, typeItems...)
	} else {
		fn = jen.Func().Id(d.Name)
	}
	fn = fn.ParamsFunc(func(gr *jen.Group) {
		for _, p := range d.Params {
			gr.Add(jen.Id(p.Name).Add(jen.Id(g.goType(p.Type, typeParamSet(d.TypeParams)))))
		}
		for _, methodName := range constraintOrder {
			cp := constraintParams[methodName]
			gr.Add(jen.Id(cp.name).Add(jen.Id(cp.typ)))
		}
	})
	if len(retTypes) == 1 {
		fn = fn.Add(jen.Id(retTypes[0]))
	} else if len(retTypes) > 1 {
		items := make([]jen.Code, 0, len(retTypes))
		for _, rt := range retTypes {
			items = append(items, jen.Id(rt))
		}
		fn = fn.Add(jen.Parens(jen.List(items...)))
	}

	// Build function body
	var bodyStmts []jen.Code
	if len(retTypes) == 0 {
		bodyExpr, _, err := g.translateExpr(d.Body, ctx, retType)
		if err != nil {
			return nil, common.ErrorAtPos(d.Line, d.Column, "function %s: %v", d.Name, err)
		}
		bodyStmts = append(bodyStmts, bodyExpr)
		bodyStmts = append(bodyStmts, jen.Return())
	} else if len(retTypes) == 1 {
		bodyExpr, _, err := g.translateExpr(d.Body, ctx, retType)
		if err != nil {
			return nil, common.ErrorAtPos(d.Line, d.Column, "function %s: %v", d.Name, err)
		}
		bodyStmts = append(bodyStmts, jen.Return().Add(bodyExpr))
	} else {
		if tuple, ok := d.Body.(*TupleLitExpr); ok {
			values := make([]jen.Code, 0, len(tuple.Elems))
			for i, elem := range tuple.Elems {
				code, _, err := g.translateExpr(elem, ctx, retTypes[i])
				if err != nil {
					return nil, common.ErrorAtPos(d.Line, d.Column, "function %s: %v", d.Name, err)
				}
				values = append(values, code)
			}
			bodyStmts = append(bodyStmts, jen.Return(values...))
		} else {
			bodyExpr, _, err := g.translateExpr(d.Body, ctx, retType)
			if err != nil {
				return nil, common.ErrorAtPos(d.Line, d.Column, "function %s: %v", d.Name, err)
			}
			bodyStmts = append(bodyStmts, jen.Return(tupleReturnValues(bodyExpr, retTypes)...))
		}
	}
	fn = fn.Block(bodyStmts...)
	return fn, nil
}

func (g *generator) genHelpers() []jen.Code {
	return []jen.Code{
		jen.Func().Id("callAny").
			Params(
				jen.Id("fn").Id("any"),
				jen.Id("args").Op("...").Id("any"),
			).
			Id("any").
			Block(
				jen.Id("values").Op(":=").Make(jen.Index().Qual("reflect", "Value"), jen.Len(jen.Id("args"))),
				jen.For(jen.List(jen.Id("i"), jen.Id("arg")).Op(":=").Range().Id("args")).Block(
					jen.Id("values").Index(jen.Id("i")).Op("=").Qual("reflect", "ValueOf").Call(jen.Id("arg")),
				),
				jen.Id("out").Op(":=").Qual("reflect", "ValueOf").Call(jen.Id("fn")).Dot("Call").Call(jen.Id("values")),
				jen.If(jen.Len(jen.Id("out")).Op("==").Lit(0)).Block(
					jen.Return(jen.Nil()),
				),
				jen.Return(jen.Id("out").Index(jen.Lit(0)).Dot("Interface").Call()),
			),
	}
}

func tupleReturnValues(bodyExpr jen.Code, retTypes []string) []jen.Code {
	if len(retTypes) == 0 {
		return nil
	}
	if len(retTypes) == 1 {
		return []jen.Code{bodyExpr}
	}
	out := make([]jen.Code, 0, len(retTypes))
	stmt, ok := bodyExpr.(*jen.Statement)
	if !ok {
		return []jen.Code{bodyExpr}
	}
	for i := range retTypes {
		out = append(out, stmt.Dot("F"+strconv.Itoa(i)))
	}
	return out
}
