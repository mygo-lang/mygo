package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
)

func TestRenderAndExtractSource(t *testing.T) {
	input := PackageMetadata{
		Package: PackageIdentity{ImportPath: "example.test/project/backend"},
		Exports: Exports{Functions: []FunctionExport{{Name: "Run"}}},
	}
	source, err := RenderSource("backend", input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), MetadataFileName, source, parser.AllErrors); err != nil {
		t.Fatalf("rendered metadata source is not valid Go: %v\n%s", err, source)
	}
	got, err := ExtractSource(MetadataFileName, source)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("ExtractSource() returned no metadata")
	}
	if got.Package.ImportPath != input.Package.ImportPath || got.Package.Name != "backend" {
		t.Fatalf("package identity = %+v", got.Package)
	}
	if got.SchemaVersion != SchemaVersion || got.AdapterABI != AdapterABI {
		t.Fatalf("version fields = schema %d, adapter %d", got.SchemaVersion, got.AdapterABI)
	}
	second, err := RenderSource("backend", input)
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != string(second) {
		t.Fatal("metadata serialization is not deterministic")
	}
}

func TestExportedDeclsReconstructsSignaturesAndPackageAliases(t *testing.T) {
	value := &PackageMetadata{
		Package: PackageIdentity{Name: "api", ImportPath: "example.com/project/api"},
		Imports: []PackageImport{{Alias: "clock", Path: "time", Go: true}},
		Exports: Exports{
			Types:   []TypeExport{{Name: "Record", Kind: "struct", Fields: []Field{{Name: "At", Type: Type{Kind: "named", Package: "time", Name: "Time"}}}}},
			Aliases: []AliasExport{{Name: "RecordAlias", Target: Type{Kind: "named", Package: "example.com/project/api", Name: "Record"}}},
			Functions: []FunctionExport{{
				Name: "Wrap", TypeParams: []TypeParam{{Name: "A", Scope: "func:Wrap", Index: 0}},
				Parameters: []Field{{Name: "value", Type: Type{Kind: "typeParam", Name: "A", Scope: "func:Wrap", Index: 0}}},
				Results:    []Type{{Kind: "named", Package: "example.com/project/api", Name: "Record"}},
			}},
			Interfaces: []InterfaceExport{
				{Name: "Show", TypeParams: []TypeParam{{Name: "A", Scope: "interface:Show", Index: 0}}, Methods: []Method{{Name: "Text", Parameters: []Field{{Name: "value", Type: Type{Kind: "typeParam", Name: "A", Scope: "interface:Show", Index: 0}}}, Results: []Type{{Kind: "builtin", Name: "string"}}}}},
				{Name: "Foldable", TypeParams: []TypeParam{{Name: "C[A]", Scope: "interface:Foldable", Index: 0}, {Name: "A", Scope: "interface:Foldable", Index: 1}}, Methods: []Method{{Name: "Fold", Parameters: []Field{{Name: "value", Type: Type{Kind: "applied", Name: "C", Scope: "interface:Foldable", Index: 0, Arguments: []Type{{Kind: "typeParam", Name: "A", Scope: "interface:Foldable", Index: 1}}}}}, Results: []Type{{Kind: "typeParam", Name: "A", Scope: "interface:Foldable", Index: 1}}}}},
			},
		},
	}
	decls, err := value.ExportedDecls()
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != 6 {
		t.Fatalf("reconstructed %d declarations, want import and five exports: %#v", len(decls), decls)
	}
	if imported, ok := decls[0].(ast2.Decl__ImportDecl); !ok || imported.F0 != "clock" || imported.F1 != "go:time" {
		t.Fatalf("reconstructed import = %#v", decls[0])
	}
	var record ast2.Decl__StructDecl
	for _, decl := range decls {
		if candidate, ok := decl.(ast2.Decl__StructDecl); ok {
			record = candidate
		}
	}
	if record.F0 != "Record" || len(record.F2) != 1 {
		t.Fatalf("reconstructed record = %#v", record)
	}
	if typ, ok := record.F2[0].Type.(ast2.TypeExpr__NamedType); !ok || typ.F0 != "clock.Time" {
		t.Fatalf("qualified field type = %#v", record.F2[0].Type)
	}
	for _, decl := range decls {
		if iface, ok := decl.(ast2.Decl__InterfaceDecl); ok && iface.F0 == "Foldable" {
			if len(iface.F1) != 2 || iface.F1[0] != "C[A]" {
				t.Fatalf("higher-kinded type parameters = %v", iface.F1)
			}
			applied, ok := iface.F2[0].Params[0].Type.(ast2.TypeExpr__NamedType)
			if !ok || applied.F0 != "C" || len(applied.F1) != 1 {
				t.Fatalf("higher-kinded application = %#v", iface.F2[0].Params[0].Type)
			}
			return
		}
	}
	t.Fatal("reconstructed declarations omit Foldable interface")
}

func TestStableImplementationID(t *testing.T) {
	identity := ImplementationIdentity{
		Package:   "example.com/app/backend",
		Interface: TypeclassID{Package: "example.com/app/contracts", Name: "Show", Arguments: []Type{{Kind: "typeParam", Name: "A", Scope: "interface", Index: 0}}},
		Target:    Type{Kind: "named", Package: "example.com/app/backend", Name: "Widget"},
	}
	first, err := StableImplementationID(identity)
	if err != nil {
		t.Fatal(err)
	}
	second, err := StableImplementationID(identity)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("IDs = %q and %q, want equal sha256 identities", first, second)
	}
	identity.Name = "custom"
	third, err := StableImplementationID(identity)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatalf("named implementation ID = %q, want identity to include implementation name", third)
	}
}

func TestLookupDynamicImplementationIDRequiresExactSelectedRecord(t *testing.T) {
	implementation := Implementation{
		ID: "sha256:chosen", Interface: TypeclassID{Package: "example.com/contracts", Name: "Show"},
		Target: Type{Kind: "named", Package: "example.com/domain", Name: "Widget"},
	}
	data, err := json.Marshal(PackageMetadata{
		Format: FormatName, SchemaVersion: SchemaVersion, CompilerABI: CompilerABI, AdapterABI: AdapterABI,
		Package:         PackageIdentity{Name: "backend", ImportPath: "example.com/backend"},
		Implementations: []Implementation{implementation},
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := LookupDynamicImplementationID(string(data), "example.com/backend", "example.com/domain", "example.com/contracts", "Show", "Widget")
	if err != nil || id != implementation.ID {
		t.Fatalf("lookup = %q, %v; want %q", id, err, implementation.ID)
	}
	if id, err := LookupDynamicImplementationID(string(data), "example.com/backend", "example.com/domain", "", "Show", "Widget"); err != nil || id != implementation.ID {
		t.Fatalf("lookup with inferred interface package = %q, %v; want %q", id, err, implementation.ID)
	}
	if _, err := LookupDynamicImplementationID(string(data), "example.com/backend", "example.com/domain", "example.com/contracts", "Fold", "Widget"); err == nil {
		t.Fatal("lookup for a different interface unexpectedly succeeded")
	}
	var incompatible PackageMetadata
	if err := json.Unmarshal(data, &incompatible); err != nil {
		t.Fatal(err)
	}
	incompatible.AdapterABI++
	data, err = json.Marshal(incompatible)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LookupDynamicImplementationID(string(data), "example.com/backend", "example.com/domain", "example.com/contracts", "Show", "Widget"); err == nil || !strings.Contains(err.Error(), "adapter ABI") {
		t.Fatalf("lookup with incompatible adapter ABI error = %v", err)
	}
}

func TestCanonicalTypeJSONRoundTripsAndRejectsAbsolutePackagePaths(t *testing.T) {
	typ := Type{
		Kind: "named", Package: "example.com/app/model", Name: "Box",
		Arguments: []Type{{Kind: "typeParam", Name: "A", Scope: "type", Index: 0}},
	}
	encoded, err := CanonicalTypeJSON(typ)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Type
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalTypeJSON(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(second) {
		t.Fatalf("canonical type JSON changed after round trip: %s != %s", encoded, second)
	}
	if strings.Contains(string(encoded), "/mnt/") || strings.Contains(string(encoded), `C:\\`) {
		t.Fatalf("canonical type JSON contains a machine-local path: %s", encoded)
	}
	typ.Package = "/workspace/model"
	if _, err := CanonicalTypeJSON(typ); err == nil {
		t.Fatal("CanonicalTypeJSON accepted an absolute package path")
	}
}

func TestWriteSourceAtomic(t *testing.T) {
	dir := t.TempDir()
	want := []byte("package sample\n")
	path, err := WriteSourceAtomic(dir, want)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != MetadataFileName {
		t.Fatalf("WriteSourceAtomic() path = %q", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("file content = %q, want %q", got, want)
	}
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSourceAtomic(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("replaced file content = %q, want %q", got, want)
	}
}

func TestExtractSourceRejectsMalformedAndDuplicateBlocks(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "missing end marker",
			src:  "// mygo:metadata-begin\n// {}\npackage sample\n",
		},
		{
			name: "malformed json",
			src:  "// mygo:metadata-begin\n// {\n// mygo:metadata-end\npackage sample\n",
		},
		{
			name: "multiple blocks",
			src: "// mygo:metadata-begin\n// {}\n// mygo:metadata-end\n" +
				"// mygo:metadata-begin\n// {}\n// mygo:metadata-end\npackage sample\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ExtractSource("sample.gen.go", []byte(tc.src)); err == nil {
				t.Fatal("ExtractSource() succeeded, want error")
			}
		})
	}
}

func TestExtractSourceRejectsIncompatibleMetadata(t *testing.T) {
	value := PackageMetadata{Package: PackageIdentity{ImportPath: "example.test/sample"}}
	source, err := RenderSource("sample", value)
	if err != nil {
		t.Fatal(err)
	}
	wrongABI := strings.Replace(string(source), `"adapterABI":1`, `"adapterABI":9`, 1)
	if _, err := ExtractSource("sample.gen.go", []byte(wrongABI)); err == nil || !strings.Contains(err.Error(), "adapter ABI") {
		t.Fatalf("ExtractSource() error = %v, want adapter ABI error", err)
	}
}

func TestReadDirAndVerifyGeneratedSources(t *testing.T) {
	dir := t.TempDir()
	generated := []byte("package sample\n\nvar Value = 42\n")
	if err := os.WriteFile(filepath.Join(dir, "sample.gen.go"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(generated)
	value := PackageMetadata{
		Package:         PackageIdentity{ImportPath: "example.test/sample"},
		GeneratedSource: []GeneratedSource{{Path: "sample.gen.go", SHA256: hex.EncodeToString(sum[:])}},
	}
	metadataSource, err := RenderSource("sample", value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, MetadataFileName), metadataSource, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("ReadDir() returned no metadata")
	}
	if err := got.VerifyGeneratedSources(dir); err != nil {
		t.Fatalf("VerifyGeneratedSources() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.gen.go"), []byte("package sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := got.VerifyGeneratedSources(dir); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("VerifyGeneratedSources() error = %v, want digest mismatch", err)
	}
	if _, err := ReadDir(dir); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("ReadDir() error = %v, want stale generated source error", err)
	}
}

func TestReadDirRejectsMultipleMetadataFiles(t *testing.T) {
	dir := t.TempDir()
	for _, filename := range []string{"a.gen.go", "b.gen.go"} {
		source, err := RenderSource("sample", PackageMetadata{Package: PackageIdentity{ImportPath: "example.test/sample"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filename), source, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReadDir(dir); err == nil || !strings.Contains(err.Error(), "multiple package metadata") {
		t.Fatalf("ReadDir() error = %v, want duplicate package metadata error", err)
	}
}

func TestVerifyGeneratedSourcesRejectsEscapingPath(t *testing.T) {
	value := PackageMetadata{GeneratedSource: []GeneratedSource{{Path: "../outside.go", SHA256: "unused"}}}
	if err := value.VerifyGeneratedSources(t.TempDir()); err == nil {
		t.Fatal("VerifyGeneratedSources() succeeded with escaping path")
	}
}

func TestTypeExprReconstructsReservedChannelTypes(t *testing.T) {
	ctx := astContext{packagePath: "example.com/project", aliases: map[string]string{"example.com/concurrency": "concurrency"}}
	for _, name := range []string{"Chan", "SendChan", "RecvChan"} {
		expr, err := ctx.typeExpr(Type{Kind: "named", Name: name, Package: "example.com/concurrency", Arguments: []Type{{Kind: "builtin", Name: "string"}}})
		if err != nil {
			t.Fatalf("typeExpr(%s) failed: %v", name, err)
		}
		named, ok := expr.(ast2.TypeExpr__NamedType)
		if !ok {
			t.Fatalf("typeExpr(%s) = %T, want named type", name, expr)
		}
		if named.F0 != name {
			t.Fatalf("typeExpr(%s) = %q, want an unqualified channel name", name, named.F0)
		}
		if len(named.F1) != 1 {
			t.Fatalf("typeExpr(%s) carries %d arguments, want 1", name, len(named.F1))
		}
	}
}
