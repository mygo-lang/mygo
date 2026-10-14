package codegen2

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

func numericGoInfo(t *testing.T, code string) (*types.Info, *types.Package, error) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "numeric.go", code, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	cfg := types.Config{Importer: importer.Default()}
	pkg, err := cfg.Check("numeric", fset, []*ast.File{file}, info)
	return info, pkg, err
}

func TestNumericSuffixBindings(t *testing.T) {
	cases := []struct{ literal, typ, value string }{
		{"0i64", "int64", "0"},
		{"127i8", "int8", "127"}, {"-128i8", "int8", "-128"},
		{"32767i16", "int16", "32767"}, {"-32768i16", "int16", "-32768"},
		{"2147483647i32", "int32", "2147483647"}, {"-2147483648i32", "int32", "-2147483648"},
		{"9223372036854775807i64", "int64", "9223372036854775807"},
		{"-9223372036854775808i64", "int64", "-9223372036854775808"},
		{"255u8", "uint8", "255"}, {"65535u16", "uint16", "65535"},
		{"4294967295u32", "uint32", "4294967295"},
		{"18446744073709551615u64", "uint64", "18446744073709551615"},
		{"42u", "uint", "42"}, {"1.5f32", "float32", "3/2"}, {"1.5f64", "float64", "3/2"},
		{"0xffu16", "uint16", "255"}, {"0o77i32", "int32", "63"},
		{"0b1010i64", "int64", "10"}, {"18_446_744_073_709_551_615u64", "uint64", "18446744073709551615"},
		{"1i64 + 2i64", "int64", "3"}, {"-1.5f32", "float32", "-3/2"},
		{"0", "int", "0"}, {"3.14", "float64", constant.MakeFloat64(3.14).ExactString()},
	}
	for _, tt := range cases {
		for _, binding := range []string{"let", "var"} {
			t.Run(binding+"/"+tt.literal, func(t *testing.T) {
				src := fmt.Sprintf("package numeric\nfunc Probe() -> Any\n  %s count = %s\n  count\nend\n", binding, tt.literal)
				if binding == "var" {
					src = fmt.Sprintf("package numeric\nfunc Probe() -> Any\n  var count = %s\n  count = count\n  count\nend\n", tt.literal)
				}
				code := generateWithGoPackages(t, src, nil)
				info, _, err := numericGoInfo(t, code)
				if err != nil {
					t.Fatalf("type check: %v\n%s", err, code)
				}
				found := false
				for id, obj := range info.Defs {
					if id.Name == "count" && obj != nil {
						found = true
						if got := obj.Type().String(); got != tt.typ {
							t.Fatalf("count type = %s, want %s\n%s", got, tt.typ, code)
						}
					}
				}
				if !found {
					t.Fatal("missing count binding")
				}
				// Check the complete initializer, including unary negation and arithmetic.
				matched := false
				for _, tv := range info.Types {
					if tv.Value != nil && tv.Value.ExactString() == tt.value {
						matched = true
					}
				}
				if !matched {
					t.Fatalf("missing constant value %s", tt.value)
				}
			})
		}
	}
}

func TestNumericSuffixOverflowRejected(t *testing.T) {
	for _, literal := range []string{"128i8", "-129i8", "256u8", "18446744073709551616u64"} {
		t.Run(literal, func(t *testing.T) {
			code := generateWithGoPackages(t, "package numeric\nfunc Probe() -> Any\nlet count = "+literal+"\ncount\nend", nil)
			if _, _, err := numericGoInfo(t, code); err == nil {
				t.Fatalf("overflow accepted: %s\n%s", literal, code)
			}
		})
	}
}

func TestNumericSuffixRuntimeContexts(t *testing.T) {
	src := `package numeric
import fmt "go:fmt"
let Global = 0i64
func Identity[A](value: A) -> A
  value
end
func Direct() -> Int64
  42i64
end
func Generic() -> Int64
  let count = Identity(42i64)
  count
end
func Annotated() -> Int64
  let count: Int64 = 42i64
  count
end
func Contextual() -> Int64
  let count: Int64 = 42
  count
end
func Items() -> Slice[Int64]
  [1i64, 2i64]
end
func Observe() -> String
  fmt.Sprintf("%T", 0i64)
end
func Rounded() -> Float32
  3.14f32
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{{
		Alias: "fmt", Path: "go:fmt",
		Funcs: []typeinference2.GoFuncSignature{{Name: "Sprintf", Params: []string{"string", "[]any"}, Results: []string{"string"}, Variadic: true}},
	}})
	dir := t.TempDir()
	for name, data := range map[string]string{
		"go.mod":     "module numeric\n\ngo 1.26\n",
		"numeric.go": code,
		"numeric_test.go": `package numeric
import "testing"
func TestValues(t *testing.T) {
 if Global != 0 || Direct() != 42 || Generic() != 42 || Annotated() != 42 || Contextual() != 42 { t.Fatal("wrong integer value") }
 if Observe() != "int64" { t.Fatal("dynamic type:", Observe()) }
 if got := Items(); len(got) != 2 || got[0] != 1 || got[1] != 2 { t.Fatal(got) }
 if Rounded() != float32(3.14) { t.Fatal(Rounded()) }
}
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated program: %v\n%s\n%s", err, out, code)
	}
}
