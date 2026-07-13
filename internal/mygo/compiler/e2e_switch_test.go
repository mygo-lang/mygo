package compiler

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen"
)

// TestE2ESwitchGeneratedCodeIsValidGo creates a full package with switch
// expressions and verifies the generated Go code is syntactically valid.
func TestE2ESwitchGeneratedCodeIsValidGo(t *testing.T) {
	// Build a package manually: with Option enum and a function using switch
	optEnum := &EnumDecl{
		Name:       "Option",
		TypeParams: []string{"A"},
		Variants: []EnumVariant{
			{Name: "Some", Fields: []Field{{Type: &NamedType{Name: "A"}}}},
			{Name: "None"},
		},
	}

	showInterface := &InterfaceDecl{
		Name:       "Show",
		TypeParams: []string{"A"},
		Methods: []*FuncDecl{
			{
				Name: "show",
				Params: []Param{
					{Name: "x", Type: &NamedType{Name: "A"}},
				},
				Ret: &NamedType{Name: "String"},
			},
		},
	}

	// describe[A](opt: Option[A]) -> String using Show[A]
	describeFunc := &FuncDecl{
		Name:       "describe",
		TypeParams: []string{"A"},
		Params: []Param{
			{Name: "opt", Type: &NamedType{Name: "Option", Args: []TypeExpr{&NamedType{Name: "A"}}}},
		},
		Ret: &NamedType{Name: "String"},
		Using: []Constraint{
			{Name: "Show", Args: []TypeExpr{&NamedType{Name: "A"}}},
		},
		Body: &SwitchExpr{
			Target: &IdentExpr{Name: "opt"},
			Cases: []SwitchCase{
				{
					Pattern: &VariantPattern{Name: "Some", Args: []string{"x"}},
					Body: &BinaryExpr{
						Op: "+",
						Left: &BinaryExpr{
							Op:    "+",
							Left:  &LiteralExpr{Kind: "string", Value: "some("},
							Right: &CallExpr{Callee: &IdentExpr{Name: "show"}, Args: []Expr{&IdentExpr{Name: "x"}}},
						},
						Right: &LiteralExpr{Kind: "string", Value: ")"},
					},
				},
				{
					Pattern: &WildcardPattern{},
					Body:    &LiteralExpr{Kind: "string", Value: "none"},
				},
			},
		},
	}

	// Build the package
	pkg := &Package{
		Name: "main",
		Decls: []Decl{
			optEnum,
			showInterface,
			describeFunc,
		},
		Enums: map[string]*EnumDecl{
			"Option": optEnum,
		},
		Structs: map[string]*StructDecl{},
		Interfaces: map[string]*InterfaceDecl{
			"Show": showInterface,
		},
		Funcs: map[string]*FuncDecl{
			"describe": describeFunc,
		},
	}

	// Setup required package fields
	pkg.ImportAliases = map[string]string{}
	pkg.Imports = map[string]struct{}{}

	// Generate Go code (Generate() creates its own generator internally)
	generated, err := codegen.Generate(pkg)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	t.Logf("Generated Go code:\n%s", generated)

	// Verify the generated Go code is syntactically valid
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, "", generated, parser.AllErrors)
	if err != nil {
		t.Fatalf("Generated Go code is not valid: %v\nGenerated code:\n%s", err, generated)
	}

	// Verify the generated code uses if-else (no type-switch)
	if strings.Contains(generated, "switch ") {
		t.Error("generated code should not contain 'switch' (use if-else instead)")
	}

	// Verify it contains type assertions
	if !strings.Contains(generated, ".(") {
		t.Error("generated code should contain type assertion '.('")
	}

	// Verify it contains else chains
	if !strings.Contains(generated, "else") {
		t.Error("generated code should contain 'else' for case chaining")
	}
}

func TestGenerateInherentImplUsesMangledMethodName(t *testing.T) {
	rect := &StructDecl{
		Name: "Rectangle",
		Fields: []Field{
			{Name: "width", Type: &NamedType{Name: "Float64"}},
			{Name: "height", Type: &NamedType{Name: "Float64"}},
		},
	}
	areaMethod := &FuncDecl{
		Name: "area",
		Params: []Param{
			{Name: "self", Type: &NamedType{Name: "Rectangle"}},
		},
		Ret: &NamedType{Name: "Float64"},
		Body: &BinaryExpr{
			Op:    "*",
			Left:  &FieldExpr{Expr: &IdentExpr{Name: "self"}, Field: "width"},
			Right: &FieldExpr{Expr: &IdentExpr{Name: "self"}, Field: "height"},
		},
	}
	impl := &ImplDecl{
		Type:    &NamedType{Name: "Rectangle"},
		Methods: []*FuncDecl{areaMethod},
	}
	demo := &FuncDecl{
		Name: "demo",
		Ret:  &NamedType{Name: "Float64"},
		Body: &BlockExpr{Stmts: []Stmt{
			&LetStmt{
				Name: "r",
				Value: &StructLitExpr{
					TypeName: "Rectangle",
					Fields: []StructLitField{
						{Name: "width", Value: &LiteralExpr{Kind: "number", Value: "10.0"}},
						{Name: "height", Value: &LiteralExpr{Kind: "number", Value: "5.0"}},
					},
				},
			},
			&ExprStmt{Expr: &CallExpr{
				Callee: &FieldExpr{Expr: &IdentExpr{Name: "r"}, Field: "area"},
			}},
		}},
	}
	pkg := &Package{
		Name:          "main",
		Decls:         []Decl{rect, impl, demo},
		Structs:       map[string]*StructDecl{"Rectangle": rect},
		Enums:         map[string]*EnumDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{"demo": demo},
		Impls:         []*ImplDecl{impl},
		ImportAliases: map[string]string{},
		Imports:       map[string]struct{}{},
		NoPrelude:     true,
	}

	generated, err := codegen.Generate(pkg)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(generated, "func Rectangle_area") {
		t.Fatalf("generated code missing mangled method:\n%s", generated)
	}
	if !strings.Contains(generated, "return Rectangle_area(r") {
		t.Fatalf("generated code missing mangled call:\n%s", generated)
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "", generated, parser.AllErrors); err != nil {
		t.Fatalf("Generated Go code is not valid: %v\nGenerated code:\n%s", err, generated)
	}
}
