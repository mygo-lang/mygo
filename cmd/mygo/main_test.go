package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureFmtStdin(t *testing.T, input string) (string, error) {
	t.Helper()
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut; inR.Close(); outW.Close() }()
	if _, err := inW.WriteString(input); err != nil {
		t.Fatal(err)
	}
	inW.Close()
	err = runFmt(nil)
	outW.Close()
	data, readErr := io.ReadAll(outR)
	outR.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(data), err
}

func TestRunFmtStdin(t *testing.T) {
	got, err := captureFmtStdin(t, "package sample\nfunc f() -> Int\n    1\nend\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != "package sample\nfunc f() -> Int\n  1\nend\n" {
		t.Fatalf("formatted stdin = %q", got)
	}
}

func TestRunFmtStdinRejectsInvalidInput(t *testing.T) {
	_, err := captureFmtStdin(t, "package sample\nfunc")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestWriteFormattedFileReplacesContentsSafely(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.mygo")
	if err := os.WriteFile(path, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeFormattedFile(path, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("contents = %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestRunFmtCheckDoesNotModifyFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.mygo")
	original := "package sample\nfunc f() -> Int\n    1\nend\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runFmt([]string{"--check", path}); err == nil {
		t.Fatal("expected check failure")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("check modified file: %q", got)
	}
}

func TestRunFmtCheckAcceptsFormattedFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.mygo")
	formatted := "package sample\nfunc f() -> Int\n  1\nend\n"
	if err := os.WriteFile(path, []byte(formatted), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runFmt([]string{"--check", path}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != formatted {
		t.Fatalf("formatted file changed: %q", got)
	}
}

func TestRunFmtProcessesFilesIndependently(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.mygo")
	bad := filepath.Join(dir, "bad.mygo")
	if err := os.WriteFile(good, []byte("package sample\nfunc f() -> Int\n    1\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("package sample\nfunc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runFmt([]string{bad, good}); err == nil {
		t.Fatal("expected aggregated error")
	} else if !strings.Contains(err.Error(), bad) {
		t.Fatalf("error = %v", err)
	}
	formatted, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(formatted), "  1\n") {
		t.Fatalf("valid file was not formatted: %q", formatted)
	}
	invalid, err := os.ReadFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	if string(invalid) != "package sample\nfunc" {
		t.Fatalf("invalid file changed: %q", invalid)
	}
}
