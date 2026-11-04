package typeinference2

import (
	"go/token"
	"go/types"
	"strings"
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

// registerGoAnonymousInterfaceFixture builds a real anonymous interface that
// embeds two named interfaces, registers it with the Go assignability service
// under the exact FFI spelling, and returns the paths needed by the MyGO test
// package metadata.
func registerGoAnonymousInterfaceFixture(t *testing.T, sourceHasWrite bool) AnonymousInterfaceFixture {
	t.Helper()
	targetPath := "example.test/anonymous-target"
	sourcePath := "example.test/anonymous-source"
	targetPkg := types.NewPackage(targetPath, "target")
	config := types.NewStruct(nil, nil)
	configPointer := types.NewPointer(config)
	method := func(name string, params *types.Tuple, results *types.Tuple) *types.Func {
		return types.NewFunc(token.NoPos, targetPkg, name, types.NewSignatureType(nil, nil, nil, params, results, false))
	}
	empty := types.NewTuple()
	setConfig := method("SetConfig", types.NewTuple(types.NewVar(token.NoPos, targetPkg, "config", configPointer)), empty)
	setHTMLOption := method("SetHTMLOption", types.NewTuple(types.NewVar(token.NoPos, targetPkg, "config", configPointer)), empty)
	write := method(
		"Write",
		types.NewTuple(types.NewVar(token.NoPos, targetPkg, "p", types.NewSlice(types.Typ[types.Byte]))),
		types.NewTuple(
			types.NewVar(token.NoPos, targetPkg, "", types.Typ[types.Int]),
			types.NewVar(token.NoPos, targetPkg, "", types.Universe.Lookup("error").Type()),
		),
	)
	badSetConfig := method("SetConfig", types.NewTuple(types.NewVar(token.NoPos, targetPkg, "value", types.Typ[types.String])), empty)
	option := types.NewNamed(
		types.NewTypeName(token.NoPos, targetPkg, "Option", nil),
		types.NewInterfaceType([]*types.Func{setConfig}, nil).Complete(),
		nil,
	)
	htmlOption := types.NewNamed(
		types.NewTypeName(token.NoPos, targetPkg, "HTMLOption", nil),
		types.NewInterfaceType([]*types.Func{setHTMLOption}, nil).Complete(),
		nil,
	)
	writer := types.NewNamed(
		types.NewTypeName(token.NoPos, targetPkg, "Writer", nil),
		types.NewInterfaceType([]*types.Func{setConfig, write}, nil).Complete(),
		nil,
	)
	badOption := types.NewNamed(
		types.NewTypeName(token.NoPos, targetPkg, "BadOption", nil),
		types.NewInterfaceType([]*types.Func{badSetConfig}, nil).Complete(),
		nil,
	)
	for _, named := range []*types.Named{option, htmlOption, writer, badOption} {
		targetPkg.Scope().Insert(named.Obj())
	}
	targetPkg.MarkComplete()
	RegisterGoPackage(targetPkg)

	methods := []*types.Func{}
	if sourceHasWrite {
		methods = append(methods, write)
	}
	anonymous := types.NewInterfaceType(methods, []types.Type{option, htmlOption}).Complete()
	qualifier := func(p *types.Package) string {
		if p == nil {
			return ""
		}
		return "go:" + strings.ReplaceAll(p.Path(), ".", "%2E")
	}
	spelling := types.TypeString(anonymous, qualifier)
	RegisterGoAnonymousType(sourcePath, spelling, anonymous)
	return AnonymousInterfaceFixture{
		SourcePath:  sourcePath,
		SourceAlias: "src",
		SourceType:  spelling,
		TargetPath:  targetPath,
		TargetAlias: "dst",
	}
}
