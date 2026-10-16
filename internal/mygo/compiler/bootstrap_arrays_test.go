package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapArrays(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"literal", "let values = [1, 2, 3] as Int32[3]", "[3]int32"},
		{"slice", "let values = [1, 2, 3]", "[]int"},
		{"sliceOfArrays", "type Rows = Slice[Int32[5]]", "[][5]int32"},
		{"matrixType", "type Matrix = Int32[5][3]", "[3][5]int32"},
		{"genericAlias", "type Pair[T] = T[2]\nlet values: Pair[Int32] = [1, 2]", "[2]T"},
		{"repeat", "let values = [0] as Int[1] * 5", "[5]int"},
		{"repeatParenthesized", "let values = ([0] as Int[1]) * 5", "[5]int"},
		{"genericRepeat", "func repeat[T](value: T[2]) -> T[6] value * 3 end", "[6]T"},
		{"nested", "let values: Int[2][2] = [[1, 2], [3, 4]]", "[2][2]int"},
		{"empty", "let values = [] as Int[0]", "[0]int"},
		{"emptyRepeat", "let values = ([] as Int[0]) * 5", "[0]int"},
		{"argument", "func identity(v: Int[2]) -> Int[2] v end\nlet values = identity([1, 2])", "[2]int"},
		{"return", "func values() -> Int[2]\nlet value: Int[2] = [1, 2]\nvalue\nend", "[2]int"},
		{"blockReturn", "func values() -> Int[2]\nlet value = 1\n[value, 2]\nend", "[2]int"},
		{"explicitReturn", "func values() -> Int[2]\nreturn [1, 2]\nend", "[2]int"},
		{"ifReturn", "func values(v: Bool) -> Int[2]\nif v then [1, 2] else [3, 4] end\nend", "[2]int"},
		{"ascribeSlice", "let values: Int[3] = [1, 2, 3]\nlet slice = values as Slice[Int]", "[]int"},
		{"ascribeSliceNested", "let values: Int[3][2] = [[1, 2, 3], [4, 5, 6]]\nlet slice = values as Slice[Int[3]]", "[][3]int"},
		{"ascribeSliceReturn", "func view() -> Slice[Int]\nlet values: Int[2] = [1, 2]\nvalues as Slice[Int]\nend", "[]int"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bootstrapTestModule(t, dir)
			if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte("package arrays\n"+tc.source+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			written, err := CompileDirBootstrap(dir)
			if err != nil {
				t.Fatal(err)
			}
			var generated string
			for _, path := range written {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				generated += string(data)
			}
			if !strings.Contains(generated, tc.want) {
				t.Fatalf("missing %s in:\n%s", tc.want, generated)
			}
			cmd := exec.Command("go", "test", "-buildvcs=false", ".")
			cmd.Dir = dir
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("generated Go failed: %v\n%s", err, output)
			}
		})
	}
}

func TestBootstrapArrayRuntime(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package arrays
import sha256 "go:crypto/sha256"
type Pair = Int[2]
struct Holder
  Values: Int32[2]
end
func identity[T](value: T[2]) -> T[2] value end
func sequence() -> Int[6]
  [1, 2] as Int[2] * 3
end
func zero() -> Int[0]
  [1] as Int[1] * 0
end
func makePair() -> Pair
  go[Pair] { code: "func() [2]int { calls++; return [2]int{1, 2} }()" }
end
func repeated() -> Int[6] makePair() * 3 end
func discarded() -> Int[0] makePair() * 0 end
func field() -> Holder Holder { Values: [1, 2] } end
func digest() -> Byte[32] sha256.Sum256([] as Slice[Byte]) end
func generic() -> Int[2] identity([1, 2] as Int[2]) end
func references(value: Ref[Int]) -> Ref[Int][3]
  [value] as Ref[Int][1] * 3
end
`
	checks := `package arrays
import "testing"
var calls int
func TestValues(t *testing.T) {
 if got := sequence(); got != [6]int{1,2,1,2,1,2} { t.Fatal(got) }
 if len(zero()) != 0 { t.Fatal("nonempty zero repeat") }
 if got := repeated(); got != [6]int{1,2,1,2,1,2} || calls != 1 { t.Fatal(got, calls) }
 discarded(); if calls != 2 { t.Fatal(calls) }
 if field().Values != [2]int32{1,2} || generic() != [2]int{1,2} { t.Fatal("field or generic") }
 if digest()[0] != 227 { t.Fatal("FFI digest") }
 value := 1; refs := references(&value); *refs[0] = 3
 if *refs[1] != 3 || *refs[2] != 3 { t.Fatal("reference sharing") }
 original := sequence(); copy := original; copy[0] = 9
 if original[0] != 1 { t.Fatal("array value copy") }
}
`
	for name, data := range map[string]string{"main.mygo": source, "arrays_test.go": checks} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-buildvcs=false", ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("array runtime failed: %v\n%s", err, output)
	}
}

func TestBootstrapArrayErrors(t *testing.T) {
	for _, source := range []string{
		"let values = [1, 2] as Int[3]",
		"let values = [1, 2] as Int[1]",
		"let values = [] as Int[5]",
		"let values = [true] as Int[1]",
		"let slice = [1]\nlet values = slice as Int[1]",
		"let values = ([1] as Int[1]) * -1",
		"let count = 3\nlet values = ([1] as Int[1]) * count",
		"let values: Int[-1] = []",
		"let values: Int[1.5] = [1]",
		"let values: Int[1 + 1] = [1, 2]",
		"let values: Int[9223372036854775808] = []",
		"let values: Int[N] = [1]",
		"let values = ([1, 2] as Int[2]) * 9223372036854775807",
		"let values = ([1] as Int[1]) * 1.5",
		"let values = ([1] as Int[1]) * (1 + 1)",
		"func take(v: Int[3]) -> Int[3] v end\nlet values = take([1, 2] as Int[2])",
		"let values: Int[2][2] = [[1], [2, 3]]",
		"struct Holder\nValues: Int[2]\nend\nlet values = Holder { Values: [1] }",
		"func values() -> Int[2]\n[1]\nend",
	} {
		dir := t.TempDir()
		bootstrapTestModule(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte("package arrays\n"+source+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := CompileDirBootstrap(dir); err == nil {
			t.Fatalf("accepted invalid array: %s", source)
		}
	}
}

func TestBootstrapArrayDiagnosticCounts(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte("package arrays\nlet bad = [1, 2] as Int[3]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDirBootstrap(dir)
	if err == nil || !strings.Contains(err.Error(), "main.mygo:2:") || !strings.Contains(err.Error(), "expected 3 elements, actual 2") {
		t.Fatalf("array diagnostic = %v", err)
	}
}

func TestBootstrapArrayToSliceRuntime(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package arrays
func makeValues() -> Int[4]
  [1, 1, 1, 1] as Int[4]
end
func view() -> Slice[Int]
  let values: Int[4] = [1, 2, 3, 4]
  values as Slice[Int]
end
func nestedView() -> Slice[Int[3]]
  let matrix: Int[3][2] = [[1, 2, 3], [4, 5, 6]]
  matrix as Slice[Int[3]]
end
func freshView() -> Slice[Int]
  makeValues() as Slice[Int]
end
func lengthsOk() -> Bool
  view().Len() == 4 && nestedView().Len() == 2 && freshView().Len() == 4
end
func aliasesOperand() -> Bool
  let values: Int[2] = [1, 2]
  let slice: Slice[Int] = values as Slice[Int]
  slice.Set(0, 9)
  (values as Slice[Int]).Get(0).UnwrapOr(0) == 9
end
`
	checks := `package arrays
import "testing"
func TestViews(t *testing.T) {
 if !lengthsOk() { t.Fatal("array-to-slice ascription length") }
 if !aliasesOperand() { t.Fatal("array-to-slice ascription should alias the operand") }
}
`
	for name, data := range map[string]string{"main.mygo": source, "arrays_test.go": checks} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, path := range written {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		generated += string(data)
	}
	if !strings.Contains(generated, "values[:]") || !strings.Contains(generated, "tmp[:]") {
		t.Fatalf("missing slice views in:\n%s", generated)
	}
	cmd := exec.Command("go", "test", "-buildvcs=false", ".")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("array-to-slice runtime failed: %v\n%s", err, output)
	}
}

func TestBootstrapArrayToSliceErrors(t *testing.T) {
	for _, source := range []string{
		"let values: Int[4] = [1, 2, 3, 4]\nlet slice: Slice[String] = values as Slice[String]",
		"let items: Slice[Int] = [1]\nlet values: Int[1] = items as Int[1]",
	} {
		dir := t.TempDir()
		bootstrapTestModule(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte("package arrays\n"+source+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := CompileDirBootstrap(dir)
		if err == nil || !strings.Contains(err.Error(), "main.mygo:") || !strings.Contains(err.Error(), "cannot unify") {
			t.Fatalf("ascription diagnostic = %v", err)
		}
	}
}

func TestBootstrapArrayCrossPackage(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	for _, sub := range []string{"data", "fixture"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"data/main.mygo": "package data\ntype Pair = Int[2]\nfunc Copy(v: Pair) -> Pair v end\nfunc Identity[T](v: T[2]) -> T[2] v end\n",
		"fixture/fixture.go": `package fixture
type Item struct { Value int }
type Holder struct { Values [2]int32; Items [2]Item }
func Copy(values [5]int32) [5]int32 { return values }
func Nested(values [2][2]int) [2][2]int { return values }
func NewHolder() Holder { return Holder{Values: [2]int32{1, 2}} }
`,
		"main.mygo": `package arrays
import data "example.com/bootstrap-test/data"
import fixture "go:example.com/bootstrap-test/fixture"
func copied() -> data.Pair data.Copy([1, 2]) end
func generic() -> Int[2] data.Identity([1, 2] as Int[2]) end
func ffi() -> Int32[5] fixture.Copy([1, 2, 3, 4, 5]) end
func nested() -> Int[2][2] fixture.Nested([[1, 2], [3, 4]]) end
func field() -> Int32[2] fixture.NewHolder().Values end
func items(holder: fixture.Holder) -> fixture.Item[2] holder.Items end
`,
		"arrays_test.go": `package arrays
import "testing"
func TestInterop(t *testing.T) {
 if copied() != [2]int{1,2} || generic() != [2]int{1,2} { t.Fatal("cross-package arrays") }
 if ffi() != [5]int32{1,2,3,4,5} || field() != [2]int32{1,2} { t.Fatal("Go arrays") }
 if nested() != [2][2]int{{1,2},{3,4}} { t.Fatal("nested Go arrays") }
}
`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CompileDirBootstrap(filepath.Join(dir, "data")); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-buildvcs=false", "./...")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cross-package arrays failed: %v\n%s", err, output)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte("package arrays\nimport data \"example.com/bootstrap-test/data\"\nlet bad = data.Copy([1, 2, 3])\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err == nil || !strings.Contains(err.Error(), "array length mismatch") {
		t.Fatalf("cross-package mismatch: %v", err)
	}
}
