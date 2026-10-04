package typeinference2

import (
	"go/token"
	"go/types"
	"testing"
)

func TestGoTypeAssignableUsesGoNamedAndPointerRules(t *testing.T) {
	backendPath := "example.test/smtpserver"
	backendPkg := types.NewPackage(backendPath, "smtpserver")
	backend := types.NewNamed(types.NewTypeName(token.NoPos, backendPkg, "Backend", nil), types.NewStruct(nil, nil), nil)
	newSessionSig := types.NewSignatureType(
		types.NewVar(token.NoPos, backendPkg, "receiver", types.NewPointer(backend)),
		nil,
		nil,
		types.NewTuple(),
		types.NewTuple(),
		false,
	)
	backend.AddMethod(types.NewFunc(token.NoPos, backendPkg, "Serve", newSessionSig))
	backendPkg.Scope().Insert(backend.Obj())
	backendPkg.MarkComplete()

	interfacePath := "example.test/smtp"
	interfacePkg := types.NewPackage(interfacePath, "smtp")
	serveSig := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)
	iface := types.NewNamed(
		types.NewTypeName(token.NoPos, interfacePkg, "ServerBackend", nil),
		types.NewInterfaceType([]*types.Func{types.NewFunc(token.NoPos, interfacePkg, "Serve", serveSig)}, nil).Complete(),
		nil,
	)
	interfacePkg.Scope().Insert(iface.Obj())
	interfacePkg.MarkComplete()

	RegisterGoPackage(backendPkg)
	RegisterGoPackage(interfacePkg)
	if !GoTypeAssignable("go:"+backendPath, "Backend", true, "go:"+interfacePath, "ServerBackend") {
		t.Fatal("*Backend with pointer receiver method should be assignable to ServerBackend")
	}
	if GoTypeAssignable("go:"+backendPath, "Backend", false, "go:"+interfacePath, "ServerBackend") {
		t.Fatal("Backend value without the pointer receiver method must not be assignable")
	}
}

func TestGoTypeAssignableRejectsDistinctNamedTypes(t *testing.T) {
	leftPath := "example.test/left"
	rightPath := "example.test/right"
	leftPkg := types.NewPackage(leftPath, "left")
	rightPkg := types.NewPackage(rightPath, "right")
	left := types.NewNamed(types.NewTypeName(token.NoPos, leftPkg, "ID", nil), types.Typ[types.Int], nil)
	right := types.NewNamed(types.NewTypeName(token.NoPos, rightPkg, "ID", nil), types.Typ[types.Int], nil)
	leftPkg.Scope().Insert(left.Obj())
	rightPkg.Scope().Insert(right.Obj())
	leftPkg.MarkComplete()
	rightPkg.MarkComplete()
	RegisterGoPackage(leftPkg)
	RegisterGoPackage(rightPkg)
	if GoTypeAssignable("go:"+leftPath, "ID", false, "go:"+rightPath, "ID") {
		t.Fatal("distinct named types with identical underlying types must remain incompatible")
	}
}

func TestGoTypeAssignableResolvesAliases(t *testing.T) {
	interfacePath := "example.test/alias-target"
	aliasPath := "example.test/alias-source"
	targetPkg := types.NewPackage(interfacePath, "target")
	method := types.NewFunc(token.NoPos, targetPkg, "Read", types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false))
	iface := types.NewNamed(types.NewTypeName(token.NoPos, targetPkg, "Reader", nil), types.NewInterfaceType([]*types.Func{method}, nil).Complete(), nil)
	targetPkg.Scope().Insert(iface.Obj())
	sourcePkg := types.NewPackage(aliasPath, "source")
	alias := types.NewAlias(types.NewTypeName(token.NoPos, sourcePkg, "ReaderAlias", nil), iface)
	sourcePkg.Scope().Insert(alias.Obj())
	sourcePkg.MarkComplete()
	targetPkg.MarkComplete()
	RegisterGoPackage(targetPkg)
	RegisterGoPackage(sourcePkg)
	if !GoTypeAssignable("go:"+aliasPath, "ReaderAlias", false, "go:"+interfacePath, "Reader") {
		t.Fatal("alias to an interface should be assignable to that interface")
	}
}
