package goast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// DynamicValueDecls emits an ordinary erased wrapper type and its package
// functions for one MyGO typeclass interface. The caller supplies a
// deterministic exported Go identifier such as MygoDynamicShow. MyGO
// typeclass operations remain package-level functions; the wrapper does not
// acquire a Go interface method set or receiver methods.
func DynamicValueDecls(typeName string) []ast.Decl {
	source := fmt.Sprintf(`package generated
type %[1]s struct { payload any; implementation string }
func New%[1]s(payload any, implementation string) %[1]s {
	return %[1]s{payload: payload, implementation: implementation}
}
func %[1]sPayload(value %[1]s) any { return value.payload }
func %[1]sImplementation(value %[1]s) string { return value.implementation }
`, typeName)
	file, err := parser.ParseFile(token.NewFileSet(), "dynamic.gen.go", source, parser.AllErrors)
	if err != nil {
		panic(fmt.Sprintf("codegen2: invalid dynamic wrapper declaration for %s: %v", typeName, err))
	}
	return file.Decls
}

// DynamicAdapterDecl emits a package-level adapter around one generated MyGO
// impl helper. hiddenIndex identifies the sole parameter whose type is erased
// into payload; all other parameters retain their Go types.
func DynamicAdapterDecl(name string, typeParams, parameterNames, parameterTypes []string, hiddenIndex int, targetType, helper string, helperTypeArgs, resultTypes []string) (ast.Decl, error) {
	if len(parameterNames) != len(parameterTypes) {
		return nil, fmt.Errorf("dynamic adapter: parameter names and types differ in length")
	}
	if hiddenIndex < 0 || hiddenIndex >= len(parameterNames) {
		return nil, fmt.Errorf("dynamic adapter: hidden parameter index %d is out of range", hiddenIndex)
	}
	params := []string{"payload any"}
	callArgs := make([]string, 0, len(parameterNames))
	for index := range parameterNames {
		if index == hiddenIndex {
			callArgs = append(callArgs, "__mygo_value")
			continue
		}
		name := fmt.Sprintf("dynamicArg%d", index)
		params = append(params, name+" "+parameterTypes[index])
		callArgs = append(callArgs, name)
	}
	callee := helper
	if len(helperTypeArgs) > 0 {
		callee += "[" + strings.Join(helperTypeArgs, ", ") + "]"
	}
	call := callee + "(" + strings.Join(callArgs, ", ") + ")"
	var body strings.Builder
	fmt.Fprintf(&body, "__mygo_value, __mygo_ok := payload.(%s)\n", targetType)
	body.WriteString("if !__mygo_ok { panic(\"MyGO dynamic adapter payload type mismatch\") }\n")
	if len(resultTypes) > 0 {
		body.WriteString("return ")
	}
	body.WriteString(call)
	if len(resultTypes) == 0 {
		body.WriteByte('\n')
	}
	returnType := ""
	if len(resultTypes) == 1 {
		returnType = resultTypes[0]
	} else if len(resultTypes) > 1 {
		returnType = "(" + strings.Join(resultTypes, ", ") + ")"
	}
	typeParamDecls := make([]string, len(typeParams))
	for index, typeParam := range typeParams {
		typeParamDecls[index] = typeParam + " any"
	}
	typeParamClause := ""
	if len(typeParamDecls) > 0 {
		typeParamClause = "[" + strings.Join(typeParamDecls, ", ") + "]"
	}
	source := fmt.Sprintf("package generated\nfunc %s%s(%s)%s{\n%s\n}\n", name, typeParamClause, strings.Join(params, ", "), resultSuffix(returnType), body.String())
	file, err := parser.ParseFile(token.NewFileSet(), "dynamic_adapter.gen.go", source, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("dynamic adapter %s: parse generated function: %w", name, err)
	}
	return file.Decls[0], nil
}

func resultSuffix(result string) string {
	if result == "" {
		return " "
	}
	return " " + result + " "
}
