package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
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
