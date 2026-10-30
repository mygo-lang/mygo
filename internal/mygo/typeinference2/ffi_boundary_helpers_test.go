package typeinference2

import (
	"go/token"
	"go/types"
	"testing"
)

// registerGoInterfaceFixture builds two named Go interfaces with go/types so
// the assignability tests exercise real method sets. The rest of the FFI
// boundary tests live in ffi_boundary_test.mygo.
func registerGoInterfaceFixture(t *testing.T, sourceHasWrite, sourceReadMismatch bool) {
	t.Helper()
	sourcePath := "example.test/source"
	targetPath := "example.test/target"
	sourcePkg := types.NewPackage(sourcePath, "source")
	targetPkg := types.NewPackage(targetPath, "target")
	byteSlice := types.NewSlice(types.Typ[types.Byte])
	intAndError := types.NewTuple(
		types.NewVar(token.NoPos, nil, "", types.Typ[types.Int]),
		types.NewVar(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
	)
	makeMethod := func(name string, mismatch bool) *types.Func {
		results := intAndError
		if mismatch {
			results = types.NewTuple(
				types.NewVar(token.NoPos, nil, "", types.Typ[types.Int]),
				types.NewVar(token.NoPos, nil, "", types.Typ[types.String]),
			)
		}
		sig := types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewVar(token.NoPos, nil, "", byteSlice)), results, false)
		return types.NewFunc(token.NoPos, nil, name, sig)
	}
	sourceMethods := []*types.Func{makeMethod("Read", sourceReadMismatch)}
	if sourceHasWrite {
		sourceMethods = append(sourceMethods, makeMethod("Write", false))
	}
	targetMethods := []*types.Func{makeMethod("Read", false), makeMethod("Write", false)}
	source := types.NewNamed(types.NewTypeName(token.NoPos, sourcePkg, "Source", nil), types.NewInterfaceType(sourceMethods, nil).Complete(), nil)
	target := types.NewNamed(types.NewTypeName(token.NoPos, targetPkg, "Reader", nil), types.NewInterfaceType(targetMethods, nil).Complete(), nil)
	sourcePkg.Scope().Insert(source.Obj())
	targetPkg.Scope().Insert(target.Obj())
	sourcePkg.MarkComplete()
	targetPkg.MarkComplete()
	RegisterGoPackage(sourcePkg)
	RegisterGoPackage(targetPkg)
}
