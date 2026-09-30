package compiler

import (
	"encoding/json"
	"fmt"

	"github.com/mygo-lang/mygo/internal/mygo/metadata"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

// bootstrapReadMetadataInputs loads a precompiled MyGO package's public
// declarations from generated Go source comments. No dependency source or
// generated implementation bodies are needed for type inference.
func bootstrapReadMetadataInputs(dir string) (BootstrapInputs, error) {
	value, err := metadata.ReadDir(dir)
	if err != nil {
		return BootstrapInputs{}, fmt.Errorf("read MyGO metadata in %s: %w", dir, err)
	}
	if value == nil {
		return BootstrapInputs{}, fmt.Errorf("read MyGO metadata in %s: no metadata block found in generated Go sources", dir)
	}
	if expected := goImportPathForDir(dir); expected != "" && value.Package.ImportPath != expected {
		return BootstrapInputs{}, fmt.Errorf("read MyGO metadata in %s: package import path %q does not match resolved path %q", dir, value.Package.ImportPath, expected)
	}
	decls, err := value.ExportedDecls()
	if err != nil {
		return BootstrapInputs{}, fmt.Errorf("reconstruct MyGO exports in %s: %w", dir, err)
	}
	return BootstrapInputs{
		Sources: []typeinference2.PkgDeclSource{{Path: value.Package.ImportPath, Decls: decls}},
	}, nil
}

// bootstrapReadMetadataJSON returns the validated canonical metadata record for
// code generation. Missing metadata is an empty string so source-built
// dependencies keep their existing static import path during sync-only runs.
func bootstrapReadMetadataJSON(dir string) string {
	value, err := metadata.ReadDir(dir)
	if err != nil || value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
