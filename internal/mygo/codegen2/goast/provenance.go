package goast

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
)

type OriginNode struct {
	Node  ast.Node
	Key   string
	Start int
	End   int
}

type SourceOrigin struct {
	Span      ast2.SourceSpan
	Category  string
	Precision string
}

type GeneratedRange struct {
	Start  int
	End    int
	Origin SourceOrigin
}

type Origins struct {
	Nodes map[ast.Node]SourceOrigin
}

type LineDirectiveOutput struct {
	Source  string
	Failure string
}

type directiveEvent struct {
	offset int
	start  bool
	item   GeneratedRange
}

type directiveInsertion struct {
	offset int
	text   string
}

func NewOrigins() *Origins {
	return &Origins{Nodes: map[ast.Node]SourceOrigin{}}
}

func structuralKey(v reflect.Value, cache map[ast.Node]string) string {
	if !v.IsValid() {
		return "nil"
	}
	if v.Type() == reflect.TypeOf(token.NoPos) {
		return ""
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return "nil"
		}
		if node, ok := v.Interface().(ast.Node); ok {
			if cached, found := cache[node]; found {
				return cached
			}
			inner := v
			for inner.Kind() == reflect.Interface || inner.Kind() == reflect.Pointer {
				inner = inner.Elem()
			}
			result := fmt.Sprintf("%T:%x", node, sha256.Sum256([]byte(structuralKey(inner, cache))))
			cache[node] = result
			return result
		}
		return structuralKey(v.Elem(), cache)
	case reflect.Struct:
		result := v.Type().String()
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			if name == "Obj" || name == "Scope" || name == "Doc" || name == "Comment" || name == "Comments" {
				continue
			}
			result += "|" + name + ":" + structuralKey(v.Field(i), cache)
		}
		return result
	case reflect.Slice:
		var out strings.Builder
		out.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			out.WriteString(structuralKey(v.Index(i), cache))
			out.WriteByte(';')
		}
		out.WriteByte(']')
		return out.String()
	default:
		return fmt.Sprint(v.Interface())
	}
}

func originNodes(decls []ast.Decl, fset *token.FileSet) []OriginNode {
	cache := map[ast.Node]string{}
	var out []OriginNode
	for _, decl := range decls {
		if d, ok := decl.(*ast.GenDecl); ok && d.Tok == token.IMPORT {
			continue
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			if node == nil {
				return true
			}
			item := OriginNode{Node: node, Key: structuralKey(reflect.ValueOf(node), cache)}
			if fset != nil {
				item.Start = fset.Position(node.Pos()).Offset
				item.End = fset.Position(node.End()).Offset
			}
			out = append(out, item)
			return true
		})
	}
	return out
}

func renderedOriginNodes(source string) ([]OriginNode, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", source, 0)
	if err != nil {
		return nil, err
	}
	return originNodes(file.Decls, fset), nil
}

func Mark(origins *Origins, node ast.Node, span ast2.SourceSpan) {
	if origins == nil || node == nil || !ast2.ValidSourcePos(span.Start) {
		return
	}
	origin := SourceOrigin{Span: span, Category: "source", Precision: "construct"}
	ast.Inspect(node, func(current ast.Node) bool {
		if current == nil {
			return true
		}
		if _, exists := origins.Nodes[current]; exists {
			return false
		}
		origins.Nodes[current] = origin
		return true
	})
}

// MarkDeclAt is the MyGO FFI face of Mark. MyGO's FFI exposes ast.Decl and
// ast.Node as distinct types, and its ast2 structs are not the Go ast2 structs,
// so the source position is passed as primitive fields.
func MarkDeclAt(origins *Origins, decl ast.Decl, sourceName string, line, column int) {
	position := ast2.SourcePos{SourceName: sourceName, Line: line, Column: column}
	Mark(origins, decl, ast2.SourceSpan{Start: position, End: position})
}

func supportOrigin() SourceOrigin {
	return SourceOrigin{Category: "generated-support", Precision: "support"}
}

func originEqual(left, right SourceOrigin) bool {
	return left.Category == right.Category && left.Precision == right.Precision &&
		left.Span.Start == right.Span.Start && left.Span.End == right.Span.End
}

func rangeEqual(left, right GeneratedRange) bool {
	return left.Start == right.Start && left.End == right.End && originEqual(left.Origin, right.Origin)
}

func generatedRanges(origins *Origins, source string, decls []ast.Decl) ([]GeneratedRange, error) {
	support := supportOrigin()
	queues := map[string][]SourceOrigin{}
	for _, item := range originNodes(decls, nil) {
		origin := support
		if origins != nil {
			if known, ok := origins.Nodes[item.Node]; ok {
				origin = known
			}
		}
		queues[item.Key] = append(queues[item.Key], origin)
	}
	rendered, err := renderedOriginNodes(source)
	if err != nil {
		return nil, err
	}
	ranges := []GeneratedRange{{Start: 0, End: len(source), Origin: support}}
	consumed := map[string]int{}
	for _, item := range rendered {
		queue := queues[item.Key]
		offset := consumed[item.Key]
		if offset >= len(queue) {
			continue
		}
		ranges = append(ranges, GeneratedRange{Start: item.Start, End: item.End, Origin: queue[offset]})
		consumed[item.Key] = offset + 1
	}
	return ranges, nil
}

func preferRange(candidate, current GeneratedRange) GeneratedRange {
	candidateSize := candidate.End - candidate.Start
	currentSize := current.End - current.Start
	if candidateSize < currentSize {
		return candidate
	}
	if candidateSize == currentSize && candidate.Origin.Category == "source" && current.Origin.Category != "source" {
		return candidate
	}
	return current
}

func bestRange(active []GeneratedRange) (GeneratedRange, bool) {
	if len(active) == 0 {
		return GeneratedRange{}, false
	}
	best := active[0]
	for _, item := range active[1:] {
		best = preferRange(item, best)
	}
	return best, true
}

func eventBefore(left, right directiveEvent) bool {
	if left.offset != right.offset {
		return left.offset < right.offset
	}
	if left.start != right.start {
		return !left.start
	}
	if left.start {
		return left.item.End-left.item.Start > right.item.End-right.item.Start
	}
	return left.item.End-left.item.Start < right.item.End-right.item.Start
}

func lineColumnAt(source string, offset int) (int, int) {
	line, column := 1, 1
	if offset > len(source) {
		offset = len(source)
	}
	for index := 0; index < offset; index++ {
		if source[index] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return line, column
}

func directiveName(name, fallback string) string {
	if strings.TrimSpace(name) == "" {
		name = fallback
	}
	name = strings.ReplaceAll(strings.ReplaceAll(name, "\n", "_"), "\r", "_")
	name = strings.ReplaceAll(name, "*/", "_")
	if strings.HasPrefix(name, "<") {
		return name
	}
	return filepath.Base(name)
}

func directiveText(origin SourceOrigin, source string, offset int, sourceName, generatedName string) string {
	position := origin.Span.Start
	name := directiveName(position.SourceName, sourceName)
	if origin.Category != "source" || !ast2.ValidSourcePos(position) {
		line, column := lineColumnAt(source, offset)
		position = ast2.SourcePos{SourceName: generatedName, Line: line, Column: column}
		name = directiveName(generatedName, generatedName)
	}
	return fmt.Sprintf(" /*line %s:%d:%d*/", name, position.Line, position.Column)
}

func directiveInsertions(source string, directives map[int]SourceOrigin, sourceName, generatedName string) []directiveInsertion {
	offsets := make([]int, 0, len(directives))
	for offset := range directives {
		if offset < len(source) {
			offsets = append(offsets, offset)
		}
	}
	sort.Ints(offsets)
	var out []directiveInsertion
	var last SourceOrigin
	hasLast := false
	for _, offset := range offsets {
		origin := directives[offset]
		if hasLast && originEqual(last, origin) {
			continue
		}
		out = append(out, directiveInsertion{offset: offset, text: directiveText(origin, source, offset, sourceName, generatedName)})
		last = origin
		hasLast = true
	}
	return out
}

func ApplyLineDirectives(origins *Origins, source string, decls []ast.Decl, sourceName, generatedName string) LineDirectiveOutput {
	ranges, err := generatedRanges(origins, source, decls)
	if err != nil {
		return LineDirectiveOutput{Failure: err.Error()}
	}
	lineStarts := []int{0}
	for index, value := range []byte(source) {
		if value == '\n' && index+1 < len(source) {
			lineStarts = append(lineStarts, index+1)
		}
	}
	lineOrigins := make([]SourceOrigin, len(lineStarts))
	lineFirst := make([]int, len(lineStarts))
	lineKnown := make([]bool, len(lineStarts))
	for index := range lineOrigins {
		lineOrigins[index] = supportOrigin()
		lineFirst[index] = len(source) + 1
	}
	for _, item := range ranges {
		if item.Start < 0 || item.Start >= len(source) {
			continue
		}
		line := sort.Search(len(lineStarts), func(index int) bool { return lineStarts[index] > item.Start }) - 1
		if line < 0 {
			line = 0
		}
		if !lineKnown[line] || item.Start < lineFirst[line] || (item.Start == lineFirst[line] && item.Origin.Category == "source" && lineOrigins[line].Category != "source") {
			lineOrigins[line] = item.Origin
			lineFirst[line] = item.Start
			lineKnown[line] = true
		}
	}
	directives := map[int]SourceOrigin{}
	for line, offset := range lineStarts {
		directives[offset] = lineOrigins[line]
	}
	entries := directiveInsertions(source, directives, sourceName, generatedName)
	var builder strings.Builder
	previous := 0
	for _, entry := range entries {
		if entry.offset < previous || entry.offset > len(source) {
			return LineDirectiveOutput{Failure: "invalid line-directive insertion boundary"}
		}
		builder.WriteString(source[previous:entry.offset])
		builder.WriteString(entry.text)
		previous = entry.offset
	}
	builder.WriteString(source[previous:])
	return LineDirectiveOutput{Source: builder.String()}
}
