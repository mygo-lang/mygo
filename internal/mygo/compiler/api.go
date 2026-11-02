package compiler

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
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
		TypeAliases:    mainPkg.TypeAliases,
		Types:          mainPkg.Types,
		Enums:          mainPkg.Enums,
		Structs:        mainPkg.Structs,
		Interfaces:     mainPkg.Interfaces,
		Funcs:          mainPkg.Funcs,
		Impls:          mainPkg.Impls,
		DotImportTypes: mainPkg.DotImportTypes,
		DotImportEnums: mainDotImportEnums,
		DotImportFuncs: mainPkg.DotImportFuncs,
	}
	expandTypeAliases(mainPkg)
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
			TypeAliases:    testPkg.TypeAliases,
			Types:          testPkg.Types,
			Enums:          testPkg.Enums,
			Structs:        testPkg.Structs,
			Interfaces:     testPkg.Interfaces,
			Funcs:          testPkg.Funcs,
			Impls:          testPkg.Impls,
			DotImportTypes: testPkg.DotImportTypes,
			DotImportEnums: testDotImportEnums,
			DotImportFuncs: testPkg.DotImportFuncs,
		}
		expandTypeAliases(testPkg)
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

// codegen3PackageOutputDir maps a package source directory below outputRoot.
// In-module packages retain their relative layout; external packages use an
// escaped canonical import path and never write beside their source.
func codegen3PackageOutputDir(outputRoot, moduleRoot, packageDir, importPath string) (string, error) {
	root, err := filepath.Abs(outputRoot)
	if err != nil {
		return "", fmt.Errorf("codegen3 output-path: resolve output root %q: %w", outputRoot, err)
	}
	if canonical, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = canonical
	}
	module, err := filepath.Abs(moduleRoot)
	if err != nil {
		return "", fmt.Errorf("codegen3 output-path: resolve module root %q: %w", moduleRoot, err)
	}
	if canonical, evalErr := filepath.EvalSymlinks(module); evalErr == nil {
		module = canonical
	}
	source, err := filepath.Abs(packageDir)
	if err != nil {
		return "", fmt.Errorf("codegen3 output-path: resolve package directory %q: %w", packageDir, err)
	}
	if canonical, evalErr := filepath.EvalSymlinks(source); evalErr == nil {
		source = canonical
	}

	rel, err := filepath.Rel(module, source)
	if err != nil {
		return "", fmt.Errorf("codegen3 output-path: make package path relative to module root: %w", err)
	}
	if rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(filepath.ToSlash(rel), "../") {
		if importPath == "" || importPath == "." || importPath == ".." {
			return "", fmt.Errorf("codegen3 output-path: external package %q has no canonical import path", packageDir)
		}
		externalModule := findGoModuleRoot(source)
		if externalModule == "" {
			return "", fmt.Errorf("codegen3 output-path: external package %q has no module root", packageDir)
		}
		moduleData, readErr := os.ReadFile(filepath.Join(externalModule, "go.mod"))
		if readErr != nil {
			return "", fmt.Errorf("codegen3 output-path: read module file for %q: %w", packageDir, readErr)
		}
		externalModulePath := modulePath(moduleData)
		packageRel, relErr := filepath.Rel(externalModule, source)
		if relErr != nil || packageRel == ".." || filepath.IsAbs(packageRel) || strings.HasPrefix(filepath.ToSlash(packageRel), "../") {
			return "", fmt.Errorf("codegen3 output-path: package %q is outside its module root %q", packageDir, externalModule)
		}
		canonicalImport := externalModulePath
		if packageRel != "." {
			canonicalImport += "/" + filepath.ToSlash(packageRel)
		}
		if externalModulePath == "" || canonicalImport != importPath {
			return "", fmt.Errorf("codegen3 output-path: import path %q does not match module path %q for %q", importPath, canonicalImport, packageDir)
		}
		// Keep each module tree in a stable, import-path-derived namespace;
		// package subdirectories mirror their paths within the module.
		rel = filepath.Join("external", url.PathEscape(externalModulePath), packageRel)
	}

	candidate := filepath.Join(root, rel)
	if err := verifyContainedOutputPath(root, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

// codegen3PackageOutputLocation returns the generated package directory,
// staged module root (empty for in-module packages), and module path (empty for
// in-module packages). External module trees are copied once per build root.
func codegen3PackageOutputLocation(outputRoot, moduleRoot, packageDir, importPath string) ([]string, error) {
	outputDir, err := codegen3PackageOutputDir(outputRoot, moduleRoot, packageDir, importPath)
	if err != nil {
		return nil, err
	}
	moduleAbs, err := filepath.Abs(moduleRoot)
	if err != nil {
		return nil, fmt.Errorf("codegen3 output-path: resolve module root %q: %w", moduleRoot, err)
	}
	sourceAbs, err := filepath.Abs(packageDir)
	if err != nil {
		return nil, fmt.Errorf("codegen3 output-path: resolve package directory %q: %w", packageDir, err)
	}
	if resolved, evalErr := filepath.EvalSymlinks(moduleAbs); evalErr == nil {
		moduleAbs = resolved
	}
	if resolved, evalErr := filepath.EvalSymlinks(sourceAbs); evalErr == nil {
		sourceAbs = resolved
	}
	if err := pathWithinRoot(moduleAbs, sourceAbs); err == nil {
		return []string{outputDir, "", ""}, nil
	}

	externalModule := findGoModuleRoot(sourceAbs)
	if externalModule == "" {
		return nil, fmt.Errorf("codegen3 output-path: external package %q has no module root", packageDir)
	}
	moduleData, err := os.ReadFile(filepath.Join(externalModule, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("codegen3 output-path: read module file for %q: %w", packageDir, err)
	}
	moduleImportPath := modulePath(moduleData)
	if moduleImportPath == "" {
		return nil, fmt.Errorf("codegen3 output-path: no module path in %q", filepath.Join(externalModule, "go.mod"))
	}
	stageRoot, err := filepath.Abs(filepath.Join(outputRoot, "external", url.PathEscape(moduleImportPath)))
	if err != nil {
		return nil, fmt.Errorf("codegen3 output-path: resolve staged module path: %w", err)
	}
	if err := verifyContainedOutputPath(outputRoot, stageRoot); err != nil {
		return nil, err
	}
	if err := codegen3CopyModuleTree(externalModule, stageRoot, outputRoot); err != nil {
		return nil, err
	}
	return []string{outputDir, stageRoot, moduleImportPath}, nil
}

func codegen3ManifestDirectory(moduleRoot, packageDir, stagedPackageDir string) string {
	module, moduleErr := filepath.Abs(moduleRoot)
	source, sourceErr := filepath.Abs(packageDir)
	if moduleErr == nil && sourceErr == nil {
		if resolved, err := filepath.EvalSymlinks(module); err == nil {
			module = resolved
		}
		if resolved, err := filepath.EvalSymlinks(source); err == nil {
			source = resolved
		}
		if pathWithinRoot(module, source) == nil {
			return source
		}
	}
	return stagedPackageDir
}

func codegen3IsExternalPackage(moduleRoot, packageDir string) bool {
	module, moduleErr := filepath.Abs(moduleRoot)
	source, sourceErr := filepath.Abs(packageDir)
	if moduleErr != nil || sourceErr != nil {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(module); err == nil {
		module = resolved
	}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	}
	return pathWithinRoot(module, source) != nil
}

func codegen3CopyModuleTree(sourceRoot, stageRoot, outputRoot string) error {
	if _, err := os.Stat(filepath.Join(stageRoot, "go.mod")); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("codegen3 stage-module: inspect %q: %w", stageRoot, err)
	}
	if err := verifyContainedOutputPath(outputRoot, stageRoot); err != nil {
		return err
	}
	parent := filepath.Dir(stageRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("codegen3 stage-module: create %q: %w", parent, err)
	}
	tmp, err := os.MkdirTemp(parent, ".module-stage-*")
	if err != nil {
		return fmt.Errorf("codegen3 stage-module: create temporary module copy: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := filepath.WalkDir(sourceRoot, func(source string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceRoot, source)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Name() == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("codegen3 stage-module: refusing symlink %q", source)
		}
		destination := filepath.Join(tmp, rel)
		if err := verifyContainedOutputPath(tmp, destination); err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("codegen3 stage-module: unsupported file type at %q", source)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		in, err := os.Open(source)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeOutErr := out.Close()
		closeInErr := in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutErr != nil {
			return closeOutErr
		}
		return closeInErr
	}); err != nil {
		return fmt.Errorf("codegen3 stage-module: copy %q: %w", sourceRoot, err)
	}
	if err := verifyContainedOutputPath(outputRoot, stageRoot); err != nil {
		return err
	}
	if err := os.Rename(tmp, stageRoot); err != nil {
		if _, statErr := os.Stat(filepath.Join(stageRoot, "go.mod")); statErr == nil {
			return nil
		}
		return fmt.Errorf("codegen3 stage-module: publish %q: %w", stageRoot, err)
	}
	return nil
}

// codegen3CreateTemporaryModFile copies the project's module files under the
// per-build root and adds local replacements for staged external modules.
func codegen3CreateTemporaryModFile(moduleRoot, buildRoot string) (string, error) {
	modData, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("codegen3 modfile: read project go.mod: %w", err)
	}
	modPath := filepath.Join(buildRoot, "codegen3.mod")
	if err := os.WriteFile(modPath, modData, 0o644); err != nil {
		return "", fmt.Errorf("codegen3 modfile: write %q: %w", modPath, err)
	}
	sumPath := filepath.Join(buildRoot, "codegen3.sum")
	if _, statErr := os.Stat(sumPath); os.IsNotExist(statErr) {
		if sumData, readErr := os.ReadFile(filepath.Join(moduleRoot, "go.sum")); readErr == nil {
			if err := os.WriteFile(sumPath, sumData, 0o644); err != nil {
				return "", fmt.Errorf("codegen3 modfile: copy project go.sum: %w", err)
			}
		} else if !os.IsNotExist(readErr) {
			return "", fmt.Errorf("codegen3 modfile: read project go.sum: %w", readErr)
		}
	} else if statErr != nil {
		return "", fmt.Errorf("codegen3 modfile: inspect temporary go.sum: %w", statErr)
	}
	externalRoot := filepath.Join(buildRoot, "external")
	entries, err := os.ReadDir(externalRoot)
	if os.IsNotExist(err) {
		return modPath, nil
	}
	if err != nil {
		return "", fmt.Errorf("codegen3 modfile: read staged module directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stageRoot := filepath.Join(externalRoot, entry.Name())
		data, err := os.ReadFile(filepath.Join(stageRoot, "go.mod"))
		if err != nil {
			return "", fmt.Errorf("codegen3 modfile: read staged module %q: %w", stageRoot, err)
		}
		module := modulePath(data)
		if module == "" {
			return "", fmt.Errorf("codegen3 modfile: staged module %q has no module path", stageRoot)
		}
		cmd := exec.Command("go", "mod", "edit", "-modfile="+modPath, "-replace="+module+"="+stageRoot)
		cmd.Dir = moduleRoot
		cmd.Env = os.Environ()
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("codegen3 modfile: add replacement for %s: %w: %s", module, err, strings.TrimSpace(string(output)))
		}
	}
	return modPath, nil
}

// verifyContainedOutputPath checks lexical containment and the canonical
// destination implied by existing symlink parents, without creating files.
func verifyContainedOutputPath(root, candidate string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("codegen3 output-path: resolve output root: %w", err)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("codegen3 output-path: resolve candidate: %w", err)
	}
	if err := pathWithinRoot(root, candidate); err != nil {
		return err
	}

	existing := candidate
	for {
		if _, statErr := os.Lstat(existing); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("codegen3 output-path: inspect %q: %w", existing, statErr)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return fmt.Errorf("codegen3 output-path: no existing ancestor for %q", candidate)
		}
		existing = parent
	}
	canonicalAncestor, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return fmt.Errorf("codegen3 output-path: resolve existing ancestor %q: %w", existing, err)
	}
	suffix, err := filepath.Rel(existing, candidate)
	if err != nil {
		return fmt.Errorf("codegen3 output-path: resolve candidate suffix: %w", err)
	}
	canonicalRoot := root
	if resolved, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		canonicalRoot = resolved
	}
	return pathWithinRoot(canonicalRoot, filepath.Join(canonicalAncestor, suffix))
}

func pathWithinRoot(root, candidate string) error {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(filepath.ToSlash(rel), "../") {
		return fmt.Errorf("codegen3 output-path: resolved path %q escapes output root %q", candidate, root)
	}
	return nil
}

// strictGoImportPathForDir never falls back to a basename: codegen3 must not
// invent an output namespace for a package whose canonical identity is unknown.
func strictGoImportPathForDir(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve package directory: %w", err)
	}
	root := findGoModuleRoot(absDir)
	if root == "" {
		return "", fmt.Errorf("no Go module root for package directory %q", dir)
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read module file for %q: %w", dir, err)
	}
	mod := modulePath(data)
	if mod == "" {
		return "", fmt.Errorf("no module path in %q", filepath.Join(root, "go.mod"))
	}
	rel, err := filepath.Rel(root, absDir)
	if err != nil {
		return "", fmt.Errorf("resolve package path for %q: %w", dir, err)
	}
	if rel == "." {
		return mod, nil
	}
	if rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(filepath.ToSlash(rel), "../") {
		return "", fmt.Errorf("package directory %q is outside module root %q", dir, root)
	}
	return mod + "/" + filepath.ToSlash(rel), nil
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
	if userPkg.DotImportTypes == nil {
		userPkg.DotImportTypes = map[string]struct{}{}
	}
	if userPkg.DotImportFuncs == nil {
		userPkg.DotImportFuncs = map[string]*FuncDecl{}
	}
	// The prelude is dot-imported in generated Go. Keep its exported function
	// signatures available to inference without adding declarations that the
	// code generator would incorrectly emit in the importing package.
	if conflictOnExisting {
		for name, fn := range importedPkg.Funcs {
			if existing, exists := userPkg.Funcs[name]; exists && existing != fn {
				return common.ErrorAtPos("", 0, 0, "function %q conflicts with imported package %q", name, importedPkg.Name)
			}
			userPkg.DotImportFuncs[name] = fn
		}
	}
	for name, st := range importedPkg.Structs {
		userPkg.DotImportTypes[name] = struct{}{}
		if _, exists := userPkg.Structs[name]; !exists {
			userPkg.Structs[name] = st
		} else if conflictOnExisting && st != userPkg.Structs[name] {
			return common.ErrorAtPos("", 0, 0, "type %q conflicts with imported package %q", name, importedPkg.Name)
		}
	}
	for name, iface := range importedPkg.Interfaces {
		userPkg.DotImportTypes[name] = struct{}{}
		if _, exists := userPkg.Interfaces[name]; !exists {
			userPkg.Interfaces[name] = iface
		} else if conflictOnExisting && iface != userPkg.Interfaces[name] {
			return common.ErrorAtPos("", 0, 0, "type %q conflicts with imported package %q", name, importedPkg.Name)
		}
	}
	for name, enum := range importedPkg.Enums {
		userPkg.DotImportTypes[name] = struct{}{}
		if _, exists := userPkg.Enums[name]; !exists {
			userPkg.Enums[name] = enum
		} else if conflictOnExisting && enum != userPkg.Enums[name] {
			return common.ErrorAtPos("", 0, 0, "type %q conflicts with imported package %q", name, importedPkg.Name)
		}
	}
	for _, impl := range importedPkg.Impls {
		if name := inherentReceiverNameForImport(impl.Type); name != "" {
			userPkg.DotImportTypes[name] = struct{}{}
		}
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

func inherentReceiverNameForImport(t TypeExpr) string {
	if nt, ok := t.(*NamedType); ok {
		return nt.Name
	}
	return ""
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
		case *TypeAliasDecl:
			d.SourceFile = sourceFile
		case *TypeDecl:
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
		Dir:            dir,
		WorkspaceRoot:  filepath.Dir(dir),
		NoPrelude:      noPrelude,
		Imports:        map[string]struct{}{},
		ImportAliases:  map[string]string{},
		Enums:          map[string]*EnumDecl{},
		TypeAliases:    map[string]*TypeAliasDecl{},
		Types:          map[string]*TypeDecl{},
		Structs:        map[string]*StructDecl{},
		Interfaces:     map[string]*InterfaceDecl{},
		Funcs:          map[string]*FuncDecl{},
		DotImportTypes: map[string]struct{}{},
		DotImportFuncs: map[string]*FuncDecl{},
	}
	testPkg := &pkg.Package{
		Dir:            dir,
		WorkspaceRoot:  filepath.Dir(dir),
		NoPrelude:      noPrelude,
		Imports:        map[string]struct{}{},
		ImportAliases:  map[string]string{},
		Enums:          map[string]*EnumDecl{},
		TypeAliases:    map[string]*TypeAliasDecl{},
		Types:          map[string]*TypeDecl{},
		Structs:        map[string]*StructDecl{},
		Interfaces:     map[string]*InterfaceDecl{},
		Funcs:          map[string]*FuncDecl{},
		DotImportTypes: map[string]struct{}{},
		DotImportFuncs: map[string]*FuncDecl{},
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
		if legacyCaseGuardUnsupported(string(src)) {
			return nil, nil, fmt.Errorf("%s: case guards are not supported by the legacy pipeline", displaySourcePath)
		}
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
		case *TypeAliasDecl:
			mainPkg.TypeAliases[d.Name] = d
		case *TypeDecl:
			mainPkg.Types[d.Name] = d
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
		case *TypeAliasDecl:
			testPkg.TypeAliases[d.Name] = d
		case *TypeDecl:
			testPkg.Types[d.Name] = d
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

// legacyCaseGuardUnsupported detects the bootstrap-only `case ... if ...`
// form so --legacy sync fails with a direct diagnostic.  The legacy parser has
// no guard production; detecting it here keeps the error specific and prevents
// partial generated output.
func legacyCaseGuardUnsupported(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		code := line
		if i := strings.Index(code, "#"); i >= 0 {
			code = code[:i]
		}
		caseIndex := strings.Index(code, "case ")
		if caseIndex < 0 {
			continue
		}
		rest := code[caseIndex+len("case "):]
		end := len(rest)
		if i := strings.Index(rest, "then"); i >= 0 && i < end {
			end = i
		}
		if i := strings.Index(rest, "=>"); i >= 0 && i < end {
			end = i
		}
		header := strings.TrimSpace(rest[:end])
		if strings.HasPrefix(header, "if ") || strings.Contains(header, " if ") {
			return true
		}
	}
	return false
}
