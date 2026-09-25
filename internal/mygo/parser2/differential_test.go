package parser2

// differential_test.go keeps the task-3.10 compatibility gate as a permanent
// regression.  It lowers every valid `.mygo` source in the repository through
// the CST path and compares the resulting ast2 tree against the legacy parser
// oracle; any lowering error, diagnostic, timeout, or shape difference fails.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/mygo-lang/mygo/prelude"
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
)

// differentialFingerprint renders a comparable string for an ast2 tree while
// skipping position and span fields, which the gate covers separately.
func differentialFingerprint(v reflect.Value, b *strings.Builder) {
	if !v.IsValid() {
		b.WriteString("<invalid>")
		return
	}
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			b.WriteString("<nil-iface>")
			return
		}
		elem := v.Elem()
		b.WriteString(elem.Type().Name())
		b.WriteByte('(')
		differentialFingerprint(elem, b)
		b.WriteByte(')')
	case reflect.Ptr:
		if v.IsNil() {
			b.WriteString("<nil-ptr>")
			return
		}
		differentialFingerprint(v.Elem(), b)
	case reflect.Slice:
		b.WriteString("len=")
		b.WriteString(strconv.Itoa(v.Len()))
		b.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			differentialFingerprint(v.Index(i), b)
			b.WriteByte(',')
		}
		b.WriteByte(']')
	case reflect.Struct:
		t := v.Type()
		b.WriteString(t.Name())
		b.WriteByte('{')
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if strings.Contains(f.Name, "Span") || strings.Contains(f.Name, "Pos") || f.Name == "SourceName" {
				continue
			}
			b.WriteString(f.Name)
			b.WriteByte(':')
			differentialFingerprint(v.Field(i), b)
			b.WriteByte(';')
		}
		b.WriteByte('}')
	case reflect.String:
		b.WriteString(strconv.Quote(v.String()))
	default:
		fmt.Fprintf(b, "%v", v.Interface())
	}
}

type differentialLowerResult struct {
	diag    string
	err     string
	file    ast2.File
	ok      bool
	timeout bool
}

func differentialLowerFile(path, src string) differentialLowerResult {
	ch := make(chan differentialLowerResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- differentialLowerResult{err: fmt.Sprintf("panic: %v", r)}
			}
		}()
		tree := ParseSyntaxAt(path, src)
		if len(tree.Diagnostics) != 0 {
			ch <- differentialLowerResult{diag: tree.Diagnostics[0].Message}
			return
		}
		lres := LowerSyntax(tree)
		okFile, ok := lres.(Result__Ok[ast2.File, string])
		if !ok {
			errRes, _ := lres.(Result__Err[ast2.File, string])
			ch <- differentialLowerResult{err: errRes.F0}
			return
		}
		ch <- differentialLowerResult{file: okFile.F0, ok: true}
	}()
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		return differentialLowerResult{timeout: true}
	}
}

// TestDifferentialCorpusMatchesLegacyParser is the differential gate for the
// completed CST lowering: every repository source the legacy parser accepts
// must lower to the same ast2 shape through the CST path.
func TestDifferentialCorpusMatchesLegacyParser(t *testing.T) {
	root := os.Getenv("ZZ_PROBE_ROOT")
	if root == "" {
		root = "../../.."
	}
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "openspec" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".mygo") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	var legacyOK, newErr, newDiag, equal, differ, timeout int
	var failures []string
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		// The oracle is the legacy combinator parser.  ParseFileAt now lowers
		// the CST, so it must not stand in for the legacy side here or the gate
		// would compare the new route against itself.
		res := parseFileLegacyAt(path, src)
		okFile, ok := res.(Result__Ok[ast2.File, string])
		if !ok {
			continue
		}
		legacyOK++
		r := differentialLowerFile(path, src)
		if r.timeout {
			timeout++
			failures = append(failures, path+" :: TIMEOUT")
			continue
		}
		if r.diag != "" {
			newDiag++
			failures = append(failures, path+" :: DIAG "+r.diag)
			continue
		}
		if r.err != "" {
			newErr++
			failures = append(failures, path+" :: LOWER "+r.err)
			continue
		}
		var lb, nb strings.Builder
		differentialFingerprint(reflect.ValueOf(okFile.F0), &lb)
		differentialFingerprint(reflect.ValueOf(r.file), &nb)
		if lb.String() == nb.String() {
			equal++
			continue
		}
		differ++
		ls, ns := lb.String(), nb.String()
		i := 0
		for i < len(ls) && i < len(ns) && ls[i] == ns[i] {
			i++
		}
		lo := i - 60
		if lo < 0 {
			lo = 0
		}
		hi := i + 120
		if hi > len(ls) {
			hi = len(ls)
		}
		hi2 := i + 120
		if hi2 > len(ns) {
			hi2 = len(ns)
		}
		failures = append(failures, fmt.Sprintf("%s\n    LEGACY ...%s\n    NEW    ...%s", path, ls[lo:hi], ns[lo:hi2]))
	}
	t.Logf("files=%d legacyOK=%d equal=%d differ=%d newErr=%d newDiag=%d timeout=%d", len(files), legacyOK, equal, differ, newErr, newDiag, timeout)
	if legacyOK == 0 {
		t.Fatal("differential gate found no legacy-parsable sources")
	}
	for _, f := range failures {
		t.Errorf("differential mismatch: %s", f)
	}
}
