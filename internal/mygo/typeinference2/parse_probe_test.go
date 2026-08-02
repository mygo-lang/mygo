package typeinference2

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestBootstrapParseSourceInferProbe(t *testing.T) {
	root := "/mnt/data-svr1-raid/xyh/code/go/mygo/internal/mygo"
	codegen2Files := []string{"codegen2.mygo", "decls.mygo", "translate_ast.mygo", "translate_expr.mygo", "types_util.mygo", "types.mygo", "gofile.mygo", "tailcall.mygo"}
	ast2Files := []string{"ast2.mygo", "ast2_ids.mygo", "monotype.mygo"}
	ti2Files := []string{"env.mygo", "infer.mygo", "types.mygo", "utils.mygo", "solver.mygo", "unify.mygo"}

	var codegen2Decls, ast2Decls, ti2Decls []ast2.Decl
	for _, f := range codegen2Files {
		codegen2Decls = append(codegen2Decls, parseMyGoFiles(t, filepath.Join(root, "codegen2", f))...)
	}
	for _, f := range ast2Files {
		ast2Decls = append(ast2Decls, parseMyGoFiles(t, filepath.Join(root, "ast2", f))...)
	}
	for _, f := range ti2Files {
		ti2Decls = append(ti2Decls, parseMyGoFiles(t, filepath.Join(root, "typeinference2", f))...)
	}

	packages := []MyGoPackageInfo{
		{Alias: "codegen2", Path: "github.com/mygo-lang/mygo/internal/mygo/codegen2", Decls: codegen2Decls},
		{Alias: "ast2", Path: "github.com/mygo-lang/mygo/internal/mygo/ast2", Decls: ast2Decls},
		{Alias: "typeinference2", Path: "github.com/mygo-lang/mygo/internal/mygo/typeinference2", Decls: ti2Decls},
	}

	src := `package probe

import ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"
import codegen2 "github.com/mygo-lang/mygo/internal/mygo/codegen2"
import typeinference2 "github.com/mygo-lang/mygo/internal/mygo/typeinference2"
import parser2 "github.com/mygo-lang/mygo/internal/mygo/parser2"

func bootstrapParseSource(path: String, sourcePath: String, source: String) -> Result[codegen2.BootstrapInputs, String]
  let parsed = parser2.ParseFileAt(sourcePath, source)
  switch parsed
    case Ok(file) =>
      let typed = ast2.AssignFileExprIDs(file)
      let input = codegen2.SourceFileInput { Path: sourcePath, File: typed }
      let pkg = typeinference2.PkgDeclSource { Path: sourcePath, Decls: typed.Decls }
      Ok(codegen2.BootstrapInputs { Inputs: [input], Sources: [pkg] })
    case Err(msg) => Err(msg)
  end
end
`
	parsed := parser2.ParseFileAt("probe.mygo", src)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("parse probe: %v", parsed)
	}
	// parser2 also needs to be in the package list for the import to resolve.
	// Its decls are not required for this probe (only ParseFileAt's signature).
	packages = append(packages, MyGoPackageInfo{
		Alias: "parser2", Path: "github.com/mygo-lang/mygo/internal/mygo/parser2", Decls: nil,
	})
	// parser2's own signature references ps.Parser etc.; without its real decls
	// those fall back through the cache. To keep ParseFileAt usable, load it.
	var parser2Decls []ast2.Decl
	parser2Decls = append(parser2Decls, parseMyGoFiles(t, filepath.Join(root, "parser2", "parser.mygo"))...)
	packages[len(packages)-1].Decls = parser2Decls

	result := InferPackageWithExternal(
		[]PkgDeclSource{{Path: "probe.mygo", Decls: file.F0.Decls}},
		[]PkgDeclSource{},
		[]GoPackageEntry{},
		packages,
	)
	switch v := result.(type) {
	case ResultOk[PackageInfo, string]:
		fmt.Println("INFER OK")
	case ResultErr[PackageInfo, string]:
		t.Fatalf("INFER FAIL: %v", v.F0)
	default:
		t.Fatalf("unexpected: %T", result)
	}
}
