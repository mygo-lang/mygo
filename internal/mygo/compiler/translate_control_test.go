package compiler

import (
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestTranslateSwitchUsesIfElse(t *testing.T) {
	optEnum := &EnumDecl{
		Name:       "Option",
		TypeParams: []string{"A"},
		Variants: []EnumVariant{
			{Name: "Some", Fields: []Field{{Type: &NamedType{Name: "A"}}}},
			{Name: "None"},
		},
	}

	g := &generator{
		pkg: &Package{
			Name: "main",
			Enums: map[string]*EnumDecl{
				"Option": optEnum,
			},
			Decls: []Decl{optEnum},
		},
		importAliases:     map[string]string{},
		interfaceByMethod: map[string]string{},
		variantByName:     map[string]string{"Some": "Option", "None": "Option"},
		goSigCache:        map[string]*goPackageSigs{},
	}

	t.Run("expression form with variant patterns", func(t *testing.T) {
		switchExpr := &SwitchExpr{
			Target: &IdentExpr{Name: "opt"},
			Cases: []SwitchCase{
				{
					Pattern: &VariantPattern{Name: "Some", Args: []string{"x"}},
					Body:    &IdentExpr{Name: "x"},
				},
				{
					Pattern: &VariantPattern{Name: "None"},
					Body:    &LiteralExpr{Kind: "number", Value: "0"},
				},
			},
		}

		ctx := &exprCtx{
			locals:     map[string]string{"opt": "Option[Int]"},
			bindings:   map[string]string{"opt": "opt"},
			mutable:    map[string]bool{},
			typeParams: map[string]struct{}{},
		}

		code, typ, err := g.translateSwitch(switchExpr, ctx, "Int")
		if err != nil {
			t.Fatalf("translateSwitch() error = %v", err)
		}
		if code == nil {
			t.Fatal("translateSwitch() returned nil code")
		}
		if typ != "Int" {
			t.Errorf("translateSwitch() type = %q, want %q", typ, "Int")
		}
	})

	t.Run("wildcard pattern", func(t *testing.T) {
		switchExpr := &SwitchExpr{
			Target: &IdentExpr{Name: "opt"},
			Cases: []SwitchCase{
				{
					Pattern: &VariantPattern{Name: "Some", Args: []string{"x"}},
					Body:    &IdentExpr{Name: "x"},
				},
				{
					Pattern: &WildcardPattern{},
					Body:    &LiteralExpr{Kind: "number", Value: "0"},
				},
			},
		}

		ctx := &exprCtx{
			locals:     map[string]string{"opt": "Option[Int]"},
			bindings:   map[string]string{"opt": "opt"},
			mutable:    map[string]bool{},
			typeParams: map[string]struct{}{},
		}

		code, typ, err := g.translateSwitch(switchExpr, ctx, "Int")
		if err != nil {
			t.Fatalf("translateSwitch() error = %v", err)
		}
		if code == nil {
			t.Fatal("translateSwitch() returned nil code")
		}
		_ = typ
	})

	t.Run("statement form (no expected type)", func(t *testing.T) {
		switchExpr := &SwitchExpr{
			Target: &IdentExpr{Name: "opt"},
			Cases: []SwitchCase{
				{
					Pattern: &VariantPattern{Name: "Some", Args: []string{"x"}},
					Body: &CallExpr{
						Callee: &IdentExpr{Name: "fn"},
						Args:   []Expr{&IdentExpr{Name: "x"}},
					},
				},
				{
					Pattern: &VariantPattern{Name: "None"},
					Body:    &IdentExpr{Name: "noop"},
				},
			},
		}

		ctx := &exprCtx{
			locals:     map[string]string{"opt": "Option[Int]", "fn": "func(Int)Unit", "noop": "Unit"},
			bindings:   map[string]string{"opt": "opt", "fn": "fn", "noop": "noop"},
			mutable:    map[string]bool{},
			typeParams: map[string]struct{}{},
		}

		code, typ, err := g.translateSwitch(switchExpr, ctx, "")
		if err != nil {
			t.Fatalf("translateSwitch() error = %v", err)
		}
		if code == nil {
			t.Fatal("translateSwitch() returned nil code")
		}
		if typ != "" {
			t.Errorf("translateSwitch() type = %q, want empty for statement form", typ)
		}
	})
}
