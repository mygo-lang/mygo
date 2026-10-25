package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// benchmarkOverlayStore builds a store whose documents are all overlays, so
// fingerprinting never touches the disk and the measurement isolates the
// per-file metadata/content work.
func benchmarkOverlayStore(count int) (DocumentStore, []string) {
	store := newDocumentStore()
	content := strings.Repeat("# benchmark padding line\n", 25)
	paths := make([]string, 0, count)
	for index := 0; index < count; index++ {
		path := filepath.Join("/bench", fmt.Sprintf("file-%05d.mygo", index))
		storePut(&store, documentURI(path), 1, content)
		paths = append(paths, path)
	}
	return store, paths
}

// benchmarkTree writes dirs packages with files source files each and returns
// the workspace root.
func benchmarkTree(b *testing.B, dirs, files int) string {
	b.Helper()
	root := b.TempDir()
	source := "package bench\nfunc Value() -> Int => 1\n"
	for dirIndex := 0; dirIndex < dirs; dirIndex++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%03d", dirIndex))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		for fileIndex := 0; fileIndex < files; fileIndex++ {
			path := filepath.Join(dir, fmt.Sprintf("file%03d.mygo", fileIndex))
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	return root
}

// benchmarkModule writes a minimal Go module that replaces the MyGO module
// with this repository, matching the directed LSP test fixtures.
func benchmarkModule(b *testing.B, name string) (string, string) {
	b.Helper()
	root := b.TempDir()
	repository, err := filepath.Abs("..")
	if err != nil {
		b.Fatal(err)
	}
	goMod := fmt.Sprintf(
		"module example.com/%s\n\ngo 1.26\n\nrequire github.com/mygo-lang/mygo v0.0.0\nreplace github.com/mygo-lang/mygo => %s\n",
		name,
		repository,
	)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o600); err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(root, "main.mygo")
	if err := os.WriteFile(path, []byte("package bench\nfunc Value() -> Int => 1\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	return root, path
}

func BenchmarkAnalysisFingerprint(b *testing.B) {
	for _, count := range []int{500, 2000, 8000} {
		b.Run(fmt.Sprintf("%d", count), func(b *testing.B) {
			store, paths := benchmarkOverlayStore(count)
			b.ReportAllocs()
			b.ResetTimer()
			for iteration := 0; iteration < b.N; iteration++ {
				_ = analysisFingerprint(&store, paths)
			}
		})
	}
}

func BenchmarkWorkspacePaths(b *testing.B) {
	root := benchmarkTree(b, 200, 10)
	store := newDocumentStore()
	store.Roots = []string{root}
	_ = workspacePaths(&store)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = workspacePaths(&store)
	}
}

func BenchmarkBuildSourceIndex(b *testing.B) {
	root := benchmarkTree(b, 1, 500)
	store := newDocumentStore()
	store.Roots = []string{root}
	paths := workspacePaths(&store)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = buildSourceIndex(&store, paths)
	}
}

func BenchmarkAnalyzePackageWarm(b *testing.B) {
	_, path := benchmarkModule(b, "lspbench")
	store := newDocumentStore()
	uri := documentURI(path)
	if snapshot := analyzePackage(&store, uri); snapshot.Error != "" {
		b.Fatalf("initial analysis failed: %s", snapshot.Error)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = analyzePackage(&store, uri)
	}
}

func BenchmarkPackageDiagnostics(b *testing.B) {
	_, path := benchmarkModule(b, "lspdiag")
	store := newDocumentStore()
	uri := documentURI(path)
	if diagnostics := packageDiagnostics(&store, uri); len(diagnostics) != 0 {
		b.Fatalf("fixture produced diagnostics: %d", len(diagnostics))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		_ = packageDiagnostics(&store, uri)
	}
}
