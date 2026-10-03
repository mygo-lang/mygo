package compiler

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

// TestBootstrapGoPackageInfoExtractsFieldsAndUnderlying pins the FFI loader's
// export surface extensions: exported struct fields flow into
// GoTypeSignature.Fields, and the rendered underlying type string flows into
// GoTypeSignature.Underlying (func() for the named function type
// context.CancelFunc).  The latter drives boundary aliasing of callable named
// types in typeinference2.
func TestBootstrapGoPackageInfoExtractsFieldsAndUnderlying(t *testing.T) {
	httpInfo, err := bootstrapLoadGoPackageGo(".", "net/http")
	if err != nil {
		t.Fatalf("load net/http: %v", err)
	}
	request, ok := findGoType(httpInfo.Types, "Request")
	if !ok {
		t.Fatalf("net/http exports no Request type; got %d types", len(httpInfo.Types))
	}
	headerField, ok := findGoField(request.Fields, "Header")
	if !ok {
		t.Fatalf("http.Request has no registered Header field: %+v", request.Fields)
	}
	if headerField.Type != "http.Header" {
		t.Fatalf("http.Request.Header type = %q, want http.Header", headerField.Type)
	}
	if len(request.Methods) == 0 {
		t.Fatalf("http.Request should export methods, got none")
	}

	ctxInfo, err := bootstrapLoadGoPackageGo(".", "context")
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	cancelFunc, ok := findGoType(ctxInfo.Types, "CancelFunc")
	if !ok {
		t.Fatalf("context exports no CancelFunc type")
	}
	if cancelFunc.Underlying != "func()" {
		t.Fatalf("context.CancelFunc underlying = %q, want func()", cancelFunc.Underlying)
	}
}

func TestBootstrapGoPackageInfoPreservesImportedTypePath(t *testing.T) {
	mailPkg := types.NewPackage("net/mail", "mail")
	message := types.NewNamed(
		types.NewTypeName(token.NoPos, mailPkg, "Message", nil),
		types.NewStruct(nil, nil),
		nil,
	)
	sig := types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(types.NewVar(token.NoPos, nil, "message", types.NewPointer(message))),
		types.NewTuple(),
		false,
	)
	serverPkg := types.NewPackage("example.test/smtpserver", "smtpserver")
	serverPkg.Scope().Insert(types.NewFunc(token.NoPos, serverPkg, "Register", sig))

	info := bootstrapGoPackageInfoFromTypes(serverPkg)
	register, ok := findGoFunc(info.Funcs, "Register")
	if !ok {
		t.Fatal("smtpserver package has no Register function")
	}
	if got, want := register.Params[0], "*go:net/mail.Message"; got != want {
		t.Fatalf("Register parameter type = %q, want %q", got, want)
	}
}

func TestBootstrapGoPackageInfoRecordsDirectFunctionAliases(t *testing.T) {
	pkg := types.NewPackage("example.test/smtpserver", "listener")
	stringType := types.Typ[types.String]
	emptyStruct := types.NewStruct(nil, nil)
	errorType := types.Universe.Lookup("error").Type()
	params := types.NewTuple(types.NewVar(token.NoPos, pkg, "", stringType))
	results := types.NewTuple(types.NewVar(token.NoPos, pkg, "", emptyStruct), types.NewVar(token.NoPos, pkg, "", errorType))
	sig := types.NewSignatureType(nil, nil, nil, params, results, false)

	aliasName := types.NewTypeName(token.NoPos, pkg, "SendAlias", nil)
	types.NewAlias(aliasName, sig)
	pkg.Scope().Insert(aliasName)
	definedName := types.NewTypeName(token.NoPos, pkg, "SendDefined", nil)
	types.NewNamed(definedName, sig, nil)
	pkg.Scope().Insert(definedName)

	info := bootstrapGoPackageInfoFromTypes(pkg)
	if info.GoName != "listener" {
		t.Fatalf("Go package name = %q, want listener", info.GoName)
	}
	for _, name := range []string{"SendAlias", "SendDefined"} {
		got, ok := findGoType(info.Types, name)
		if !ok {
			t.Fatalf("missing callback type %s in metadata", name)
		}
		if got.Underlying != "func(string) (struct{}, error)" {
			t.Fatalf("%s underlying = %q, want func(string) (struct{}, error)", name, got.Underlying)
		}
	}
}

func TestGoInterfaceMethodIdentityIncludesUnexportedPackagePath(t *testing.T) {
	makeInterface := func(path string) *types.Named {
		pkg := types.NewPackage(path, "fixture")
		sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false)
		method := types.NewFunc(token.NoPos, pkg, "private", sig)
		iface := types.NewInterfaceType([]*types.Func{method}, nil).Complete()
		name := types.NewTypeName(token.NoPos, pkg, "Hidden", nil)
		return types.NewNamed(name, iface, nil)
	}
	left := goInterfaceMethodSet(makeInterface("example.test/left"))
	right := goInterfaceMethodSet(makeInterface("example.test/right"))
	if len(left) != 1 || len(right) != 1 {
		t.Fatalf("method sets = %d and %d, want one private method each", len(left), len(right))
	}
	if left[0].PackagePath != "example.test/left" || right[0].PackagePath != "example.test/right" {
		t.Fatalf("unexported method package identities = %q and %q", left[0].PackagePath, right[0].PackagePath)
	}
	if left[0].Signature != right[0].Signature {
		t.Fatalf("identical private signatures differ unexpectedly: %q vs %q", left[0].Signature, right[0].Signature)
	}
}

func TestBootstrapGoPackageInfoExtractsCompleteInterfaceMethodSets(t *testing.T) {
	info, err := bootstrapLoadGoPackageGo(".", "io")
	if err != nil {
		t.Fatalf("load io: %v", err)
	}
	reader, ok := findGoType(info.Types, "Reader")
	if !ok {
		t.Fatal("io exports no Reader type")
	}
	readCloser, ok := findGoType(info.Types, "ReadCloser")
	if !ok {
		t.Fatal("io exports no ReadCloser type")
	}
	findMethod := func(methods []typeinference2.GoInterfaceMethodSignature, name string) (typeinference2.GoInterfaceMethodSignature, bool) {
		for _, method := range methods {
			if method.Name == name {
				return method, true
			}
		}
		return typeinference2.GoInterfaceMethodSignature{}, false
	}
	readerRead, ok := findMethod(reader.InterfaceMethods, "Read")
	if !ok || readerRead.Signature == "" {
		t.Fatalf("io.Reader complete method set lacks signature identity for Read: %+v", reader.InterfaceMethods)
	}
	readCloserRead, ok := findMethod(readCloser.InterfaceMethods, "Read")
	if !ok || readCloserRead.Signature != readerRead.Signature {
		t.Fatalf("io.ReadCloser.Read identity = %+v, want %+v", readCloserRead, readerRead)
	}
	if _, ok := findMethod(readCloser.InterfaceMethods, "Close"); !ok {
		t.Fatalf("io.ReadCloser complete method set lacks promoted/declared Close: %+v", readCloser.InterfaceMethods)
	}
}

func findGoType(types []typeinference2.GoTypeSignature, name string) (typeinference2.GoTypeSignature, bool) {
	for _, t := range types {
		if t.TypeName == name {
			return t, true
		}
	}
	return typeinference2.GoTypeSignature{}, false
}

func findGoFunc(funcs []typeinference2.GoFuncSignature, name string) (typeinference2.GoFuncSignature, bool) {
	for _, fn := range funcs {
		if fn.Name == name {
			return fn, true
		}
	}
	return typeinference2.GoFuncSignature{}, false
}

func findGoField(fields []typeinference2.GoFieldSignature, name string) (typeinference2.GoFieldSignature, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return typeinference2.GoFieldSignature{}, false
}
