package codegen2

import (
	"github.com/mygo-lang/mygo/internal/mygo/abi"
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/metadata"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	"github.com/mygo-lang/mygo/prelude"
)

// The dynamic metadata generator is written in MyGO (dynamic_metadata.mygo).
// MyGO has no multi-value error return, so the entry points that the Go
// toolchain calls keep their (value, error) shape here and convert the MyGO
// Result through the ABI-neutral helpers.
func dynamicStringResult(result prelude.Result[string, error]) (string, error) {
	if abi.HasOk[string, error](result) {
		return abi.OkValue[string, error](result, ""), nil
	}
	return "", abi.ErrValue[string, error](result, nil)
}

func dynamicMetadataSource(packageName, importPath string, generated map[string]string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	return dynamicStringResult(dynamicMetadataSourceResult(packageName, importPath, generated, files, dependencies))
}

func dynamicMetadataSourceWithDecls(packageName, importPath string, generated map[string]string, decls []ast2.Decl, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	return dynamicStringResult(dynamicMetadataSourceWithDeclsResult(packageName, importPath, generated, decls, dependencies))
}

func dynamicDispatchSource(packageName string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	return dynamicStringResult(dynamicDispatchSourceResult(packageName, files, dependencies))
}

func dynamicDispatchSourceWithLocal(packageName, packageImportPath string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) (string, error) {
	return dynamicStringResult(dynamicDispatchSourceWithLocalResult(packageName, packageImportPath, files, dependencies))
}

func dynamicGoType(value metadata.Type, imports dynamicImportTable) (string, error) {
	return dynamicStringResult(dynamicGoTypeResult(value, imports))
}
