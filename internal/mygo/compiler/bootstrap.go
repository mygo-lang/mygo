package compiler

import (
	"fmt"
	"go/importer"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/codegen2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

// CompileDirBootstrap compiles one package through the self-hosted pipeline:
// parser2 -> typeinference2 -> codegen2. It intentionally does not merge the
// legacy prelude or imported MyGO packages; those capabilities remain on the
// legacy backend until the bootstrap lane supports package resolution.
func CompileDirBootstrap(dir string) ([]string, error) {
	return compileDirBootstrap(dir, map[string]bool{}, map[string][]string{})
}

func compileDirBootstrap(dir string, compiling map[string]bool, compiled map[string][]string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if files, ok := compiled[absDir]; ok {
		return files, nil
	}
	if compiling[absDir] {
		return nil, fmt.Errorf("bootstrap import cycle at %s", absDir)
	}
	compiling[absDir] = true
	defer delete(compiling, absDir)

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	var inputs []codegen2.SourceFileInput
	var sources []typeinference2.PkgDeclSource
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") {
			continue
		}
		path := filepath.Join(absDir, name)
		sourcePath, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil, err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// Generated files remain rooted at absDir, while diagnostics are relative
		// to the invoking process's working directory.
		parsed := parser2.ParseFileAt(sourcePath, string(source))
		file, ok := parsed.(ResultOk[ast2.File, string])
		if !ok {
			return nil, bootstrapResultError("parse", path, parsed)
		}
		inputs = append(inputs, codegen2.SourceFileInput{Path: name, File: file.F0})
		sources = append(sources, typeinference2.PkgDeclSource{Path: sourcePath, Decls: file.F0.Decls})
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	var written []string
	for _, input := range inputs {
		for _, decl := range input.File.Decls {
			imp, ok := decl.(ast2.DeclImportDecl)
			if !ok || strings.HasPrefix(imp.F1, "go:") {
				continue
			}
			dependencyDir, err := resolveMyGoImport(absDir, absDir, imp.F1)
			if err != nil {
				return nil, err
			}
			dependencyFiles, err := compileDirBootstrap(dependencyDir, compiling, compiled)
			if err != nil {
				return nil, err
			}
			written = append(written, dependencyFiles...)
		}
	}

	inferred := typeinference2.InferPackage(sources)
	info, ok := inferred.(ResultOk[typeinference2.PackageInfo, string])
	if !ok {
		return nil, bootstrapResultError("infer", absDir, inferred)
	}
	if err := populateBootstrapGoSignatures(&info.F0); err != nil {
		return nil, err
	}
	generated := codegen2.GenerateFiles(inputs, info.F0)
	files, ok := generated.(ResultOk[map[string]string, string])
	if !ok {
		return nil, bootstrapResultError("generate", absDir, generated)
	}

	written = append(written, make([]string, 0, len(files.F0))...)
	for name, source := range files.F0 {
		path := filepath.Join(absDir, name)
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			return nil, err
		}
		written = append(written, path)
	}
	sort.Strings(written)
	compiled[absDir] = written
	return written, nil
}

func populateBootstrapGoSignatures(info *typeinference2.PackageInfo) error {
	for i := range info.GoPackages {
		entry := &info.GoPackages[i]
		if !strings.HasPrefix(entry.Path, "go:") {
			continue
		}
		pkg, err := importer.Default().Import(strings.TrimPrefix(entry.Path, "go:"))
		if err != nil {
			return fmt.Errorf("load Go FFI package %q: %w", entry.Path, err)
		}
		for _, name := range pkg.Scope().Names() {
			obj, ok := pkg.Scope().Lookup(name).(*types.Func)
			if !ok {
				continue
			}
			sig, ok := obj.Type().(*types.Signature)
			if !ok {
				continue
			}
			entry.Funcs = append(entry.Funcs, typeinference2.GoFuncSignature{
				Name:     name,
				Params:   goTupleTypes(sig.Params()),
				Results:  goTupleTypes(sig.Results()),
				Variadic: sig.Variadic(),
			})
		}
	}
	return nil
}

func goTupleTypes(tuple *types.Tuple) []string {
	items := make([]string, tuple.Len())
	for i := range items {
		items[i] = types.TypeString(tuple.At(i).Type(), func(p *types.Package) string { return p.Name() })
	}
	return items
}

// SyncBootstrap walks root and compiles every MyGO package using the
// self-hosted pipeline.
func SyncBootstrap(root string) ([]string, error) {
	dirs, err := mygoDirs(root)
	if err != nil {
		return nil, err
	}
	var written []string
	for _, dir := range dirs {
		files, err := CompileDirBootstrap(dir)
		if err != nil {
			return nil, err
		}
		written = append(written, files...)
	}
	sort.Strings(written)
	return written, nil
}

func bootstrapResultError(stage, path string, value any) error {
	return fmt.Errorf("bootstrap %s %s: %v", stage, path, value)
}
