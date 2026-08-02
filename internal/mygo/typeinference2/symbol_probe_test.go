package typeinference2

import (
	"fmt"
	"path/filepath"
	"testing"

	ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestStructFieldSymbolProbe(t *testing.T) {
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
	ti2Env := importedPackageEnv(packages[2], packages, []GoPackageEntry{})
	if s, ok := envGet(ti2Env, "ast2.Decl").(Option__Some[Scheme]); ok {
		fmt.Printf("ENV ti2 ast2.Decl body=%s\n", MonoStringFull(s.F0.Body))
	} else {
		fmt.Printf("ENV ti2 ast2.Decl = NONE\n")
	}
	cd2Env := importedPackageEnv(packages[0], packages, []GoPackageEntry{})
	if s, ok := envGet(cd2Env, "ast2.Decl").(Option__Some[Scheme]); ok {
		fmt.Printf("ENV cd2 ast2.Decl body=%s\n", MonoStringFull(s.F0.Body))
	} else {
		fmt.Printf("ENV cd2 ast2.Decl = NONE\n")
	}
	state := InferState{FreshVarID: 1, PkgInfo: None[PkgInfo](), GoPackages: []GoPackageEntry{}, MyGoPackages: packages, MyGoPackageCache: packages, Symbols: []Symbol{}, ActiveConstraints: []Predicate{}, NamedImpls: []string{}, ResolvedConstraintArgs: map[MethodConstraintKey][]ast2.MonoType{}}
	for _, d := range ti2Decls {
		if sd, ok := d.(ast2.Decl__StructDecl); ok && sd.F0 == "PkgDeclSource" {
			for _, f := range sd.F2 {
				if f.Name == "Decls" {
					resolved := typeFromASTInEnvWithParams(f.Type, []string{}, ti2Env, state)
					fmt.Printf("RESOLVED ti2 PkgDeclSource.Decls = %s\n", MonoStringFull(resolved))
				}
			}
		}
	}
	typeNames2 := collectMyGoTypeNames(packages[2].Decls, 0, []string{})
	manual := myGoPackageStructDeclSymbols(packages[2].Decls, "typeinference2", packages[2].Path, typeNames2, ti2Env, state, []Symbol{})
	for _, s := range manual {
		if ss, ok := s.(Symbol__StructField); ok && ss.F0 == "typeinference2.PkgDeclSource" && ss.F1 == "Decls" {
			fmt.Printf("MANUAL ti2 PkgDeclSource.Decls = %s\n", MonoStringFull(ss.F2))
		}
	}
	var loopSyms []Symbol
	for loopIdx, lpkg := range packages {
		lpkgTypeNames := collectMyGoTypeNames(lpkg.Decls, 0, []string{})
		lpkgEnv := importedPackageEnv(lpkg, packages, []GoPackageEntry{})
		if lpkg.Alias == "typeinference2" {
			if s, ok := envGet(lpkgEnv, "ast2.Decl").(Option__Some[Scheme]); ok {
				fmt.Printf("LOOP[%d] ti2 pkgEnv ast2.Decl = %s\n", loopIdx, MonoStringFull(s.F0.Body))
			} else {
				fmt.Printf("LOOP[%d] ti2 pkgEnv ast2.Decl = NONE\n", loopIdx)
			}
		}
		lstate := InferState{FreshVarID: 1, PkgInfo: None[PkgInfo](), GoPackages: []GoPackageEntry{}, MyGoPackages: packages, MyGoPackageCache: packages, Symbols: []Symbol{}, ActiveConstraints: []Predicate{}, NamedImpls: []string{}, ResolvedConstraintArgs: map[MethodConstraintKey][]ast2.MonoType{}}
		loopSyms = myGoPackageStructDeclSymbols(lpkg.Decls, lpkg.Alias, lpkg.Path, lpkgTypeNames, lpkgEnv, lstate, loopSyms)
	}
	for _, s := range loopSyms {
		if ss, ok := s.(Symbol__StructField); ok && ss.F1 == "Decls" {
			fmt.Printf("LOOPSYM %s.Decls = %s\n", ss.F0, MonoStringFull(ss.F2))
		}
	}
	symbols := myGoPackageStructSymbols(packages, []GoPackageEntry{}, []Symbol{})
	found := 0
	for _, s := range symbols {
		if ss, ok := s.(Symbol__StructField); ok {
			if ss.F0 == "codegen2.SourceFileInput" && ss.F1 == "File" {
				fmt.Printf("SYM codegen2.SourceFileInput.File type=%s\n", MonoStringFull(ss.F2))
				found++
			}
			if ss.F0 == "typeinference2.PkgDeclSource" && ss.F1 == "Decls" {
				fmt.Printf("SYM typeinference2.PkgDeclSource.Decls type=%s\n", MonoStringFull(ss.F2))
				found++
			}
			if ss.F0 == "ast2.File" && ss.F1 == "Decls" {
				fmt.Printf("SYM ast2.File.Decls type=%s\n", MonoStringFull(ss.F2))
				found++
			}
		}
	}
	if found == 0 {
		t.Fatal("no relevant symbols found")
	}
}
