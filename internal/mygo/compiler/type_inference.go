package compiler

import (
	"go/types"
	"strconv"
	"strings"
	"unicode"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func (g *generator) findGoMethodSig(baseType, name string) (*goFuncSig, bool) {
	baseType = strings.TrimSpace(baseType)
	if strings.HasPrefix(baseType, "Ref[") && strings.HasSuffix(baseType, "]") {
		baseType = "*" + strings.TrimSuffix(strings.TrimPrefix(baseType, "Ref["), "]")
	}
	for _, imp := range g.pkg.ImportDecls {
		if !strings.HasPrefix(imp.Path, "go:") {
			continue
		}
		sigs, err := g.goPackageSigsFor(importPathForGo(imp.Path))
		if err != nil || sigs == nil {
			continue
		}
		if sig, ok := sigs.methods[baseType][name]; ok {
			return sig, true
		}
		if strings.HasPrefix(baseType, "*") {
			if sig, ok := sigs.methods[strings.TrimPrefix(baseType, "*")][name]; ok {
				return sig, true
			}
		} else {
			if sig, ok := sigs.methods["*"+baseType][name]; ok {
				return sig, true
			}
		}
	}
	return nil, false
}

func (g *generator) goTypeCompatible(expected, actual string) bool {
	if strings.TrimSpace(expected) == "any" {
		return true
	}
	expectedType, ok := goTypeFromString(expected)
	if !ok {
		if strings.HasPrefix(strings.TrimSpace(actual), "Ref[") && strings.HasSuffix(strings.TrimSpace(actual), "]") {
			inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(actual), "Ref["), "]")
			expectedNorm := normalizeGoTypeName(expected)
			if expectedNorm == inner {
				return true
			}
			if strings.HasPrefix(strings.TrimSpace(expected), "*") {
				if normalizeGoTypeName(expected[1:]) == inner {
					return true
				}
				if resolved, ok := g.resolveGoNamedType(strings.TrimSpace(expected[1:])); ok {
					if namedResolved, ok := g.resolveGoNamedType(inner); ok {
						ptrResolved := types.NewPointer(resolved)
						return types.Identical(namedResolved, resolved) || types.Identical(namedResolved.Underlying(), resolved.Underlying()) || types.AssignableTo(namedResolved, ptrResolved)
					}
				}
			}
			if resolved, ok := g.resolveGoNamedType(expected); ok {
				if namedResolved, ok := g.resolveGoNamedType(inner); ok {
					return types.AssignableTo(namedResolved, resolved) || types.Identical(namedResolved, resolved) || types.Identical(namedResolved.Underlying(), resolved.Underlying())
				}
			}
		}
		if strings.HasPrefix(strings.TrimSpace(expected), "*") {
			if resolved, ok := g.resolveGoNamedType(strings.TrimSpace(expected[1:])); ok {
				if actualType, ok := goTypeFromString(actual); ok {
					ptrResolved := types.NewPointer(resolved)
					return types.AssignableTo(actualType, ptrResolved) || types.Identical(actualType, ptrResolved) || types.Identical(actualType.Underlying(), ptrResolved.Underlying())
				}
			}
		}
		if resolved, ok := g.resolveGoNamedType(expected); ok {
			if actualType, ok := goTypeFromString(actual); ok {
				return types.AssignableTo(actualType, resolved) || types.Identical(actualType, resolved) || types.Identical(actualType.Underlying(), resolved.Underlying())
			}
		}
		if actualType, ok := goTypeFromString(actual); ok {
			if basicExpected, ok := goNamedUnderlyingBasic(expected); ok {
				return types.Identical(actualType.Underlying(), basicExpected)
			}
		}
		return strings.TrimSpace(expected) == strings.TrimSpace(actual)
	}
	actualType, ok := goTypeFromString(actual)
	if !ok {
		return false
	}
	if types.Identical(expectedType, actualType) || types.AssignableTo(actualType, expectedType) || types.Identical(actualType.Underlying(), expectedType.Underlying()) {
		return true
	}
	if isAnyType(expectedType) {
		return true
	}
	return false
}

func (g *generator) resolveGoNamedType(name string) (types.Type, bool) {
	name = normalizeGoTypeName(name)
	if name == "" || strings.Contains(name, "[") || strings.Contains(name, "(") {
		return nil, false
	}
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return nil, false
	}
	pkgName, typeName := parts[0], parts[1]
	for _, imp := range g.pkg.ImportDecls {
		if !strings.HasPrefix(imp.Path, "go:") {
			continue
		}
		sigs, err := g.goPackageSigsFor(importPathForGo(imp.Path))
		if err != nil || sigs == nil || sigs.pkg == nil || sigs.pkg.Name() != pkgName {
			continue
		}
		if obj := sigs.pkg.Scope().Lookup(typeName); obj != nil {
			return obj.Type(), true
		}
	}
	return nil, false
}

func goTypeFromString(s string) (types.Type, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	switch s {
	case "any":
		return types.NewInterfaceType(nil, nil), true
	case "int":
		return types.Typ[types.Int], true
	case "int64":
		return types.Typ[types.Int64], true
	case "int32":
		return types.Typ[types.Int32], true
	case "int16":
		return types.Typ[types.Int16], true
	case "int8":
		return types.Typ[types.Int8], true
	case "uint":
		return types.Typ[types.Uint], true
	case "uint64":
		return types.Typ[types.Uint64], true
	case "uint32":
		return types.Typ[types.Uint32], true
	case "uint16":
		return types.Typ[types.Uint16], true
	case "uint8":
		return types.Typ[types.Uint8], true
	case "float64":
		return types.Typ[types.Float64], true
	case "float32":
		return types.Typ[types.Float32], true
	case "string":
		return types.Typ[types.String], true
	case "bool":
		return types.Typ[types.Bool], true
	case "error":
		return types.Universe.Lookup("error").Type(), true
	}
	if strings.HasPrefix(s, "*") {
		if elem, ok := goTypeFromString(s[1:]); ok {
			return types.NewPointer(elem), true
		}
		return nil, false
	}
	if strings.HasPrefix(s, "[]") {
		if elem, ok := goTypeFromString(s[2:]); ok {
			return types.NewSlice(elem), true
		}
		return nil, false
	}
	return nil, false
}

func goNamedUnderlyingBasic(name string) (types.Type, bool) {
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) != 2 {
		return nil, false
	}
	switch parts[1] {
	case "Month", "Weekday", "Mode", "Flag", "Status", "Side":
		return types.Typ[types.Int], true
	case "Duration":
		return types.Typ[types.Int64], true
	case "Byte":
		return types.Typ[types.Uint8], true
	case "Rune":
		return types.Typ[types.Int32], true
	}
	return nil, false
}

func normalizeGoTypeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	if strings.HasPrefix(name, "*") {
		return "*" + normalizeGoTypeName(name[1:])
	}
	if strings.HasPrefix(name, "Ref[") && strings.HasSuffix(name, "]") {
		return "Ref[" + normalizeGoTypeName(strings.TrimSuffix(strings.TrimPrefix(name, "Ref["), "]")) + "]"
	}
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func isGoErrorType(t string) bool {
	t = strings.TrimSpace(t)
	return t == "error" || strings.HasSuffix(t, ".Error")
}

func (g *generator) hmExprType(e Expr) string {
	if g == nil || g.typedInfo == nil || e == nil {
		return ""
	}
	t, ok := g.typedInfo.ExprTypes[e]
	if !ok {
		return ""
	}
	return hmTypeString(t)
}

func hmTypeString(t typeinference.MonoType) string {
	switch t := t.(type) {
	case typeinference.TVar:
		return "any"
	case typeinference.TCon:
		switch t.Name {
		case "Tuple":
			parts := make([]string, 0, len(t.Args))
			for i, a := range t.Args {
				parts = append(parts, "F"+strconv.Itoa(i)+" "+hmTypeString(a))
			}
			return "struct { " + strings.Join(parts, "; ") + " }"
		case "Int":
			return "int"
		case "Int64":
			return "int64"
		case "Int32":
			return "int32"
		case "Int16":
			return "int16"
		case "Int8":
			return "int8"
		case "UInt":
			return "uint"
		case "UInt64":
			return "uint64"
		case "UInt32":
			return "uint32"
		case "UInt16":
			return "uint16"
		case "UInt8":
			return "uint8"
		case "Float64":
			return "float64"
		case "Float32":
			return "float32"
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Any", "any", "interface{}":
			return "any"
		case "Unit":
			return ""
		case "Ref":
			if len(t.Args) == 1 {
				return "*" + hmTypeString(t.Args[0])
			}
		case "Slice":
			if len(t.Args) == 1 {
				return "[]" + hmTypeString(t.Args[0])
			}
		case "Map":
			if len(t.Args) == 2 {
				return "map[" + hmTypeString(t.Args[0]) + "]" + hmTypeString(t.Args[1])
			}
		case "Set":
			if len(t.Args) == 1 {
				return "map[" + hmTypeString(t.Args[0]) + "]struct{}"
			}
		}
		if len(t.Args) == 0 {
			return t.Name
		}
		args := make([]string, 0, len(t.Args))
		for _, a := range t.Args {
			args = append(args, hmTypeString(a))
		}
		return t.Name + "[" + strings.Join(args, ", ") + "]"
	case typeinference.TFunc:
		args := make([]string, 0, len(t.Args))
		for i, a := range t.Args {
			arg := hmTypeString(a)
			if arg == "" {
				arg = "struct{}"
			}
			if t.Variadic && i == len(t.Args)-1 {
				arg = "..." + arg
			}
			args = append(args, arg)
		}
		ret := hmTypeString(t.Ret)
		if ret == "" {
			return "func(" + strings.Join(args, ", ") + ")"
		}
		return "func(" + strings.Join(args, ", ") + ") " + ret
	case typeinference.TUnit:
		return ""
	case typeinference.TGoPackage:
		return ""
	default:
		return ""
	}
}

func (g *generator) lookupFieldType(baseType, field string) string {
	baseType = strings.TrimSpace(baseType)
	if strings.HasPrefix(baseType, "*") {
		return g.lookupFieldType(strings.TrimPrefix(baseType, "*"), field)
	}
	base, args := splitTypeArgs(baseType)
	st := g.pkg.Structs[base]
	if st == nil {
		return ""
	}
	subst := map[string]string{}
	for i, tp := range st.TypeParams {
		if i < len(args) {
			subst[tp] = args[i]
		}
	}
	for _, f := range st.Fields {
		if f.Name == field {
			return typeString(f.Type, subst)
		}
	}
	for _, f := range st.Fields {
		if f.Name != "embed" {
			continue
		}
		embeddedType := typeString(f.Type, subst)
		if t := g.lookupFieldType(embeddedType, field); t != "" {
			return t
		}
	}
	return ""
}

func (g *generator) goType(t TypeExpr, typeParams map[string]struct{}) string {
	switch tt := t.(type) {
	case *NamedType:
		if typeParams != nil {
			if _, ok := typeParams[tt.Name]; ok && len(tt.Args) == 0 {
				return tt.Name
			}
		}
		if tt.Name == "Tuple" {
			parts := make([]string, 0, len(tt.Args))
			for i, a := range tt.Args {
				parts = append(parts, "F"+strconv.Itoa(i)+" "+g.goType(a, typeParams))
			}
			return "struct { " + strings.Join(parts, "; ") + " }"
		}
		switch tt.Name {
		case "Int":
			return "int"
		case "Int64":
			return "int64"
		case "Int32":
			return "int32"
		case "Int16":
			return "int16"
		case "Int8":
			return "int8"
		case "UInt":
			return "uint"
		case "UInt64":
			return "uint64"
		case "UInt32":
			return "uint32"
		case "UInt16":
			return "uint16"
		case "UInt8":
			return "uint8"
		case "Float64":
			return "float64"
		case "Float32":
			return "float32"
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Any":
			return "any"
		case "Unit":
			return "struct{}"
		case "Ref":
			if len(tt.Args) == 1 {
				return "*" + g.goType(tt.Args[0], typeParams)
			}
		case "Slice":
			if len(tt.Args) == 1 {
				return "[]" + g.goType(tt.Args[0], typeParams)
			}
		case "Map":
			if len(tt.Args) == 2 {
				return "map[" + g.goType(tt.Args[0], typeParams) + "]" + g.goType(tt.Args[1], typeParams)
			}
		case "Set":
			if len(tt.Args) == 1 {
				return "map[" + g.goType(tt.Args[0], typeParams) + "]struct{}"
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, g.goType(a, typeParams))
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, g.goType(p, typeParams))
		}
		ret := g.goReturnType(tt.Ret, typeParams)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	case *TupleType:
		if len(tt.Elems) == 0 {
			return "struct{}"
		}
		parts := make([]string, 0, len(tt.Elems))
		for i, e := range tt.Elems {
			parts = append(parts, "F"+strconv.Itoa(i)+" "+g.goType(e, typeParams))
		}
		return "struct { " + strings.Join(parts, "; ") + " }"
	default:
		return "any"
	}
}

func (g *generator) goReturnType(t TypeExpr, typeParams map[string]struct{}) string {
	if isUnitType(t) {
		return ""
	}
	return g.goType(t, typeParams)
}

func (g *generator) goReturnTypes(t TypeExpr, typeParams map[string]struct{}) []string {
	if tt, ok := t.(*TupleType); ok {
		out := make([]string, 0, len(tt.Elems))
		for _, e := range tt.Elems {
			out = append(out, g.goType(e, typeParams))
		}
		return out
	}
	if rt := g.goReturnType(t, typeParams); rt != "" {
		return []string{rt}
	}
	return nil
}

func (g *generator) goHKTType(t TypeExpr, hktSet, typeParams map[string]struct{}) string {
	switch tt := t.(type) {
	case *NamedType:
		if hktSet != nil {
			if _, ok := hktSet[tt.Name]; ok && len(tt.Args) > 0 {
				args := make([]string, 0, len(tt.Args))
				for _, a := range tt.Args {
					args = append(args, g.hktArgType(a, hktSet, typeParams))
				}
				return "HKT[" + tt.Name + ", " + strings.Join(args, ", ") + "]"
			}
		}
		return g.goType(tt, typeParams)
	default:
		return g.goType(t, typeParams)
	}
}

func (g *generator) hktArgType(t TypeExpr, hktSet, typeParams map[string]struct{}) string {
	switch tt := t.(type) {
	case *NamedType:
		if typeParams != nil {
			if _, ok := typeParams[tt.Name]; ok && len(tt.Args) == 0 {
				return tt.Name
			}
		}
		if tt.Name == "Tuple" {
			parts := make([]string, 0, len(tt.Args))
			for i, a := range tt.Args {
				parts = append(parts, "F"+strconv.Itoa(i)+" "+g.hktArgType(a, hktSet, typeParams))
			}
			return "struct { " + strings.Join(parts, "; ") + " }"
		}
		if hktSet != nil {
			if _, ok := hktSet[tt.Name]; ok && len(tt.Args) > 0 {
				args := make([]string, 0, len(tt.Args))
				for _, a := range tt.Args {
					args = append(args, g.hktArgType(a, hktSet, typeParams))
				}
				return "HKT[" + tt.Name + ", " + strings.Join(args, ", ") + "]"
			}
		}
		switch tt.Name {
		case "Int":
			return "int"
		case "Int64":
			return "int64"
		case "Int32":
			return "int32"
		case "Int16":
			return "int16"
		case "Int8":
			return "int8"
		case "UInt":
			return "uint"
		case "UInt64":
			return "uint64"
		case "UInt32":
			return "uint32"
		case "UInt16":
			return "uint16"
		case "UInt8":
			return "uint8"
		case "Float64":
			return "float64"
		case "Float32":
			return "float32"
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Any":
			return "any"
		case "Ref":
			if len(tt.Args) == 1 {
				return "*" + g.hktArgType(tt.Args[0], hktSet, typeParams)
			}
		case "Slice":
			if len(tt.Args) == 1 {
				return "[]" + g.hktArgType(tt.Args[0], hktSet, typeParams)
			}
		case "Map":
			if len(tt.Args) == 2 {
				return "map[" + g.hktArgType(tt.Args[0], hktSet, typeParams) + "]" + g.hktArgType(tt.Args[1], hktSet, typeParams)
			}
		case "Set":
			if len(tt.Args) == 1 {
				return "map[" + g.hktArgType(tt.Args[0], hktSet, typeParams) + "]struct{}"
			}
		case "Option":
			if len(tt.Args) == 1 {
				return "Option[" + g.hktArgType(tt.Args[0], hktSet, typeParams) + "]"
			}
		case "Result":
			if len(tt.Args) == 2 {
				return "Result[" + g.hktArgType(tt.Args[0], hktSet, typeParams) + ", " + g.hktArgType(tt.Args[1], hktSet, typeParams) + "]"
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, g.hktArgType(a, hktSet, typeParams))
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, g.hktArgType(p, hktSet, typeParams))
		}
		ret := g.hktArgType(tt.Ret, hktSet, typeParams)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	default:
		return "any"
	}
}

func (g *generator) goHKTReturnType(t TypeExpr, hktSet, typeParams map[string]struct{}) string {
	if isUnitType(t) {
		return ""
	}
	return g.goHKTType(t, hktSet, typeParams)
}

func (g *generator) constraintTypeArgs(args []TypeExpr, typeParams map[string]struct{}) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, g.goType(a, typeParams))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func containsTypeParam(t TypeExpr, typeParams map[string]struct{}) bool {
	switch tt := t.(type) {
	case *NamedType:
		if typeParams != nil {
			if _, ok := typeParams[tt.Name]; ok && len(tt.Args) == 0 {
				return true
			}
		}
		for _, a := range tt.Args {
			if containsTypeParam(a, typeParams) {
				return true
			}
		}
	case *FuncType:
		for _, p := range tt.Params {
			if containsTypeParam(p, typeParams) {
				return true
			}
		}
		return containsTypeParam(tt.Ret, typeParams)
	}
	return false
}

func exprUsesIdent(e Expr, name string) bool {
	switch n := e.(type) {
	case *IdentExpr:
		return n.Name == name
	case *CallExpr:
		if exprUsesIdent(n.Callee, name) {
			return true
		}
		for _, arg := range n.Args {
			if exprUsesIdent(arg, name) {
				return true
			}
		}
	case *StructLitExpr:
		for _, field := range n.Fields {
			if exprUsesIdent(field.Value, name) {
				return true
			}
		}
	case *BinaryExpr:
		return exprUsesIdent(n.Left, name) || exprUsesIdent(n.Right, name)
	case *PrefixExpr:
		return exprUsesIdent(n.Expr, name)
	case *FieldExpr:
		return exprUsesIdent(n.Expr, name)
	case *FuncLitExpr:
		for _, p := range n.Params {
			if p.Name == name {
				return false
			}
		}
		return exprUsesIdent(n.Body, name)
	case *SwitchExpr:
		if exprUsesIdent(n.Target, name) {
			return true
		}
		for _, c := range n.Cases {
			if exprUsesIdent(c.Body, name) {
				return true
			}
		}
	case *IfExpr:
		return exprUsesIdent(n.Cond, name) || exprUsesIdent(n.Then, name) || exprUsesIdent(n.Else, name)
	case *GoExpr:
		for _, op := range n.Operands {
			if exprUsesIdent(op.Value, name) {
				return true
			}
		}
	case *BlockExpr:
		for _, stmt := range n.Stmts {
			switch s := stmt.(type) {
			case *ExprStmt:
				if exprUsesIdent(s.Expr, name) {
					return true
				}
			case *LetStmt:
				if exprUsesIdent(s.Value, name) {
					return true
				}
			case *AssignStmt:
				if exprUsesIdent(s.Value, name) {
					return true
				}
			}
		}
	}
	return false
}

func (g *generator) implTypeKey(args []TypeExpr) string {
	if len(args) == 0 {
		return ""
	}
	var out []string
	for _, a := range args {
		out = append(out, typeKeyFromType(g.goType(a, nil)))
	}
	return "_" + strings.Join(out, "_")
}

func genTypeParams(params []string) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, p+" any")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func typeArgList(params []string) string {
	if len(params) == 0 {
		return ""
	}
	return "[" + strings.Join(params, ", ") + "]"
}

func typeParamSet(params []string) map[string]struct{} {
	m := make(map[string]struct{}, len(params))
	for _, p := range params {
		m[p] = struct{}{}
	}
	return m
}

func exportName(name string) string {
	if name == "" {
		return name
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func toPackageName(name string) string {
	if name == "" {
		return "main"
	}
	return strings.ToLower(sanitizeIdent(name))
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if i == 0 && unicode.IsDigit(r) {
				b.WriteRune('_')
			}
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

func variantGoTypeName(enumName, variant string) string {
	return enumName + variant
}

func (g *generator) paramTypes(params []Param, subst map[string]string) []string {
	out := make([]string, 0, len(params))
	for _, p := range params {
		out = append(out, typeString(p.Type, subst))
	}
	return out
}

func helperFuncName(method, typeKey string) string {
	typeKey = strings.TrimPrefix(typeKey, "_")
	return sanitizeIdent(method + "_" + typeKey)
}

func typeKeyFromType(typ string) string {
	typ = strings.ReplaceAll(typ, "[", "_")
	typ = strings.ReplaceAll(typ, "]", "")
	typ = strings.ReplaceAll(typ, ", ", "_")
	typ = strings.ReplaceAll(typ, ",", "_")
	typ = strings.ReplaceAll(typ, " ", "")
	typ = strings.ReplaceAll(typ, "*", "Ptr")
	typ = strings.ReplaceAll(typ, ".", "_")
	typ = strings.ReplaceAll(typ, "func(", "Func_")
	typ = strings.ReplaceAll(typ, ")", "")
	return sanitizeIdent(strings.ToLower(typ))
}

func splitTypeArgs(typ string) (string, []string) {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return "", nil
	}
	idx := strings.Index(typ, "[")
	if idx < 0 {
		return typ, nil
	}
	end := strings.LastIndex(typ, "]")
	if end < 0 || end < idx {
		return typ, nil
	}
	name := typ[:idx]
	inner := typ[idx+1 : end]
	if inner == "" {
		return name, nil
	}
	return name, splitTopLevel(inner, ',')
}

func splitFuncType(typ string) ([]string, string) {
	typ = strings.TrimSpace(typ)
	if !strings.HasPrefix(typ, "func(") {
		return nil, ""
	}
	start := strings.Index(typ, "(")
	depth := 0
	for i := start; i < len(typ); i++ {
		switch typ[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
			if depth == 0 && typ[i] == ')' {
				params := strings.TrimSpace(typ[start+1 : i])
				ret := strings.TrimSpace(typ[i+1:])
				if params == "" {
					return nil, ret
				}
				return splitTopLevel(params, ','), ret
			}
		}
	}
	return nil, ""
}

func funcReturnType(typ string) string {
	_, ret := splitFuncType(typ)
	return ret
}

func splitTopLevel(s string, sep rune) []string {
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			if depth > 0 {
				depth--
			}
		default:
			if r == sep && depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + len(string(r))
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

func methodReturnType(iface *InterfaceDecl, method string) string {
	for _, m := range iface.Methods {
		if m.Name == method {
			return typeStringReturn(m.Ret, nil)
		}
	}
	return "any"
}

func inferFuncTypeArgs(fn *FuncDecl, argTypes []string, expectedRet string, inScope map[string]struct{}) map[string]string {
	subst := map[string]string{}
	params := map[string]struct{}{}
	for _, tp := range fn.TypeParams {
		params[tp] = struct{}{}
	}
	for i, p := range fn.Params {
		if i >= len(argTypes) {
			break
		}
		unifyType(p.Type, argTypes[i], params, subst)
	}
	if expectedRet != "" {
		unifyType(fn.Ret, expectedRet, params, subst)
	}
	return subst
}

func unifyType(pattern TypeExpr, actual string, params map[string]struct{}, subst map[string]string) {
	if actual == "" || actual == "any" {
		return
	}
	switch p := pattern.(type) {
	case *NamedType:
		if _, ok := params[p.Name]; ok && len(p.Args) == 0 {
			subst[p.Name] = actual
			return
		}
		patternName := primitiveGoName(p.Name)
		if patternName == "" {
			// Slice, Map, Set are lowered to Go builtin container types.
			switch p.Name {
			case "Slice":
				if len(p.Args) == 1 {
					patternName = "[]" + primitiveGoNameOr(p.Args[0])
				}
			case "Map":
				if len(p.Args) == 2 {
					patternName = "map[" + primitiveGoNameOr(p.Args[0]) + "]" + primitiveGoNameOr(p.Args[1])
				}
			case "Set":
				if len(p.Args) == 1 {
					patternName = "map[" + primitiveGoNameOr(p.Args[0]) + "]struct{}"
				}
			}
		}
		if patternName == "" {
			patternName = p.Name
		}
		actualName, actualArgs := splitTypeArgs(actual)
		if patternName != actualName || len(p.Args) != len(actualArgs) {
			return
		}
		for i, arg := range p.Args {
			unifyType(arg, actualArgs[i], params, subst)
		}
	case *FuncType:
		actualParams, actualRet := splitFuncType(actual)
		if len(actualParams) != len(p.Params) {
			return
		}
		for i, param := range p.Params {
			unifyType(param, actualParams[i], params, subst)
		}
		unifyType(p.Ret, actualRet, params, subst)
	}
}

func primitiveGoName(name string) string {
	switch name {
	case "Int":
		return "int"
	case "Int8":
		return "int8"
	case "Int16":
		return "int16"
	case "Int32":
		return "int32"
	case "Int64":
		return "int64"
	case "UInt":
		return "uint"
	case "UInt8":
		return "uint8"
	case "UInt16":
		return "uint16"
	case "UInt32":
		return "uint32"
	case "UInt64":
		return "uint64"
	case "Float32":
		return "float32"
	case "Float64":
		return "float64"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Unit":
		return "struct{}"
	default:
		return ""
	}
}

// primitiveGoNameOr returns the Go primitive name for a NamedType.
func primitiveGoNameOr(t TypeExpr) string {
	if nt, ok := t.(*NamedType); ok {
		return primitiveGoName(nt.Name)
	}
	return ""
}

func typeString(t TypeExpr, subst map[string]string) string {
	switch tt := t.(type) {
	case *NamedType:
		if subst != nil {
			if repl, ok := subst[tt.Name]; ok && len(tt.Args) == 0 {
				return repl
			}
		}
		switch tt.Name {
		case "Int":
			return "int"
		case "Int64":
			return "int64"
		case "Int32":
			return "int32"
		case "Int16":
			return "int16"
		case "Int8":
			return "int8"
		case "UInt":
			return "uint"
		case "UInt64":
			return "uint64"
		case "UInt32":
			return "uint32"
		case "UInt16":
			return "uint16"
		case "UInt8":
			return "uint8"
		case "Float64":
			return "float64"
		case "Float32":
			return "float32"
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Any":
			return "any"
		case "Unit":
			return "struct{}"
		case "Ref":
			if len(tt.Args) == 1 {
				return "*" + typeString(tt.Args[0], subst)
			}
		case "Slice":
			if len(tt.Args) == 1 {
				return "[]" + typeString(tt.Args[0], subst)
			}
		case "Map":
			if len(tt.Args) == 2 {
				return "map[" + typeString(tt.Args[0], subst) + "]" + typeString(tt.Args[1], subst)
			}
		case "Set":
			if len(tt.Args) == 1 {
				return "map[" + typeString(tt.Args[0], subst) + "]struct{}"
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, typeString(a, subst))
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, typeString(p, subst))
		}
		ret := typeStringReturn(tt.Ret, subst)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	default:
		return "any"
	}
}

func typeStringReturn(t TypeExpr, subst map[string]string) string {
	if isUnitType(t) {
		return ""
	}
	return typeString(t, subst)
}

func isUnitType(t TypeExpr) bool {
	if tt, ok := t.(*NamedType); ok && tt.Name == "Unit" && len(tt.Args) == 0 {
		return true
	}
	if tt, ok := t.(*TupleType); ok && len(tt.Elems) == 0 {
		return true
	}
	return false
}
