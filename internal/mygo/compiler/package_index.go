package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	"github.com/mygo-lang/mygo/internal/mygo/pkg"
)

type packageIndex struct {
	root     string
	packages map[string]*pkg.Package
	byDir    map[string]*pkg.Package
}

type packageExports struct {
	Name          string
	Funcs         map[string]*FuncDecl
	Structs       map[string]*StructDecl
	Enums         map[string]*EnumDecl
	Interfaces    map[string]*InterfaceDecl
	ImportAliases map[string]string
}

func buildPackageIndex(root string, noPrelude bool) (*packageIndex, error) {
	dirs, err := mygoDirs(root)
	if err != nil {
		return nil, err
	}
	idx := &packageIndex{
		root:     root,
		packages: map[string]*pkg.Package{},
		byDir:    map[string]*pkg.Package{},
	}
	for _, dir := range dirs {
		pi, err := loadPackage(dir, noPrelude)
		if err != nil {
			return nil, err
		}
		pi.WorkspaceRoot = root
		if prev, ok := idx.packages[pi.Name]; ok {
			return nil, common.ErrorAtPos(0, 0, "package name %q conflicts between %q and %q", pi.Name, prev.Dir, dir)
		}
		idx.packages[pi.Name] = pi
		idx.byDir[dir] = pi
	}
	return idx, nil
}

func (idx *packageIndex) packageForDir(dir string) (*pkg.Package, bool) {
	pi, ok := idx.byDir[dir]
	return pi, ok
}

func (idx *packageIndex) exportsFor(name string) (*packageExports, bool) {
	pi, ok := idx.packages[name]
	if !ok {
		return nil, false
	}
	return exportView(pi), true
}

func exportView(p *pkg.Package) *packageExports {
	exports := &packageExports{
		Name:          p.Name,
		Funcs:         map[string]*FuncDecl{},
		Structs:       map[string]*StructDecl{},
		Enums:         map[string]*EnumDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		ImportAliases: map[string]string{},
	}
	for name, fn := range p.Funcs {
		if isExportedIdent(name) {
			exports.Funcs[name] = fn
		}
	}
	for name, st := range p.Structs {
		if isExportedIdent(name) {
			exports.Structs[name] = st
		}
	}
	for name, en := range p.Enums {
		if isExportedIdent(name) {
			exports.Enums[name] = en
		}
	}
	for name, iface := range p.Interfaces {
		if isExportedIdent(name) {
			exports.Interfaces[name] = iface
		}
	}
	for alias, path := range p.ImportAliases {
		exports.ImportAliases[alias] = path
	}
	return exports
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

func resolveMyGoImport(workspaceRoot, fromDir, importPath string) (string, error) {
	if filepath.IsAbs(importPath) {
		return importPath, nil
	}
	if strings.HasPrefix(importPath, ".") {
		return filepath.Clean(filepath.Join(fromDir, importPath)), nil
	}
	if workspaceRoot != "" {
		candidate := filepath.Clean(filepath.Join(workspaceRoot, importPath))
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	cur := fromDir
	for {
		candidate := filepath.Clean(filepath.Join(cur, importPath))
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", common.ErrorAtPos(0, 0, "cannot resolve MyGO import %q from %q", importPath, fromDir)
}

func loadImportedMyGoPackage(workspaceRoot, fromDir, importPath string, noPrelude bool) (*pkg.Package, error) {
	dir, err := resolveMyGoImport(workspaceRoot, fromDir, importPath)
	if err != nil {
		return nil, err
	}
	return loadPackage(dir, noPrelude)
}
