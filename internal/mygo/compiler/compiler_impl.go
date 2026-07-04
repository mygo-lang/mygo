package compiler

import (
	"bytes"
	"sort"
	"strconv"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) genGlobals() (string, error) {
	file := jen.NewFile("")
	ctx := &exprCtx{
		locals:          map[string]string{},
		bindings:        map[string]string{},
		sourceTypes:     map[string]string{},
		mutable:         map[string]bool{},
		typeParams:      map[string]struct{}{},
		constraintFuncs: map[string]string{},
	}
	for _, decl := range g.pkg.Decls {
		s, ok := decl.(*LetStmt)
		if !ok {
			continue
		}
		code, typ, err := g.translateExpr(s.Value, ctx, g.goType(s.Type, nil))
		if err != nil {
			return "", common.ErrorAtPos(s.Line, s.Column, "global binding %q: %v", s.Name, err)
		}
		actual := sanitizeIdent(s.Name)
		if actual == "" || actual == "_" {
			actual = "tmp"
		}
		if s.Name == "_" {
			file.Var().Id("_").Op("=").Add(code)
			continue
		}
		stmt := file.Var().Id(actual)
		if s.Type != nil {
			stmt.Add(jenTypeExpr(s.Type))
		}
		stmt.Op("=").Add(code)
		ctx.bindings[s.Name] = actual
		if typ == "" && s.Type != nil {
			typ = g.goType(s.Type, nil)
		}
		ctx.locals[s.Name] = typ
		ctx.sourceTypes[s.Name] = typ
		ctx.mutable[actual] = s.Mutable
	}
	var out bytes.Buffer
	if err := file.Render(&out); err != nil {
		return "", err
	}
	if out.Len() > 0 {
		out.WriteString("\n")
	}
	out.WriteString(g.genHKTType())
	return out.String(), nil
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

func (g *generator) genHKTType() string {
	needsHKT := false
	for _, iface := range g.pkg.Interfaces {
		hktSet := g.hktParams(iface)
		if len(hktSet) > 0 {
			needsHKT = true
			break
		}
	}
	if !needsHKT {
		return ""
	}
	file := jen.NewFile("")
	file.Type().Id("HKTType").Interface()
	file.Type().Id("HKT1").Index(jen.Id("F").Id("any")).Interface()
	file.Type().Id("HKT2").Index(jen.Id("A").Id("any")).Interface()
	file.Type().Id("HKT").Index(jen.Id("F").Id("any"), jen.Id("A").Id("any")).Interface()
	return file.GoString() + "\n"
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
		jen.Type().Id(d.Name).Interface(jen.Id("is" + d.Name).Params()),
	}
	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		fields := make([]jen.Code, 0, len(v.Fields))
		for i, f := range v.Fields {
			fields = append(fields, jen.Id("F"+strconv.Itoa(i)).Add(jenTypeExpr(f.Type)))
		}
		out = append(out,
			jen.Type().Id(tname).Struct(fields...),
			jen.Func().Params(jen.Id("_").Id(tname).Params()).Id("is"+d.Name).Params(),
		)
	}
	return out
}

func (g *generator) genStruct(d *StructDecl) []jen.Code {
	fields := make([]jen.Code, 0, len(d.Fields))
	for _, f := range d.Fields {
		if f.Name == "embed" {
			fields = append(fields, jenTypeExpr(f.Type))
			continue
		}
		fields = append(fields, jen.Id(exportName(f.Name)).Add(jenTypeExpr(f.Type)))
	}
	return []jen.Code{jen.Type().Id(d.Name).Struct(fields...)}
}

func (g *generator) genInterface(d *InterfaceDecl) []jen.Code {
	methods := make([]jen.Code, 0, len(d.Methods))
	for _, m := range d.Methods {
		if len(m.TypeParams) > 0 {
			continue
		}
		params := make([]jen.Code, 0, len(m.Params))
		for _, p := range m.Params {
			params = append(params, jen.Id(p.Name).Add(jenTypeExpr(p.Type)))
		}
		methods = append(methods, jen.Id(m.Name).Params(params...).Add(jenTypeExpr(m.Ret)))
	}
	return []jen.Code{jen.Type().Id(d.Name).Interface(methods...)}
}

func (g *generator) genImpl(d *ImplDecl) (string, error) {
	ifaceName := d.InterfaceName
	if ifaceName == "" {
		ifaceName = d.Name
	}
	iface := g.pkg.Interfaces[ifaceName]
	if iface == nil {
		return "", common.ErrorAtPos(d.Line, d.Column, "impl %s: missing interface declaration", ifaceName)
	}
	typeArgs := d.InterfaceArgs
	if len(typeArgs) == 0 {
		typeArgs = d.TypeArgs
	}
	if len(iface.TypeParams) != len(typeArgs) {
		return "", common.ErrorAtPos(d.Line, d.Column, "impl %s: type arity mismatch", ifaceName)
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
		}

		fn := jen.Func().Id(helperFuncName(sig.Name, typeKey))
		if len(sig.TypeParams) > 0 {
			fn = fn.IndexFunc(func(gg *jen.Group) {
				for i, tp := range sig.TypeParams {
					if i > 0 {
						gg.Add(jen.Op(","))
					}
					gg.Add(jen.Id(tp), jen.Id("any"))
				}
			})
		}
		paramList := make([]jen.Code, 0, len(params))
		for _, p := range params {
			if len(paramList) > 0 {
				paramList = append(paramList, jen.Op(","))
			}
			goType := g.goHKTType(p.Type, hktSet, combinedTypeParams)
			paramList = append(paramList, jen.Id(p.Name), jen.Id(goType))
			ctx.locals[p.Name] = goType
			ctx.sourceTypes[p.Name] = typeString(p.Type, subst)
			ctx.bindings[p.Name] = p.Name
			ctx.mutable[p.Name] = false
		}
		fn = fn.Params(paramList...)

		if retType != "" {
			fn = fn.Add(jen.Id(retType))
		}

		bodyBlock := jen.BlockFunc(func(b *jen.Group) {
			if bodyExpr == nil {
				if retType == "" {
					b.Add(jen.Return())
				} else {
					b.Add(jen.Id("panic").Call(jen.Lit("unimplemented")))
				}
				return
			}
			expr, _, err := g.translateExpr(bodyExpr, ctx, retType)
			if err != nil {
				return
			}
			unitCode := codeString(expr)
			if retType == "" {
				if unitCode != "" {
					b.Add(jen.Id(unitCode))
				}
				b.Add(jen.Return())
			} else {
				b.Add(jen.Return(jen.Id(unitCode)))
			}
		})
		fn = fn.Block(bodyBlock)
		allCode = append(allCode, fn)
	}

	// Render all code into a single file
	file := jen.NewFile("")
	for _, c := range allCode {
		file.Add(c)
		file.Line()
	}
	var out bytes.Buffer
	if err := file.Render(&out); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (g *generator) genFunc(d *FuncDecl) (string, error) {
	ctx := &exprCtx{
		locals:           map[string]string{},
		bindings:         map[string]string{},
		sourceTypes:      map[string]string{},
		mutable:          map[string]bool{},
		typeParams:       typeParamSet(d.TypeParams),
		constraintFuncs:  map[string]string{},
		typeclassMethods: map[string][]typeclassBinding{},
		retType:          g.goReturnType(d.Ret, typeParamSet(d.TypeParams)),
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
			return "", common.ErrorAtPos(c.Line, c.Column, "function %s: missing interface %s", d.Name, c.Name)
		}
		if len(iface.TypeParams) != len(c.Args) {
			return "", common.ErrorAtPos(c.Line, c.Column, "function %s: type arity mismatch for %s", d.Name, c.Name)
		}
		subst := map[string]string{}
		for i, tp := range iface.TypeParams {
			subst[tp] = g.goType(c.Args[i], typeParamSet(d.TypeParams))
		}
		for _, m := range iface.Methods {
			paramName := m.Name + "Fn"
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
	retType := g.goReturnType(d.Ret, typeParamSet(d.TypeParams))
	fn := jen.Func().Id(d.Name)
	if len(d.TypeParams) > 0 {
		fn = fn.IndexFunc(func(gg *jen.Group) {
			for i, tp := range d.TypeParams {
				if i > 0 {
					gg.Add(jen.Op(","))
				}
				gg.Add(jen.Id(tp), jen.Id("any"))
			}
		})
	}
	fn = fn.ParamsFunc(func(gr *jen.Group) {
		first := true
		for _, p := range d.Params {
			if !first {
				gr.Add(jen.Op(","))
			}
			first = false
			gr.Add(jen.Id(p.Name), jen.Id(g.goType(p.Type, typeParamSet(d.TypeParams))))
		}
		for _, methodName := range constraintOrder {
			cp := constraintParams[methodName]
			if !first {
				gr.Add(jen.Op(","))
			}
			first = false
			gr.Add(jen.Id(cp.name), jen.Id(cp.typ))
		}
	})
	if retType != "" {
		fn = fn.Add(jen.Id(retType))
	}

	expr, _, err := g.translateExpr(d.Body, ctx, retType)
	if err != nil {
		return "", err
	}

	// Build function body
	bodyBlock := jen.BlockFunc(func(b *jen.Group) {
		if retType == "" {
			// Unit body: write expression without return
			unitCode := codeString(expr)
			if unitCode != "" {
				b.Add(jen.Id(unitCode))
			}
			b.Add(jen.Return())
		} else {
			b.Add(jen.Return(jen.Id(codeString(expr))))
		}
	})
	fn = fn.Block(bodyBlock)

	var out bytes.Buffer
	if err := fn.Render(&out); err != nil {
		return "", err
	}
	return out.String() + "\n", nil
}

func (g *generator) genHelpers() string {
	file := jen.NewFile("")
	file.Func().Id("callAny").
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
		)
	return file.GoString() + "\n"
}
