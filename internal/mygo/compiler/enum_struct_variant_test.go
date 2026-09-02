package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsDuplicateEnumVariantFields(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Point
  Origin { x: Float64, x: Float64 }
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDir(dir)
	if err == nil || !strings.Contains(err.Error(), `duplicate field "x" in enum Point variant Origin`) {
		t.Fatalf("CompileDir() error = %v, want duplicate field", err)
	}
}

func TestValidateRejectsUnknownStructPatternField(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Shape
  Circle { radius: Float64 }
end

func area(shape: Shape) -> Float64
  switch shape
    case Circle { radus: r } => r
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDir(dir)
	if err == nil || !strings.Contains(err.Error(), `enum Shape variant Circle has no field "radus"`) {
		t.Fatalf("CompileDir() error = %v, want unknown field", err)
	}
}

func TestValidateRejectsDuplicateStructPatternFields(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Shape
  Circle { radius: Float64, width: Float64 }
end

func radius(shape: Shape) -> Float64
  switch shape
    case Circle { radius, width: radius } => radius
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDir(dir)
	if err == nil || (!strings.Contains(err.Error(), "field \"radius\" specified more than once") && !strings.Contains(err.Error(), "pattern binds \"radius\" more than once")) {
		t.Fatalf("CompileDir() error = %v, want duplicate binding", err)
	}
}

func TestCompileDirNoPreludeNamedStructEnumVariants(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Shape
  Circle { radius: Float64 }
  Rectangle(Float64, Float64)
end

func area(shape: Shape) -> Float64
  switch shape
    case Circle { radius } => 3.14 * radius * radius
    case Rectangle(width, height) => width * height
  end
end

func CombinedArea() -> Float64
  let circle = Shape.Circle { radius: 2.0 }
  let rectangle = Shape.Rectangle(3.0, 4.0)
  area(circle) + area(rectangle)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirNoPrelude(dir); err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestCombinedArea(t *testing.T) {
	const want = 24.56
	if got := CombinedArea(); got < want-0.0001 || got > want+0.0001 {
		t.Fatalf("CombinedArea() = %v, want %v", got, want)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated named struct variant package failed: %v\n%s", err, output)
	}
}

func TestCompileDirNoPreludeLowersTupleStructVariantPatterns(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum AgentEvent
  Timeout { Where: Int }
  Started
end

func timeoutWhere(state: Int, event: AgentEvent) -> Int
  switch (state, event)
    case (current, Timeout { Where }) => current + Where
    case (_, _) => -1
  end
end

func matched() -> Int
  timeoutWhere(3, AgentEvent.Timeout { Where: 4 })
end

func unmatched() -> Int
  timeoutWhere(3, AgentEvent.Started)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirNoPrelude(dir); err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestTupleStructVariant(t *testing.T) {
	if got := matched(); got != 7 {
		t.Fatalf("matched() = %d, want 7", got)
	}
	if got := unmatched(); got != -1 {
		t.Fatalf("unmatched() = %d, want -1", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated tuple struct variant package failed: %v\n%s", err, output)
	}
}

func TestCompileDirNoPreludeResolvesQualifiedNamedVariantWithCollidingName(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

struct ContextWindow
  Loop: Int
end

enum AgentState
  Running { Loop: ContextWindow }
end

enum ToolExecutionStatus
  Running
end

enum OtherState
  Running { Label: String }
end

func MakeAgentState() -> AgentState
  AgentState.Running { Loop: ContextWindow { Loop: 1 } }
end

func MakeOtherState() -> OtherState
  OtherState.Running { Label: "other" }
end

func LoopOf(state: AgentState) -> Int
  switch state
    case Running { Loop } => Loop.Loop
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirNoPrelude(dir); err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestQualifiedNamedVariant(t *testing.T) {
	if MakeAgentState() == nil || MakeOtherState() == nil {
		t.Fatal("qualified enum construction returned nil")
	}
	if got := LoopOf(MakeAgentState()); got != 1 {
		t.Fatalf("LoopOf(MakeAgentState()) = %d, want 1", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated collision package failed: %v\n%s", err, output)
	}
}
