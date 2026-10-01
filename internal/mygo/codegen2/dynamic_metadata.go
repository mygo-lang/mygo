package codegen2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

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
	importList := imports.List()
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

func dynamicBareTypeNames(files []SourceFileInput) map[string]bool {
	namedType := reflect.TypeOf(ast2.TypeExpr__NamedType{})
	names := make(map[string]bool)
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
		if value.Type() == namedType {
			if value.Field(1).Len() == 0 {
				names[value.Field(0).String()] = true
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
		dynamicQualifiedName(ifaceAlias, ifaceName),
		dynamicQualifiedName(ifaceAlias, "New"+ifaceName),
		strconv.Quote(ref.value.ID),
	), nil
}

func dynamicConstructorName(ref dynamicImplementationRef) string {
	implementationName := ref.value.Name
	if implementationName == "" {
		implementationName = ref.value.Target.Name
	}
	packageName := pathBase(ref.producerPath)
	if packageName != "" {
		packageName = strings.ToUpper(packageName[:1]) + packageName[1:]
	}
	return "MygoDynamic" + sanitizeGoIdent(ref.value.Interface.Name) + "From" + sanitizeGoIdent(packageName) + sanitizeGoIdent(implementationName)
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
		item, ok := decl.(ast2.Decl__ImportDecl)
		if !ok {
			continue
		}
		goImport := strings.HasPrefix(item.F1, "go:")
		path := strings.TrimPrefix(item.F1, "go:")
		alias := item.F0
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
		if imported, ok := decl.(ast2.Decl__ImportDecl); ok {
			path := strings.TrimPrefix(imported.F1, "go:")
			alias := imported.F0
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

func dynamicMetadataExports(decls []ast2.Decl, importPath string, aliases map[string]string) metadata.Exports {
	var exports metadata.Exports
	for _, decl := range decls {
		switch value := decl.(type) {
		case ast2.Decl__TypeDecl:
			if !ast2Exported(value.F0) {
				continue
			}
			exports.Types = append(exports.Types, metadata.TypeExport{
				Name: value.F0, Kind: dynamicNamedTypeKind(value.F2), TypeParams: dynamicMetadataTypeParams(value.F1, "type:"+importPath+":"+value.F0),
				Underlying: typePtr(dynamicMetadataType(value.F2, importPath, aliases, value.F1, "type:"+value.F0)),
			})
		case ast2.Decl__TypeAliasDecl:
			if !ast2Exported(value.F0) {
				continue
			}
			exports.Aliases = append(exports.Aliases, metadata.AliasExport{
				Name: value.F0, TypeParams: dynamicMetadataTypeParams(value.F1, "alias:"+importPath+":"+value.F0),
				Target: dynamicMetadataType(value.F2, importPath, aliases, value.F1, "alias:"+value.F0),
			})
		case ast2.Decl__StructDecl:
			if !ast2Exported(value.F0) {
				continue
			}
			fields := make([]metadata.Field, 0, len(value.F2))
			for _, field := range value.F2 {
				fields = append(fields, metadata.Field{Name: field.Name, Type: dynamicMetadataType(field.Type, importPath, aliases, value.F1, "type:"+value.F0), Tag: dynamicOptionString(field.Tag)})
			}
			exports.Types = append(exports.Types, metadata.TypeExport{Name: value.F0, Kind: "struct", TypeParams: dynamicMetadataTypeParams(value.F1, "type:"+importPath+":"+value.F0), Fields: fields})
		case ast2.Decl__EnumDecl:
			if !ast2Exported(value.F0) {
				continue
			}
			variants := make([]metadata.Variant, 0, len(value.F2))
			for _, variant := range value.F2 {
				fields := make([]metadata.Field, 0, len(variant.Fields))
				for index, fieldType := range variant.Fields {
					fieldName := ""
					if variant.Named && index < len(variant.Names) {
						fieldName = variant.Names[index]
					}
					fields = append(fields, metadata.Field{Name: fieldName, Type: dynamicMetadataType(fieldType, importPath, aliases, value.F1, "type:"+value.F0)})
				}
				variants = append(variants, metadata.Variant{Name: variant.Name, Fields: fields})
			}
			exports.Types = append(exports.Types, metadata.TypeExport{Name: value.F0, Kind: "enum", TypeParams: dynamicMetadataTypeParams(value.F1, "type:"+importPath+":"+value.F0), Variants: variants})
		case ast2.Decl__FuncDecl:
			if !ast2Exported(value.F0) {
				continue
			}
			exports.Functions = append(exports.Functions, metadata.FunctionExport{
				Name: value.F0, TypeParams: dynamicMetadataTypeParams(value.F1, "func:"+importPath+":"+value.F0),
				Parameters: dynamicMetadataFields(value.F2, importPath, aliases, value.F1, "func:"+value.F0),
				Results:    dynamicMetadataResults(value.F3, importPath, aliases, value.F1, "func:"+value.F0),
				Using:      dynamicMetadataConstraints(value.F5, importPath, aliases, value.F1, "func:"+value.F0),
			})
		case ast2.Decl__InterfaceDecl:
			if !ast2Exported(value.F0) {
				continue
			}
			methods := make([]metadata.Method, 0, len(value.F2))
			for _, method := range value.F2 {
				methods = append(methods, dynamicMetadataMethod(method, importPath, aliases, value.F1, "interface:"+value.F0))
			}
			exports.Interfaces = append(exports.Interfaces, metadata.InterfaceExport{Name: value.F0, TypeParams: dynamicMetadataTypeParams(value.F1, "interface:"+importPath+":"+value.F0), Methods: methods})
			exports.Types = append(exports.Types, metadata.TypeExport{
				Name: "MygoDynamic" + sanitizeGoIdent(value.F0), Kind: "defined",
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
		impl, ok := decl.(ast2.Decl__ImplDecl)
		if !ok {
			continue
		}
		iface, dynamicTarget, adapterTarget, ok := dynamicMetadataImplInterface(impl, decls, dependencies)
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
				if interfaceDecl, match := candidate.(ast2.Decl__InterfaceDecl); match && interfaceDecl.F0 == ifaceName {
					ifacePackage = importPath
					break
				}
			}
		}
		var ifaceDecl *ast2.Decl__InterfaceDecl
		for i := range allDecls {
			candidate, match := allDecls[i].(ast2.Decl__InterfaceDecl)
			if match && candidate.F0 == ifaceName {
				ifaceDecl = &candidate
				break
			}
		}
		if ifaceDecl == nil {
			continue
		}
		if ifacePackage == "" {
			for _, dependency := range dependencies {
				for _, candidate := range dependency.Decls {
					if interfaceDecl, match := candidate.(ast2.Decl__InterfaceDecl); match && interfaceDecl.F0 == ifaceName {
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
			Interface: metadata.TypeclassID{Package: ifacePackage, Name: ifaceName, Arguments: dynamicMetadataTypes(ifaceArgs, importPath, aliases, impl.F0, "impl:"+instanceName)},
			Target:    dynamicMetadataType(dynamicTarget, importPath, aliases, impl.F0, "impl:"+instanceName),
			Name:      instanceName,
		}
		id, err := metadata.StableImplementationID(identity)
		if err != nil {
			continue
		}
		record := metadata.Implementation{ID: id, Name: identity.Name, Interface: identity.Interface, Target: identity.Target, TypeParams: dynamicMetadataTypeParams(impl.F0, "impl:"+id)}
		for _, method := range impl.F3 {
			ifaceMethod, exists := dynamicMetadataMethodByName(ifaceDecl.F2, method.Sig.Name)
			adapter := metadata.Adapter{Method: method.Sig.Name, ABI: metadata.AdapterABI, Supported: false}
			if exists {
				adapter.LogicalParameters = dynamicMetadataFields(method.Sig.Params, importPath, aliases, append(impl.F0, method.Sig.TypeParams...), "method:"+id+":"+method.Sig.Name)
				adapter.LogicalResults = dynamicMetadataResults(method.Sig.Ret, importPath, aliases, append(impl.F0, method.Sig.TypeParams...), "method:"+id+":"+method.Sig.Name)
				adapter.LogicalUsing = dynamicMetadataConstraints(method.Sig.Using, importPath, aliases, append(impl.F0, method.Sig.TypeParams...), "method:"+id+":"+method.Sig.Name)
				adapter.TypeParams = dynamicMetadataTypeParams(method.Sig.TypeParams, "method:"+id+":"+method.Sig.Name)
				if len(impl.F0) > 0 {
					adapter.UnsupportedReason = "adapter cannot capture implementation target type parameters"
				} else if len(method.Sig.Using) > 0 {
					adapter.UnsupportedReason = "method requires using dictionaries outside the dynamic adapter ABI"
				} else {
					methodOption := prelude.Some[ast2.FuncSig](ifaceMethod)
						hidden := dynamicHiddenParameter(method.Sig.Params, dynamicTarget, methodOption, ifaceDecl.F1, ifaceArgs, 0, -1)
					if hiddenIndex, ok := hidden.(prelude.Option__Some[int]); ok {
						if dynamicHasNestedHiddenParams(ifaceMethod, ifaceDecl.F1, ifaceArgs, hiddenIndex.F0) {
							adapter.UnsupportedReason = "method parameter contains a callback or nested value of the hidden type"
						} else {
							adapter.Supported = true
							adapter.Symbol = dynamicAdapterSymbol(implStem(adapterTarget, prelude.Some[ast2.TypeExpr](iface), typeParamSet(impl.F0)), ifaceName, method.Sig.Name)
							adapter.HiddenParameterIndex = hiddenIndex.F0
							adapter.GoParameters = []metadata.Type{{Kind: "builtin", Name: "any"}}
							for index, param := range method.Sig.Params {
								if index != hiddenIndex.F0 {
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
func dynamicMetadataImplInterface(impl ast2.Decl__ImplDecl, localDecls []ast2.Decl, dependencies []typeinference2.MyGoPackageInfo) (iface ast2.TypeExpr, dynamicTarget ast2.TypeExpr, adapterTarget ast2.TypeExpr, ok bool) {
	if named, exists := impl.F2.(prelude.Option__Some[ast2.TypeExpr]); exists {
		iface = named.F0
		_, args, _, valid := dynamicMetadataNamedType(iface)
		if !valid || len(args) == 0 {
			return nil, nil, nil, false
		}
		return iface, args[0], impl.F1, true
	}

	qualifiedName, args, alias, valid := dynamicMetadataNamedType(impl.F1)
	if !valid || len(args) == 0 {
		return nil, nil, nil, false
	}
	shortName := qualifiedName
	if dot := strings.LastIndex(shortName, "."); dot >= 0 {
		shortName = shortName[dot+1:]
	}
	known := false
	for _, decl := range localDecls {
		if interfaceDecl, match := decl.(ast2.Decl__InterfaceDecl); match && interfaceDecl.F0 == shortName {
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
				if interfaceDecl, match := decl.(ast2.Decl__InterfaceDecl); match && interfaceDecl.F0 == shortName {
					known = true
					break
				}
			}
		}
	}
	if !known {
		return nil, nil, nil, false
	}
	return impl.F1, args[0], args[0], true
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
	switch value := typ.(type) {
	case ast2.TypeExpr__NamedType:
		for _, arg := range value.F1 {
			if dynamicTypeContains(arg, target) {
				return true
			}
		}
	case ast2.TypeExpr__FuncType:
		for _, param := range value.F0 {
			if dynamicTypeContains(param, target) {
				return true
			}
		}
		return dynamicTypeContains(*value.F1, target)
	case ast2.TypeExpr__TupleType:
		for _, item := range value.F0 {
			if dynamicTypeContains(item, target) {
				return true
			}
		}
	case ast2.TypeExpr__InlineGo:
		return dynamicTypeContains(*value.F0, target)
	}
	return false
}

func dynamicSubstitute(typ ast2.TypeExpr, params []string, args []ast2.TypeExpr) ast2.TypeExpr {
	switch value := typ.(type) {
	case ast2.TypeExpr__NamedType:
		if len(value.F1) == 0 {
			for index, param := range params {
				if param == value.F0 && index < len(args) {
					return args[index]
				}
			}
		}
		result := make([]ast2.TypeExpr, len(value.F1))
		for index, arg := range value.F1 {
			result[index] = dynamicSubstitute(arg, params, args)
		}
		return ast2.TypeExpr__NamedType{F0: value.F0, F1: result}
	case ast2.TypeExpr__FuncType:
		result := make([]ast2.TypeExpr, len(value.F0))
		for index, arg := range value.F0 {
			result[index] = dynamicSubstitute(arg, params, args)
		}
		ret := dynamicSubstitute(*value.F1, params, args)
		return ast2.TypeExpr__FuncType{F0: result, F1: &ret}
	case ast2.TypeExpr__TupleType:
		result := make([]ast2.TypeExpr, len(value.F0))
		for index, arg := range value.F0 {
			result[index] = dynamicSubstitute(arg, params, args)
		}
		return ast2.TypeExpr__TupleType{F0: result}
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
	value, ok := ret.(prelude.Option__Some[ast2.TypeExpr])
	if !ok {
		return nil
	}
	if tuple, ok := value.F0.(ast2.TypeExpr__TupleType); ok {
		return dynamicMetadataTypes(tuple.F0, packagePath, aliases, params, scope)
	}
	if _, unit := value.F0.(ast2.TypeExpr__UnitType); unit {
		return nil
	}
	return []metadata.Type{dynamicMetadataType(value.F0, packagePath, aliases, params, scope)}
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
	switch value := typ.(type) {
	case ast2.TypeExpr__NamedType:
		for index, param := range params {
			if dynamicMetadataTypeParamName(param) == value.F0 {
				if len(value.F1) == 0 {
					return metadata.Type{Kind: "typeParam", Name: dynamicMetadataTypeParamName(param), Scope: scope, Index: index}
				}
				return metadata.Type{Kind: "applied", Name: dynamicMetadataTypeParamName(param), Scope: scope, Index: index, Arguments: dynamicMetadataTypes(value.F1, packagePath, aliases, params, scope)}
			}
		}
		switch value.F0 {
		case "Ref":
			if len(value.F1) == 1 {
				item := dynamicMetadataType(value.F1[0], packagePath, aliases, params, scope)
				return metadata.Type{Kind: "reference", Element: &item}
			}
		case "Slice", "Set":
			if len(value.F1) == 1 {
				item := dynamicMetadataType(value.F1[0], packagePath, aliases, params, scope)
				return metadata.Type{Kind: "slice", Element: &item}
			}
		case "Map":
			if len(value.F1) == 2 {
				key := dynamicMetadataType(value.F1[0], packagePath, aliases, params, scope)
				item := dynamicMetadataType(value.F1[1], packagePath, aliases, params, scope)
				return metadata.Type{Kind: "map", Key: &key, Value: &item}
			}
		}
		if primitive := dynamicPrimitiveName(value.F0); primitive != "" && len(value.F1) == 0 {
			return metadata.Type{Kind: "builtin", Name: primitive}
		}
		name := value.F0
		typePackage := packagePath
		if dot := strings.Index(name, "."); dot >= 0 {
			if aliasPath := aliases[name[:dot]]; aliasPath != "" {
				typePackage = aliasPath
				name = name[dot+1:]
			}
		}
		kind := "named"
		if len(value.F1) > 0 {
			kind = "named"
		}
		return metadata.Type{Kind: kind, Name: name, Package: typePackage, Arguments: dynamicMetadataTypes(value.F1, packagePath, aliases, params, scope)}
	case ast2.TypeExpr__FuncType:
		return metadata.Type{Kind: "function", Parameters: dynamicMetadataTypes(value.F0, packagePath, aliases, params, scope), Results: []metadata.Type{dynamicMetadataType(*value.F1, packagePath, aliases, params, scope)}}
	case ast2.TypeExpr__TupleType:
		return metadata.Type{Kind: "tuple", Arguments: dynamicMetadataTypes(value.F0, packagePath, aliases, params, scope)}
	case ast2.TypeExpr__UnitType:
		return metadata.Type{Kind: "tuple"}
	case ast2.TypeExpr__InlineGo:
		return dynamicMetadataType(*value.F0, packagePath, aliases, params, scope)
	}
	return metadata.Type{Kind: "builtin", Name: "any"}
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
	value, ok := typ.(ast2.TypeExpr__NamedType)
	if !ok {
		return "", nil, "", false
	}
	name := value.F0
	if dot := strings.Index(name, "."); dot >= 0 {
		return name[dot+1:], value.F1, name[:dot], true
	}
	return name, value.F1, "", true
}

func dynamicMetadataNamedType(typ ast2.TypeExpr) (string, []ast2.TypeExpr, string, bool) {
	return dynamicNamedType(typ)
}

func dynamicNamedTypeKind(typ ast2.TypeExpr) string {
	if _, ok := typ.(ast2.TypeExpr__NamedType); ok {
		return "named"
	}
	return "defined"
}

func dynamicMetadataName(typ ast2.TypeExpr) string {
	if named, ok := typ.(ast2.TypeExpr__NamedType); ok {
		return named.F0
	}
	return ""
}

func dynamicOptionString(value prelude.Option[string]) string {
	if some, ok := value.(prelude.Option__Some[string]); ok {
		return some.F0
	}
	return ""
}

func typePtr(value metadata.Type) *metadata.Type { return &value }

func ast2Exported(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}
