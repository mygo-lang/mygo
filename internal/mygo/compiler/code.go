package compiler

import jen "github.com/dave/jennifer/jen"

func codeString(code jen.Code) string {
	if code == nil {
		return ""
	}
	if gs, ok := code.(interface{ GoString() string }); ok {
		return gs.GoString()
	}
	return ""
}
