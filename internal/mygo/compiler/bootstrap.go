package compiler

import (
	"fmt"
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var inputs []codegen2.SourceFileInput
	var sources []typeinference2.PkgDeclSource
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") {
			continue
		}
		path := filepath.Join(dir, name)
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed := parser2.ParseFileAt(path, string(source))
		file, ok := parsed.(ResultOk[ast2.File, string])
		if !ok {
			return nil, bootstrapResultError("parse", path, parsed)
		}
		inputs = append(inputs, codegen2.SourceFileInput{Path: name, File: file.F0})
		sources = append(sources, typeinference2.PkgDeclSource{Path: path, Decls: file.F0.Decls})
	}
	if len(inputs) == 0 {
		return nil, nil
	}

	inferred := typeinference2.InferPackage(sources)
	info, ok := inferred.(ResultOk[typeinference2.PackageInfo, string])
	if !ok {
		return nil, bootstrapResultError("infer", dir, inferred)
	}
	generated := codegen2.GenerateFiles(inputs, info.F0)
	files, ok := generated.(ResultOk[map[string]string, string])
	if !ok {
		return nil, bootstrapResultError("generate", dir, generated)
	}

	written := make([]string, 0, len(files.F0))
	for name, source := range files.F0 {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			return nil, err
		}
		written = append(written, path)
	}
	sort.Strings(written)
	return written, nil
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
