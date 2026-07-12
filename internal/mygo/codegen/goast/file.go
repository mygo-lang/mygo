package goast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
)

// SourceFile represents a Go source file being built.
// It provides a similar interface to jennifer's *jen.File.
type SourceFile struct {
	pkgName     string
	header      string
	imports     map[string]string // path -> alias (empty alias means default)
	importOrder []string
	decls       []ast.Decl
}

// NewSourceFile creates a new SourceFile with the given package name.
func NewSourceFile(pkgName string) *SourceFile {
	return &SourceFile{
		pkgName:     pkgName,
		imports:     map[string]string{},
		importOrder: nil,
	}
}

// HeaderComment sets the header comment for the file.
func (f *SourceFile) HeaderComment(comment string) {
	f.header = comment
}

// AddImport registers an import. Use empty alias for default import name.
// For dot-imports, set alias to ".".
func (f *SourceFile) AddImport(path, alias string) {
	if _, ok := f.imports[path]; ok {
		return
	}
	f.imports[path] = alias
	f.importOrder = append(f.importOrder, path)
}

// AddDecl adds a top-level declaration to the file.
func (f *SourceFile) AddDecl(decl ast.Decl) {
	f.decls = append(f.decls, decl)
}

// AddDecls adds multiple top-level declarations.
func (f *SourceFile) AddDecls(decls []ast.Decl) {
	f.decls = append(f.decls, decls...)
}

// ImportSpecs returns the sorted import specs for the file.
func (f *SourceFile) ImportSpecs() []*ast.ImportSpec {
	sorted := sort.StringSlice(f.importOrder)
	sorted.Sort()
	specs := make([]*ast.ImportSpec, 0, len(sorted))
	for _, path := range sorted {
		alias := f.imports[path]
		spec := ImportSpec(path, alias)
		specs = append(specs, spec)
	}
	return specs
}

// Render produces the formatted Go source as a string.
func (f *SourceFile) Render() (string, error) {
	// Build all declarations.
	decls := make([]ast.Decl, 0, len(f.decls)+1)

	// Add import declaration if we have imports.
	if len(f.imports) > 0 {
		importDecl := &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: make([]ast.Spec, 0, len(f.importOrder)),
		}
		sorted := sort.StringSlice(f.importOrder)
		sorted.Sort()
		for _, path := range sorted {
			alias := f.imports[path]
			spec := ImportSpec(path, alias)
			importDecl.Specs = append(importDecl.Specs, spec)
		}
		decls = append(decls, importDecl)
	}

	// Add user declarations.
	decls = append(decls, f.decls...)

	// Build the AST file.
	file := &ast.File{
		Name:  ast.NewIdent(f.pkgName),
		Decls: decls,
	}

	// Add header comment as a comment group.
	if f.header != "" {
		file.Comments = []*ast.CommentGroup{
			{
				List: []*ast.Comment{
					{Text: "// " + f.header},
				},
			},
		}
	}

	// Render with go/format.
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), file); err != nil {
		return "", fmt.Errorf("go/format: %w", err)
	}
	return buf.String(), nil
}

// MustRender renders the source and panics on error (for testing).
func (f *SourceFile) MustRender() string {
	s, err := f.Render()
	if err != nil {
		panic(err)
	}
	return s
}

// RenderAST renders the AST file and returns the raw *ast.File.
// Useful for testing or further AST manipulation.
func (f *SourceFile) RenderAST() *ast.File {
	decls := make([]ast.Decl, 0, len(f.decls)+1)
	if len(f.imports) > 0 {
		importDecl := &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: make([]ast.Spec, 0, len(f.importOrder)),
		}
		sorted := sort.StringSlice(f.importOrder)
		sorted.Sort()
		for _, path := range sorted {
			alias := f.imports[path]
			spec := ImportSpec(path, alias)
			importDecl.Specs = append(importDecl.Specs, spec)
		}
		decls = append(decls, importDecl)
	}
	decls = append(decls, f.decls...)
	return &ast.File{
		Name:  ast.NewIdent(f.pkgName),
		Decls: decls,
	}
}

// RenderNode renders any AST node as a string (useful for testing).
func RenderNode(node ast.Node) (string, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), node); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ParseCheck checks if the given source is valid Go.
// Returns nil if valid, or the parser error.
func ParseCheck(src string) error {
	_, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	return err
}
