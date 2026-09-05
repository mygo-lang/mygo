package compiler

import (
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

func findGoType(types []typeinference2.GoTypeSignature, name string) (typeinference2.GoTypeSignature, bool) {
	for _, t := range types {
		if t.TypeName == name {
			return t, true
		}
	}
	return typeinference2.GoTypeSignature{}, false
}

func findGoField(fields []typeinference2.GoFieldSignature, name string) (typeinference2.GoFieldSignature, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return typeinference2.GoFieldSignature{}, false
}
