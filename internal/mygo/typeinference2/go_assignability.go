package typeinference2

import (
	"go/types"
	"strings"
	"sync"
)

var goAssignablePackages sync.Map // map[importPath]*types.Package

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

func lookupGoAssignablePackage(path string) (*types.Package, bool) {
	path = strings.TrimPrefix(path, "go:")
	value, ok := goAssignablePackages.Load(path)
	if !ok {
		return nil, false
	}
	pkg, ok := value.(*types.Package)
	return pkg, ok
}
