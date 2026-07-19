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
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
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
	mainPkg, testPkg, err := loadPackage(dir, noPrelude)
	if err != nil {
		return nil, err
	}
	mainPkg.WorkspaceRoot = workspaceRoot

	// Merge declarations from all imported MyGO packages (non-go: imports)
	// so their interfaces, impls, enums, and structs are available for
	// method resolution during code generation (typeclass matching, etc.).
	if !noPrelude && mainPkg.Name != "prelude" {
		if preludePkg := loadPreludePackage(dir, workspaceRoot); preludePkg != nil {
			if err := mergeImportedDecls(mainPkg, preludePkg, true); err != nil {
				return nil, err
			}
		} else if findGoModuleRoot(dir) != "" || findGoModuleRoot(workspaceRoot) != "" {
			return nil, common.ErrorAtPos("", 0, 0, "cannot locate prelude MyGO sources for auto-import; add github.com/mygo-lang/mygo to go.mod or compile with --no-prelude")
		}
		for _, path := range mainPkg.ImportAliases {
			if strings.HasPrefix(path, "go:") {
				continue
			}
			imported, err := loadImportedMyGoPackage(workspaceRoot, dir, path, true)
			if err != nil {
				continue
			}
			if err := mergeImportedDecls(mainPkg, imported, false); err != nil {
				return nil, err
			}
		}
	}

	var written []string

	// --- Main package: type inference -> validate -> codegen ---
	mainDotImportEnums := map[string]*EnumDecl{}
	if preludePkg := codegen.LoadPreludePackageForEnums(mainPkg.Dir, mainPkg.WorkspaceRoot); preludePkg != nil {
		for name, enum := range preludePkg.Enums {
			mainDotImportEnums[name] = enum
		}
	}
	mainPkgInfo := &typeinference.PkgInfo{
		Dir:            mainPkg.Dir,
		WorkspaceRoot:  mainPkg.WorkspaceRoot,
		Name:           mainPkg.Name,
		Decls:          mainPkg.Decls,
		Enums:          mainPkg.Enums,
		Structs:        mainPkg.Structs,
		Interfaces:     mainPkg.Interfaces,
		Funcs:          mainPkg.Funcs,
		Impls:          mainPkg.Impls,
		DotImportEnums: mainDotImportEnums,
	}
	infState := typeinference.NewInferState()
	mainTypedInfo, err := typeinference.InferPackage(mainPkgInfo, infState)
	if err != nil {
		return nil, err
	}
	if err := Validate(mainPkg, mainTypedInfo); err != nil {
		return nil, err
	}

	files, err := codegen.GenerateFiles(mainPkg, mainTypedInfo)
	if err != nil {
		return nil, err
	}
	for genFilename, src := range files {
		out := filepath.Join(dir, genFilename)
		if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
			return nil, err
		}
		written = append(written, out)
	}

	// --- Test package: type inference -> validate -> codegen ---
	if len(testPkg.Decls) > 0 {
		testPkg.WorkspaceRoot = workspaceRoot

		// Auto-import the main package into test package so exported symbols
		// are accessible. The codegen will add a dot-import for direct access.
		mainImportPath := goImportPathForDir(dir)
		testPkg.ImportAliases[mainPkg.Name] = mainImportPath
		testPkg.Imports[mainImportPath] = struct{}{}
		testPkg.ImportDecls = append(testPkg.ImportDecls, &ImportDecl{
			Path:       mainImportPath,
			Alias:      mainPkg.Name,
			SourceFile: "__auto_import__",
		})
		if !noPrelude && testPkg.Name != "prelude" {
			if preludePkg := loadPreludePackage(dir, workspaceRoot); preludePkg != nil {
				if err := mergeImportedDecls(testPkg, preludePkg, true); err != nil {
					return nil, err
				}
			} else if findGoModuleRoot(dir) != "" || findGoModuleRoot(workspaceRoot) != "" {
				return nil, common.ErrorAtPos("", 0, 0, "cannot locate prelude MyGO sources for auto-import; add github.com/mygo-lang/mygo to go.mod or compile with --no-prelude")
			}
			for _, path := range testPkg.ImportAliases {
				if strings.HasPrefix(path, "go:") {
					continue
				}
				// Skip the auto-imported main package — it's handled separately
				if path == mainImportPath {
					continue
				}
				imported, err := loadImportedMyGoPackage(workspaceRoot, dir, path, true)
				if err != nil {
					continue
				}
				if err := mergeImportedDecls(testPkg, imported, false); err != nil {
					return nil, err
				}
			}
		}

		testDotImportEnums := map[string]*EnumDecl{}
		if preludePkg := codegen.LoadPreludePackageForEnums(testPkg.Dir, testPkg.WorkspaceRoot); preludePkg != nil {
			for name, enum := range preludePkg.Enums {
				testDotImportEnums[name] = enum
			}
		}
		testPkgInfo := &typeinference.PkgInfo{
			Dir:            testPkg.Dir,
			WorkspaceRoot:  testPkg.WorkspaceRoot,
			Name:           testPkg.Name,
			Decls:          testPkg.Decls,
			Enums:          testPkg.Enums,
			Structs:        testPkg.Structs,
			Interfaces:     testPkg.Interfaces,
			Funcs:          testPkg.Funcs,
			Impls:          testPkg.Impls,
			DotImportEnums: testDotImportEnums,
		}
		testInfState := typeinference.NewInferState()
		testTypedInfo, err := typeinference.InferPackage(testPkgInfo, testInfState)
		if err != nil {
			return nil, err
		}
		if err := Validate(testPkg, testTypedInfo); err != nil {
			return nil, err
		}

		testFiles, err := codegen.GenerateFiles(testPkg, testTypedInfo)
		if err != nil {
			return nil, err
		}
		for genFilename, src := range testFiles {
			out := filepath.Join(dir, genFilename)
			if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
				return nil, err
			}
			written = append(written, out)
		}
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
	var candidates []string
	if root := findMyGoModuleRoot(dir); root != "" {
		candidates = append(candidates, filepath.Join(root, "prelude"))
	}
	if root := findMyGoModuleRoot(workspaceRoot); root != "" {
		candidates = append(candidates, filepath.Join(root, "prelude"))
	}
	if root := findMyGoDependencyRoot(dir); root != "" {
		candidates = append(candidates, filepath.Join(root, "prelude"))
	}
	if root := findMyGoDependencyRoot(workspaceRoot); root != "" {
		candidates = append(candidates, filepath.Join(root, "prelude"))
	}
	candidates = append(candidates, filepath.Join(workspaceRoot, "prelude"))
	candidates = append(candidates, "prelude")
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
			if mainPkg, _, err := loadPackage(abs, true); err == nil {
				return mainPkg
			}
		}
	}
	return nil
}

func findMyGoModuleRoot(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for cur := absStart; ; cur = filepath.Dir(cur) {
		data, err := os.ReadFile(filepath.Join(cur, "go.mod"))
		if err == nil && modulePath(data) == "github.com/mygo-lang/mygo" {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
	}
}

func findGoModuleRoot(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for cur := absStart; ; cur = filepath.Dir(cur) {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
	}
}

func modulePath(goMod []byte) string {
	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func goImportPathForDir(dir string) string {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	root := findGoModuleRoot(absDir)
	if root == "" {
		return filepath.Base(dir)
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return filepath.Base(dir)
	}
	mod := modulePath(data)
	if mod == "" {
		return filepath.Base(dir)
	}
	rel, err := filepath.Rel(root, absDir)
	if err != nil || rel == "." {
		return mod
	}
	return mod + "/" + filepath.ToSlash(rel)
}

func findMyGoDependencyRoot(start string) string {
	root := findGoModuleRoot(start)
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	if modulePath(data) == "github.com/mygo-lang/mygo" {
		return root
	}
	repl := goModReplacePath(data, "github.com/mygo-lang/mygo")
	if repl != "" {
		if !filepath.IsAbs(repl) {
			repl = filepath.Join(root, repl)
		}
		repl = filepath.Clean(repl)
		if st, err := os.Stat(filepath.Join(repl, "go.mod")); err == nil && !st.IsDir() {
			return repl
		}
	}
	version := goModRequireVersion(data, "github.com/mygo-lang/mygo")
	if version == "" {
		return ""
	}
	for _, cacheRoot := range goModCacheRoots() {
		modRoot := filepath.Join(cacheRoot, moduleCachePath("github.com/mygo-lang/mygo", version))
		if st, err := os.Stat(filepath.Join(modRoot, "go.mod")); err == nil && !st.IsDir() {
			return modRoot
		}
	}
	return ""
}

func goModReplacePath(goMod []byte, module string) string {
	inReplaceBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
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
			if f == "=>" && i > 0 && fields[0] == module && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return ""
}

func goModRequireVersion(goMod []byte, module string) string {
	inRequireBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
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
		if len(fields) >= 2 && fields[0] == module {
			return fields[1]
		}
	}
	return ""
}

func goModCacheRoots() []string {
	var roots []string
	if gomodcache := os.Getenv("GOMODCACHE"); gomodcache != "" {
		roots = append(roots, gomodcache)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, p := range filepath.SplitList(gopath) {
			if p != "" {
				roots = append(roots, filepath.Join(p, "pkg", "mod"))
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, filepath.Join(home, "go", "pkg", "mod"))
	}
	seen := map[string]bool{}
	out := roots[:0]
	for _, root := range roots {
		root = filepath.Clean(root)
		if !seen[root] {
			out = append(out, root)
			seen[root] = true
		}
	}
	return out
}

func moduleCachePath(module, version string) string {
	parts := strings.Split(module, "/")
	for i, p := range parts {
		parts[i] = escapeModulePathElem(p)
	}
	return filepath.Join(strings.Join(parts, string(filepath.Separator)) + "@" + version)
}

func escapeModulePathElem(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// mergeImportedDecls merges declarations from an imported MyGO package into
// the user package for method resolution during code generation.
func mergeImportedDecls(userPkg, importedPkg *pkg.Package, conflictOnExisting bool) error {
	for name, st := range importedPkg.Structs {
		if _, exists := userPkg.Structs[name]; !exists {
			userPkg.Structs[name] = st
		} else if conflictOnExisting && st != userPkg.Structs[name] {
			return common.ErrorAtPos("", 0, 0, "type %q conflicts with imported package %q", name, importedPkg.Name)
		}
	}
	for name, iface := range importedPkg.Interfaces {
		if _, exists := userPkg.Interfaces[name]; !exists {
			userPkg.Interfaces[name] = iface
		} else if conflictOnExisting && iface != userPkg.Interfaces[name] {
			return common.ErrorAtPos("", 0, 0, "type %q conflicts with imported package %q", name, importedPkg.Name)
		}
	}
	for name, enum := range importedPkg.Enums {
		if _, exists := userPkg.Enums[name]; !exists {
			userPkg.Enums[name] = enum
		} else if conflictOnExisting && enum != userPkg.Enums[name] {
			return common.ErrorAtPos("", 0, 0, "type %q conflicts with imported package %q", name, importedPkg.Name)
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
	return nil
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

func displayPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	wd, err := os.Getwd()
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(wd, abs)
	if err != nil {
		return abs
	}
	return rel
}

func loadPackage(dir string, noPrelude bool) (*pkg.Package, *pkg.Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	mainPkg := &pkg.Package{
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
	testPkg := &pkg.Package{
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

	var mainDecls, testDecls []Decl
	mainPkgName := ""
	testPkgName := ""

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") || strings.HasSuffix(name, ".gen.go") {
			continue
		}
		sourcePath := filepath.Join(dir, name)
		src, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, nil, err
		}
		displaySourcePath := displayPath(sourcePath)
		parsed, err := parserpkg.ParseFile(displaySourcePath, string(src))
		if err != nil {
			return nil, nil, err
		}
		setSourceFile(parsed, displaySourcePath)

		pkgName := parsed.PackageName
		isSplitTestPackage := strings.HasSuffix(pkgName, "_test")
		if isSplitTestPackage {
			if testPkgName == "" {
				testPkgName = pkgName
			} else if testPkgName != pkgName {
				return nil, nil, common.ErrorAtPos(displaySourcePath, parsed.PackageLine, 0, "package %q conflicts with %q", pkgName, testPkgName)
			}
			testDecls = append(testDecls, parsed.Decls...)
		} else {
			// Main file.
			if pkgName != "" {
				if mainPkgName == "" {
					mainPkgName = pkgName
				} else if mainPkgName != pkgName {
					return nil, nil, common.ErrorAtPos(displaySourcePath, parsed.PackageLine, 0, "package %q conflicts with %q", pkgName, mainPkgName)
				}
			}
			mainDecls = append(mainDecls, parsed.Decls...)
		}
	}

	// Build main package.
	if mainPkgName == "" {
		mainPkgName = filepath.Base(dir)
	}
	mainPkg.Name = toPackageName(mainPkgName)
	mainPkg.Decls = append(mainPkg.Decls, mainDecls...)
	for _, decl := range mainPkg.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			mainPkg.Imports[d.Path] = struct{}{}
			mainPkg.ImportDecls = append(mainPkg.ImportDecls, d)
			alias := d.Alias
			if alias == "" {
				alias = importAliasForPath(d.Path)
			}
			if prev, ok := mainPkg.ImportAliases[alias]; ok && prev != d.Path {
				return nil, nil, common.ErrorAtPos(d.SourceFile, d.Line, d.Column, "import alias %q conflicts between %q and %q", alias, prev, d.Path)
			}
			mainPkg.ImportAliases[alias] = d.Path
		case *EnumDecl:
			mainPkg.Enums[d.Name] = d
		case *StructDecl:
			mainPkg.Structs[d.Name] = d
		case *InterfaceDecl:
			mainPkg.Interfaces[d.Name] = d
		case *FuncDecl:
			mainPkg.Funcs[d.Name] = d
		case *ImplDecl:
			mainPkg.Impls = append(mainPkg.Impls, d)
		}
	}

	// Build test package.
	if testPkgName == "" {
		testPkgName = filepath.Base(dir) + "_test"
	}
	testPkg.Name = toPackageName(testPkgName)
	testPkg.Decls = append(testPkg.Decls, testDecls...)
	for _, decl := range testPkg.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			testPkg.Imports[d.Path] = struct{}{}
			testPkg.ImportDecls = append(testPkg.ImportDecls, d)
			alias := d.Alias
			if alias == "" {
				alias = importAliasForPath(d.Path)
			}
			if prev, ok := testPkg.ImportAliases[alias]; ok && prev != d.Path {
				return nil, nil, common.ErrorAtPos(d.SourceFile, d.Line, d.Column, "import alias %q conflicts between %q and %q", alias, prev, d.Path)
			}
			testPkg.ImportAliases[alias] = d.Path
		case *EnumDecl:
			testPkg.Enums[d.Name] = d
		case *StructDecl:
			testPkg.Structs[d.Name] = d
		case *InterfaceDecl:
			testPkg.Interfaces[d.Name] = d
		case *FuncDecl:
			testPkg.Funcs[d.Name] = d
		case *ImplDecl:
			testPkg.Impls = append(testPkg.Impls, d)
		}
	}

	return mainPkg, testPkg, nil
}
