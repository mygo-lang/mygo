package compiler

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// go test reports elapsed time per package; it is not observable behavior.
var elapsedPattern = regexp.MustCompile(`[0-9]+\.[0-9]+s`)

const differentialLibrary = `package lib

func Fib(n: Int) -> Int
  if n < 2 then
    n
  else
    Fib(n - 1) + Fib(n - 2)
  end
end

func Sum(n: Int) -> Int
  var acc = 0
  var i = 1
  while i <= n
    acc = acc + i
    i = i + 1
  end
  acc
end

func Classify(n: Int) -> String
  if n < 0 then
    "negative"
  elsif n == 0 then
    "zero"
  else
    "positive"
  end
end
`

// The checks are ordinary Go tests so they run unchanged against generated
// code from either backend; only the generation step differs.
const differentialChecks = `package lib

import "testing"

func TestFib(t *testing.T) {
	if got := Fib(10); got != 55 {
		t.Fatalf("Fib(10) = %d, want 55", got)
	}
}

func TestSum(t *testing.T) {
	if got := Sum(10); got != 55 {
		t.Fatalf("Sum(10) = %d, want 55", got)
	}
}

func TestClassify(t *testing.T) {
	cases := map[int]string{-3: "negative", 0: "zero", 7: "positive"}
	for input, want := range cases {
		if got := Classify(input); got != want {
			t.Fatalf("Classify(%d) = %q, want %q", input, got, want)
		}
	}
}
`

// TestCodegen3DifferentialBehavior compiles one package with codegen2 and with
// codegen3 and runs the same Go test suite against both outputs.
func TestCodegen3DifferentialBehavior(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module example.invalid/differential\n\ngo 1.24\n")
	writeFixture(t, dir, "lib.mygo", differentialLibrary)
	writeFixture(t, dir, "lib_check_test.go", differentialChecks)

	// Baseline: codegen2 writes zz_lib.gen.go beside the source.
	if _, err := SyncBootstrapWithOptions(dir, PipelineOptions{OptimizationLevel: 0, NoPrelude: true}); err != nil {
		t.Fatalf("codegen2 sync: %v", err)
	}
	codegen2Output := runGoTest(t, dir, nil, "codegen2")
	removeGenerated(t, dir)

	// Same package through codegen3: generated Go stays in build/mygo3 and the
	// overlay feeds it to the toolchain.
	emit := t.TempDir()
	if _, err := SyncBootstrapWithOptions(
		dir,
		PipelineOptions{Codegen3: true, OptimizationLevel: 2, NoPrelude: true, EmitDir: emit},
	); err != nil {
		t.Fatalf("codegen3 sync: %v", err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	writeEmitOverlay(t, dir, emit, overlay)
	codegen3Output := runGoTest(t, dir, []string{"-overlay", overlay}, "codegen3")

	normalized2 := elapsedPattern.ReplaceAllString(codegen2Output, "<elapsed>")
	normalized3 := elapsedPattern.ReplaceAllString(codegen3Output, "<elapsed>")
	if normalized2 != normalized3 {
		t.Fatalf("backends disagree:\ncodegen2:\n%s\ncodegen3:\n%s", codegen2Output, codegen3Output)
	}
	if !strings.Contains(codegen3Output, "ok") {
		t.Fatalf("codegen3 run did not pass:\n%s", codegen3Output)
	}
}

func writeEmitOverlay(t *testing.T, moduleRoot, emitDir, overlayPath string) {
	t.Helper()
	replace := map[string]string{}
	walkErr := filepath.WalkDir(emitDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "zz_") || (!strings.HasSuffix(name, ".gen.go") && !strings.HasSuffix(name, ".gen_test.go")) {
			return nil
		}
		relative, err := filepath.Rel(emitDir, path)
		if err != nil {
			return err
		}
		replace[filepath.Join(moduleRoot, relative)] = path
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk emitted files: %v", walkErr)
	}
	payload := struct {
		Replace map[string]string `json:"Replace"`
	}{Replace: replace}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overlayPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runGoTest(t *testing.T, dir string, extra []string, label string) string {
	t.Helper()
	argv := append([]string{"test"}, extra...)
	argv = append(argv, ".")
	cmd := exec.Command("go", argv...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s go test failed: %v\n%s", label, err, output)
	}
	return string(output)
}

func removeGenerated(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "zz_") && strings.HasSuffix(name, ".gen.go") {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatalf("remove %s: %v", name, err)
			}
		}
	}
}
