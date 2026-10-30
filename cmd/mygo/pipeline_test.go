package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the actual CLI in a subprocess so exit statuses and build dispatch are observable.
func TestPipelineIR3Scaffolding(t *testing.T) {
	dir := pipelinePackage(t, "package main\nfunc main() -> ()\n  ()\nend\n")

	out, stderr, code := pipelineCLI(t, dir, "--opt-level=0", "sync", ".")
	if code != 0 {
		t.Fatalf("--opt-level=0 exit %d: %s %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "zz_main.gen.go")); err != nil {
		t.Fatal(err)
	}

	out, stderr, code = pipelineCLI(t, dir, "--ir3-dump=ir3.txt", "sync", ".")
	if code != 0 {
		t.Fatalf("--ir3-dump exit %d: %s %s", code, out, stderr)
	}

	out, stderr, code = pipelineCLI(t, dir, "--codegen3", "sync", ".")
	if code != 0 {
		t.Fatalf("--codegen3 exit %d: %s %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
		t.Fatalf("--codegen3 created a project-local build directory: %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, ".mygo-cache", "build", "*", "*-d", "zz_*.gen.go")); len(matches) == 0 {
		t.Fatal("--codegen3 did not write a cache entry")
	}

	out, stderr, code = pipelineCLI(t, dir, "--opt-level=9", "sync", ".")
	if code != 2 || !strings.Contains(stderr, "optimization level") {
		t.Fatalf("--opt-level=9 exit %d, stderr %q; want a validation error", code, stderr)
	}
}

func TestPipelineCLIProcess(t *testing.T) {
	if os.Getenv("MYGO_PIPELINE_CLI_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"mygo"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(99)
}

func pipelineCLI(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestPipelineCLIProcess$", "--"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"MYGO_PIPELINE_CLI_PROCESS=1",
		"MYGO_CACHE="+filepath.Join(dir, ".mygo-cache"),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return stdout.String(), stderr.String(), exit.ExitCode()
	}
	t.Fatal(err)
	return "", "", -1
}

func pipelinePackage(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":    "module example.com/cli-test\n\ngo 1.26\n\nrequire github.com/mygo-lang/mygo v0.0.0\nreplace github.com/mygo-lang/mygo => " + filepath.ToSlash(root) + "\n",
		"main.mygo": source,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPipelineSelection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  []string
		legacy bool
		timing bool
	}{
		{name: "default"},
		{name: "explicit", flags: []string{"--bootstrap"}},
		{name: "legacy", flags: []string{"--legacy"}, legacy: true},
		{name: "no-prelude", flags: []string{"--no-prelude"}},
		{name: "legacy-no-prelude", flags: []string{"--legacy", "--no-prelude"}, legacy: true},
		{name: "timing", flags: []string{"--bootstrap-timing"}, timing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := pipelinePackage(t, "package main\nfunc main() -> ()\n  ()\nend\n")
			out, stderr, code := pipelineCLI(t, dir, append(tc.flags, "sync", ".")...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			if _, err := os.Stat(filepath.Join(dir, "zz_main.gen.go")); err != nil {
				t.Fatal(err)
			}
			// Bootstrap emits dynamic API metadata; legacy does not.
			_, err := os.Stat(filepath.Join(dir, "zz_mygo_metadata.gen.go"))
			if (!tc.legacy && err != nil) || (tc.legacy && !os.IsNotExist(err)) {
				t.Fatalf("unexpected pipeline metadata: %v", err)
			}
			seen := map[string]bool{}
			for _, path := range strings.Fields(out) {
				if seen[path] {
					t.Fatalf("duplicate reported path %q", path)
				}
				seen[path] = true
			}
			if !seen["zz_main.gen.go"] {
				t.Fatalf("missing generated path: %q", out)
			}
			if tc.timing && !strings.Contains(stderr, "parser2") {
				t.Fatalf("missing stage timing: %q", stderr)
			}
		})
	}
}

func TestPipelineCLITestCommand(t *testing.T) {
	dir := pipelinePackage(t, "package main\nfunc main() -> ()\n  ()\nend\n")
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main_test.mygo", "package main\n\nimport testing \"go:testing\"\n\nfunc TestPipelineSmoke(t: Ref[testing.T]) -> ()\n  if 1 + 1 != 2 then\n    t.Fatal(\"bad math\")\n  end\nend\n")
	out, stderr, code := pipelineCLI(t, dir, "--codegen3", "--opt-level=0", "test", "-run=TestPipelineSmoke", ".")
	if code != 0 {
		t.Fatalf("test exit %d: %s %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "zz_main.gen_test.go")); !os.IsNotExist(err) {
		t.Fatalf("generated test file was written beside the source: %v", err)
	}

	write("main_test.mygo", "package main\n\nimport testing \"go:testing\"\n\nfunc TestPipelineSmoke(t: Ref[testing.T]) -> ()\n  if 1 + 1 != 3 then\n    t.Fatal(\"bad math\")\n  end\nend\n")
	out, stderr, code = pipelineCLI(t, dir, "--codegen3", "--opt-level=0", "test", "-run=TestPipelineSmoke", ".")
	if code != 1 {
		t.Fatalf("failing test exit %d, want 1: %s", code, stderr)
	}
	if !strings.Contains(out, "bad math") && !strings.Contains(stderr, "bad math") {
		t.Fatalf("failing test output did not reach the CLI: %s %s", out, stderr)
	}
}

func TestPipelineInvalidFlags(t *testing.T) {
	for _, flags := range [][]string{
		{"--legacy", "--bootstrap", "sync"},
		{"--bootstrap", "--legacy", "sync"},
		{"--legacy", "--bootstrap-timing", "sync"},
		{"--bootstrap-timing", "--legacy", "sync"},
		{"--legacy", "build"},
		{"--legacy", "fmt", "main.mygo"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			const source = "package main\nfunc main() -> ()\n    ()\nend\n"
			dir := pipelinePackage(t, source)
			_, stderr, code := pipelineCLI(t, dir, flags...)
			if code != 2 || !strings.Contains(stderr, "--legacy") {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 2 {
				t.Fatalf("command changed fixture: %v, %v", entries, err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.mygo"))
			if err != nil || string(got) != source {
				t.Fatalf("source changed: %q, %v", got, err)
			}
		})
	}
}

func TestPipelineBuild(t *testing.T) {
	t.Setenv("GOFLAGS", os.Getenv("GOFLAGS")+" -buildvcs=false")
	for _, flags := range [][]string{nil, {"--bootstrap"}} {
		dir := pipelinePackage(t, "package main\nfunc main() -> ()\n  ()\nend\n")
		_, stderr, code := pipelineCLI(t, dir, append(flags, "build", ".")...)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if _, err := os.Stat(filepath.Join(dir, "cli-test")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "zz_mygo_metadata.gen.go")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPipelineFailureStopsBuild(t *testing.T) {
	dir := pipelinePackage(t, "package main\nfunc main() -> ()\n  missingName\nend\n")
	bin := t.TempDir()
	marker := filepath.Join(bin, "build-invoked")
	// Signature loading can invoke Go; only mark an actual build invocation.
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" = build ]; then touch \"$MYGO_BUILD_MARKER\"; exit 99; fi\nexec \"$MYGO_REAL_GO\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MYGO_REAL_GO", goPath)
	t.Setenv("MYGO_BUILD_MARKER", marker)
	for _, command := range []string{"sync", "build"} {
		_, stderr, code := pipelineCLI(t, dir, command, ".")
		if code != 1 || !strings.Contains(stderr, "missingName") {
			t.Fatalf("exit %d: %s", code, stderr)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("Go build was invoked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "zz_main.gen.go")); !os.IsNotExist(err) {
		t.Fatalf("failed compilation generated output: %v", err)
	}
}

func TestPipelineUsage(t *testing.T) {
	_, stderr, code := pipelineCLI(t, t.TempDir())
	if code != 2 {
		t.Fatalf("exit %d", code)
	}
	for _, text := range []string{"bootstrap pipeline by default", "sync only", "compatibility alias"} {
		if !strings.Contains(stderr, text) {
			t.Fatalf("usage missing %q: %s", text, stderr)
		}
	}
}
