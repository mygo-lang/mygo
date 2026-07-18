package codegen

import (
	"bytes"
	"go/printer"
	"go/token"
	"reflect"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func TestSplitTypeArgsHandlesGoMapTypes(t *testing.T) {
	tests := []struct {
		typ  string
		base string
		args []string
	}{
		{
			typ:  "Option[map[string]any]",
			base: "Option",
			args: []string{"map[string]any"},
		},
		{
			typ:  "Result[map[string]any, error]",
			base: "Result",
			args: []string{"map[string]any", "error"},
		},
		{
			typ:  "Option[map[string][]int]",
			base: "Option",
			args: []string{"map[string][]int"},
		},
	}

	for _, tt := range tests {
		base, args := splitTypeArgs(tt.typ)
		if base != tt.base || !reflect.DeepEqual(args, tt.args) {
			t.Fatalf("splitTypeArgs(%q) = (%q, %#v), want (%q, %#v)", tt.typ, base, args, tt.base, tt.args)
		}
	}
}

func TestGoTypeExprForAssertionHandlesGoMapTypes(t *testing.T) {
	expr := genericIdent("OptionSome", goTypeExprForAssertion("map[string]any"))

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		t.Fatalf("printer.Fprint() error = %v", err)
	}
	if got, want := buf.String(), "OptionSome[map[string]any]"; got != want {
		t.Fatalf("printed assertion type = %q, want %q", got, want)
	}
}

func TestInferredTypeSkipsUnresolvedTypeVariables(t *testing.T) {
	expr := &IdentExpr{Name: "value"}
	g := &gen{
		typedInfo: &typeinference.TypedInfo{
			ExprTypes: map[Expr]typeinference.MonoType{
				expr: typeinference.TCon{
					Name: "Option",
					Args: []typeinference.MonoType{
						typeinference.TCon{
							Name: "Ref",
							Args: []typeinference.MonoType{
								typeinference.TCon{
									Name: "List",
									Args: []typeinference.MonoType{typeinference.TVar{ID: 55}},
								},
							},
						},
					},
				},
			},
		},
	}

	if got := g.inferredType(expr); got != "" {
		t.Fatalf("inferredType() = %q, want empty for unresolved type variable", got)
	}
}

func TestTranslateSwitchUsesIfElse(t *testing.T) {
	optEnum := &EnumDecl{
		Name:       "Option",
		TypeParams: []string{"A"},
		Variants: []EnumVariant{
			{Name: "Some", Fields: []Field{{Type: &NamedType{Name: "A"}}}},
			{Name: "None"},
		},
	}

	g := &Generator{
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
		goSigCache:        map[string]*GoPackageSigs{},
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
