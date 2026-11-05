package codegen2

import (
	"strings"
	"testing"
)

func TestGenerateSourceInlineGoRawOptionRef(t *testing.T) {
	src := "package sample\n\nimport rsa \"go:crypto/rsa\"\n\nfunc maybe(present: Bool) -> Option[Ref[rsa.PublicKey]]\n  go[Option[Ref[rsa.PublicKey]]] {\n    raw: Ref[rsa.PublicKey]\n    code: \"func() *rsa.PublicKey { if !{present} { return nil }; return &rsa.PublicKey{} }()\"\n    in present = present\n  }\nend\n"
	code := generateRawSnippet(t, src)
	for _, want := range []string{"Some[*rsa.PublicKey]", "None[*rsa.PublicKey]", "== nil"} {
		if !strings.Contains(code, want) {
			t.Fatalf("raw Option[Ref] output missing %q:\n%s", want, code)
		}
	}
}

func TestGenerateSourceInlineGoRawOptionTuple(t *testing.T) {
	src := "package sample\n\nfunc pair(present: Bool) -> Option[Int]\n  go[Option[Int]] {\n    raw: (Int, Bool)\n    code: \"func() (int, bool) { if {present} { return 42, true }; return 0, false }()\"\n    in present = present\n  }\nend\n"
	code := generateRawSnippet(t, src)
	for _, want := range []string{"Some[int]", "None[int]"} {
		if !strings.Contains(code, want) {
			t.Fatalf("raw Option tuple output missing %q:\n%s", want, code)
		}
	}
}

func TestGenerateSourceInlineGoRawResultString(t *testing.T) {
	src := "package sample\n\nimport errors \"go:errors\"\n\nfunc run(ok: Bool) -> Result[Int, String]\n  go[Result[Int, String]] {\n    raw: (Int, Error)\n    code: \"func() (int, error) { if {ok} { return 7, nil }; return 0, errors.New(\\\"failed\\\") }()\"\n    in ok = ok\n  }\nend\n"
	code := generateRawSnippet(t, src)
	for _, want := range []string{"Ok[int, string]", "Err[int, string]", ".Error()"} {
		if !strings.Contains(code, want) {
			t.Fatalf("raw Result output missing %q:\n%s", want, code)
		}
	}
}

func TestGenerateSourceInlineGoRawLoneError(t *testing.T) {
	src := "package sample\n\nimport errors \"go:errors\"\n\nfunc run(ok: Bool) -> Result[(), String]\n  go[Result[(), String]] {\n    raw: Error\n    code: \"func() error { if {ok} { return nil }; return errors.New(\\\"failed\\\") }()\"\n    in ok = ok\n  }\nend\n"
	code := generateRawSnippet(t, src)
	for _, want := range []string{"Ok[struct{}, string]", "Err[struct{}, string]", ".Error()"} {
		if !strings.Contains(code, want) {
			t.Fatalf("raw lone-error output missing %q:\n%s", want, code)
		}
	}
}

func TestGenerateSourceInlineGoRawRejectsUnsupportedPair(t *testing.T) {
	src := "package sample\n\nfunc bad() -> Option[Int]\n  go[Option[Int]] {\n    raw: String\n    code: \"\\\"x\\\"\"\n  }\nend\n"
	result := GenerateSource(src)
	_, yes := resultOk[string, string](result)
	if yes {
		t.Fatalf("unsupported raw pair unexpectedly generated")
	}
	err, isErr := resultErr[string, string](result)
	if !isErr {
		t.Fatalf("unsupported raw pair did not return an error")
	}
	if !strings.Contains(err.F0, "unsupported inline-Go raw boundary") {
		t.Fatalf("unsupported raw pair diagnostic = %q", err.F0)
	}
}

func generateRawSnippet(t *testing.T, src string) string {
	t.Helper()
	got := GenerateSource(src)
	ok, yes := resultOk[string, string](got)
	if !yes {
		err, _ := resultErr[string, string](got)
		t.Fatalf("GenerateSource failed: %s", err.F0)
	}
	return ok.F0
}
