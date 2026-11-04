package typeinference2

import (
	"go/types"
	"strings"
	"sync"
)

var goAssignablePackages sync.Map // map[importPath]*types.Package

var goAnonymousTypes sync.Map // map[ownerPath + NUL + renderedType]types.Type

// RegisterGoPackage makes type information loaded by the bootstrap FFI
// loader available to the inference boundary compatibility query.
func RegisterGoPackage(pkg *types.Package) {
	if pkg == nil {
		return
	}
	if pkg.Complete() {
		if existing, loaded := goAssignablePackages.LoadOrStore(pkg.Path(), pkg); loaded {
			if existing == pkg {
				return
			}
			goAssignablePackages.Store(pkg.Path(), pkg)
		}
	}
	for _, imported := range pkg.Imports() {
		RegisterGoPackage(imported)
	}
}

// RegisterGoAnonymousType makes an anonymous Go type from an FFI signature
// available to boundary assignability checks. The key is the exact rendering
// used in the serialized signature so the self-hosted parser can carry it as an
// opaque type identity without reconstructing the anonymous type itself.
func RegisterGoAnonymousType(ownerPath, rendered string, typ types.Type) {
	if rendered == "" || typ == nil {
		return
	}
	goAnonymousTypes.Store(anonymousTypeKey(ownerPath, rendered), typ)
}

// GoTypeAssignable reports Go assignability for two imported named types.
// sourcePointer models MyGO Ref[T], which lowers to *T at the Go boundary.
func GoTypeAssignable(sourcePath, sourceName string, sourcePointer bool, targetPath, targetName string) bool {
	sourcePkg, sourceOK := lookupGoAssignablePackage(sourcePath)
	targetPkg, targetOK := lookupGoAssignablePackage(targetPath)
	if !sourceOK || !targetOK {
		return false
	}
	sourceObj, sourceOK := sourcePkg.Scope().Lookup(sourceName).(*types.TypeName)
	targetObj, targetOK := targetPkg.Scope().Lookup(targetName).(*types.TypeName)
	if !sourceOK || !targetOK {
		return false
	}
	sourceType := types.Unalias(sourceObj.Type())
	if sourcePointer {
		sourceType = types.NewPointer(sourceType)
	}
	return types.AssignableTo(sourceType, types.Unalias(targetObj.Type()))
}

// GoAnonymousTypeAssignable reports Go assignability from an anonymous source
// type registered by the FFI loader to a named target type.
func GoAnonymousTypeAssignable(ownerPath, sourceKey, targetPath, targetName string) bool {
	sourceValue, ok := goAnonymousTypes.Load(anonymousTypeKey(ownerPath, sourceKey))
	if !ok {
		return false
	}
	sourceType, ok := sourceValue.(types.Type)
	if !ok {
		return false
	}
	targetPkg, targetOK := lookupGoAssignablePackage(targetPath)
	if !targetOK {
		return false
	}
	targetObj, targetOK := targetPkg.Scope().Lookup(targetName).(*types.TypeName)
	if !targetOK {
		return false
	}
	return types.AssignableTo(types.Unalias(sourceType), types.Unalias(targetObj.Type()))
}

func anonymousTypeKey(ownerPath, rendered string) string {
	return strings.TrimPrefix(ownerPath, "go:") + "\x00" + rendered
}

func lookupGoAssignablePackage(path string) (*types.Package, bool) {
	path = strings.TrimPrefix(path, "go:")
	value, ok := goAssignablePackages.Load(path)
	if !ok {
		return nil, false
	}
	pkg, ok := value.(*types.Package)
	return pkg, ok
}
