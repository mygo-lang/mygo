package compiler

import (
	"bytes"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

var returnFuncLineBreakRE = regexp.MustCompile(`(?m)^([ \t]*)return[ \t]*\n([ \t]*)func\(`)
var genericSpaceBeforeBracketRE = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_\.]*) \[`)
var genericSpaceBeforeParenRE = regexp.MustCompile(`\] \(`)

func renderFile(file *jen.File) (string, error) {
	out := &bytes.Buffer{}
	if err := file.Render(out); err != nil {
		os.WriteFile("/tmp/mygo_raw.txt", out.Bytes(), 0o644)
		return "", err
	}
	src := out.String()
	src = returnFuncLineBreakRE.ReplaceAllString(src, "${1}return func(")
	src = genericSpaceBeforeBracketRE.ReplaceAllString(src, `${1}[`)
	src = genericSpaceBeforeParenRE.ReplaceAllString(src, "](")
	return src, nil
}

func skipSourceFile(name string) bool {
	return name == "" || name == "__prelude__"
}

func sourceToGenName(sourceFile string) string {
	ext := filepath.Ext(sourceFile)
	base := strings.TrimSuffix(sourceFile, ext)
	return "zz_" + base + ".gen.go"
}

func declsHaveInterface(decls []Decl) bool {
	for _, decl := range decls {
		if _, ok := decl.(*InterfaceDecl); ok {
			return true
		}
	}
	return false
}

func genImportCode(file *jen.File, needsCallAny bool, p *Package) {
	imports := p.sortedImports()
	if p.Name != "prelude" && !p.NoPrelude {
		imports = append(imports, importSpec{Path: "github.com/mygo-lang/mygo/lib/prelude", Alias: "."})
	}
	if needsCallAny && !hasImportPath(imports, "reflect") {
		imports = append(imports, importSpec{Path: "reflect"})
		sort.Slice(imports, func(i, j int) bool {
			if imports[i].Alias == imports[j].Alias {
				return imports[i].Path < imports[j].Path
			}
			return imports[i].Alias < imports[j].Alias
		})
	}
	for _, imp := range imports {
		path := importPathForGo(imp.Path)
		alias := imp.Alias
		if alias == importAliasForPath(path) {
			alias = ""
		}
		if alias == "" {
			file.ImportName(path, "")
			continue
		}
		if alias == "." {
			// Skip: Jennifer can't render import . "path" correctly.
			// We inject these manually in renderFile.
			continue
		}
		file.ImportName(path, alias)
	}
	if p.Name != "prelude" && !p.NoPrelude {
		file.Var().Id("_").Op("=").Id("None").Types(jen.Any())
	}
	for _, imp := range imports {
		path := importPathForGo(imp.Path)
		if path == "" || path == "github.com/mygo-lang/mygo/lib/prelude" {
			continue
		}
		alias := imp.Alias
		if alias == "" {
			alias = importAliasForPath(path)
		}
		if alias == "" {
			continue
		}
		if keep := goImportKeepAlive(alias, path); keep != nil {
			file.Var().Id("_").Op("=").Add(keep)
		}
	}
}

func goImportKeepAlive(alias, path string) jen.Code {
	sigs, err := loadGoPackageSigs(path)
	if err != nil || sigs == nil || sigs.pkg == nil {
		return nil
	}
	scope := sigs.pkg.Scope()
	for _, name := range scope.Names() {
		if !isExportedGoIdent(name) {
			continue
		}
		obj := scope.Lookup(name)
		switch obj.(type) {
		case *types.Func, *types.Const, *types.Var:
			return jen.Qual(path, name)
		}
	}
	return nil
}

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

func (p *Package) GenerateFiles() (map[string]string, error) {
	pkgInfo := &typeinference.PkgInfo{
		Dir: p.Dir, WorkspaceRoot: p.WorkspaceRoot, Name: p.Name,
		Decls: p.Decls, Enums: p.Enums, Structs: p.Structs,
		Interfaces: p.Interfaces, Funcs: p.Funcs, Impls: p.Impls,
	}
	infState := typeinference.NewInferState()
	typedInfo, infErr := typeinference.InferPackage(pkgInfo, infState)
	if infErr != nil {
		return nil, infErr
	}

	g := &generator{
		pkg: p, importAliases: p.ImportAliases,
		interfaceByMethod: map[string]string{},
		inherentMethods:   map[string]map[string]*inherentMethod{},
		variantByName:     map[string]string{},
		goSigCache:        map[string]*goPackageSigs{},
		typedInfo:         typedInfo,
	}
	for name, iface := range p.Interfaces {
		for _, m := range iface.Methods {
			g.interfaceByMethod[m.Name] = name
		}
	}
	for enumName, enum := range p.Enums {
		for _, variant := range enum.Variants {
			g.variantByName[variant.Name] = enumName
		}
	}
	for _, impl := range p.Impls {
		if impl.InterfaceName != "" || impl.Name != "" {
			continue
		}
		receiverName := inherentReceiverName(impl.Type)
		if receiverName == "" {
			continue
		}
		if g.inherentMethods[receiverName] == nil {
			g.inherentMethods[receiverName] = map[string]*inherentMethod{}
		}
		for _, m := range impl.Methods {
			g.inherentMethods[receiverName][m.Name] = &inherentMethod{Impl: impl, Func: m}
		}
	}

	files := make(map[string][]Decl)
	for _, decl := range p.Decls {
		files[sourceFileOf(decl)] = append(files[sourceFileOf(decl)], decl)
	}

	importSpecs := p.sortedImports()
	if g.needsCallAny && !hasImportPath(importSpecs, "reflect") {
		importSpecs = append(importSpecs, importSpec{Path: "reflect"})
		sort.Slice(importSpecs, func(i, j int) bool {
			if importSpecs[i].Alias == importSpecs[j].Alias {
				return importSpecs[i].Path < importSpecs[j].Path
			}
			return importSpecs[i].Alias < importSpecs[j].Alias
		})
	}

	result := make(map[string]string)
	hktEmitted := false

	sortedSourceFiles := make([]string, 0, len(files))
	for name := range files {
		sortedSourceFiles = append(sortedSourceFiles, name)
	}
	sort.Strings(sortedSourceFiles)

	// Generate prelude declarations to zz_prelude.gen.go if present.
	if preludeDecls, ok := files[""]; ok {
		pfile := jen.NewFile(p.Name)
		pfile.HeaderComment("Code generated by mygo; DO NOT EDIT.")
		if declsHaveInterface(preludeDecls) {
			g.genHKTType(pfile)
			hktEmitted = true
		}
		genImportCode(pfile, g.needsCallAny, p)
		for _, decl := range preludeDecls {
			switch d := decl.(type) {
			case *EnumDecl:
				for _, item := range g.genEnum(d) {
					pfile.Add(item)
				}
			case *StructDecl:
				pfile.Add(g.genStruct(d)...)
			case *InterfaceDecl:
				pfile.Add(g.genInterface(d)...)
			}
		}
		for _, decl := range preludeDecls {
			if impl, ok := decl.(*ImplDecl); ok {
				implCode, err := g.genImpl(impl)
				if err != nil {
					return nil, err
				}
				for _, item := range implCode {
					pfile.Add(item)
				}
			}
		}
		for _, decl := range preludeDecls {
			if fn, ok := decl.(*FuncDecl); ok {
				fnCode, err := g.genFunc(fn)
				if err != nil {
					return nil, err
				}
				pfile.Add(fnCode)
			}
		}
		if g.needsCallAny {
			for _, item := range g.genHelpers() {
				pfile.Add(item)
			}
		}
		pout, err := renderFile(pfile)
		if err != nil {
			return nil, err
		}
		result["zz_prelude.gen.go"] = pout
		delete(files, "")
	}

	// Recompute sortedSourceFiles after prelude generation.
	sortedSourceFiles = make([]string, 0, len(files))
	for name := range files {
		sortedSourceFiles = append(sortedSourceFiles, name)
	}
	sort.Strings(sortedSourceFiles)

	// If all remaining files have empty SourceFile (e.g., manually built Packages
	// in tests that don't use loadPackage), fall back to treating all as one file.
	if len(sortedSourceFiles) == 0 && len(p.Decls) > 0 && len(result) == 0 {
		sortedSourceFiles = []string{"__fallback__"}
		files["__fallback__"] = p.Decls
	} else {
		allEmpty := true
		for _, name := range sortedSourceFiles {
			if !skipSourceFile(name) {
				allEmpty = false
				break
			}
		}
		if allEmpty && len(p.Decls) > 0 && len(result) == 0 {
			sortedSourceFiles = []string{"__fallback__"}
			files["__fallback__"] = p.Decls
		}
	}

	for i, sourceFile := range sortedSourceFiles {
		if skipSourceFile(sourceFile) {
			continue
		}
		decls := files[sourceFile]

		file := jen.NewFile(p.Name)
		file.HeaderComment("Code generated by mygo; DO NOT EDIT.")

		if !hktEmitted && declsHaveInterface(decls) && (p.NoPrelude || p.Name == "prelude") {
			g.genHKTType(file)
			hktEmitted = true
		}
		genImportCode(file, g.needsCallAny, p)

		for _, decl := range decls {
			switch d := decl.(type) {
			case *EnumDecl:
				for _, item := range g.genEnum(d) {
					file.Add(item)
				}
			case *StructDecl:
				file.Add(g.genStruct(d)...)
			case *InterfaceDecl:
				file.Add(g.genInterface(d)...)
			}
		}

		for _, decl := range decls {
			if impl, ok := decl.(*ImplDecl); ok {
				implCode, err := g.genImpl(impl)
				if err != nil {
					return nil, err
				}
				for _, item := range implCode {
					file.Add(item)
				}
			}
		}

		for _, decl := range decls {
			if fn, ok := decl.(*FuncDecl); ok {
				fnCode, err := g.genFunc(fn)
				if err != nil {
					return nil, err
				}
				file.Add(fnCode)
			}
		}

		for _, decl := range decls {
			if s, ok := decl.(*LetStmt); ok {
				code, _, err := g.translateExpr(s.Value, &exprCtx{
					locals: map[string]string{}, bindings: map[string]string{},
					sourceTypes: map[string]string{}, mutable: map[string]bool{},
					typeParams: map[string]struct{}{}, constraintFuncs: map[string]string{},
				}, g.goType(s.Type, nil))
				if err != nil {
					return nil, common.ErrorAtPos(s.Line, s.Column, "global binding %q: %v", s.Name, err)
				}
				actual := sanitizeIdent(s.Name)
				if actual == "" || actual == "_" {
					actual = "tmp"
				}
				var stmt jen.Code
				if s.Name == "_" {
					stmt = jen.Var().Id("_").Op("=").Add(code)
				} else {
					v := jen.Var().Id(actual)
					if s.Type != nil {
						v.Add(jenTypeExpr(s.Type))
					}
					v.Op("=").Add(code)
					stmt = v
				}
				file.Add(stmt)
			}
		}

		if g.needsCallAny && i == len(sortedSourceFiles)-1 {
			for _, item := range g.genHelpers() {
				file.Add(item)
			}
		}

		out, err := renderFile(file)
		if err != nil {
			return nil, err
		}
		// Inject the dot import for prelude if needed.
		if p.Name != "prelude" && !p.NoPrelude {
			// Find the import block and insert . "prelude_path".
			// The import block looks like:
			//   import (
			//   "bufio"
			//   ...
			//   )
			preludeImport := "\t. \"github.com/mygo-lang/mygo/lib/prelude\"\n"
			// Insert after the first "import (\n" line.
			importMarker := "import (\n"
			if idx := strings.Index(out, importMarker); idx >= 0 {
				out = out[:idx+len(importMarker)] + preludeImport + out[idx+len(importMarker):]
			} else {
				if pkgIdx := strings.Index(out, "package "); pkgIdx >= 0 {
					if afterPkg := strings.Index(out[pkgIdx:], "\n\n"); afterPkg >= 0 {
						insertAt := pkgIdx + afterPkg + len("\n\n")
						out = out[:insertAt] + "import (\n" + preludeImport + ")\n\n" + out[insertAt:]
					}
				}
			}
		}
		result[sourceToGenName(sourceFile)] = out
	}

	return result, nil
}

func (p *Package) Generate() (string, error) {
	files, err := p.GenerateFiles()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	for name, src := range files {
		buf.WriteString("// === " + name + " ===\n\n")
		buf.WriteString(src)
		buf.WriteString("\n\n")
	}
	return buf.String(), nil
}
