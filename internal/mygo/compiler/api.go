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
)

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
