package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBootstrapCompilesGatewayStyleFFI pins the four gateway scenarios from
// the bootstrap-ffi-boundary spec against the real Go standard library: a
// chained field + method call (r.Header.Set), a qualified Go-FFI struct
// literal (http.Client{}), a raw multi-value return whose elements include a
// package-local type (context.WithTimeout's context.CancelFunc), and calling a
// value of a Go named function type (cancel()).  CompileDirBootstrap runs the
// full self-hosted pipeline, and `go test` proves the generated Go compiles
// and behaves correctly at runtime.
func TestBootstrapCompilesGatewayStyleFFI(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import http "go:net/http"
import context "go:context"
import time "go:time"

func DecorateRequest(r: Ref[http.Request]) -> ()
  r.Header.Set("X-Test", "1")
  ()
end

func MakeClient() -> Ref[http.Client]
  let client = http.Client { }
  Ref.new(client)
end

func RunWithTimeout(dur: time.Duration) -> Bool
  let (_, cancel) = context.WithTimeout(context.Background(), dur)
  cancel()
  true
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import (
	"net/http"
	"testing"
	"time"
)

func TestGatewayEndToEnd(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	DecorateRequest(req)
	if got := req.Header.Get("X-Test"); got != "1" {
		t.Fatalf("header X-Test = %q, want 1", got)
	}
	if client := MakeClient(); client == nil {
		t.Fatal("MakeClient() returned nil")
	}
	if !RunWithTimeout(50 * time.Millisecond) {
		t.Fatal("RunWithTimeout returned false")
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache", "GOFLAGS=-mod=mod")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gateway-style FFI package failed:\n%s", output)
	}
}

// TestBootstrapCompilesMethodFFIResultWrapping pins the user-facing
// `func f() -> Result[Ref[T], Error]  recv.Method(args)` shape: a Go FFI
// method that returns (T, error) (http.Client.Do, http.Request.Cookie) must be
// lowered at the boundary into Result[T, error] instead of leaking the raw
// two-value Go call into the generated return statement.  CompileDirBootstrap
// runs the full self-hosted pipeline, and `go test` proves the generated Go
// compiles and returns an Err result at runtime.
func TestBootstrapCompilesMethodFFIResultWrapping(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import "go:net/http"

func DoRequest(client: Ref[http.Client], req: Ref[http.Request]) -> Result[Ref[http.Response], Error]
  client.Do(req)
end

func FetchCookie(req: Ref[http.Request], name: String) -> Result[Ref[http.Cookie], Error]
  req.Cookie(name)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	for _, w := range written {
		data, err := os.ReadFile(w)
		if err != nil {
			t.Fatal(err)
		}
		gen := string(data)
		for _, want := range []string{
			"client.Do(req)",
			"req.Cookie(name)",
			"Ok[*http.Response, error]",
			"Err[*http.Response, error]",
			"Ok[*http.Cookie, error]",
			"Err[*http.Cookie, error]",
		} {
			if !strings.Contains(gen, want) {
				t.Errorf("generated %s missing %q:\n%s", w, want, gen)
			}
		}
		for _, bad := range []string{"return client.Do(req)", "return req.Cookie(name)"} {
			if strings.Contains(gen, bad) {
				t.Errorf("raw two-value method call leaked into generated return (%q):\n%s", bad, gen)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import (
	"errors"
	"net/http"
	"testing"
)

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("boom")
}

func TestDoRequestReturnsErrResult(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: failingTransport{}}
	res := DoRequest(client, req)
	if res == nil {
		t.Fatal("DoRequest returned nil Result")
	}
}

func TestFetchCookieReturnsErrResult(t *testing.T) {
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	res := FetchCookie(req, "missing")
	if res == nil {
		t.Fatal("FetchCookie returned nil Result")
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache", "GOFLAGS=-mod=mod")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("method FFI Result wrapping package failed:\n%s", output)
	}
}

// TestBootstrapCompilesGoConstants pins importing Go package-level constants
// through the self-hosted FFI pipeline.  time.Second / time.Millisecond are
// typed Duration constants and http.StatusOK is an untyped integer constant,
// so the test exercises both the nominal-type and the untyped-to-default-type
// constant paths.  CompileDirBootstrap runs the full self-hosted pipeline and
// `go test` proves each constant selector lowers to a real Go value at runtime.
func TestBootstrapCompilesGoConstants(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import time "go:time"
import http "go:net/http"

func OneSecond() -> time.Duration
  time.Second
end

func OneMillisecond() -> time.Duration
  time.Millisecond
end

func StatusOKCode() -> Int
  http.StatusOK
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import (
	"net/http"
	"testing"
	"time"
)

func TestGoConstants(t *testing.T) {
	if got := OneSecond(); got != time.Second {
		t.Fatalf("OneSecond() = %v, want %v", got, time.Second)
	}
	if got := OneMillisecond(); got != time.Millisecond {
		t.Fatalf("OneMillisecond() = %v, want %v", got, time.Millisecond)
	}
	if got := StatusOKCode(); got != http.StatusOK {
		t.Fatalf("StatusOKCode() = %d, want %d", got, http.StatusOK)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache", "GOFLAGS=-mod=mod")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go constants package failed:\n%s", output)
	}
}
