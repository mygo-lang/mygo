package compiler

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"
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
	pkg, err := loadPackage(dir, noPrelude)
	if err != nil {
		return nil, err
	}
	pkg.WorkspaceRoot = workspaceRoot

	files, err := pkg.GenerateFiles()
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

func loadPackage(dir string, noPrelude bool) (*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkg := &Package{
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
		parsed, err := parserpkg.ParseFile(string(src))
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
	pkg.Name = toPackageName(pkgName)
	if !pkg.NoPrelude {
		preludeDecls, err := loadPreludeDecls()
		if err != nil {
			return nil, err
		}
		pkg.Decls = append(pkg.Decls, preludeDecls...)
	}
	pkg.Decls = append(pkg.Decls, fileDecls...)
	for _, decl := range pkg.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			pkg.Imports[d.Path] = struct{}{}
			pkg.ImportDecls = append(pkg.ImportDecls, d)
			alias := d.Alias
			if alias == "" {
				alias = importAliasForPath(d.Path)
			}
			if prev, ok := pkg.ImportAliases[alias]; ok && prev != d.Path {
				return nil, common.ErrorAtPos(d.Line, d.Column, "import alias %q conflicts between %q and %q", alias, prev, d.Path)
			}
			pkg.ImportAliases[alias] = d.Path
		case *EnumDecl:
			pkg.Enums[d.Name] = d
		case *StructDecl:
			pkg.Structs[d.Name] = d
		case *InterfaceDecl:
			pkg.Interfaces[d.Name] = d
		case *FuncDecl:
			pkg.Funcs[d.Name] = d
		case *ImplDecl:
			pkg.Impls = append(pkg.Impls, d)
		}
	}
	return pkg, nil
}
