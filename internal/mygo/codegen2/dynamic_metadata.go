package codegen2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/abi"
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/metadata"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	"github.com/mygo-lang/mygo/prelude"
)

// dynamicMetadataSource renders the compiler metadata source after all package
// Go files and dynamic adapter symbols have been generated.
func dynamicMetadataSource(packageName, importPath string, generated map[string]string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	if packageName == "" || importPath == "" {
		return "", fmt.Errorf("dynamic metadata: package name and canonical import path are required")
	}
	decls := make([]ast2.Decl, 0)
	for _, file := range files {
		decls = append(decls, file.File.Decls...)
	}
	aliases := dynamicMetadataAliases(decls)
	value := metadata.PackageMetadata{
		Package:         metadata.PackageIdentity{Name: packageName, ImportPath: importPath},
		Imports:         dynamicMetadataImports(decls),
		Exports:         dynamicMetadataExports(decls, importPath, aliases),
		Implementations: dynamicMetadataImplementations(decls, importPath, aliases, dependencies),
	}
	for name, source := range generated {
		if name == metadata.MetadataFileName {
			continue
		}
		sum := sha256.Sum256([]byte(source))
		value.GeneratedSource = append(value.GeneratedSource, metadata.GeneratedSource{Path: name, SHA256: hex.EncodeToString(sum[:])})
	}
	sort.Slice(value.GeneratedSource, func(i, j int) bool { return value.GeneratedSource[i].Path < value.GeneratedSource[j].Path })
	source, err := metadata.RenderSource(packageName, value)
	if err != nil {
		return "", err
	}
	if _, err := parser.ParseFile(token.NewFileSet(), metadata.MetadataFileName, source, parser.AllErrors); err != nil {
		return "", fmt.Errorf("dynamic metadata: generated source is invalid Go: %w", err)
	}
	return string(source), nil
}

type dynamicDependency struct {
	alias string
	meta  metadata.PackageMetadata
}

type dynamicImport struct {
	alias string
	path  string
}

// dynamicDispatchSource generates ordinary caller-package constructors and
// dispatch functions. Every dispatch arm is a direct reference to an adapter
// in a declared MyGO dependency, which keeps that producer linked.
func dynamicDispatchSource(packageName string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	return dynamicDispatchSourceWithLocal(packageName, "", files, dependencies)
}

func dynamicDispatchSourceWithLocal(packageName, packageImportPath string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	if packageName == "" {
		return "", nil
	}
	var packages []dynamicDependency
	if packageImportPath != "" {
		decls := make([]ast2.Decl, 0)
		for _, file := range files {
			decls = append(decls, file.File.Decls...)
		}
		aliases := dynamicMetadataAliases(decls)
		localMeta := metadata.PackageMetadata{
			Package:         metadata.PackageIdentity{Name: packageName, ImportPath: packageImportPath},
			Imports:         dynamicMetadataImports(decls),
			Exports:         dynamicMetadataExports(decls, packageImportPath, aliases),
			Implementations: dynamicMetadataImplementations(decls, packageImportPath, aliases, dependencies),
		}
		usedInterfaces := dynamicUsedLocalInterfaces(files, packageImportPath, localMeta.Exports.Interfaces)
		if len(usedInterfaces) > 0 {
			localMeta.Implementations = filterDynamicImplementations(localMeta.Implementations, packageImportPath, usedInterfaces)
			packages = append(packages, dynamicDependency{meta: localMeta})
		}
	}
	for _, dependency := range dependencies {
		if dependency.DynamicMetadataJSON == "" {
			continue
		}
		var value metadata.PackageMetadata
		if err := json.Unmarshal([]byte(dependency.DynamicMetadataJSON), &value); err != nil {
			return "", fmt.Errorf("dynamic metadata for dependency %s: %w", dependency.Path, err)
		}
		if err := value.Validate(value.Package.Name); err != nil {
			return "", fmt.Errorf("dynamic metadata for dependency %s: %w", dependency.Path, err)
		}
		packages = append(packages, dynamicDependency{alias: dependency.Alias, meta: value})
	}
	if len(packages) == 0 {
		return "", nil
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].meta.Package.ImportPath < packages[j].meta.Package.ImportPath })
	byPath := make(map[string]dynamicDependency, len(packages))
	for _, item := range packages {
		if previous, exists := byPath[item.meta.Package.ImportPath]; exists {
			if previous.meta.Package.Name != item.meta.Package.Name {
				return "", fmt.Errorf("dynamic metadata package identity conflict for %s", item.meta.Package.ImportPath)
			}
			continue
		}
		byPath[item.meta.Package.ImportPath] = item
	}
	imports := newDynamicImportTable(packageName, packageImportPath, files, packages)
	var implementations []dynamicImplementationRef
	seenIDs := make(map[string]metadata.Implementation)
	for _, item := range packages {
		for _, implementation := range item.meta.Implementations {
			if previous, exists := seenIDs[implementation.ID]; exists {
				if !reflect.DeepEqual(previous, implementation) {
					return "", fmt.Errorf("dynamic implementation %s has conflicting metadata", implementation.ID)
				}
				continue
			}
			seenIDs[implementation.ID] = implementation
			implementations = append(implementations, dynamicImplementationRef{producerPath: item.meta.Package.ImportPath, value: implementation})
		}
	}
	sort.Slice(implementations, func(i, j int) bool { return implementations[i].value.ID < implementations[j].value.ID })
	if len(implementations) == 0 {
		return "", nil
	}
	emittedInterfaces := dynamicEmittedInterfaces(files, packageImportPath, implementations, byPath)
	var declarations strings.Builder
	aliases := make(map[string]string)
	for _, implementation := range implementations {
		if _, exists := byPath[implementation.value.Interface.Package]; !exists {
			available := make([]string, 0, len(byPath))
			for importPath := range byPath {
				available = append(available, importPath)
			}
			sort.Strings(available)
			return "", fmt.Errorf("dynamic implementation %s references interface package %s missing from caller dependency graph (metadata available for %s)", implementation.value.ID, implementation.value.Interface.Package, strings.Join(available, ", "))
		}
		if !emittedInterfaces[implementation.value.Interface.Package+"\x00"+implementation.value.Interface.Name] {
			continue
		}
		ifaceTypeName := "MygoDynamic" + sanitizeGoIdent(implementation.value.Interface.Name)
		ifaceAlias, err := imports.packageName(implementation.value.Interface.Package)
		if err != nil {
			return "", err
		}
		qualifiedType := dynamicQualifiedName(ifaceAlias, ifaceTypeName)
		if previous, exists := aliases[ifaceTypeName]; exists {
			if previous != qualifiedType {
				return "", fmt.Errorf("dynamic wrapper type name %s is ambiguous between %s and %s", ifaceTypeName, previous, qualifiedType)
			}
			continue
		}
		aliases[ifaceTypeName] = qualifiedType
		if ifaceAlias != "" {
			fmt.Fprintf(&declarations, "type %s = %s\n", ifaceTypeName, qualifiedType)
		}
	}
	constructorNames := make(map[string]string)
	for _, implementation := range implementations {
		interfacePackage := byPath[implementation.value.Interface.Package]
		if interfacePackage.meta.Package.ImportPath == "" {
			return "", fmt.Errorf("dynamic implementation %s references interface package %s missing from caller dependency graph", implementation.value.ID, implementation.value.Interface.Package)
		}
		if !emittedInterfaces[implementation.value.Interface.Package+"\x00"+implementation.value.Interface.Name] {
			continue
		}
		constructor, err := dynamicConstructorSource(implementation, imports, interfacePackage)
		if err != nil {
			return "", err
		}
		constructorName := dynamicConstructorName(implementation)
		if previous, exists := constructorNames[constructorName]; exists {
			return "", fmt.Errorf("dynamic constructor name %s collides for implementations %s and %s", constructorName, previous, implementation.value.ID)
		}
		constructorNames[constructorName] = implementation.value.ID
		declarations.WriteString(constructor)
		declarations.WriteByte('\n')
	}
	groups := groupDynamicImplementations(implementations)
	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	for _, key := range groupKeys {
		refs := groups[key]
		interfacePackage := byPath[refs[0].value.Interface.Package]
		iface, ok := dynamicFindMetadataInterface(interfacePackage.meta, refs[0].value.Interface.Name)
		if !ok {
			return "", fmt.Errorf("dynamic interface %s.%s is missing from metadata", refs[0].value.Interface.Package, refs[0].value.Interface.Name)
		}
		for _, method := range iface.Methods {
			allSupported := true
			var firstAdapter metadata.Adapter
			for index, ref := range refs {
				adapter, exists := dynamicFindMetadataAdapter(ref.value, method.Name)
				if !exists || !adapter.Supported {
					allSupported = false
					break
				}
				if index == 0 {
					firstAdapter = adapter
				} else if firstAdapter.HiddenParameterIndex != adapter.HiddenParameterIndex || !sameDynamicTypeParams(firstAdapter.TypeParams, adapter.TypeParams) {
					return "", fmt.Errorf("dynamic interface %s method %s has inconsistent adapter signatures", iface.Name, method.Name)
				}
			}
			if !allSupported {
				continue
			}
			function, err := dynamicDispatchMethodSource(iface, method, refs, firstAdapter, imports, interfacePackage)
			if err != nil {
				return "", err
			}
			declarations.WriteString(function)
			declarations.WriteByte('\n')
		}
	}
	if declarations.Len() == 0 {
		return "", nil
	}
	var source strings.Builder
	fmt.Fprintf(&source, "// Code generated by MyGO; DO NOT EDIT.\npackage %s\n", packageName)
	importList := dynamicUsedImports(packageName, declarations.String(), imports.List())
	if len(importList) > 0 {
		source.WriteString("import (\n")
		for _, item := range importList {
			fmt.Fprintf(&source, "%s %s\n", item.alias, strconv.Quote(item.path))
		}
		source.WriteString(")\n")
	}
	source.WriteString(declarations.String())
	if _, err := parser.ParseFile(token.NewFileSet(), "zz_mygo_dynamic.gen.go", source.String(), parser.AllErrors); err != nil {
		return "", fmt.Errorf("dynamic dispatch generated invalid Go: %w\n%s", err, source.String())
	}
	return source.String(), nil
}

func dynamicUsedImports(packageName, declarations string, candidates []dynamicImport) []dynamicImport {
	file, err := parser.ParseFile(token.NewFileSet(), "dynamic_declarations.go", "package "+packageName+"\n"+declarations, parser.AllErrors)
	if err != nil {
		return candidates
	}
	used := make(map[string]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if alias, ok := selector.X.(*ast.Ident); ok {
			used[alias.Name] = true
		}
		return true
	})
	filtered := make([]dynamicImport, 0, len(candidates))
	for _, candidate := range candidates {
		if used[candidate.alias] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func dynamicUsedLocalInterfaces(files []SourceFileInput, packageImportPath string, interfaces []metadata.InterfaceExport) map[string]bool {
	localNames := make(map[string]bool, len(interfaces))
	for _, iface := range interfaces {
		localNames[iface.Name] = true
	}
	used := make(map[string]bool)
	for name := range dynamicBareTypeNames(files) {
		if localNames[name] {
			used[name] = true
		}
	}
	requested := dynamicReferencedIdentifiers(files)
	for name := range requested {
		for _, iface := range interfaces {
			prefix := "MygoDynamic" + sanitizeGoIdent(iface.Name)
			if strings.HasPrefix(name, prefix+"Call") || strings.HasPrefix(name, prefix+"From") {
				used[iface.Name] = true
			}
		}
	}
	return used
}

// dynamicBareTypeNames lists the argument-less type names the source mentions.
// The traversal lives in ast2 because it has to match on MyGO enums, whose Go
// shape differs between the legacy interface ABI and the canonical struct ABI.
func dynamicBareTypeNames(files []SourceFileInput) map[string]bool {
	names := make(map[string]bool)
	for _, file := range files {
		for _, name := range ast2.CollectBareTypeNames(file.File) {
			names[name] = true
		}
	}
	return names
}

func filterDynamicImplementations(implementations []metadata.Implementation, packagePath string, interfaces map[string]bool) []metadata.Implementation {
	filtered := make([]metadata.Implementation, 0, len(implementations))
	for _, implementation := range implementations {
		if implementation.Interface.Package == packagePath && interfaces[implementation.Interface.Name] {
			filtered = append(filtered, implementation)
		}
	}
	return filtered
}

func sameDynamicTypeParams(left, right []metadata.TypeParam) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || left[index].Index != right[index].Index {
			return false
		}
	}
	return true
}

type dynamicImplementationRef struct {
	producerPath string
	value        metadata.Implementation
}

type dynamicImportTable struct {
	byPath      map[string]string
	byName      map[string]bool
	currentPath string
}

const dynamicPreludeImportPath = "github.com/mygo-lang/mygo/prelude"

func newDynamicImportTable(packageName, packageImportPath string, files []SourceFileInput, packages []dynamicDependency) *dynamicImportTable {
	table := &dynamicImportTable{byPath: make(map[string]string), byName: make(map[string]bool), currentPath: packageImportPath}
	for _, file := range files {
		for _, imported := range dynamicMetadataImports(file.File.Decls) {
			if imported.Alias != "." && imported.Alias != "_" {
				table.byPath[imported.Path] = imported.Alias
				table.byName[imported.Alias] = true
			}
		}
	}
	for _, pkg := range packages {
		pathValue := pkg.meta.Package.ImportPath
		if pathValue == packageImportPath {
			continue
		}
		alias := table.byPath[pathValue]
		if alias == "" {
			alias = uniqueDynamicAlias(pkg.meta.Package.Name, pathValue, table.byName)
			table.byPath[pathValue] = alias
		}
		table.byName[alias] = true
	}
	if dynamicPackagesUsePrelude(packages) && table.byPath[dynamicPreludeImportPath] == "" {
		alias := uniqueDynamicAlias("prelude", dynamicPreludeImportPath, table.byName)
		table.byPath[dynamicPreludeImportPath] = alias
		table.byName[alias] = true
	}
	for _, pkg := range packages {
		for _, imported := range pkg.meta.Imports {
			if table.byPath[imported.Path] == "" {
				alias := imported.Alias
				if alias == "." || alias == "_" || alias == "" {
					alias = pathBase(imported.Path)
				}
				alias = uniqueDynamicAlias(alias, imported.Path, table.byName)
				table.byPath[imported.Path] = alias
				table.byName[alias] = true
			}
		}
	}
	return table
}

func dynamicPackagesUsePrelude(packages []dynamicDependency) bool {
	for _, pkg := range packages {
		for _, iface := range pkg.meta.Exports.Interfaces {
			for _, method := range iface.Methods {
				for _, param := range method.Parameters {
					if dynamicTypeUsesPackage(param.Type, dynamicPreludeImportPath) {
						return true
					}
				}
				for _, result := range method.Results {
					if dynamicTypeUsesPackage(result, dynamicPreludeImportPath) {
						return true
					}
				}
			}
		}
	}
	return false
}

func dynamicTypeUsesPackage(value metadata.Type, packagePath string) bool {
	if value.Package == packagePath {
		return true
	}
	if value.Element != nil && dynamicTypeUsesPackage(*value.Element, packagePath) {
		return true
	}
	if value.Key != nil && dynamicTypeUsesPackage(*value.Key, packagePath) {
		return true
	}
	if value.Value != nil && dynamicTypeUsesPackage(*value.Value, packagePath) {
		return true
	}
	for _, child := range value.Arguments {
		if dynamicTypeUsesPackage(child, packagePath) {
			return true
		}
	}
	for _, child := range value.Parameters {
		if dynamicTypeUsesPackage(child, packagePath) {
			return true
		}
	}
	for _, child := range value.Results {
		if dynamicTypeUsesPackage(child, packagePath) {
			return true
		}
	}
	return false
}

func (t *dynamicImportTable) List() []dynamicImport {
	items := make([]dynamicImport, 0, len(t.byPath))
	for pathValue, alias := range t.byPath {
		items = append(items, dynamicImport{alias: alias, path: pathValue})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].path < items[j].path })
	return items
}

func (t *dynamicImportTable) packageName(pathValue string) (string, error) {
	if pathValue == t.currentPath && pathValue != "" {
		return "", nil
	}
	if alias := t.byPath[pathValue]; alias != "" {
		return alias, nil
	}
	return "", fmt.Errorf("dynamic API type refers to package %q outside the import graph", pathValue)
}

func dynamicQualifiedName(alias, name string) string {
	if alias == "" {
		return name
	}
	return alias + "." + name
}

func uniqueDynamicAlias(preferred, pathValue string, used map[string]bool) string {
	alias := sanitizeGoIdent(preferred)
	if alias == "" || alias == "_" || alias == "." {
		alias = sanitizeGoIdent(pathBase(pathValue))
	}
	if !used[alias] {
		return alias
	}
	base := alias
	for index := 2; ; index++ {
		alias = fmt.Sprintf("%s%d", base, index)
		if !used[alias] {
			return alias
		}
	}
}

func sanitizeGoIdent(value string) string {
	var out strings.Builder
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (index > 0 && r >= '0' && r <= '9') {
			out.WriteRune(r)
		} else if index == 0 && r >= '0' && r <= '9' {
			out.WriteByte('_')
			out.WriteRune(r)
		}
	}
	return out.String()
}

func groupDynamicImplementations(values []dynamicImplementationRef) map[string][]dynamicImplementationRef {
	groups := make(map[string][]dynamicImplementationRef)
	for _, value := range values {
		key := value.value.Interface.Package + "\x00" + value.value.Interface.Name
		groups[key] = append(groups[key], value)
	}
	return groups
}

// dynamicEmittedInterfaces reports the interface groups that get emitted into
// the caller's dynamic API. An interface qualifies when at least one method is
// dispatchable, or when the compiling package references its dynamic API. This
// keeps dead constructors (such as the concurrency channel wrappers, whose
// adapters are all unsupported) out of packages that never use them.
func dynamicEmittedInterfaces(files []SourceFileInput, localImportPath string, implementations []dynamicImplementationRef, byPath map[string]dynamicDependency) map[string]bool {
	requested := dynamicReferencedIdentifiers(files)
	bare := dynamicBareTypeNames(files)
	dispatchable := dynamicDispatchableInterfaces(implementations, byPath)
	result := make(map[string]bool)
	for key, refs := range groupDynamicImplementations(implementations) {
		iface := refs[0].value.Interface
		if dispatchable[key] ||
			dynamicInterfaceRequested(requested, iface.Name) ||
			(iface.Package == localImportPath && bare[iface.Name]) {
			result[key] = true
		}
	}
	return result
}

// dynamicDispatchableInterfaces reports the interface groups for which at
// least one method is supported by every implementation in the group, i.e. the
// method gets a generated Call* dispatch function.
func dynamicDispatchableInterfaces(implementations []dynamicImplementationRef, byPath map[string]dynamicDependency) map[string]bool {
	result := make(map[string]bool)
	for key, refs := range groupDynamicImplementations(implementations) {
		ifacePackage, ok := byPath[refs[0].value.Interface.Package]
		if !ok {
			continue
		}
		iface, ok := dynamicFindMetadataInterface(ifacePackage.meta, refs[0].value.Interface.Name)
		if !ok {
			continue
		}
		for _, method := range iface.Methods {
			supported := true
			for _, ref := range refs {
				adapter, exists := dynamicFindMetadataAdapter(ref.value, method.Name)
				if !exists || !adapter.Supported {
					supported = false
					break
				}
			}
			if supported {
				result[key] = true
				break
			}
		}
	}
	return result
}

// dynamicInterfaceRequested reports whether the compiling package references
// interface ifaceName's dynamic API: the MygoDynamic<Iface> type or one of its
// Call*/From* functions.
func dynamicInterfaceRequested(requested map[string]bool, ifaceName string) bool {
	prefix := "MygoDynamic" + sanitizeGoIdent(ifaceName)
	for name := range requested {
		if name == prefix {
			return true
		}
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			if strings.HasPrefix(rest, "Call") || strings.HasPrefix(rest, "From") {
				return true
			}
		}
	}
	return false
}

func dynamicFindMetadataInterface(value metadata.PackageMetadata, name string) (metadata.InterfaceExport, bool) {
	for _, iface := range value.Exports.Interfaces {
		if iface.Name == name {
			return iface, true
		}
	}
	return metadata.InterfaceExport{}, false
}

func dynamicFindMetadataAdapter(value metadata.Implementation, name string) (metadata.Adapter, bool) {
	for _, adapter := range value.Methods {
		if adapter.Method == name {
			return adapter, true
		}
	}
	return metadata.Adapter{}, false
}

func dynamicConstructorSource(ref dynamicImplementationRef, imports *dynamicImportTable, ifacePackage dynamicDependency) (string, error) {
	target, err := dynamicGoType(ref.value.Target, imports)
	if err != nil {
		return "", fmt.Errorf("dynamic implementation %s target: %w", ref.value.ID, err)
	}
	ifaceAlias, err := imports.packageName(ref.value.Interface.Package)
	if err != nil {
		return "", err
	}
	ifaceName := "MygoDynamic" + sanitizeGoIdent(ref.value.Interface.Name)
	functionName := dynamicConstructorName(ref)
	params := make([]string, 0, len(ref.value.TypeParams))
	for _, typeParam := range ref.value.TypeParams {
		if strings.Contains(typeParam.Name, "[") {
			return "", fmt.Errorf("dynamic implementation %s has unsupported higher-kinded target parameter %q", ref.value.ID, typeParam.Name)
		}
		params = append(params, typeParam.Name+" any")
	}
	typeParams := ""
	if len(params) > 0 {
		typeParams = "[" + strings.Join(params, ", ") + "]"
	}
	return fmt.Sprintf(
		"func %s%s(payload %s) %s { return %s(payload, %s) }",
		functionName,
		typeParams,
		target,
		ifaceName,
		dynamicQualifiedName(ifaceAlias, "New"+ifaceName),
		strconv.Quote(ref.value.ID),
	), nil
}

func dynamicConstructorName(ref dynamicImplementationRef) string {
	implementationName := ref.value.Name
	if implementationName == "" {
		implementationName = ref.value.Target.Name
	}
	return metadata.DynamicConstructorName(ref.producerPath, ref.value.Interface.Name, implementationName)
}

func dynamicDispatchMethodSource(iface metadata.InterfaceExport, method metadata.Method, refs []dynamicImplementationRef, adapter metadata.Adapter, imports *dynamicImportTable, ifacePackage dynamicDependency) (string, error) {
	if adapter.HiddenParameterIndex < 0 || adapter.HiddenParameterIndex >= len(method.Parameters) {
		return "", fmt.Errorf("dynamic interface %s method %s has invalid hidden parameter index", iface.Name, method.Name)
	}
	ifaceAlias, err := imports.packageName(refs[0].value.Interface.Package)
	if err != nil {
		return "", err
	}
	ifaceName := "MygoDynamic" + sanitizeGoIdent(iface.Name)
	params := make([]string, 0, len(method.Parameters))
	for index, param := range method.Parameters {
		if index == adapter.HiddenParameterIndex {
			continue
		}
		typ, err := dynamicGoType(param.Type, imports)
		if err != nil {
			return "", fmt.Errorf("dynamic interface %s method %s parameter %s: %w", iface.Name, method.Name, param.Name, err)
		}
		params = append(params, safeGoParamName(param.Name, index)+" "+typ)
	}
	typeParams := make([]string, 0, len(method.TypeParams))
	typeArgs := make([]string, 0, len(method.TypeParams))
	for _, typeParam := range method.TypeParams {
		if strings.Contains(typeParam.Name, "[") {
			return "", fmt.Errorf("dynamic interface %s method %s has unsupported higher-kinded method parameter %q", iface.Name, method.Name, typeParam.Name)
		}
		typeParams = append(typeParams, typeParam.Name+" any")
		typeArgs = append(typeArgs, typeParam.Name)
	}
	genericDecl := ""
	if len(typeParams) > 0 {
		genericDecl = "[" + strings.Join(typeParams, ", ") + "]"
	}
	resultTypes := make([]string, 0, len(method.Results))
	for _, result := range method.Results {
		typ, err := dynamicGoType(result, imports)
		if err != nil {
			return "", fmt.Errorf("dynamic interface %s method %s result: %w", iface.Name, method.Name, err)
		}
		resultTypes = append(resultTypes, typ)
	}
	resultDecl, returnPrefix := "", ""
	if len(resultTypes) == 1 {
		resultDecl = " " + resultTypes[0]
		returnPrefix = "return "
	} else if len(resultTypes) > 1 {
		resultDecl = "(" + strings.Join(resultTypes, ", ") + ")"
		returnPrefix = "return "
	}
	argNames := make([]string, 0, len(params))
	for index := range method.Parameters {
		if index != adapter.HiddenParameterIndex {
			argNames = append(argNames, safeGoParamName(method.Parameters[index].Name, index))
		}
	}
	qualifiedInterface := dynamicQualifiedName(ifaceAlias, ifaceName)
	allArgs := append([]string{qualifiedInterface + "Payload(value)"}, argNames...)
	callArgs := strings.Join(allArgs, ", ")
	callSuffix := ""
	if len(typeArgs) > 0 {
		callSuffix = "[" + strings.Join(typeArgs, ", ") + "]"
	}
	var body strings.Builder
	fmt.Fprintf(&body, "switch %sImplementation(value) {\n", qualifiedInterface)
	for _, ref := range refs {
		methodAdapter, exists := dynamicFindMetadataAdapter(ref.value, method.Name)
		if !exists || !methodAdapter.Supported {
			return "", fmt.Errorf("dynamic interface %s method %s has inconsistent adapter set", iface.Name, method.Name)
		}
		producerAlias, err := imports.packageName(ref.producerPath)
		if err != nil {
			return "", err
		}
		comma := ""
		if len(callArgs) > 0 {
			comma = callArgs
		}
		adapterName := dynamicQualifiedName(producerAlias, methodAdapter.Symbol)
		if len(method.TypeParams) > 0 {
			fmt.Fprintf(&body, "case %s:\n%s%s%s(%s)\n", strconv.Quote(ref.value.ID), returnPrefix, adapterName, callSuffix, comma)
		} else {
			fmt.Fprintf(&body, "case %s:\n%s%s(%s)\n", strconv.Quote(ref.value.ID), returnPrefix, adapterName, comma)
		}
	}
	body.WriteString("default:\npanic(\"MyGO dynamic implementation is not available for this operation\")\n}\n")
	functionName := "MygoDynamic" + sanitizeGoIdent(iface.Name) + "Call" + sanitizeGoIdent(method.Name)
	paramsDecl := append([]string{"value " + ifaceName}, params...)
	if len(resultTypes) == 0 {
		resultDecl = " error"
		returnPrefix = "return nil"
	}
	return fmt.Sprintf("func %s%s(%s)%s {\n%s}\n", functionName, genericDecl, strings.Join(paramsDecl, ", "), resultDecl, body.String()), nil
}

func dynamicGoType(value metadata.Type, imports *dynamicImportTable) (string, error) {
	switch value.Kind {
	case "builtin":
		return value.Name, nil
	case "typeParam":
		return value.Name, nil
	case "named":
		if len(value.Arguments) == 1 {
			prefix := typeinference2.ChannelGoPrefix(value.Name)
			if abi.HasSome[string](prefix) {
				spelling := abi.ValueOr[string](prefix, "")
				element, err := dynamicGoType(value.Arguments[0], imports)
				if err != nil {
					return "", err
				}
				return spelling + element, nil
			}
		}
		name := value.Name
		if value.Package != "" {
			alias, err := imports.packageName(value.Package)
			if err != nil {
				return "", err
			}
			if alias != "" {
				name = alias + "." + name
			}
		}
		if len(value.Arguments) > 0 {
			args := make([]string, 0, len(value.Arguments))
			for _, arg := range value.Arguments {
				typ, err := dynamicGoType(arg, imports)
				if err != nil {
					return "", err
				}
				args = append(args, typ)
			}
			name += "[" + strings.Join(args, ", ") + "]"
		}
		return name, nil
	case "slice":
		if value.Element == nil {
			return "", fmt.Errorf("incomplete slice descriptor")
		}
		element, err := dynamicGoType(*value.Element, imports)
		return "[]" + element, err
	case "reference":
		if value.Element == nil {
			return "", fmt.Errorf("incomplete reference descriptor")
		}
		element, err := dynamicGoType(*value.Element, imports)
		return "*" + element, err
	case "map":
		if value.Key == nil || value.Value == nil {
			return "", fmt.Errorf("incomplete map descriptor")
		}
		key, err := dynamicGoType(*value.Key, imports)
		if err != nil {
			return "", err
		}
		item, err := dynamicGoType(*value.Value, imports)
		return "map[" + key + "]" + item, err
	case "applied":
		return dynamicGoType(metadata.Type{Kind: "named", Name: value.Name, Arguments: value.Arguments}, imports)
	case "tuple":
		if len(value.Arguments) == 0 {
			return "struct{}", nil
		}
		items := make([]string, 0, len(value.Arguments))
		for _, item := range value.Arguments {
			typ, err := dynamicGoType(item, imports)
			if err != nil {
				return "", err
			}
			items = append(items, typ)
		}
		return "(" + strings.Join(items, ", ") + ")", nil
	case "function":
		params := make([]string, 0, len(value.Parameters))
		for _, param := range value.Parameters {
			typ, err := dynamicGoType(param, imports)
			if err != nil {
				return "", err
			}
			params = append(params, typ)
		}
		results := make([]string, 0, len(value.Results))
		for _, result := range value.Results {
			typ, err := dynamicGoType(result, imports)
			if err != nil {
				return "", err
			}
			results = append(results, typ)
		}
		result := ""
		if len(results) == 1 {
			result = " " + results[0]
		} else if len(results) > 1 {
			result = " (" + strings.Join(results, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ")" + result, nil
	default:
		return "", fmt.Errorf("unsupported descriptor kind %q", value.Kind)
	}
}

func safeGoParamName(name string, index int) string {
	name = sanitizeGoIdent(name)
	if name == "" || name == "value" || name == "payload" {
		return fmt.Sprintf("dynamicArg%d", index)
	}
	return name
}

func dynamicMetadataImports(decls []ast2.Decl) []metadata.PackageImport {
	var imports []metadata.PackageImport
	seen := make(map[string]bool)
	for _, decl := range decls {
		if !ast2.DeclIsImport(decl) {
			continue
		}
		rawPath := ast2.DeclImportPath(decl)
		goImport := strings.HasPrefix(rawPath, "go:")
		path := strings.TrimPrefix(rawPath, "go:")
		alias := ast2.DeclImportAlias(decl)
		if alias == "" {
			alias = pathBase(path)
		}
		key := alias + "\x00" + path
		if seen[key] {
			continue
		}
		seen[key] = true
		imports = append(imports, metadata.PackageImport{Alias: alias, Path: path, Go: goImport})
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias != imports[j].Alias {
			return imports[i].Alias < imports[j].Alias
		}
		if imports[i].Path != imports[j].Path {
			return imports[i].Path < imports[j].Path
		}
		return !imports[i].Go && imports[j].Go
	})
	return imports
}

func dynamicMetadataAliases(decls []ast2.Decl) map[string]string {
	aliases := make(map[string]string)
	for _, decl := range decls {
		if ast2.DeclIsImport(decl) {
			path := strings.TrimPrefix(ast2.DeclImportPath(decl), "go:")
			alias := ast2.DeclImportAlias(decl)
			if alias == "" {
				alias = pathBase(path)
			}
			aliases[alias] = path
		}
	}
	return aliases
}

func pathBase(value string) string {
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		return value[slash+1:]
	}
	return path.Base(value)
}

// dynamicMetadataExports records the exported API of one package. Declarations
// are inspected through ast2's ABI-neutral accessors so the pass compiles
// against both enum representations.
func dynamicMetadataExports(decls []ast2.Decl, importPath string, aliases map[string]string) metadata.Exports {
	var exports metadata.Exports
	for _, decl := range decls {
		switch {
		case ast2.DeclIsType(decl):
			name := ast2.DeclTypeName(decl)
			if !ast2Exported(name) {
				continue
			}
			typeParams := ast2.DeclTypeParams(decl)
			target := ast2.DeclTypeTarget(decl)
			exports.Types = append(exports.Types, metadata.TypeExport{
				Name: name, Kind: dynamicNamedTypeKind(target), TypeParams: dynamicMetadataTypeParams(typeParams, "type:"+importPath+":"+name),
				Underlying: typePtr(dynamicMetadataType(target, importPath, aliases, typeParams, "type:"+name)),
			})
		case ast2.DeclIsTypeAlias(decl):
			name := ast2.DeclTypeAliasName(decl)
			if !ast2Exported(name) {
				continue
			}
			typeParams := ast2.DeclTypeAliasParams(decl)
			exports.Aliases = append(exports.Aliases, metadata.AliasExport{
				Name: name, TypeParams: dynamicMetadataTypeParams(typeParams, "alias:"+importPath+":"+name),
				Target: dynamicMetadataType(ast2.DeclTypeAliasTarget(decl), importPath, aliases, typeParams, "alias:"+name),
			})
		case ast2.DeclIsStruct(decl):
			name := ast2.DeclStructName(decl)
			if !ast2Exported(name) {
				continue
			}
			typeParams := ast2.DeclStructParams(decl)
			declFields := ast2.DeclStructFields(decl)
			fields := make([]metadata.Field, 0, len(declFields))
			for _, field := range declFields {
				fields = append(fields, metadata.Field{Name: field.Name, Type: dynamicMetadataType(field.Type, importPath, aliases, typeParams, "type:"+name), Tag: dynamicOptionString(field.Tag)})
			}
			exports.Types = append(exports.Types, metadata.TypeExport{Name: name, Kind: "struct", TypeParams: dynamicMetadataTypeParams(typeParams, "type:"+importPath+":"+name), Fields: fields})
		case ast2.DeclIsEnum(decl):
			name := ast2.DeclEnumName(decl)
			if !ast2Exported(name) {
				continue
			}
			typeParams := ast2.DeclEnumParams(decl)
			declVariants := ast2.DeclEnumVariants(decl)
			variants := make([]metadata.Variant, 0, len(declVariants))
			for _, variant := range declVariants {
				fields := make([]metadata.Field, 0, len(variant.Fields))
				for index, fieldType := range variant.Fields {
					fieldName := ""
					if variant.Named && index < len(variant.Names) {
						fieldName = variant.Names[index]
					}
					fields = append(fields, metadata.Field{Name: fieldName, Type: dynamicMetadataType(fieldType, importPath, aliases, typeParams, "type:"+name)})
				}
				variants = append(variants, metadata.Variant{Name: variant.Name, Fields: fields})
			}
			exports.Types = append(exports.Types, metadata.TypeExport{Name: name, Kind: "enum", TypeParams: dynamicMetadataTypeParams(typeParams, "type:"+importPath+":"+name), Variants: variants})
		case ast2.DeclIsFunc(decl):
			name := ast2.DeclFuncName(decl)
			if !ast2Exported(name) {
				continue
			}
			typeParams := ast2.DeclFuncParams(decl)
			exports.Functions = append(exports.Functions, metadata.FunctionExport{
				Name: name, TypeParams: dynamicMetadataTypeParams(typeParams, "func:"+importPath+":"+name),
				Parameters: dynamicMetadataFields(ast2.DeclFuncArguments(decl), importPath, aliases, typeParams, "func:"+name),
				Results:    dynamicMetadataResults(ast2.DeclFuncResultOption(decl), importPath, aliases, typeParams, "func:"+name),
				Using:      dynamicMetadataConstraints(ast2.DeclFuncUsing(decl), importPath, aliases, typeParams, "func:"+name),
			})
		case ast2.DeclIsInterface(decl):
			name := ast2.DeclInterfaceName(decl)
			if !ast2Exported(name) {
				continue
			}
			typeParams := ast2.DeclInterfaceParams(decl)
			declMethods := ast2.DeclInterfaceMethods(decl)
			methods := make([]metadata.Method, 0, len(declMethods))
			for _, method := range declMethods {
				methods = append(methods, dynamicMetadataMethod(method, importPath, aliases, typeParams, "interface:"+name))
			}
			exports.Interfaces = append(exports.Interfaces, metadata.InterfaceExport{Name: name, TypeParams: dynamicMetadataTypeParams(typeParams, "interface:"+importPath+":"+name), Methods: methods})
			exports.Types = append(exports.Types, metadata.TypeExport{
				Name: "MygoDynamic" + sanitizeGoIdent(name), Kind: "defined",
				Underlying: typePtr(metadata.Type{Kind: "builtin", Name: "any"}),
			})
		}
	}
	return exports
}

func dynamicMetadataImplementations(decls []ast2.Decl, importPath string, aliases map[string]string, dependencies []typeinference2.MyGoPackageInfo) []metadata.Implementation {
	allDecls := append([]ast2.Decl(nil), decls...)
	for _, dependency := range dependencies {
		allDecls = append(allDecls, dependency.Decls...)
	}
	var output []metadata.Implementation
	for _, decl := range decls {
		if !ast2.DeclIsImpl(decl) {
			continue
		}
		implTypeParams := ast2.DeclImplParams(decl)
		iface, dynamicTarget, adapterTarget, ok := dynamicMetadataImplInterface(decl, decls, dependencies)
		if !ok {
			continue
		}
		qualifiedIfaceName, ifaceArgs, qualifiedAlias, ok := dynamicMetadataNamedType(iface)
		if !ok || len(ifaceArgs) == 0 {
			continue
		}
		ifaceName := qualifiedIfaceName
		if dot := strings.LastIndex(ifaceName, "."); dot >= 0 {
			ifaceName = ifaceName[dot+1:]
		}
		ifacePackage := ""
		if qualifiedAlias != "" {
			ifacePackage = aliases[qualifiedAlias]
		} else {
			for _, candidate := range decls {
				if ast2.DeclIsInterface(candidate) && ast2.DeclInterfaceName(candidate) == ifaceName {
					ifacePackage = importPath
					break
				}
			}
		}
		var ifaceDecl ast2.Decl
		foundIfaceDecl := false
		for _, candidate := range allDecls {
			if ast2.DeclIsInterface(candidate) && ast2.DeclInterfaceName(candidate) == ifaceName {
				ifaceDecl = candidate
				foundIfaceDecl = true
				break
			}
		}
		if !foundIfaceDecl {
			continue
		}
		ifaceTps := ast2.DeclInterfaceParams(ifaceDecl)
		ifaceMethods := ast2.DeclInterfaceMethods(ifaceDecl)
		if ifacePackage == "" {
			for _, dependency := range dependencies {
				for _, candidate := range dependency.Decls {
					if ast2.DeclIsInterface(candidate) && ast2.DeclInterfaceName(candidate) == ifaceName {
						ifacePackage = dependency.Path
						break
					}
				}
			}
		}
		if ifacePackage == "" {
			continue
		}
		instanceName := dynamicMetadataName(adapterTarget)
		identity := metadata.ImplementationIdentity{
			Package:   importPath,
			Interface: metadata.TypeclassID{Package: ifacePackage, Name: ifaceName, Arguments: dynamicMetadataTypes(ifaceArgs, importPath, aliases, implTypeParams, "impl:"+instanceName)},
			Target:    dynamicMetadataType(dynamicTarget, importPath, aliases, implTypeParams, "impl:"+instanceName),
			Name:      instanceName,
		}
		id, err := metadata.StableImplementationID(identity)
		if err != nil {
			continue
		}
		record := metadata.Implementation{ID: id, Name: identity.Name, Interface: identity.Interface, Target: identity.Target, TypeParams: dynamicMetadataTypeParams(implTypeParams, "impl:"+id)}
		for _, method := range ast2.DeclImplMethods(decl) {
			ifaceMethod, exists := dynamicMetadataMethodByName(ifaceMethods, method.Sig.Name)
			adapter := metadata.Adapter{Method: method.Sig.Name, ABI: metadata.AdapterABI, Supported: false}
			if exists {
				methodTypeParams := append(append([]string(nil), implTypeParams...), method.Sig.TypeParams...)
				adapter.LogicalParameters = dynamicMetadataFields(method.Sig.Params, importPath, aliases, methodTypeParams, "method:"+id+":"+method.Sig.Name)
				adapter.LogicalResults = dynamicMetadataResults(method.Sig.Ret, importPath, aliases, methodTypeParams, "method:"+id+":"+method.Sig.Name)
				adapter.LogicalUsing = dynamicMetadataConstraints(method.Sig.Using, importPath, aliases, methodTypeParams, "method:"+id+":"+method.Sig.Name)
				adapter.TypeParams = dynamicMetadataTypeParams(method.Sig.TypeParams, "method:"+id+":"+method.Sig.Name)
				if len(implTypeParams) > 0 {
					adapter.UnsupportedReason = "adapter cannot capture implementation target type parameters"
				} else if len(method.Sig.Using) > 0 {
					adapter.UnsupportedReason = "method requires using dictionaries outside the dynamic adapter ABI"
				} else {
					methodOption := abi.SomeOf(ifaceMethod)
					hidden := dynamicHiddenParameter(method.Sig.Params, dynamicTarget, methodOption, ifaceTps, ifaceArgs, 0, -1)
					if abi.HasSome[int](hidden) {
						hiddenIndex := abi.ValueOr[int](hidden, -1)
						if dynamicHasNestedHiddenParams(ifaceMethod, ifaceTps, ifaceArgs, hiddenIndex) {
							adapter.UnsupportedReason = "method parameter contains a callback or nested value of the hidden type"
						} else {
							adapter.Supported = true
							adapter.Symbol = dynamicAdapterSymbol(implStem(adapterTarget, abi.SomeOf(iface), typeParamSet(implTypeParams)), ifaceName, method.Sig.Name)
							adapter.HiddenParameterIndex = hiddenIndex
							adapter.GoParameters = []metadata.Type{{Kind: "builtin", Name: "any"}}
							for index, param := range method.Sig.Params {
								if index != hiddenIndex {
									adapter.GoParameters = append(adapter.GoParameters, dynamicMetadataType(param.Type, importPath, aliases, method.Sig.TypeParams, "method:"+id+":"+method.Sig.Name))
								}
							}
							adapter.GoResults = append([]metadata.Type(nil), adapter.LogicalResults...)
						}
					} else {
						adapter.UnsupportedReason = "method does not have exactly one direct hidden-type parameter"
					}
				}
			} else {
				adapter.UnsupportedReason = "implementation method is missing from the interface"
			}
			record.Methods = append(record.Methods, adapter)
		}
		output = append(output, record)
	}
	return output
}

// Anonymous default instances use the interface application as ImplDecl.F1,
// while named instances keep the concrete target there and put the interface
// application in F2. Normalize both forms before metadata and adapter naming.
func dynamicMetadataImplInterface(impl ast2.Decl, localDecls []ast2.Decl, dependencies []typeinference2.MyGoPackageInfo) (iface ast2.TypeExpr, dynamicTarget ast2.TypeExpr, adapterTarget ast2.TypeExpr, ok bool) {
	var zero ast2.TypeExpr
	target := ast2.DeclImplTarget(impl)
	if abi.HasSome[ast2.TypeExpr](ast2.DeclImplInterfaceOption(impl)) {
		iface = abi.ValueOr[ast2.TypeExpr](ast2.DeclImplInterfaceOption(impl), ast2.UnitTypeExpr())
		_, args, _, valid := dynamicMetadataNamedType(iface)
		if !valid || len(args) == 0 {
			return zero, zero, zero, false
		}
		return iface, args[0], target, true
	}

	qualifiedName, args, alias, valid := dynamicMetadataNamedType(target)
	if !valid || len(args) == 0 {
		return zero, zero, zero, false
	}
	shortName := qualifiedName
	if dot := strings.LastIndex(shortName, "."); dot >= 0 {
		shortName = shortName[dot+1:]
	}
	known := false
	for _, decl := range localDecls {
		if ast2.DeclIsInterface(decl) && ast2.DeclInterfaceName(decl) == shortName {
			known = true
			break
		}
	}
	if !known && alias != "" {
		for _, dependency := range dependencies {
			if dependency.Alias != alias {
				continue
			}
			for _, decl := range dependency.Decls {
				if ast2.DeclIsInterface(decl) && ast2.DeclInterfaceName(decl) == shortName {
					known = true
					break
				}
			}
		}
	}
	if !known {
		return zero, zero, zero, false
	}
	return target, args[0], args[0], true
}

func dynamicHasNestedHiddenParams(method ast2.FuncSig, ifaceTps []string, ifaceArgs []ast2.TypeExpr, hiddenIndex int) bool {
	for index, param := range method.Params {
		if index == hiddenIndex {
			continue
		}
		if dynamicTypeContains(dynamicSubstitute(param.Type, ifaceTps, ifaceArgs), ifaceArgs[0]) {
			return true
		}
	}
	return false
}

func dynamicTypeContains(typ, target ast2.TypeExpr) bool {
	if ast2.TypesEqual(typ, target) {
		return true
	}
	switch {
	case ast2.TypeIsNamed(typ):
		for _, arg := range ast2.TypeNamedArgs(typ) {
			if dynamicTypeContains(arg, target) {
				return true
			}
		}
	case ast2.TypeIsFunc(typ):
		for _, param := range ast2.TypeFuncParams(typ) {
			if dynamicTypeContains(param, target) {
				return true
			}
		}
		return dynamicTypeContains(ast2.TypeFuncRet(typ), target)
	case ast2.TypeIsTuple(typ):
		for _, item := range ast2.TypeTupleItems(typ) {
			if dynamicTypeContains(item, target) {
				return true
			}
		}
	case ast2.TypeIsInlineGo(typ):
		return dynamicTypeContains(ast2.TypeInlineGoInner(typ), target)
	case ast2.TypeIsArray(typ):
		return dynamicTypeContains(ast2.TypeArrayElement(typ), target)
	}
	return false
}

func dynamicSubstitute(typ ast2.TypeExpr, params []string, args []ast2.TypeExpr) ast2.TypeExpr {
	switch {
	case ast2.TypeIsNamed(typ):
		name := ast2.TypeNamedName(typ)
		namedArgs := ast2.TypeNamedArgs(typ)
		if len(namedArgs) == 0 {
			for index, param := range params {
				if param == name && index < len(args) {
					return args[index]
				}
			}
		}
		result := make([]ast2.TypeExpr, len(namedArgs))
		for index, arg := range namedArgs {
			result[index] = dynamicSubstitute(arg, params, args)
		}
		return ast2.NamedTypeExpr(name, result)
	case ast2.TypeIsFunc(typ):
		funcParams := ast2.TypeFuncParams(typ)
		result := make([]ast2.TypeExpr, len(funcParams))
		for index, arg := range funcParams {
			result[index] = dynamicSubstitute(arg, params, args)
		}
		ret := dynamicSubstitute(ast2.TypeFuncRet(typ), params, args)
		return ast2.FuncTypeExpr(result, ret)
	case ast2.TypeIsTuple(typ):
		items := ast2.TypeTupleItems(typ)
		result := make([]ast2.TypeExpr, len(items))
		for index, arg := range items {
			result[index] = dynamicSubstitute(arg, params, args)
		}
		return ast2.TupleTypeExpr(result)
	case ast2.TypeIsArray(typ):
		return ast2.ArrayTypeExpr(
			dynamicSubstitute(ast2.TypeArrayElement(typ), params, args),
			ast2.TypeArrayLength(typ),
		)
	}
	return typ
}

func dynamicMetadataMethodByName(methods []ast2.FuncSig, name string) (ast2.FuncSig, bool) {
	for _, method := range methods {
		if method.Name == name {
			return method, true
		}
	}
	return ast2.FuncSig{}, false
}

func dynamicMetadataMethod(method ast2.FuncSig, packagePath string, aliases map[string]string, outer []string, scope string) metadata.Method {
	params := append(append([]string(nil), outer...), method.TypeParams...)
	return metadata.Method{Name: method.Name, TypeParams: dynamicMetadataTypeParams(method.TypeParams, scope+":"+method.Name), Parameters: dynamicMetadataFields(method.Params, packagePath, aliases, params, scope+":"+method.Name), Results: dynamicMetadataResults(method.Ret, packagePath, aliases, params, scope+":"+method.Name), Using: dynamicMetadataConstraints(method.Using, packagePath, aliases, params, scope+":"+method.Name)}
}

func dynamicMetadataFields(fields []ast2.Param, packagePath string, aliases map[string]string, params []string, scope string) []metadata.Field {
	result := make([]metadata.Field, 0, len(fields))
	for _, field := range fields {
		result = append(result, metadata.Field{Name: field.Name, Type: dynamicMetadataType(field.Type, packagePath, aliases, params, scope)})
	}
	return result
}

func dynamicMetadataResults(ret prelude.Option[ast2.TypeExpr], packagePath string, aliases map[string]string, params []string, scope string) []metadata.Type {
	if !abi.HasSome[ast2.TypeExpr](ret) {
		return nil
	}
	value := abi.ValueOr[ast2.TypeExpr](ret, ast2.UnitTypeExpr())
	if ast2.TypeIsTuple(value) {
		return dynamicMetadataTypes(ast2.TypeTupleItems(value), packagePath, aliases, params, scope)
	}
	if ast2.TypeIsUnit(value) {
		return nil
	}
	return []metadata.Type{dynamicMetadataType(value, packagePath, aliases, params, scope)}
}

func dynamicMetadataConstraints(constraints []ast2.Constraint, packagePath string, aliases map[string]string, params []string, scope string) []metadata.Constraint {
	result := make([]metadata.Constraint, 0, len(constraints))
	for _, constraint := range constraints {
		result = append(result, metadata.Constraint{Interface: constraint.Name, Arguments: dynamicMetadataTypes(constraint.Args, packagePath, aliases, params, scope)})
	}
	return result
}

func dynamicMetadataTypes(types []ast2.TypeExpr, packagePath string, aliases map[string]string, params []string, scope string) []metadata.Type {
	result := make([]metadata.Type, 0, len(types))
	for _, typ := range types {
		result = append(result, dynamicMetadataType(typ, packagePath, aliases, params, scope))
	}
	return result
}

func dynamicMetadataType(typ ast2.TypeExpr, packagePath string, aliases map[string]string, params []string, scope string) metadata.Type {
	switch {
	case ast2.TypeIsNamed(typ):
		name := ast2.TypeNamedName(typ)
		namedArgs := ast2.TypeNamedArgs(typ)
		for index, param := range params {
			if dynamicMetadataTypeParamName(param) == name {
				if len(namedArgs) == 0 {
					return metadata.Type{Kind: "typeParam", Name: dynamicMetadataTypeParamName(param), Scope: scope, Index: index}
				}
				return metadata.Type{Kind: "applied", Name: dynamicMetadataTypeParamName(param), Scope: scope, Index: index, Arguments: dynamicMetadataTypes(namedArgs, packagePath, aliases, params, scope)}
			}
		}
		switch name {
		case "Ref":
			if len(namedArgs) == 1 {
				item := dynamicMetadataType(namedArgs[0], packagePath, aliases, params, scope)
				return metadata.Type{Kind: "reference", Element: &item}
			}
		case "Slice", "Set":
			if len(namedArgs) == 1 {
				item := dynamicMetadataType(namedArgs[0], packagePath, aliases, params, scope)
				return metadata.Type{Kind: "slice", Element: &item}
			}
		case "Map":
			if len(namedArgs) == 2 {
				key := dynamicMetadataType(namedArgs[0], packagePath, aliases, params, scope)
				item := dynamicMetadataType(namedArgs[1], packagePath, aliases, params, scope)
				return metadata.Type{Kind: "map", Key: &key, Value: &item}
			}
		}
		if primitive := dynamicPrimitiveName(name); primitive != "" && len(namedArgs) == 0 {
			return metadata.Type{Kind: "builtin", Name: primitive}
		}
		typePackage := packagePath
		if dynamicPreludeTypeName(name) {
			typePackage = dynamicPreludeImportPath
		}
		if dot := strings.Index(name, "."); dot >= 0 {
			if aliasPath := aliases[name[:dot]]; aliasPath != "" {
				typePackage = aliasPath
				name = name[dot+1:]
			}
		}
		return metadata.Type{Kind: "named", Name: name, Package: typePackage, Arguments: dynamicMetadataTypes(namedArgs, packagePath, aliases, params, scope)}
	case ast2.TypeIsFunc(typ):
		return metadata.Type{Kind: "function", Parameters: dynamicMetadataTypes(ast2.TypeFuncParams(typ), packagePath, aliases, params, scope), Results: []metadata.Type{dynamicMetadataType(ast2.TypeFuncRet(typ), packagePath, aliases, params, scope)}}
	case ast2.TypeIsTuple(typ):
		return metadata.Type{Kind: "tuple", Arguments: dynamicMetadataTypes(ast2.TypeTupleItems(typ), packagePath, aliases, params, scope)}
	case ast2.TypeIsUnit(typ):
		return metadata.Type{Kind: "tuple"}
	case ast2.TypeIsInlineGo(typ):
		return dynamicMetadataType(ast2.TypeInlineGoInner(typ), packagePath, aliases, params, scope)
	}
	return metadata.Type{Kind: "builtin", Name: "any"}
}

func dynamicPreludeTypeName(name string) bool {
	switch name {
	case "Option", "Result", "Range":
		return true
	default:
		return false
	}
}

func dynamicMetadataTypeParamName(param string) string {
	if index := strings.IndexByte(param, '['); index >= 0 {
		return strings.TrimSpace(param[:index])
	}
	return param
}

func dynamicPrimitiveName(name string) string {
	switch name {
	case "Int":
		return "int"
	case "Int8":
		return "int8"
	case "Int16":
		return "int16"
	case "Int32", "rune":
		return "int32"
	case "Int64":
		return "int64"
	case "UInt":
		return "uint"
	case "UInt8", "byte":
		return "uint8"
	case "UInt16":
		return "uint16"
	case "UInt32":
		return "uint32"
	case "UInt64":
		return "uint64"
	case "Float32":
		return "float32"
	case "Float64":
		return "float64"
	case "Bool":
		return "bool"
	case "String":
		return "string"
	case "Any":
		return "any"
	case "Error":
		return "error"
	}
	return ""
}

func dynamicMetadataTypeParams(params []string, scope string) []metadata.TypeParam {
	result := make([]metadata.TypeParam, 0, len(params))
	for index, name := range params {
		result = append(result, metadata.TypeParam{Name: name, Scope: scope, Index: index})
	}
	return result
}

func dynamicNamedType(typ ast2.TypeExpr) (string, []ast2.TypeExpr, string, bool) {
	if !ast2.TypeIsNamed(typ) {
		return "", nil, "", false
	}
	name := ast2.TypeNamedName(typ)
	args := ast2.TypeNamedArgs(typ)
	if dot := strings.Index(name, "."); dot >= 0 {
		return name[dot+1:], args, name[:dot], true
	}
	return name, args, "", true
}

func dynamicMetadataNamedType(typ ast2.TypeExpr) (string, []ast2.TypeExpr, string, bool) {
	return dynamicNamedType(typ)
}

func dynamicNamedTypeKind(typ ast2.TypeExpr) string {
	if ast2.TypeIsNamed(typ) {
		return "named"
	}
	return "defined"
}

func dynamicMetadataName(typ ast2.TypeExpr) string {
	if ast2.TypeIsNamed(typ) {
		return ast2.TypeNamedName(typ)
	}
	return ""
}

func dynamicOptionString(value prelude.Option[string]) string {
	if abi.HasSome[string](value) {
		return abi.ValueOr[string](value, "")
	}
	return ""
}

func typePtr(value metadata.Type) *metadata.Type { return &value }

func ast2Exported(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}
