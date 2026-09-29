package compiler

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

// TestBootstrapGoPackageInfoCollectsPackageLevelVars pins that exported
// package-level `var`s reach the FFI surface, not just exported `const`s.
// Libraries routinely publish their sentinel values as vars (GORM's
// ErrRecordNotFound, os.ErrProcessDone); a selector over one has to type-check
// like any other imported value.
func TestBootstrapGoPackageInfoCollectsPackageLevelVars(t *testing.T) {
	osInfo, err := bootstrapLoadGoPackageGo(".", "os")
	if err != nil {
		t.Fatalf("load os: %v", err)
	}

	// os.ErrProcessDone is `var ErrProcessDone = errors.New(...)`, i.e. an
	// exported package-level var of type error.
	varType, ok := findGoConstant(osInfo.Constants, "ErrProcessDone")
	if !ok {
		t.Fatalf("os.ErrProcessDone not collected as a package-level value; got %d constants", len(osInfo.Constants))
	}
	if varType != "error" {
		t.Fatalf("os.ErrProcessDone type = %q, want error", varType)
	}

	// An exported const of the same shape must still be collected too, so the
	// var pass does not regress the const path.
	if _, ok := findGoConstant(osInfo.Constants, "O_RDONLY"); !ok {
		t.Fatal("os.O_RDONLY (an exported const) is no longer collected")
	}
}

// TestBootstrapGoPackageInfoSkipsUnexportedVars guards the exported-name
// filter on the shared const/var pass: an unexported package-level value must
// not enter the FFI surface.  time keeps unexported package-level state
// (startNano and friends), so it is a stable place to assert the filter.
func TestBootstrapGoPackageInfoSkipsUnexportedVars(t *testing.T) {
	timeInfo, err := bootstrapLoadGoPackageGo(".", "time")
	if err != nil {
		t.Fatalf("load time: %v", err)
	}
	for _, c := range timeInfo.Constants {
		if c.Name != "" && c.Name[0] >= 'a' && c.Name[0] <= 'z' {
			t.Fatalf("unexported package-level value %q leaked into the FFI surface", c.Name)
		}
	}
}

func findGoConstant(constants []typeinference2.GoConstSignature, name string) (string, bool) {
	for _, c := range constants {
		if c.Name == name {
			return c.Type, true
		}
	}
	return "", false
}
