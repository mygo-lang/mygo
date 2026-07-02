package compiler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) genGlobals() (string, error) {
	var b strings.Builder
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
			b.WriteString("var _ = ")
			b.WriteString(code)
			b.WriteString("\n")
			continue
		}
		b.WriteString("var ")
		b.WriteString(actual)
		if s.Type != nil {
			b.WriteString(" ")
			b.WriteString(g.goType(s.Type, nil))
		}
		b.WriteString(" = ")
		b.WriteString(code)
		b.WriteString("\n")
		ctx.bindings[s.Name] = actual
		if typ == "" && s.Type != nil {
			typ = g.goType(s.Type, nil)
		}
		ctx.locals[s.Name] = typ
		ctx.sourceTypes[s.Name] = typ
		ctx.mutable[actual] = s.Mutable
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(g.genTypeclassDispatchers())
	return b.String(), nil
}

func (g *generator) genTypeclassDispatchers() string {
	var b strings.Builder
	for _, ifaceName := range g.sortedTypeclassNames() {
		iface := g.pkg.Interfaces[ifaceName]
		for _, m := range iface.Methods {
			retType := typeStringReturn(m.Ret, nil)
			b.WriteString("var ")
			b.WriteString(dispatchRegistryName(ifaceName, m.Name))
			b.WriteString(" = map[string]func(")
			for i := range m.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString("any")
			}
			b.WriteString(")")
			if retType != "" {
				b.WriteString(" ")
				b.WriteString(retType)
			}
			b.WriteString("{}\n")
			b.WriteString("func ")
			b.WriteString(dispatchFuncName(ifaceName, m.Name))
			b.WriteString("(")
			for i, p := range m.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(p.Name)
				b.WriteString(" any")
			}
			b.WriteString(")")
			if retType != "" {
				b.WriteString(" ")
				b.WriteString(retType)
			}
			b.WriteString(" {\n")
			b.WriteString("\tkey := ")
			b.WriteString(dispatchKeyExpr(m.Params, nil))
			b.WriteString("\n")
			b.WriteString("\tif fn, ok := ")
			b.WriteString(dispatchRegistryName(ifaceName, m.Name))
			b.WriteString("[key]; ok {\n")
			if retType != "" {
				b.WriteString("\t\treturn fn(")
				for i, p := range m.Params {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(p.Name)
				}
				b.WriteString(")\n\t}\n")
			} else {
				b.WriteString("\t\tfn(")
				for i, p := range m.Params {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(p.Name)
				}
				b.WriteString(")\n\t\treturn\n\t}\n")
			}
			b.WriteString("\tpanic(\"missing typeclass implementation\")\n")
			if retType == "" {
				b.WriteString("\treturn\n")
			}
			b.WriteString("}\n\n")
		}
	}
	return b.String()
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

func (g *generator) genEnum(d *EnumDecl) string {
	var b strings.Builder
	typeParams := genTypeParams(d.TypeParams)
	typeArgs := typeArgList(d.TypeParams)
	b.WriteString("type ")
	b.WriteString(d.Name)
	b.WriteString(typeParams)
	b.WriteString(" interface{ is")
	b.WriteString(d.Name)
	b.WriteString("() }\n")
	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		b.WriteString("type ")
		b.WriteString(tname)
		b.WriteString(typeParams)
		b.WriteString(" struct {\n")
		for i := range v.Fields {
			b.WriteString("\tF")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(" ")
			b.WriteString(g.goType(v.Fields[i].Type, typeParamSet(d.TypeParams)))
			b.WriteString("\n")
		}
		b.WriteString("}\n")
		b.WriteString("func (")
		b.WriteString(tname)
		b.WriteString(typeArgList(d.TypeParams))
		b.WriteString(") is")
		b.WriteString(d.Name)
		b.WriteString("() {}\n")
	}
	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		b.WriteString("func ")
		b.WriteString(v.Name)
		b.WriteString(typeParams)
		b.WriteString("(")
		if len(v.Fields) > 0 {
			args := make([]string, 0, len(v.Fields))
			for i, f := range v.Fields {
				args = append(args, fmt.Sprintf("a%d %s", i, g.goType(f.Type, typeParamSet(d.TypeParams))))
			}
			b.WriteString(strings.Join(args, ", "))
		}
		b.WriteString(") ")
		b.WriteString(d.Name)
		b.WriteString(typeArgs)
		b.WriteString(" {\n")
		b.WriteString("\treturn ")
		b.WriteString(tname)
		b.WriteString(typeArgs)
		b.WriteString("{")
		for i := range v.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("F%d: a%d", i, i))
		}
		b.WriteString("}\n}\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (g *generator) genStruct(d *StructDecl) string {
	var b strings.Builder
	b.WriteString("type ")
	b.WriteString(d.Name)
	b.WriteString(genTypeParams(d.TypeParams))
	b.WriteString(" struct {\n")
	for _, f := range d.Fields {
		b.WriteString("\t")
		if f.Name == "embed" {
			b.WriteString(g.goType(f.Type, typeParamSet(d.TypeParams)))
			b.WriteString("\n")
			continue
		}
		b.WriteString(exportName(f.Name))
		b.WriteString(" ")
		b.WriteString(g.goType(f.Type, typeParamSet(d.TypeParams)))
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	return b.String()
}

func (g *generator) genInterface(d *InterfaceDecl) string {
	var b strings.Builder
	b.WriteString("type ")
	b.WriteString(d.Name)
	b.WriteString(genTypeParams(d.TypeParams))
	b.WriteString(" interface {\n")
	for _, m := range d.Methods {
		b.WriteString("\t")
		b.WriteString(m.Name)
		b.WriteString("(")
		for i, p := range m.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
			b.WriteString(" ")
			b.WriteString(g.goType(p.Type, typeParamSet(d.TypeParams)))
		}
		b.WriteString(") ")
		b.WriteString(g.goReturnType(m.Ret, typeParamSet(d.TypeParams)))
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	return b.String()
}

func (g *generator) genImpl(d *ImplDecl) (string, error) {
	iface := g.pkg.Interfaces[d.Name]
	if iface == nil {
		return "", common.ErrorAtPos(d.Line, d.Column, "impl %s: missing interface declaration", d.Name)
	}
	if len(iface.TypeParams) != len(d.TypeArgs) {
		return "", common.ErrorAtPos(d.Line, d.Column, "impl %s: type arity mismatch", d.Name)
	}
	subst := map[string]string{}
	for i, tp := range iface.TypeParams {
		subst[tp] = g.goType(d.TypeArgs[i], nil)
	}
	typeKey := g.implTypeKey(d.TypeArgs)
	var b strings.Builder
	methodBodies := map[string]*FuncDecl{}
	for _, m := range d.Methods {
		methodBodies[m.Name] = m
	}
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
		ctx := &exprCtx{
			locals:          map[string]string{},
			bindings:        map[string]string{},
			sourceTypes:     map[string]string{},
			mutable:         map[string]bool{},
			typeParams:      map[string]struct{}{},
			constraintFuncs: map[string]string{},
			retType:         typeStringReturn(ret, subst),
			currentImpl:     d.Name,
		}
		b.WriteString("func ")
		b.WriteString(helperFuncName(sig.Name, typeKey))
		b.WriteString("(")
		for i, p := range params {
			if i > 0 {
				b.WriteString(", ")
			}
			goType := typeString(p.Type, subst)
			b.WriteString(p.Name)
			b.WriteString(" ")
			b.WriteString(goType)
			ctx.locals[p.Name] = goType
			ctx.sourceTypes[p.Name] = typeString(p.Type, subst)
			ctx.bindings[p.Name] = p.Name
			ctx.mutable[p.Name] = false
		}
		retType := typeStringReturn(ret, subst)
		b.WriteString(") ")
		b.WriteString(retType)
		b.WriteString(" {\n")
		if bodyExpr == nil {
			if retType == "" {
				b.WriteString("\treturn\n")
			} else {
				b.WriteString("\tpanic(\"unimplemented\")\n")
			}
		} else {
			expr, exprType, err := g.translateExpr(bodyExpr, ctx, retType)
			if err != nil {
				return "", err
			}
			if retType == "" {
				g.writeUnitBody(&b, expr, exprType)
			} else {
				b.WriteString("\treturn ")
				b.WriteString(expr)
				b.WriteString("\n")
			}
		}
		b.WriteString("}\n")
		b.WriteString("func init() {\n")
		b.WriteString("\t")
		b.WriteString(dispatchRegistryName(d.Name, sig.Name))
		b.WriteString("[")
		b.WriteString(strconv.Quote(g.implDispatchKey(sig.Params, subst)))
		b.WriteString("] = func(")
		for i, p := range sig.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
			b.WriteString(" any")
		}
		b.WriteString(")")
		if retType != "" {
			b.WriteString(" ")
			b.WriteString(retType)
		}
		b.WriteString(" {\n")
		for i, p := range sig.Params {
			b.WriteString("\t\t")
			b.WriteString(p.Name)
			b.WriteString("Typed := ")
			b.WriteString(p.Name)
			b.WriteString(".(")
			b.WriteString(typeString(p.Type, subst))
			b.WriteString(")\n")
			_ = i
		}
		if retType != "" {
			b.WriteString("\t\treturn ")
		} else {
			b.WriteString("\t\t")
		}
		b.WriteString(helperFuncName(sig.Name, typeKey))
		b.WriteString("(")
		for i, p := range sig.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
			b.WriteString("Typed")
		}
		b.WriteString(")\n")
		if retType == "" {
			b.WriteString("\t\treturn\n")
		}
		b.WriteString("\t}\n")
		b.WriteString("}\n")
	}
	b.WriteString("\n")
	return b.String(), nil
}

func (g *generator) genFunc(d *FuncDecl) (string, error) {
	var b strings.Builder
	b.WriteString("func ")
	b.WriteString(d.Name)
	b.WriteString(genTypeParams(d.TypeParams))
	b.WriteString("(")
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
	for i, p := range d.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		goType := g.goType(p.Type, typeParamSet(d.TypeParams))
		b.WriteString(p.Name)
		b.WriteString(" ")
		b.WriteString(goType)
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
	for _, c := range d.Where {
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
	for _, methodName := range constraintOrder {
		cp := constraintParams[methodName]
		b.WriteString(", ")
		b.WriteString(cp.name)
		b.WriteString(" ")
		b.WriteString(cp.typ)
	}
	retType := g.goReturnType(d.Ret, typeParamSet(d.TypeParams))
	b.WriteString(")")
	if retType != "" {
		b.WriteString(" ")
		b.WriteString(retType)
	}
	b.WriteString(" {\n")
	expr, exprType, err := g.translateExpr(d.Body, ctx, retType)
	if err != nil {
		return "", err
	}
	if retType == "" {
		g.writeUnitBody(&b, expr, exprType)
	} else {
		b.WriteString("\treturn ")
		b.WriteString(expr)
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	return b.String(), nil
}

func (g *generator) genHelpers() string {
	return `
func callAny(fn any, args ...any) any {
	values := make([]reflect.Value, len(args))
	for i, arg := range args {
		values[i] = reflect.ValueOf(arg)
	}
	out := reflect.ValueOf(fn).Call(values)
	if len(out) == 0 {
		return nil
	}
	return out[0].Interface()
}

`
}
