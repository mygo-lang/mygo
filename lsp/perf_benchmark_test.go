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

// benchmarkEditorWorkspace creates the realistic editor fixture used by the
// parse-cache benchmarks: an app package, a small util dependency, and a large
// never-edited dependency package.  Baseline before the parse cache (measured
// on a Ryzen 9 PRO 7940HS): one edit with one open document ~195ms, with five
// open documents ~971ms; adding a ~74KB unchanged dependency raised them to
// ~423ms and ~2.2s because every publication re-analyzed the package.
func benchmarkEditorWorkspace(b testing.TB, appFiles int, withDependency bool) (string, []string) {
	b.Helper()
	root := b.TempDir()
	repository, err := filepath.Abs("..")
	if err != nil {
		b.Fatal(err)
	}
	goMod := fmt.Sprintf(
		"module example.com/editorbench\n\ngo 1.26\n\nrequire github.com/mygo-lang/mygo v0.0.0\nreplace github.com/mygo-lang/mygo => %s\n",
		repository,
	)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goMod), 0o600); err != nil {
		b.Fatal(err)
	}
	utilDir := filepath.Join(root, "util")
	if err := os.MkdirAll(utilDir, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(utilDir, "u.mygo"), []byte("package util\n\nfunc Clamp(v: Int) -> Int => v\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	if withDependency {
		depDir := filepath.Join(root, "bigdep")
		if err := os.MkdirAll(depDir, 0o755); err != nil {
			b.Fatal(err)
		}
		for file := 0; file < 6; file++ {
			var dep strings.Builder
			dep.WriteString("package bigdep\n\n")
			for index := 0; index < 120; index++ {
				fmt.Fprintf(&dep, "func Compute%d_%d(v: Int) -> Int => v + %d\n", file, index, index)
			}
			depPath := filepath.Join(depDir, fmt.Sprintf("b%d.mygo", file))
			if err := os.WriteFile(depPath, []byte(dep.String()), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		b.Fatal(err)
	}
	paths := make([]string, 0, appFiles)
	for file := 0; file < appFiles; file++ {
		var source strings.Builder
		source.WriteString("package app\n\nimport u \"example.com/editorbench/util\"\n")
		if withDependency {
			source.WriteString("import dep \"example.com/editorbench/bigdep\"\n")
		}
		source.WriteString("\n")
		for index := 0; index < 6; index++ {
			if withDependency {
				fmt.Fprintf(&source, "func F%d_%d(v: Int) -> Int\n  let c = u.Clamp(v)\n  dep.Compute0_%d(c)\nend\n\n", file, index, index)
			} else {
				fmt.Fprintf(&source, "func F%d_%d(v: Int) -> Int\n  let c = u.Clamp(v)\n  c + %d\nend\n\n", file, index, index)
			}
		}
		path := filepath.Join(appDir, fmt.Sprintf("a%02d.mygo", file))
		if err := os.WriteFile(path, []byte(source.String()), 0o600); err != nil {
			b.Fatal(err)
		}
		paths = append(paths, path)
	}
	return root, paths
}

func benchmarkEditorEdit(b *testing.B, openDocs int, withDependency bool) {
	_, paths := benchmarkEditorWorkspace(b, 12, withDependency)
	store := newDocumentStore()
	contents := make(map[string]string, openDocs)
	for _, path := range paths[:openDocs] {
		content, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
		contents[path] = string(content)
		storePut(&store, documentURI(path), 1, string(content))
	}
	edited := paths[0]
	if snapshot := analyzePackage(&store, documentURI(edited)); snapshot.Error != "" {
		b.Fatalf("initial analysis failed: %s", snapshot.Error)
	}
	misses := store.ParseCache.Misses
	analyses := store.PackageAnalyses
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		body := fmt.Sprintf("%s\n# edit %d\n", contents[edited], iteration)
		storePut(&store, documentURI(edited), 2+iteration, body)
		for _, path := range paths[:openDocs] {
			_ = packageDiagnostics(&store, documentURI(path))
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(store.ParseCache.Misses-misses)/float64(b.N), "parses/edit")
	b.ReportMetric(float64(store.PackageAnalyses-analyses)/float64(b.N), "analyses/edit")
}

func BenchmarkEditorAnalyzeEditOneOpen(b *testing.B) {
	benchmarkEditorEdit(b, 1, true)
}

func BenchmarkEditorAnalyzeEditFiveOpen(b *testing.B) {
	benchmarkEditorEdit(b, 5, true)
}

func BenchmarkEditorAnalyzeEditOneOpenNoDep(b *testing.B) {
	benchmarkEditorEdit(b, 1, false)
}

// TestEditorEditReparsesOnlyChangedFile pins the cache contract: one edited
// version performs one package analysis and parses only the changed file, no
// matter how many documents of the package are open.
func TestEditorEditReparsesOnlyChangedFile(t *testing.T) {
	_, paths := benchmarkEditorWorkspace(t, 5, true)
	store := newDocumentStore()
	contents := make(map[string]string, 5)
	for _, path := range paths[:5] {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents[path] = string(content)
		storePut(&store, documentURI(path), 1, string(content))
	}
	if snapshot := analyzePackage(&store, documentURI(paths[0])); snapshot.Error != "" {
		t.Fatalf("initial analysis failed: %s", snapshot.Error)
	}
	misses := store.ParseCache.Misses
	analyses := store.PackageAnalyses
	edited := paths[0]
	rounds := 3
	for round := 0; round < rounds; round++ {
		body := fmt.Sprintf("%s\n# edit %d\n", contents[edited], round)
		storePut(&store, documentURI(edited), 2+round, body)
		for _, path := range paths[:5] {
			_ = packageDiagnostics(&store, documentURI(path))
		}
	}
	if got := store.ParseCache.Misses - misses; got != rounds {
		t.Fatalf("edits reparsed %d files, want %d", got, rounds)
	}
	if got := store.PackageAnalyses - analyses; got != rounds {
		t.Fatalf("edits ran %d analyses, want %d", got, rounds)
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
