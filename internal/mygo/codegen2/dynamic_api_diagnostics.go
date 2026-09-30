package codegen2

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/metadata"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

// diagnoseDynamicAPIRequests turns unresolved generated API names into
// dependency-specific diagnostics. Without this pass those cases look like
// ordinary unknown identifiers, hiding a missing metadata block or an
// implementation whose adapter cannot support the requested method.
func diagnoseDynamicAPIRequests(files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo, generated string) error {
	requested := dynamicReferencedIdentifiers(files)
	if len(requested) == 0 {
		return nil
	}
	available := make(map[string]bool)
	if generated != "" {
		file, err := parser.ParseFile(token.NewFileSet(), "zz_mygo_dynamic.gen.go", generated, parser.AllErrors)
		if err != nil {
			return fmt.Errorf("parse generated dynamic API: %w", err)
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				available[function.Name.Name] = true
			}
		}
	}
	for name := range requested {
		if !strings.HasPrefix(name, "MygoDynamic") || available[name] {
			continue
		}
		if err := diagnoseMissingDynamicName(name, dependencies); err != nil {
			return err
		}
	}
	return nil
}

func diagnoseMissingDynamicName(name string, dependencies []typeinference2.MyGoPackageInfo) error {
	var packages []dynamicDependency
	var missingMetadata []string
	for _, dependency := range dependencies {
		if dependency.DynamicMetadataJSON == "" {
			missingMetadata = append(missingMetadata, dependency.Path)
			continue
		}
		var value metadata.PackageMetadata
		if err := json.Unmarshal([]byte(dependency.DynamicMetadataJSON), &value); err != nil {
			return fmt.Errorf("dynamic API %s: invalid metadata for dependency %s: %w", name, dependency.Path, err)
		}
		if err := value.Validate(value.Package.Name); err != nil {
			return fmt.Errorf("dynamic API %s: incompatible metadata for dependency %s: %w", name, dependency.Path, err)
		}
		packages = append(packages, dynamicDependency{alias: dependency.Alias, meta: value})
	}
	for _, pkg := range packages {
		for _, implementation := range pkg.meta.Implementations {
			if dynamicConstructorName(dynamicImplementationRef{producerPath: pkg.meta.Package.ImportPath, value: implementation}) == name {
				return fmt.Errorf("dynamic constructor %s has metadata, but its interface package %s is missing from the caller dependency graph", name, implementation.Interface.Package)
			}
		}
	}
	for _, ifacePackage := range packages {
		for _, iface := range ifacePackage.meta.Exports.Interfaces {
			constructorPrefix := "MygoDynamic" + sanitizeGoIdent(iface.Name) + "From"
			if strings.HasPrefix(name, constructorPrefix) {
				foundImplementation := false
				for _, producer := range packages {
					for _, implementation := range producer.meta.Implementations {
						if implementation.Interface.Package == ifacePackage.meta.Package.ImportPath && implementation.Interface.Name == iface.Name {
							foundImplementation = true
						}
					}
				}
				if !foundImplementation {
					return fmt.Errorf("dynamic constructor %s has no implementation of interface %s.%s in the caller dependency graph", name, ifacePackage.meta.Package.ImportPath, iface.Name)
				}
				return fmt.Errorf("dynamic constructor %s does not match an implementation in the caller dependency graph for interface %s.%s", name, ifacePackage.meta.Package.ImportPath, iface.Name)
			}
			prefix := "MygoDynamic" + sanitizeGoIdent(iface.Name) + "Call"
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			methodName := strings.TrimPrefix(name, prefix)
			foundImplementation := false
			for _, producer := range packages {
				for _, implementation := range producer.meta.Implementations {
					if implementation.Interface.Package != ifacePackage.meta.Package.ImportPath || implementation.Interface.Name != iface.Name {
						continue
					}
					foundImplementation = true
					adapter, found := dynamicFindMetadataAdapter(implementation, methodName)
					if !found || !adapter.Supported {
						reason := "adapter is missing"
						if found && adapter.UnsupportedReason != "" {
							reason = adapter.UnsupportedReason
						}
						return fmt.Errorf("dynamic method %s.%s is unavailable for implementation %s in dependency %s: %s", ifacePackage.meta.Package.ImportPath, methodName, implementation.ID, producer.meta.Package.ImportPath, reason)
					}
				}
			}
			if !foundImplementation {
				return fmt.Errorf("dynamic method %s.%s has no implementation in the caller dependency graph; import a producer package with compatible MyGO metadata", ifacePackage.meta.Package.ImportPath, methodName)
			}
		}
	}
	if len(missingMetadata) > 0 {
		return fmt.Errorf("dynamic API %s is unresolved because dependency metadata is missing for %s", name, strings.Join(missingMetadata, ", "))
	}
	return fmt.Errorf("dynamic API %s is unavailable in the caller dependency graph; check the interface name and imported producer packages", name)
}

// dynamicReferencedIdentifiers walks the typed AST representation because
// SourceFileInput intentionally does not retain source text. MyGO AST nodes
// are immutable value trees; pointer tracking protects the walk if a future
// type adds shared/cyclic references.
func dynamicReferencedIdentifiers(files []SourceFileInput) map[string]bool {
	identType := reflect.TypeOf(ast2.ExprKind__IdentExpr{})
	requested := make(map[string]bool)
	seen := make(map[uintptr]bool)
	var visit func(reflect.Value)
	visit = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				visit(value.Elem())
			}
			return
		}
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return
			}
			pointer := value.Pointer()
			if seen[pointer] {
				return
			}
			seen[pointer] = true
			visit(value.Elem())
			return
		}
		if value.Type() == identType {
			name := value.Field(0).String()
			if strings.HasPrefix(name, "MygoDynamic") {
				requested[name] = true
			}
			return
		}
		switch value.Kind() {
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				visit(value.Field(index))
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				visit(value.Index(index))
			}
		case reflect.Map:
			iterator := value.MapRange()
			for iterator.Next() {
				visit(iterator.Key())
				visit(iterator.Value())
			}
		}
	}
	for _, file := range files {
		visit(reflect.ValueOf(file.File))
	}
	return requested
}
