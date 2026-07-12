package compiler

import (
	"strings"
	"unicode"

	"github.com/mygo-lang/mygo/internal/mygo/pkg"
)

// importAliasForPath extracts a default alias from an import path.
func importAliasForPath(path string) string {
	path = importPathForGo(path)
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return toPackageName(path[idx+1:])
	}
	return toPackageName(path)
}

// importPathForGo strips the "go:" prefix from import paths.
func importPathForGo(path string) string {
	return strings.TrimPrefix(path, "go:")
}

// toPackageName sanitizes a string to be a valid Go package name.
func toPackageName(name string) string {
	if name == "" {
		return "main"
	}
	return strings.ToLower(sanitizeIdent(name))
}

// sanitizeIdent sanitizes a string to be a valid Go identifier.
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
	result := b.String()
	if isGoKeyword(result) {
		result += "_"
	}
	return result
}

// isGoKeyword reports whether the given identifier is a Go keyword.
func isGoKeyword(s string) bool {
	switch s {
	case "break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var":
		return true
	}
	return false
}

// PackageFromPkg wraps a *pkg.Package into the local Package type (alias).
func PackageFromPkg(p *pkg.Package) *Package {
	return p
}

// exportName capitalizes the first letter of a name.
func exportName(name string) string {
	if name == "" {
		return name
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
