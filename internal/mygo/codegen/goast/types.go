package goast

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// TypeExprToGo converts a MyGO TypeExpr string to a go/ast expression.
// The typ string uses MyGO naming conventions (Int, String, Ref[T], Slice[T], etc.)
// and is converted to Go native types (int, string, *T, []T, etc.).
func TypeExprToGo(typ string) (ast.Expr, error) {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return nil, fmt.Errorf("empty type expression")
	}

	// Handle pointer types
	if strings.HasPrefix(typ, "*") {
		inner, err := TypeExprToGo(typ[1:])
		if err != nil {
			return nil, err
		}
		return StarExpr(inner), nil
	}

	// Handle slice types
	if strings.HasPrefix(typ, "[]") {
		inner, err := TypeExprToGo(typ[2:])
		if err != nil {
			return nil, err
		}
		return ArrayType(nil, inner), nil
	}

	// Handle map types
	if strings.HasPrefix(typ, "map[") {
		end := strings.Index(typ, "]")
		if end < 0 {
			return nil, fmt.Errorf("invalid map type: %s", typ)
		}
		key, err := TypeExprToGo(typ[4:end])
		if err != nil {
			return nil, err
		}
		val, err := TypeExprToGo(typ[end+1:])
		if err != nil {
			return nil, err
		}
		return MapType(key, val), nil
	}

	// Handle generic types: Name[T1, T2, ...]
	if idx := strings.Index(typ, "["); idx >= 0 {
		name := typ[:idx]
		inner := typ[idx+1 : len(typ)-1] // strip [ ]
		args := splitGenericArgs(inner)
		typeArgs := make([]ast.Expr, len(args))
		for i, a := range args {
			var err error
			typeArgs[i], err = TypeExprToGo(strings.TrimSpace(a))
			if err != nil {
				return nil, err
			}
		}
		return GenericType(name, typeArgs...), nil
	}

	// Handle chan types
	if strings.HasPrefix(typ, "chan<- ") {
		inner, err := TypeExprToGo(typ[7:])
		if err != nil {
			return nil, err
		}
		return &ast.ChanType{Dir: ast.SEND, Value: inner}, nil
	}
	if strings.HasPrefix(typ, "<-chan ") {
		inner, err := TypeExprToGo(typ[7:])
		if err != nil {
			return nil, err
		}
		return &ast.ChanType{Dir: ast.RECV, Value: inner}, nil
	}
	if strings.HasPrefix(typ, "chan ") {
		inner, err := TypeExprToGo(typ[5:])
		if err != nil {
			return nil, err
		}
		return &ast.ChanType{Dir: ast.SEND | ast.RECV, Value: inner}, nil
	}

	// Handle struct types
	if strings.HasPrefix(typ, "struct {") || strings.HasPrefix(typ, "struct{") {
		return parseStructType(typ), nil
	}

	// Handle func types
	if strings.HasPrefix(typ, "func(") {
		return parseFuncType(typ), nil
	}

	// Handle interface types
	if strings.HasPrefix(typ, "interface{") || strings.HasPrefix(typ, "interface {") {
		return ast.NewIdent("any"), nil // simplified
	}

	// Handle MyGO type names
	switch typ {
	case "Int", "int":
		return PrimitiveType("int"), nil
	case "Int8", "int8":
		return PrimitiveType("int8"), nil
	case "Int16", "int16":
		return PrimitiveType("int16"), nil
	case "Int32", "int32":
		return PrimitiveType("int32"), nil
	case "Int64", "int64":
		return PrimitiveType("int64"), nil
	case "UInt", "uint":
		return PrimitiveType("uint"), nil
	case "UInt8", "uint8":
		return PrimitiveType("uint8"), nil
	case "UInt16", "uint16":
		return PrimitiveType("uint16"), nil
	case "UInt32", "uint32":
		return PrimitiveType("uint32"), nil
	case "UInt64", "uint64":
		return PrimitiveType("uint64"), nil
	case "Float32", "float32":
		return PrimitiveType("float32"), nil
	case "Float64", "float64":
		return PrimitiveType("float64"), nil
	case "String", "string":
		return PrimitiveType("string"), nil
	case "Bool", "bool":
		return PrimitiveType("bool"), nil
	case "Any", "any":
		return PrimitiveType("any"), nil
	case "Unit", "struct{}":
		return StructType(nil), nil
	case "error":
		return ast.NewIdent("error"), nil
	case "nil":
		return Nil(), nil
	case "true":
		return BoolLit(true), nil
	case "false":
		return BoolLit(false), nil
	}

	// If it starts with uppercase, it's an exported type name.
	if len(typ) > 0 && typ[0] >= 'A' && typ[0] <= 'Z' {
		return ast.NewIdent(typ), nil
	}

	// Default: treat as an identifier
	return ast.NewIdent(typ), nil
}

// TypeStringToGo converts a MyGO type string to a Go type string.
// This is the string-based equivalent used for type compatibility checks.
func TypeStringToGo(typ string) string {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ""
	}
	// Handle Ref[T] → *T
	if strings.HasPrefix(typ, "Ref[") && strings.HasSuffix(typ, "]") {
		return "*" + TypeStringToGo(typ[4:len(typ)-1])
	}
	// Handle Slice[T] → []T
	if strings.HasPrefix(typ, "Slice[") && strings.HasSuffix(typ, "]") {
		return "[]" + TypeStringToGo(typ[6:len(typ)-1])
	}
	// Handle Set[T] → map[T]struct{}
	if strings.HasPrefix(typ, "Set[") && strings.HasSuffix(typ, "]") {
		return "map[" + TypeStringToGo(typ[4:len(typ)-1]) + "]struct{}"
	}
	// Handle Map[K, V] → map[K]V
	if strings.HasPrefix(typ, "Map[") && strings.HasSuffix(typ, "]") {
		inner := typ[4 : len(typ)-1]
		parts := splitGenericArgs(inner)
		if len(parts) == 2 {
			return "map[" + TypeStringToGo(parts[0]) + "]" + TypeStringToGo(parts[1])
		}
	}
	// Handle Chan[T] → chan T
	if strings.HasPrefix(typ, "Chan[") && strings.HasSuffix(typ, "]") {
		return "chan " + TypeStringToGo(typ[5:len(typ)-1])
	}
	if strings.HasPrefix(typ, "SendChan[") && strings.HasSuffix(typ, "]") {
		return "chan<- " + TypeStringToGo(typ[9:len(typ)-1])
	}
	if strings.HasPrefix(typ, "RecvChan[") && strings.HasSuffix(typ, "]") {
		return "<-chan " + TypeStringToGo(typ[9:len(typ)-1])
	}
	// Handle generic types Name[T1, T2, ...]
	if idx := strings.Index(typ, "["); idx >= 0 && strings.HasSuffix(typ, "]") {
		name := typ[:idx]
		inner := typ[idx+1 : len(typ)-1]
		args := splitGenericArgs(inner)
		for i, a := range args {
			args[i] = TypeStringToGo(a)
		}
		return name + "[" + strings.Join(args, ", ") + "]"
	}
	// MyGO primitive names → Go names
	switch typ {
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
	case "Any":
		return "any"
	case "Unit":
		return "struct{}"
	}
	return typ
}

// GoStringToMyGo converts a Go type string to a MyGO type string.
func GoStringToMyGo(typ string) string {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return ""
	}
	if strings.HasPrefix(typ, "[]") {
		return "Slice[" + GoStringToMyGo(typ[2:]) + "]"
	}
	if strings.HasPrefix(typ, "*") {
		return "Ref[" + GoStringToMyGo(typ[1:]) + "]"
	}
	if strings.HasPrefix(typ, "map[") {
		end := strings.Index(typ, "]")
		if end > 0 {
			key := GoStringToMyGo(typ[4:end])
			val := GoStringToMyGo(typ[end+1:])
			if val == "struct{}" {
				return "Set[" + key + "]"
			}
			return "Map[" + key + ", " + val + "]"
		}
	}
	if strings.HasPrefix(typ, "chan<- ") {
		return "SendChan[" + GoStringToMyGo(typ[7:]) + "]"
	}
	if strings.HasPrefix(typ, "<-chan ") {
		return "RecvChan[" + GoStringToMyGo(typ[7:]) + "]"
	}
	if strings.HasPrefix(typ, "chan ") {
		return "Chan[" + GoStringToMyGo(typ[5:]) + "]"
	}
	switch typ {
	case "string":
		return "String"
	case "bool":
		return "Bool"
	case "int":
		return "Int"
	case "int8":
		return "Int8"
	case "int16":
		return "Int16"
	case "int32":
		return "Int32"
	case "int64":
		return "Int64"
	case "uint":
		return "UInt"
	case "uint8":
		return "UInt8"
	case "uint16":
		return "UInt16"
	case "uint32":
		return "UInt32"
	case "uint64":
		return "UInt64"
	case "float32":
		return "Float32"
	case "float64":
		return "Float64"
	case "any":
		return "Any"
	case "struct{}":
		return "Unit"
	}
	return typ
}

// splitGenericArgs splits a generic type argument list into individual arguments.
// e.g., "int, string" → ["int", "string"]; "Map[int, string]" → ["Map[int, string]"]
func splitGenericArgs(s string) []string {
	var args []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start <= len(s) {
		args = append(args, strings.TrimSpace(s[start:]))
	}
	return args
}

// parseStructType parses a simplified struct type string into a go/ast expression.
func parseStructType(s string) ast.Expr {
	// Remove "struct {" prefix and "}" suffix
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "struct{")
	s = strings.TrimPrefix(s, "struct {")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimSpace(s)
	if s == "" {
		return StructType(nil)
	}
	// Parse semicolon-separated fields: "F0 int; F1 string"
	parts := strings.Split(s, ";")
	fields := make([]*ast.Field, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Split on last space: "F0 int" → name="F0", type="int"
		if sp := strings.LastIndex(part, " "); sp >= 0 {
			name := strings.TrimSpace(part[:sp])
			typ := strings.TrimSpace(part[sp+1:])
			typExpr, err := TypeExprToGo(typ)
			if err != nil {
				typExpr = ast.NewIdent(typ)
			}
			fields = append(fields, Field([]string{name}, typExpr, ""))
		} else {
			// No name (embedded type)
			typExpr, err := TypeExprToGo(part)
			if err != nil {
				typExpr = ast.NewIdent(part)
			}
			fields = append(fields, &ast.Field{Type: typExpr})
		}
	}
	return StructType(fields)
}

// parseFuncType parses a simplified function type string.
func parseFuncType(s string) ast.Expr {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "func(") {
		return ast.NewIdent(s)
	}
	parenIdx := strings.Index(s, ")")
	if parenIdx < 0 {
		return ast.NewIdent(s)
	}
	paramsStr := s[5:parenIdx]
	retStr := strings.TrimSpace(s[parenIdx+1:])

	params := parseParamList(paramsStr)
	var results []*ast.Field
	if retStr != "" {
		retExpr, err := TypeExprToGo(retStr)
		if err != nil {
			retExpr = ast.NewIdent(retStr)
		}
		results = []*ast.Field{{Type: retExpr}}
	}
	return FuncType(params, results)
}

// parseParamList parses a comma-separated parameter/field list.
func parseParamList(s string) []*ast.Field {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := splitGenericArgs(s)
	fields := make([]*ast.Field, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Simplified: just treat each as a type
		typExpr, err := TypeExprToGo(part)
		if err != nil {
			typExpr = ast.NewIdent(part)
		}
		fields = append(fields, &ast.Field{Type: typExpr})
	}
	return fields
}

// IsUnitType checks if a type expression represents the unit type.
func IsUnitType(typ string) bool {
	typ = strings.TrimSpace(typ)
	return typ == "" || typ == "Unit" || typ == "struct{}" || typ == "()"
}

// TypeKey generates a safe key string for a type (for helper function naming).
func TypeKey(typ string) string {
	typ = strings.ReplaceAll(typ, "[", "_")
	typ = strings.ReplaceAll(typ, "]", "")
	typ = strings.ReplaceAll(typ, ", ", "_")
	typ = strings.ReplaceAll(typ, ",", "_")
	typ = strings.ReplaceAll(typ, " ", "")
	typ = strings.ReplaceAll(typ, "*", "Ptr")
	typ = strings.ReplaceAll(typ, ".", "_")
	typ = strings.ReplaceAll(typ, "func(", "Func_")
	typ = strings.ReplaceAll(typ, ")", "")
	return strings.ToLower(typ)
}

// EnsureString wraps a string as an AST expression.
// If the string looks like a type, it's converted; otherwise it's treated as an identifier.
func EnsureString(s string) ast.Expr {
	if s == "" {
		return nil
	}
	// Try as type expression first
	if expr, err := TypeExprToGo(s); err == nil {
		return expr
	}
	return ast.NewIdent(s)
}

// StrLit creates a string literal for use as an AST expression.
func StrLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: tokenPos(s), Value: s}
}

// Helper to get token kind from string.
func tokenPos(s string) token.Token {
	if len(s) > 0 && s[0] == '"' {
		return token.STRING
	}
	if isIntStr(s) {
		return token.INT
	}
	return token.STRING
}

func isIntStr(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
