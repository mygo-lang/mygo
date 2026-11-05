package codegen2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/abi"
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/metadata"
)

func typedValueDecl(name string, typ ast2.MonoType, mutable bool) ast2.Decl {
	bind := ast2.Bind{
		Name:     name,
		Type:     abi.NoneOf[ast2.TypeExpr](),
		TypeSpan: abi.NoneOf[ast2.SourceSpan](),
		Value:    ast2.Expr{Type: abi.SomeOf(typ)},
		Span:     ast2.SourceSpan{},
	}
	if mutable {
		return ast2.VarDeclNode(bind)
	}
	return ast2.LetDeclNode(bind)
}

func TestDynamicMetadataExportsTypedPackageValues(t *testing.T) {
	decls := []ast2.Decl{
		ast2.StructDeclNode("Box", nil, nil),
		typedValueDecl("DefaultPort", ast2.TConType("Int"), false),
		typedValueDecl("Active", ast2.TConType("Bool"), true),
		typedValueDecl("Default", ast2.TConType("Box"), false),
		typedValueDecl("Names", ast2.TAppType(ast2.TConType("List"), []ast2.MonoType{ast2.TConType("String")}), false),
		typedValueDecl("Now", ast2.TQualifiedType("go:time", ast2.TConType("Time")), false),
		typedValueDecl("private", ast2.TConType("Int"), false),
	}
	exports := dynamicMetadataExports(decls, "example.com/dep", map[string]string{"time": "time"})
	values := map[string]metadata.ValueExport{}
	for _, value := range exports.Values {
		values[value.Name] = value
	}
	if len(values) != 5 {
		t.Fatalf("exported values = %d, want 5", len(values))
	}
	if value := values["DefaultPort"]; value.Type.Kind != "builtin" || value.Type.Name != "int" || value.Mutable {
		t.Fatalf("DefaultPort = %#v, want immutable int", value)
	}
	if value := values["Active"]; value.Type.Kind != "builtin" || value.Type.Name != "bool" || !value.Mutable {
		t.Fatalf("Active = %#v, want mutable bool", value)
	}
	if value := values["Default"]; value.Type.Kind != "named" || value.Type.Name != "Box" || value.Type.Package != "example.com/dep" {
		t.Fatalf("Default = %#v, want named Box in the defining package", value)
	}
	if value := values["Names"]; value.Type.Kind != "named" || value.Type.Name != "List" || value.Type.Package != "" {
		t.Fatalf("Names = %#v, want unqualified prelude List", value)
	}
	if value := values["Now"]; value.Type.Kind != "named" || value.Type.Name != "Time" || value.Type.Package != "time" {
		t.Fatalf("Now = %#v, want named Time in package time", value)
	}
	if _, ok := values["private"]; ok {
		t.Fatal("lowercase package value leaked into metadata exports")
	}
}
