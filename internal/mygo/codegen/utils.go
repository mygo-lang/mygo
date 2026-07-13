package codegen

import (
	"path/filepath"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func sourceFileOf(decl Decl) string {
	switch d := decl.(type) {
	case *ImportDecl:
		return d.SourceFile
	case *EnumDecl:
		return d.SourceFile
	case *StructDecl:
		return d.SourceFile
	case *InterfaceDecl:
		return d.SourceFile
	case *ImplDecl:
		return d.SourceFile
	case *FuncDecl:
		return d.SourceFile
	case *LetStmt:
		return d.SourceFile
	default:
		return ""
	}
}

func declsHaveInterface(decls []Decl) bool {
	for _, decl := range decls {
		if _, ok := decl.(*InterfaceDecl); ok {
			return true
		}
	}
	return false
}

func sourceToGenName(sourceFile string) string {
	ext := filepath.Ext(sourceFile)
	base := strings.TrimSuffix(sourceFile, ext)
	return "zz_" + base + ".gen.go"
}

func skipSourceFile(name string) bool {
	return name == "" || name == "__prelude__"
}
