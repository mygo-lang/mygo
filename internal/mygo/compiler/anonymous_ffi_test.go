package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapCompilesAnonymousInterfaceFFI(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	rendererDir := filepath.Join(dir, "fixture", "renderer")
	htmlDir := filepath.Join(dir, "fixture", "html")
	if err := os.MkdirAll(rendererDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(htmlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rendererFixture := `package renderer

type Config struct{}

type Option interface {
	SetConfig(*Config)
}
`
	if err := os.WriteFile(filepath.Join(rendererDir, "renderer.go"), []byte(rendererFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	htmlFixture := `package html

import "example.com/bootstrap-test/fixture/renderer"

type HTMLOption interface {
	SetHTMLOption(*renderer.Config)
}

type hardWraps struct{}

func (hardWraps) SetConfig(*renderer.Config)     {}
func (hardWraps) SetHTMLOption(*renderer.Config) {}

func WithHardWraps() interface {
	renderer.Option
	HTMLOption
} {
	return hardWraps{}
}

func WithUnsafe() interface {
	renderer.Option
	HTMLOption
} {
	return hardWraps{}
}

type Option func(*Markdown)

type Markdown struct{}

func (Markdown) Value() string { return "rendered" }

func WithRendererOptions(opts ...renderer.Option) Option {
	return func(*Markdown) {}
}

func New(options ...Option) Markdown {
	markdown := Markdown{}
	for _, option := range options {
		option(&markdown)
	}
	return markdown
}
`
	if err := os.WriteFile(filepath.Join(htmlDir, "html.go"), []byte(htmlFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `package sample

import fixture "go:example.com/bootstrap-test/fixture/html"

func Build() -> fixture.Markdown
  fixture.New(fixture.WithRendererOptions(fixture.WithHardWraps(), fixture.WithUnsafe()))
end

func BuildOne() -> fixture.Markdown
  fixture.New(fixture.WithRendererOptions(fixture.WithHardWraps()))
end

func BuildBound() -> fixture.Markdown
  let hard = fixture.WithHardWraps()
  fixture.New(fixture.WithRendererOptions(hard, fixture.WithUnsafe()))
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	var generated strings.Builder
	for _, path := range written {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		generated.Write(contents)
	}
	code := generated.String()
	for _, want := range []string{
		"fixture.WithRendererOptions(fixture.WithHardWraps(), fixture.WithUnsafe())",
		"fixture.WithRendererOptions(fixture.WithHardWraps())",
		"fixture.WithRendererOptions(hard, fixture.WithUnsafe())",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated Go missing direct anonymous-interface call %q:\n%s", want, code)
		}
	}
	for _, bad := range []string{"go:anonymous", "interface{}"} {
		if strings.Contains(code, bad) {
			t.Errorf("generated Go leaked anonymous-type machinery %q:\n%s", bad, code)
		}
	}
	testSource := `package sample

import "testing"

func TestAnonymousInterfaceBuild(t *testing.T) {
	if got := Build().Value(); got != "rendered" {
		t.Fatalf("Build().Value() = %q, want rendered", got)
	}
	if got := BuildOne().Value(); got != "rendered" {
		t.Fatalf("BuildOne().Value() = %q, want rendered", got)
	}
	if got := BuildBound().Value(); got != "rendered" {
		t.Fatalf("BuildBound().Value() = %q, want rendered", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache", "GOFLAGS=-mod=mod", "TMPDIR=/tmp", "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("anonymous-interface FFI package failed:\n%s\nGenerated:\n%s", output, code)
	}
}
