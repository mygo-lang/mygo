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
		pi, _, err := loadPackage(dir, noPrelude)
		if err != nil {
			return nil, err
		}
		pi.WorkspaceRoot = root
		if prev, ok := idx.packages[pi.Name]; ok {
			return nil, common.ErrorAtPos("", 0, 0, "package name %q conflicts between %q and %q", pi.Name, prev.Dir, dir)
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
	for _, start := range []string{fromDir, workspaceRoot} {
		if dir := resolveGoModuleImportDir(start, importPath); dir != "" {
			return dir, nil
		}
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
	return "", common.ErrorAtPos("", 0, 0, "cannot resolve MyGO import %q from %q", importPath, fromDir)
}

func resolveGoModuleImportDir(start, importPath string) string {
	if start == "" {
		return ""
	}
	root := findGoModuleRoot(start)
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	if dir := moduleImportDir(root, modulePath(data), importPath); dir != "" {
		return dir
	}
	for _, repl := range goModReplaceEntries(data) {
		if dir := moduleImportDir(resolveReplaceRoot(root, repl.Path), repl.Module, importPath); dir != "" {
			return dir
		}
	}
	for _, req := range goModRequireEntries(data) {
		if suffix, ok := moduleImportSuffix(req.Module, importPath); ok {
			for _, cacheRoot := range goModCacheRoots() {
				modRoot := filepath.Join(cacheRoot, moduleCachePath(req.Module, req.Version))
				if dir := existingDir(filepath.Join(modRoot, filepath.FromSlash(suffix))); dir != "" {
					return dir
				}
			}
		}
	}
	return ""
}

type goModReplaceEntry struct {
	Module string
	Path   string
}

type goModRequireEntry struct {
	Module  string
	Version string
}

func moduleImportDir(root, module, importPath string) string {
	if root == "" || module == "" {
		return ""
	}
	suffix, ok := moduleImportSuffix(module, importPath)
	if !ok {
		return ""
	}
	return existingDir(filepath.Join(root, filepath.FromSlash(suffix)))
}

func moduleImportSuffix(module, importPath string) (string, bool) {
	if importPath == module {
		return "", true
	}
	prefix := module + "/"
	if strings.HasPrefix(importPath, prefix) {
		return strings.TrimPrefix(importPath, prefix), true
	}
	return "", false
}

func resolveReplaceRoot(moduleRoot, repl string) string {
	if repl == "" || strings.HasPrefix(repl, ".") || filepath.IsAbs(repl) {
		if !filepath.IsAbs(repl) {
			repl = filepath.Join(moduleRoot, repl)
		}
		return filepath.Clean(repl)
	}
	return ""
}

func existingDir(path string) string {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return filepath.Clean(path)
	}
	return ""
}

func goModReplaceEntries(goMod []byte) []goModReplaceEntry {
	var entries []goModReplaceEntry
	inReplaceBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		line = cleanGoModLine(line)
		if line == "" {
			continue
		}
		if line == "replace (" {
			inReplaceBlock = true
			continue
		}
		if inReplaceBlock && line == ")" {
			inReplaceBlock = false
			continue
		}
		if strings.HasPrefix(line, "replace ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "replace "))
		} else if !inReplaceBlock {
			continue
		}
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "=>" && i > 0 && i+1 < len(fields) {
				entries = append(entries, goModReplaceEntry{Module: fields[0], Path: fields[i+1]})
				break
			}
		}
	}
	return entries
}

func goModRequireEntries(goMod []byte) []goModRequireEntry {
	var entries []goModRequireEntry
	inRequireBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		line = cleanGoModLine(line)
		if line == "" {
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else if !inRequireBlock {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			entries = append(entries, goModRequireEntry{Module: fields[0], Version: fields[1]})
		}
	}
	return entries
}

func cleanGoModLine(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.Index(line, "//"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line
}

func loadImportedMyGoPackage(workspaceRoot, fromDir, importPath string, noPrelude bool) (*pkg.Package, error) {
	dir, err := resolveMyGoImport(workspaceRoot, fromDir, importPath)
	if err != nil {
		return nil, err
	}
	mainPkg, _, err := loadPackage(dir, noPrelude)
	if err != nil {
		return nil, err
	}
	return mainPkg, nil
}
