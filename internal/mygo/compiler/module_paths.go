package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	return "", fmt.Errorf("cannot resolve MyGO import %q from %q", importPath, fromDir)
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
