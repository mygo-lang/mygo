package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"
	"github.com/mygo-lang/mygo/internal/mygo/pkg"
)

// CompileDir compiles all .mygo files in a directory, generating one .gen.go file per source.
// Returns the list of generated file paths.
func CompileDir(dir string) ([]string, error) {
	return compileDir(dir, filepath.Dir(dir), false)
}

// CompileDirNoPrelude compiles a directory without auto-importing prelude declarations.
func CompileDirNoPrelude(dir string) ([]string, error) {
	return compileDir(dir, filepath.Dir(dir), true)
}

func compileDir(dir, workspaceRoot string, noPrelude bool) ([]string, error) {
	p, err := loadPackage(dir, noPrelude)
	if err != nil {
		return nil, err
	}
	p.WorkspaceRoot = workspaceRoot

	// Merge declarations from all imported MyGO packages (non-go: imports)
	// so their interfaces, impls, enums, and structs are available for
	// method resolution during code generation (typeclass matching, etc.).
	if !noPrelude && p.Name != "prelude" {
		for _, path := range p.ImportAliases {
			if strings.HasPrefix(path, "go:") {
				continue
			}
			imported, err := loadImportedMyGoPackage(workspaceRoot, dir, path, true)
			if err != nil {
				continue
			}
			mergeImportedDecls(p, imported)
		}
		// Also merge prelude (auto-imported at Go level, not in ImportAliases).
		// Try to find prelude directory: look relative to workspaceRoot, then walk up.
		if preludePkg := loadPreludePackage(dir, workspaceRoot); preludePkg != nil {
			mergeImportedDecls(p, preludePkg)
		}
	}

	files, err := codegen.GenerateFiles(p)
	if err != nil {
		return nil, err
	}

	var written []string
	for genFilename, src := range files {
		out := filepath.Join(dir, genFilename)
		if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
			return nil, err
		}
		written = append(written, out)
	}
	sort.Strings(written)
	return written, nil
}

func Sync(root string) ([]string, error) {
	return syncDir(root, false)
}

// SyncNoPrelude walks root and compiles all found .mygo packages,
// skipping the prelude auto-import for each package.
func SyncNoPrelude(root string) ([]string, error) {
	return syncDir(root, true)
}

func syncDir(root string, noPrelude bool) ([]string, error) {
	if _, err := buildPackageIndex(root, noPrelude); err != nil {
		return nil, err
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, "bak") || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dirs, err := mygoDirs(root)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, dir := range dirs {
		var out []string
		if noPrelude {
			out, err = compileDir(dir, root, true)
		} else {
			out, err = compileDir(dir, root, false)
		}
		if err != nil {
			return nil, err
		}
		written = append(written, out...)
	}
	sort.Strings(written)
	return written, nil
}

// loadPreludePackage finds and loads the prelude package by trying various paths.
func loadPreludePackage(dir, workspaceRoot string) *pkg.Package {
	candidates := []string{
		filepath.Join(workspaceRoot, "prelude"),
		filepath.Join(dir, "prelude"),
		filepath.Join(dir, "..", "prelude"),
		filepath.Join(dir, "..", "..", "prelude"),
		"prelude",
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		if st, err := os.Stat(abs); err == nil && st.IsDir() {
			if pkg, err := loadPackage(abs, true); err == nil {
				return pkg
			}
		}
	}
	return nil
}

// mergeImportedDecls merges declarations from an imported MyGO package into
// the user package for method resolution during code generation.
func mergeImportedDecls(userPkg, importedPkg *pkg.Package) {
	for name, iface := range importedPkg.Interfaces {
		if _, exists := userPkg.Interfaces[name]; !exists {
			userPkg.Interfaces[name] = iface
		}
	}
	for name, enum := range importedPkg.Enums {
		if _, exists := userPkg.Enums[name]; !exists {
			userPkg.Enums[name] = enum
		}
	}
	for _, impl := range importedPkg.Impls {
		dup := false
		for _, existing := range userPkg.Impls {
			if existing == impl {
				dup = true
				break
			}
		}
		if !dup {
			userPkg.Impls = append(userPkg.Impls, impl)
		}
	}
}

func mygoDirs(root string) ([]string, error) {
	seen := map[string]struct{}{}
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if path != root && (strings.HasPrefix(base, "bak") || base == ".git" || base == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".mygo") {
			dir := filepath.Dir(path)
			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				dirs = append(dirs, dir)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

// setSourceFile sets the SourceFile field on all decls in a parsed file.
func setSourceFile(file *parserpkg.File, sourceFile string) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			d.SourceFile = sourceFile
		case *EnumDecl:
			d.SourceFile = sourceFile
		case *StructDecl:
			d.SourceFile = sourceFile
		case *InterfaceDecl:
			d.SourceFile = sourceFile
		case *ImplDecl:
			d.SourceFile = sourceFile
		case *FuncDecl:
			d.SourceFile = sourceFile
		case *LetStmt:
			d.SourceFile = sourceFile
		}
	}
}

func loadPackage(dir string, noPrelude bool) (*pkg.Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	p := &pkg.Package{
		Dir:           dir,
		WorkspaceRoot: filepath.Dir(dir),
		NoPrelude:     noPrelude,
		Imports:       map[string]struct{}{},
		ImportAliases: map[string]string{},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
	}
	pkgName := ""
	var fileDecls []Decl
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") || strings.HasSuffix(name, ".gen.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		parsed, err := parserpkg.ParseFile(filepath.Join(dir, name), string(src))
		if err != nil {
			return nil, err
		}
		if parsed.PackageName != "" {
			if pkgName == "" {
				pkgName = parsed.PackageName
			} else if pkgName != parsed.PackageName {
				return nil, common.ErrorAtPos(parsed.PackageLine, 0, "package %q conflicts with %q", parsed.PackageName, pkgName)
			}
		}
		setSourceFile(parsed, name)
		fileDecls = append(fileDecls, parsed.Decls...)
	}
	if pkgName == "" {
		pkgName = filepath.Base(dir)
	}
	p.Name = toPackageName(pkgName)
	p.Decls = append(p.Decls, fileDecls...)
	for _, decl := range p.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			p.Imports[d.Path] = struct{}{}
			p.ImportDecls = append(p.ImportDecls, d)
			alias := d.Alias
			if alias == "" {
				alias = importAliasForPath(d.Path)
			}
			if prev, ok := p.ImportAliases[alias]; ok && prev != d.Path {
				return nil, common.ErrorAtPos(d.Line, d.Column, "import alias %q conflicts between %q and %q", alias, prev, d.Path)
			}
			p.ImportAliases[alias] = d.Path
		case *EnumDecl:
			p.Enums[d.Name] = d
		case *StructDecl:
			p.Structs[d.Name] = d
		case *InterfaceDecl:
			p.Interfaces[d.Name] = d
		case *FuncDecl:
			p.Funcs[d.Name] = d
		case *ImplDecl:
			p.Impls = append(p.Impls, d)
		}
	}
	return p, nil
}
