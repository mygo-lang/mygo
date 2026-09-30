// Package metadata defines and reads compiler metadata embedded in generated
// MyGO Go source files.
package metadata

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	FormatName       = "mygo-package"
	SchemaVersion    = 1
	CompilerABI      = "1"
	AdapterABI       = 1
	MetadataBegin    = "// mygo:metadata-begin"
	MetadataEnd      = "// mygo:metadata-end"
	MetadataFileName = "zz_mygo_metadata.gen.go"
)

// PackageMetadata describes the exported MyGO API and generated dynamic
// adapters for one Go package.
type PackageMetadata struct {
	Format          string            `json:"format"`
	SchemaVersion   int               `json:"schemaVersion"`
	CompilerABI     string            `json:"compilerABI"`
	AdapterABI      int               `json:"adapterABI"`
	Package         PackageIdentity   `json:"package"`
	Imports         []PackageImport   `json:"imports,omitempty"`
	GeneratedSource []GeneratedSource `json:"generatedSources"`
	Exports         Exports           `json:"exports"`
	Implementations []Implementation  `json:"implementations"`
}

// PackageImport preserves the source package's aliases while making package
// identity canonical. Go FFI imports use Go=true and Path without the MyGO
// `go:` marker.
type PackageImport struct {
	Alias string `json:"alias"`
	Path  string `json:"path"`
	Go    bool   `json:"go,omitempty"`
}

type PackageIdentity struct {
	Name       string `json:"name"`
	ImportPath string `json:"importPath"`
}

type GeneratedSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Exports struct {
	Types      []TypeExport      `json:"types"`
	Aliases    []AliasExport     `json:"aliases"`
	Functions  []FunctionExport  `json:"functions"`
	Interfaces []InterfaceExport `json:"interfaces"`
}

type TypeExport struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	TypeParams []TypeParam `json:"typeParameters,omitempty"`
	Fields     []Field     `json:"fields,omitempty"`
	Variants   []Variant   `json:"variants,omitempty"`
	Underlying *Type       `json:"underlying,omitempty"`
}

type AliasExport struct {
	Name       string      `json:"name"`
	TypeParams []TypeParam `json:"typeParameters,omitempty"`
	Target     Type        `json:"target"`
}

type FunctionExport struct {
	Name       string       `json:"name"`
	TypeParams []TypeParam  `json:"typeParameters,omitempty"`
	Parameters []Field      `json:"parameters"`
	Results    []Type       `json:"results"`
	Using      []Constraint `json:"using,omitempty"`
}

type InterfaceExport struct {
	Name       string      `json:"name"`
	TypeParams []TypeParam `json:"typeParameters,omitempty"`
	Methods    []Method    `json:"methods"`
}

type TypeParam struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Index int    `json:"index"`
}

type Field struct {
	Name string `json:"name,omitempty"`
	Type Type   `json:"type"`
	Tag  string `json:"tag,omitempty"`
}

type Variant struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields,omitempty"`
}

type Constraint struct {
	Interface string `json:"interface"`
	Arguments []Type `json:"arguments"`
}

type Method struct {
	Name       string       `json:"name"`
	TypeParams []TypeParam  `json:"typeParameters,omitempty"`
	Parameters []Field      `json:"parameters"`
	Results    []Type       `json:"results"`
	Using      []Constraint `json:"using,omitempty"`
}

// Type is a canonical, structured type descriptor. Fields are interpreted by
// Kind: builtin, typeParam, named, slice, map, tuple, function, reference, or
// applied (higher-kinded application).
type Type struct {
	Kind       string `json:"kind"`
	Name       string `json:"name,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Index      int    `json:"index,omitempty"`
	Package    string `json:"package,omitempty"`
	Arguments  []Type `json:"arguments,omitempty"`
	Parameters []Type `json:"parameters,omitempty"`
	Results    []Type `json:"results,omitempty"`
	Element    *Type  `json:"element,omitempty"`
	Key        *Type  `json:"key,omitempty"`
	Value      *Type  `json:"value,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

type Implementation struct {
	ID          string       `json:"implementationId"`
	Name        string       `json:"name,omitempty"`
	Interface   TypeclassID  `json:"interface"`
	Target      Type         `json:"target"`
	TypeParams  []TypeParam  `json:"typeParameters,omitempty"`
	Constraints []Constraint `json:"constraints,omitempty"`
	Methods     []Adapter    `json:"methods"`
}

type TypeclassID struct {
	Package   string `json:"package"`
	Name      string `json:"name"`
	Arguments []Type `json:"arguments,omitempty"`
}

type Adapter struct {
	Method               string       `json:"interfaceMethod"`
	Symbol               string       `json:"adapterSymbol"`
	HiddenParameterIndex int          `json:"hiddenParameterIndex"`
	TypeParams           []TypeParam  `json:"methodTypeParameters,omitempty"`
	LogicalParameters    []Field      `json:"logicalParameters"`
	LogicalResults       []Type       `json:"logicalResults"`
	LogicalUsing         []Constraint `json:"logicalUsing,omitempty"`
	GoParameters         []Type       `json:"goParameters"`
	GoResults            []Type       `json:"goResults"`
	ABI                  int          `json:"adapterABI"`
	Supported            bool         `json:"supported"`
	UnsupportedReason    string       `json:"unsupportedReason,omitempty"`
}

// RenderSource returns a valid Go file containing the metadata in a delimited
// line-comment block. JSON serialization is deterministic because the schema
// uses structs and all descriptor slices are sorted before rendering.
func RenderSource(packageName string, value PackageMetadata) ([]byte, error) {
	if packageName == "" {
		return nil, fmt.Errorf("metadata: missing Go package name")
	}
	value.Format = FormatName
	value.SchemaVersion = SchemaVersion
	value.CompilerABI = CompilerABI
	value.AdapterABI = AdapterABI
	value.Package.Name = packageName
	value.GeneratedSource = sortedSources(value.GeneratedSource)
	value.Exports = sortedExports(value.Exports)
	value.Implementations = append([]Implementation(nil), value.Implementations...)
	sort.Slice(value.Implementations, func(i, j int) bool {
		return value.Implementations[i].ID < value.Implementations[j].ID
	})

	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("metadata: encode package metadata: %w", err)
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by MyGO; DO NOT EDIT.")
	fmt.Fprintln(&out, MetadataBegin)
	// Keep JSON on one line so extraction is independent of Go comment
	// formatting and does not need a second JSON framing grammar.
	fmt.Fprintf(&out, "// %s\n", payload)
	fmt.Fprintln(&out, MetadataEnd)
	fmt.Fprintf(&out, "package %s\n", packageName)
	return out.Bytes(), nil
}

// ExtractSource extracts and validates the sole metadata block in a generated
// Go source file. Files without a block return (nil, nil).
func ExtractSource(filename string, src []byte) (*PackageMetadata, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("metadata: parse generated Go source %s: %w", filename, err)
	}
	return ExtractFile(parsed)
}

// ExtractFile extracts metadata from a Go AST parsed with parser.ParseComments.
func ExtractFile(file *ast.File) (*PackageMetadata, error) {
	if file == nil {
		return nil, fmt.Errorf("metadata: nil Go file")
	}
	var found *PackageMetadata
	for _, group := range file.Comments {
		start, end := -1, -1
		for i, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment.Text), "//"))
			if text == strings.TrimSpace(strings.TrimPrefix(MetadataBegin, "//")) {
				if start >= 0 {
					return nil, fmt.Errorf("metadata: duplicate begin marker")
				}
				start = i
			}
			if text == strings.TrimSpace(strings.TrimPrefix(MetadataEnd, "//")) {
				if end >= 0 {
					return nil, fmt.Errorf("metadata: duplicate end marker")
				}
				end = i
			}
		}
		if start < 0 && end < 0 {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("metadata: more than one metadata block in Go source")
		}
		if start < 0 || end <= start {
			return nil, fmt.Errorf("metadata: malformed metadata block markers")
		}
		var payload bytes.Buffer
		for _, comment := range group.List[start+1 : end] {
			raw := strings.TrimSpace(comment.Text)
			if !strings.HasPrefix(raw, "//") {
				return nil, fmt.Errorf("metadata: expected line comment in metadata block")
			}
			text := strings.TrimSpace(strings.TrimPrefix(raw, "//"))
			payload.WriteString(text)
		}
		value := &PackageMetadata{}
		if err := json.Unmarshal(payload.Bytes(), value); err != nil {
			return nil, fmt.Errorf("metadata: decode JSON block: %w", err)
		}
		if err := value.Validate(file.Name.Name); err != nil {
			return nil, err
		}
		found = value
	}
	return found, nil
}

func (m *PackageMetadata) Validate(goPackageName string) error {
	if m.Format != FormatName {
		return fmt.Errorf("metadata: unsupported format %q", m.Format)
	}
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("metadata: schema version %d is incompatible with supported version %d", m.SchemaVersion, SchemaVersion)
	}
	if m.CompilerABI != CompilerABI {
		return fmt.Errorf("metadata: compiler ABI %q is incompatible with supported ABI %q", m.CompilerABI, CompilerABI)
	}
	if m.AdapterABI != AdapterABI {
		return fmt.Errorf("metadata: adapter ABI %d is incompatible with supported ABI %d", m.AdapterABI, AdapterABI)
	}
	if m.Package.Name == "" || m.Package.Name != goPackageName {
		return fmt.Errorf("metadata: package name %q does not match Go package clause %q", m.Package.Name, goPackageName)
	}
	if m.Package.ImportPath == "" {
		return fmt.Errorf("metadata: missing canonical package import path")
	}
	return nil
}

// ReadDir finds metadata blocks in generated Go files under dir. It rejects
// duplicate package metadata rather than depending on filesystem iteration.
func ReadDir(dir string) (*PackageMetadata, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gen.go") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(files)
	var found *PackageMetadata
	for _, filename := range files {
		src, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		value, err := ExtractSource(filename, src)
		if err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("metadata: multiple package metadata blocks in %s", dir)
		}
		found = value
	}
	if found != nil {
		if err := found.VerifyGeneratedSources(dir); err != nil {
			return nil, err
		}
	}
	return found, nil
}

// VerifyGeneratedSources checks every generated source listed in metadata.
// The metadata file itself must not appear in GeneratedSource.
func (m *PackageMetadata) VerifyGeneratedSources(dir string) error {
	for _, source := range m.GeneratedSource {
		if filepath.Base(source.Path) == MetadataFileName {
			return fmt.Errorf("metadata: metadata file cannot include its own digest")
		}
		if !filepath.IsLocal(source.Path) {
			return fmt.Errorf("metadata: generated source path %q is not local to the package", source.Path)
		}
		path := filepath.Join(dir, filepath.Clean(source.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("metadata: read generated source %s: %w", source.Path, err)
		}
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		if digest != source.SHA256 {
			return fmt.Errorf("metadata: generated source digest mismatch for %s", source.Path)
		}
	}
	return nil
}

// ImplementationIdentity is the canonical identity input used to derive a
// stable ID for an implementation. Type nodes must already be normalized by
// the compiler (including alias expansion and canonical package paths).
type ImplementationIdentity struct {
	Package   string      `json:"package"`
	Interface TypeclassID `json:"interface"`
	Target    Type        `json:"target"`
	Name      string      `json:"name,omitempty"`
}

// StableImplementationID derives an implementation ID from canonical,
// machine-independent identity data. It does not include source locations.
func StableImplementationID(identity ImplementationIdentity) (string, error) {
	if identity.Package == "" || identity.Interface.Package == "" || identity.Interface.Name == "" {
		return "", fmt.Errorf("metadata: implementation identity is missing package or interface identity")
	}
	if err := validateImportPath(identity.Package); err != nil {
		return "", fmt.Errorf("metadata: invalid implementation package: %w", err)
	}
	if err := validateImportPath(identity.Interface.Package); err != nil {
		return "", fmt.Errorf("metadata: invalid interface package: %w", err)
	}
	if err := validateType(identity.Target, "target"); err != nil {
		return "", err
	}
	for index, arg := range identity.Interface.Arguments {
		if err := validateType(arg, fmt.Sprintf("interface.arguments[%d]", index)); err != nil {
			return "", err
		}
	}
	payload, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("metadata: encode implementation identity: %w", err)
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// CanonicalTypeJSON validates and serializes a structured type descriptor.
// Struct field order gives this representation a stable canonical encoding.
func CanonicalTypeJSON(typ Type) ([]byte, error) {
	if err := validateType(typ, "type"); err != nil {
		return nil, err
	}
	return json.Marshal(typ)
}

func validateType(typ Type, at string) error {
	validKind := false
	switch typ.Kind {
	case "builtin":
		validKind = typ.Name != ""
	case "typeParam":
		validKind = typ.Name != "" && typ.Scope != ""
	case "named":
		validKind = typ.Name != "" && typ.Package != ""
	case "slice", "reference":
		validKind = typ.Element != nil
	case "map":
		validKind = typ.Key != nil && typ.Value != nil
	case "tuple":
		validKind = true
	case "function":
		validKind = true
	case "applied":
		validKind = typ.Name != "" && len(typ.Arguments) > 0
	default:
		return fmt.Errorf("metadata: %s has unknown type kind %q", at, typ.Kind)
	}
	if !validKind {
		return fmt.Errorf("metadata: %s has incomplete %s type descriptor", at, typ.Kind)
	}
	if typ.Package != "" {
		if err := validateImportPath(typ.Package); err != nil {
			return fmt.Errorf("metadata: %s has invalid type package: %w", at, err)
		}
	}
	for index, arg := range typ.Arguments {
		if err := validateType(arg, fmt.Sprintf("%s.arguments[%d]", at, index)); err != nil {
			return err
		}
	}
	for index, param := range typ.Parameters {
		if err := validateType(param, fmt.Sprintf("%s.parameters[%d]", at, index)); err != nil {
			return err
		}
	}
	for index, result := range typ.Results {
		if err := validateType(result, fmt.Sprintf("%s.results[%d]", at, index)); err != nil {
			return err
		}
	}
	if typ.Element != nil {
		if err := validateType(*typ.Element, at+".element"); err != nil {
			return err
		}
	}
	if typ.Key != nil {
		if err := validateType(*typ.Key, at+".key"); err != nil {
			return err
		}
	}
	if typ.Value != nil {
		if err := validateType(*typ.Value, at+".value"); err != nil {
			return err
		}
	}
	return nil
}

func validateImportPath(path string) error {
	if path == "" || filepath.IsAbs(path) || strings.Contains(path, "\\") {
		return fmt.Errorf("package path %q is not a canonical import path", path)
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("package path %q is not a canonical import path", path)
		}
	}
	return nil
}

// WriteSourceAtomic writes generated metadata through a temporary file in the
// destination directory and renames it into place, so readers never observe a
// partial metadata source.
func WriteSourceAtomic(dir string, src []byte) (string, error) {
	path := filepath.Join(dir, MetadataFileName)
	file, err := os.CreateTemp(dir, ".mygo-metadata-*.tmp")
	if err != nil {
		return "", fmt.Errorf("metadata: create temporary source: %w", err)
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err := file.Write(src); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("metadata: write temporary source: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("metadata: sync temporary source: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("metadata: close temporary source: %w", err)
	}
	if err := os.Chmod(temp, 0o644); err != nil {
		return "", fmt.Errorf("metadata: set generated source permissions: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return "", fmt.Errorf("metadata: replace generated source: %w", err)
	}
	return path, nil
}

func sortedSources(values []GeneratedSource) []GeneratedSource {
	out := append([]GeneratedSource(nil), values...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func sortedExports(value Exports) Exports {
	out := value
	sort.Slice(out.Types, func(i, j int) bool { return out.Types[i].Name < out.Types[j].Name })
	sort.Slice(out.Aliases, func(i, j int) bool { return out.Aliases[i].Name < out.Aliases[j].Name })
	sort.Slice(out.Functions, func(i, j int) bool { return out.Functions[i].Name < out.Functions[j].Name })
	sort.Slice(out.Interfaces, func(i, j int) bool { return out.Interfaces[i].Name < out.Interfaces[j].Name })
	return out
}
