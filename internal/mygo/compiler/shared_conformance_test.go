package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type sharedConformanceFixture struct {
	name      string
	source    string
	goTest    string
	wantError string
}

var sharedConformanceFixtures = []sharedConformanceFixture{
	{
		name: "verbatim_triple_quoted_string",
		source: `package sample

func Value() -> String
  """A\nB"""
end
`,
		goTest: `package sample

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != "A\\nB" {
		t.Fatalf("Value() = %q, want A\\nB", got)
	}
}
`,
	},
	{
		name: "multiline_raw_string",
		source: "package sample\n\nfunc Value() -> String\n  `A\nB\\C`\nend\n",
		goTest: `package sample

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != "A\nB\\C" {
		t.Fatalf("Value() = %q, want A\\nB\\C", got)
	}
}
`,
	},
	{
		name: "nested_tuple_binding",
		source: `package sample

func pair() -> (Int, (Int, Int))
  (1, (2, 3))
end

func Value() -> Int
  let (a, (_, c)) = pair()
  a + c
end
`,
		goTest: `package sample

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 4 {
		t.Fatalf("Value() = %d, want 4", got)
	}
}
`,
	},
	{
		name: "statement_context_switch_patterns",
		source: `package sample

enum Marker
  Marked(Int)
  Empty
end

func Value() -> Int
  var total: Int = 0
  switch 1
    case 1 => total = total + 1
    case _ => total = 99
  end
  switch 2
    case item => total = total + item
  end
  switch (3, 4)
    case (left, right) => total = total + left + right
  end
  switch Marker.Marked(5)
    case Marked(value) => total = total + value
    case _ => total = 99
  end
  total
end
`,
		goTest: `package sample

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 15 {
		t.Fatalf("Value() = %d, want 15", got)
	}
}
`,
	},
	{
		name: "loop_control",
		source: `package sample

func Value() -> Int
  var outer: Int = 0
  var total: Int = 0
  while outer < 3
    outer = outer + 1
    var inner: Int = 0
    while inner < 3
      inner = inner + 1
      if inner == 2 then continue end
      if outer == 2 then break end
      total = total + 1
    end
  end
  total
end
`,
		goTest: `package sample

import "testing"

func TestValue(t *testing.T) {
	if got := Value(); got != 4 {
		t.Fatalf("Value() = %d, want 4", got)
	}
}
`,
	},
	{
		name: "break_outside_loop",
		source: `package sample

func Bad() -> ()
  break
end
`,
		wantError: "break is only valid inside a while loop",
	},
	{
		name: "continue_outside_loop",
		source: `package sample

func Bad() -> ()
  continue
end
`,
		wantError: "continue is only valid inside a while loop",
	},
}

func TestSharedConformanceFixtures(t *testing.T) {
	for _, fixture := range sharedConformanceFixtures {
		t.Run(fixture.name+"/production", func(t *testing.T) {
			runSharedConformanceFixture(t, fixture, false)
		})
		t.Run(fixture.name+"/bootstrap", func(t *testing.T) {
			runSharedConformanceFixture(t, fixture, true)
		})
	}
}

func runSharedConformanceFixture(t *testing.T, fixture sharedConformanceFixture, bootstrap bool) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/conformance\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(fixture.source), 0o644); err != nil {
		t.Fatal(err)
	}
	var err error
	if bootstrap {
		_, err = SyncBootstrapNoPrelude(dir)
	} else {
		_, err = CompileDirNoPrelude(dir)
	}
	if fixture.wantError != "" {
		if err == nil {
			t.Fatalf("compilation succeeded, want error %q", fixture.wantError)
		}
		if !strings.Contains(err.Error(), fixture.wantError) {
			t.Fatalf("compilation error = %v, want %q", err, fixture.wantError)
		}
		return
	}
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gen.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), entry.Name(), data, parser.AllErrors); err != nil {
			t.Fatalf("generated Go is invalid: %v\n%s", err, data)
		}
	}
	if fixture.goTest == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(fixture.goTest), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".", "-timeout=5s")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runtime test failed: %v\n%s", err, output)
	}
}
