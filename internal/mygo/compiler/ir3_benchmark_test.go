package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ir3BenchModule creates a temporary Go module that points back at this
// repository so the bootstrap pipeline can resolve the prelude.
func ir3BenchModule(tb testing.TB, dir string) {
	tb.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		tb.Fatal(err)
	}
	goMod := "module example.com/ir3bench\n\ngo 1.26\n\nrequire github.com/mygo-lang/mygo v0.0.0\n\nreplace github.com/mygo-lang/mygo => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		tb.Fatal(err)
	}
}

func prepareIR3BenchPackage(tb testing.TB, source string) string {
	tb.Helper()
	dir := tb.TempDir()
	ir3BenchModule(tb, dir)
	if err := os.WriteFile(filepath.Join(dir, "bench.mygo"), []byte(source), 0o644); err != nil {
		tb.Fatal(err)
	}
	return dir
}

// prepareIR3IsolatedBenchPackage copies the prelude sources into a temporary
// module so a cold codegen3 benchmark never generates files in this checkout.
func prepareIR3IsolatedBenchPackage(tb testing.TB, source string) string {
	tb.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		tb.Fatal(err)
	}
	dir := tb.TempDir()
	if err := os.WriteFile(
		filepath.Join(dir, "go.mod"),
		[]byte("module github.com/mygo-lang/mygo\n\ngo 1.26\n"),
		0o644,
	); err != nil {
		tb.Fatal(err)
	}
	preludeDst := filepath.Join(dir, "prelude")
	if err := os.MkdirAll(preludeDst, 0o755); err != nil {
		tb.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "prelude"))
	if err != nil {
		tb.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") || strings.HasSuffix(name, "_test.mygo") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "prelude", name))
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(preludeDst, name), data, 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bench.mygo"), []byte(source), 0o644); err != nil {
		tb.Fatal(err)
	}
	return dir
}

const ir3BenchOptionListSource = `package ir3bench

func sum(values: List[Int]) -> Int
  let items = ListToSlice(values)
  var total: Int = 0
  var index: Int = 0
  while index < items.Len()
    switch items.Get(index)
      case Some(value) => total = total + value
      case None => ()
    end
    index = index + 1
  end
  total
end

func lookup(values: Slice[Int], target: Int) -> Option[Int]
  var index: Int = 0
  while index < values.Len()
    switch values.Get(index)
      case Some(current) =>
        if current == target then
          return Some(current)
        end
      case None => ()
    end
    index = index + 1
  end
  None
end
`

const ir3BenchDictionarySource = `package ir3bench

func contains[A](values: Slice[A], target: A) -> Bool using Eq[A]
  var index: Int = 0
  while index < values.Len()
    switch values.Get(index)
      case Some(current) =>
        if current.Equals(target) then
          return true
        end
      case None => ()
    end
    index = index + 1
  end
  false
end

func hasTwo(values: Slice[Int]) -> Bool
  contains(values, 2)
end
`

const ir3BenchTailSource = `package ir3bench

func even(n: Int) -> Bool
  if n == 0 then
    return true
  end
  odd(n - 1)
end

func odd(n: Int) -> Bool
  if n == 0 then
    return false
  end
  even(n - 1)
end

func total(n: Int, acc: Int) -> Int
  if n == 0 then
    return acc
  end
  total(n - 1, acc + 1)
end
`

func BenchmarkBootstrapIR3Baseline(b *testing.B) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{name: "option-list", source: ir3BenchOptionListSource},
		{name: "dictionary", source: ir3BenchDictionarySource},
		{name: "tail-recursion", source: ir3BenchTailSource},
	} {
		b.Run(tc.name, func(b *testing.B) {
			dir := prepareIR3BenchPackage(b, tc.source)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := SyncBootstrap(dir); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBootstrapIR3Codegen3(b *testing.B) {
	for _, level := range []int{0, 1, 2} {
		b.Run(fmt.Sprintf("o%d", level), func(b *testing.B) {
			b.Setenv("MYGO_CACHE", "off")
			for _, tc := range []struct {
				name   string
				source string
			}{
				{name: "option-list", source: ir3BenchOptionListSource},
				{name: "dictionary", source: ir3BenchDictionarySource},
				{name: "tail-recursion", source: ir3BenchTailSource},
			} {
				b.Run(tc.name, func(b *testing.B) {
					dir := prepareIR3IsolatedBenchPackage(b, tc.source)
					options := PipelineOptions{
						Codegen3:          true,
						OptimizationLevel: level,
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err := SyncBootstrapWithOptions(dir, options); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

func TestBootstrapIR3BaselineStackDepth(t *testing.T) {
	if testing.Short() {
		t.Skip("stack-depth baseline runs the Go compiler and a generated test")
	}
	dir := prepareIR3IsolatedBenchPackage(t, ir3BenchTailSource)
	if _, err := SyncBootstrap(dir); err != nil {
		t.Fatalf("SyncBootstrap() error = %v", err)
	}
	check := `package ir3bench

import "testing"

func TestTailDepth(t *testing.T) {
	if !even(200000) || odd(200000) {
		t.Fatal("mutual tail recursion returned the wrong result")
	}
	if total(200000, 0) != 200000 {
		t.Fatal("self tail recursion returned the wrong result")
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "stack_test.go"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-run", "TestTailDepth", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated stack-depth test failed: %v\n%s", err, output)
	}
}

func TestBootstrapIR3Codegen3StackDepth(t *testing.T) {
	if testing.Short() {
		t.Skip("codegen3 stack-depth verification runs the Go compiler and a generated test")
	}
	dir := prepareIR3BenchPackage(t, ir3BenchTailSource)
	check := `package ir3bench

import "testing"

func TestTailDepth(t *testing.T) {
	if !even(200000) || odd(200000) {
		t.Fatal("mutual tail recursion returned the wrong result")
	}
	if total(200000, 0) != 200000 {
		t.Fatal("self tail recursion returned the wrong result")
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "stack_test.go"), []byte(check), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYGO_CACHE", t.TempDir())
	if err := BuildCodegen3(
		dir,
		PipelineOptions{Codegen3: true, OptimizationLevel: 2},
		true,
		[]string{"-run", "TestTailDepth", "."},
	); err != nil {
		t.Fatalf("codegen3 stack-depth test failed: %v", err)
	}
}
