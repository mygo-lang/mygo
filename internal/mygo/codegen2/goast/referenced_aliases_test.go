package goast

import (
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func referencedAliasesFor(t *testing.T, source string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse generated Go: %v", err)
	}
	return ReferencedImportAliases(file.Decls)
}

func TestReferencedImportAliasesIgnoresLocalBindings(t *testing.T) {
	aliases := referencedAliasesFor(t, `package sample

func hover(token SourceToken) string {
	return token.URI + ":" + token.Token.Raw
}
`)
	if len(aliases) != 0 {
		t.Fatalf("local selector kept aliases %v", aliases)
	}
}

func TestReferencedImportAliasesKeepsPackageQualifier(t *testing.T) {
	aliases := referencedAliasesFor(t, `package sample

func join(parts []string) string {
	return strings.Join(parts, ",")
}
`)
	if !reflect.DeepEqual(aliases, []string{"strings"}) {
		t.Fatalf("aliases = %v, want [strings]", aliases)
	}
}

func TestReferencedImportAliasesDistinguishesShadowedAndPackageUse(t *testing.T) {
	aliases := referencedAliasesFor(t, `package sample

type token struct{ URI string }

func shadowed(value token) string {
	return value.URI
}

func qualified(value token) string {
	return token.New(value.URI).String()
}
`)
	if !reflect.DeepEqual(aliases, []string{"token"}) {
		t.Fatalf("aliases = %v, want [token]", aliases)
	}
}

func TestReferencedImportAliasesExcludesShortVariableDeclarations(t *testing.T) {
	aliases := referencedAliasesFor(t, `package sample

func render() string {
	token := load()
	return token.Text()
}
`)
	if len(aliases) != 0 {
		t.Fatalf("short variable declaration kept aliases %v", aliases)
	}
}
