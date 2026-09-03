package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compileSourceWithPrelude(t *testing.T, source string) error {
	t.Helper()
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDir(dir)
	return err
}

func TestValidateSwitchMissingVariant(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(c: Color) -> Int
  switch c
    case Red => 1
    case Green => 2
  end
end
`
	err := compileSourceWithPrelude(t, source)
	if err == nil || !strings.Contains(err.Error(), "non-exhaustive switch: missing variant(s) Blue") {
		t.Fatalf("CompileDir() error = %v, want missing Blue variant error", err)
	}
}

func TestValidateSwitchAllVariantsCovered(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(c: Color) -> Int
  switch c
    case Red => 1
    case Green => 2
    case Blue => 3
  end
end
`
	if err := compileSourceWithPrelude(t, source); err != nil {
		t.Fatalf("CompileDir() error = %v, want success", err)
	}
}

func TestValidateSwitchWildcardAcceptsIncomplete(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(c: Color) -> Int
  switch c
    case Red => 1
    case _ => 0
  end
end
`
	if err := compileSourceWithPrelude(t, source); err != nil {
		t.Fatalf("CompileDir() error = %v, want wildcard to satisfy exhaustiveness", err)
	}
}

func TestValidateSwitchNestedTupleMissingVariant(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func f(pair: (Color, Int)) -> Int
  switch pair
    case (Red, _) => 1
    case (Green, _) => 2
  end
end
`
	err := compileSourceWithPrelude(t, source)
	if err == nil || !strings.Contains(err.Error(), "non-exhaustive switch: missing variant(s) Blue") {
		t.Fatalf("CompileDir() error = %v, want missing Blue variant error in nested tuple", err)
	}
}

func TestValidateSwitchNestedTupleWithWildcardEscapes(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func f(pair: (Color, Int)) -> Int
  switch pair
    case (Red, _) => 1
    case _ => 0
  end
end
`
	if err := compileSourceWithPrelude(t, source); err != nil {
		t.Fatalf("CompileDir() error = %v, want wildcard to satisfy exhaustiveness in nested tuple", err)
	}
}

func TestValidateSwitchNonEnumNotChecked(t *testing.T) {
	source := `package sample

func describe(x: Int) -> Int
  switch x
    case 1 => 10
    case 2 => 20
  end
end
`
	if err := compileSourceWithPrelude(t, source); err != nil {
		t.Fatalf("CompileDir() error = %v, want non-enum switch to skip exhaustiveness", err)
	}
}

func TestValidateSwitchStatementFormMissingVariant(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(c: Color) -> Int
  var total: Int = 0
  switch c
    case Red => total = total + 1
    case Green => total = total + 2
  end
  total
end
`
	err := compileSourceWithPrelude(t, source)
	if err == nil || !strings.Contains(err.Error(), "non-exhaustive switch: missing variant(s) Blue") {
		t.Fatalf("CompileDir() error = %v, want missing Blue variant error in statement switch", err)
	}
}

func TestValidateSwitchStatementFormWithWildcard(t *testing.T) {
	source := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(c: Color) -> Int
  var total: Int = 0
  switch c
    case Red => total = total + 1
    case _ => total = total + 2
  end
  total
end
`
	if err := compileSourceWithPrelude(t, source); err != nil {
		t.Fatalf("CompileDir() error = %v, want wildcard to satisfy exhaustiveness in statement switch", err)
	}
}
