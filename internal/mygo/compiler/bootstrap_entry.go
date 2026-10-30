package compiler

import (
	"os"
	"path/filepath"

	"github.com/mygo-lang/mygo/internal/mygo/abi"
	. "github.com/mygo-lang/mygo/prelude"
)

// Error is the Go boundary name for MyGO's Error type.
type Error = error

// SyncBootstrapWithOptions is the option-aware SyncBootstrap entry point. The
// default path keeps the existing codegen2 behavior; Codegen3 selects the IR
// backend, which emits into <root>/build/mygo3 and leaves the source tree
// clean.
func SyncBootstrapWithOptions(root string, options PipelineOptions) ([]string, error) {
	if err := PipelineOptionsValidate(options); err != nil {
		return nil, err
	}
	if options.Codegen3 {
		session, err := newCodegen3Session(true)
		if err != nil {
			return nil, err
		}
		defer session.Close()
		var result Result[[]string, error]
		if options.NoPrelude {
			result = syncBootstrapMyGO3NoPrelude(root, options.OptimizationLevel, session.buildRoot, session.manifest, options.EmitDir)
		} else {
			result = syncBootstrapMyGO3(root, options.OptimizationLevel, session.buildRoot, session.manifest, options.EmitDir)
		}
		if _, err := unwrapBootstrapResult(result); err != nil {
			return nil, err
		}
		return codegen3ManifestFiles(session.manifest), nil
	}
	if options.NoPrelude {
		return SyncBootstrapNoPrelude(root)
	}
	return SyncBootstrap(root)
}

// SyncBootstrapCodegen3 compiles every MyGO package below root through the
// IR backend and returns the generated files, which live under
// <root>/build/mygo3 rather than beside the sources.
func SyncBootstrapCodegen3(root string, level int) ([]string, error) {
	session, err := newCodegen3Session(true)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	if _, err := unwrapBootstrapResult(syncBootstrapMyGO3(root, level, session.buildRoot, session.manifest, "")); err != nil {
		return nil, err
	}
	return codegen3ManifestFiles(session.manifest), nil
}

// SyncBootstrapCodegen3Dirs compiles only the given package directories (and
// their MyGO imports) through the IR backend. BuildCodegen3 uses it to mirror
// `go build`'s dependency selection instead of regenerating the whole module.
func syncCodegen3Dirs(root string, options PipelineOptions, dirs []string, buildRoot, manifest string) ([]string, error) {
	return unwrapBootstrapResult(syncBootstrapMyGO3Dirs(
		root,
		options.OptimizationLevel,
		options.NoPrelude,
		dirs,
		buildRoot,
		manifest,
		options.EmitDir,
	))
}

// SyncBootstrapCodegen3NoPrelude is SyncBootstrapCodegen3 without prelude
// auto-import.
func SyncBootstrapCodegen3NoPrelude(root string, level int) ([]string, error) {
	session, err := newCodegen3Session(true)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	if _, err := unwrapBootstrapResult(syncBootstrapMyGO3NoPrelude(root, level, session.buildRoot, session.manifest, "")); err != nil {
		return nil, err
	}
	return codegen3ManifestFiles(session.manifest), nil
}

// codegen3Session owns the per-build staging directory and the shared cache
// lock. Generated files live in cache entries; the staging directory only
// holds new output before publication and the overlay manifest.
type codegen3Session struct {
	buildRoot string
	manifest  string
	release   func()
}

func newCodegen3Session(lock bool) (*codegen3Session, error) {
	root, err := unwrapBootstrapStringResult(codegen3NewBuildRoot())
	if err != nil {
		return nil, err
	}
	session := &codegen3Session{
		buildRoot: root,
		manifest:  filepath.Join(root, "manifest.jsonl"),
		release:   func() {},
	}
	if lock {
		release, lockErr := codegen3AcquireCacheLock(false, 30000)
		if lockErr != nil {
			os.RemoveAll(root)
			return nil, lockErr
		}
		session.release = release
	}
	return session, nil
}

func (s *codegen3Session) Close() {
	if s == nil {
		return
	}
	s.release()
	os.RemoveAll(s.buildRoot)
}

func codegen3AcquireCacheLock(exclusive bool, waitMillis int) (func(), error) {
	result := codegen3LockCache(exclusive, waitMillis)
	if !abi.HasOk[bool, error](result) {
		return nil, abi.ErrValue[bool, error](result, nil)
	}
	return func() { codegen3UnlockCache() }, nil
}

// CleanCodegen3Cache removes every cached entry and temporary directory. The
// returned count is the number of files removed.
func CleanCodegen3Cache() (int64, error) {
	release, err := codegen3AcquireCacheLock(true, 10000)
	if err != nil {
		return 0, err
	}
	defer release()
	result := codegen3CacheClean()
	if !abi.HasOk[int64, error](result) {
		return 0, abi.ErrValue[int64, error](result, nil)
	}
	return abi.OkValue[int64, error](result, 0), nil
}

// Codegen3CacheRoot is the active cache directory, or "" when caching is off.
func Codegen3CacheRoot() string {
	return codegen3CacheRootPath()
}

// BuildCodegen3 runs the IR backend and the Go toolchain, exposing the MyGO
// Result as a conventional Go error.
func BuildCodegen3(root string, options PipelineOptions, test bool, args []string) error {
	result := buildCodegen3(root, options, test, args)
	if abi.HasOk[struct{}, error](result) {
		return nil
	}
	return abi.ErrValue[struct{}, error](result, nil)
}

// CompileDirBootstrap exposes the self-hosted MyGO bootstrap compiler through
// the conventional Go ([]string, error) API.
func CompileDirBootstrap(dir string) ([]string, error) {
	return unwrapBootstrapResult(compileDirBootstrapEntry(dir))
}

// CompileDirBootstrapWithTiming is CompileDirBootstrap with per-package stage
// timings written to standard error.
func CompileDirBootstrapWithTiming(dir string) ([]string, error) {
	return unwrapBootstrapResult(compileDirBootstrapEntryWithTiming(dir, true))
}

// SyncBootstrap compiles every MyGO package below root through the self-hosted
// pipeline and exposes its Result value as a Go error return.
func SyncBootstrap(root string) ([]string, error) {
	return unwrapBootstrapResult(syncBootstrapMyGO(root))
}

// SyncBootstrapNoPrelude compiles with the self-hosted pipeline without
// resolving or injecting the built-in prelude.
func SyncBootstrapNoPrelude(root string) ([]string, error) {
	return unwrapBootstrapResult(syncBootstrapMyGONoPrelude(root))
}

// SyncBootstrapWithTiming is SyncBootstrap with parser2, import/FFI,
// typeinference2, codegen2, write, and total timings written to standard error.
func SyncBootstrapWithTiming(root string) ([]string, error) {
	return unwrapBootstrapResult(syncBootstrapMyGOWithTiming(root, true))
}

// GenerateSource compiles one MyGO source string through the self-hosted
// pipeline with the normal automatic prelude context.
func GenerateSource(input string) (string, error) {
	return GenerateSourceAt("<input>", input)
}

// GenerateSourceAt is GenerateSource with a source name preserved in
// diagnostics.
func GenerateSourceAt(sourceName, input string) (string, error) {
	return unwrapBootstrapStringResult(bootstrapGenerateSourceAt(sourceName, input))
}

func unwrapBootstrapStringResult(result Result[string, error]) (string, error) {
	if abi.HasOk[string, error](result) {
		return abi.OkValue[string, error](result, ""), nil
	}
	return "", abi.ErrValue[string, error](result, nil)
}

func unwrapBootstrapResult(result Result[[]string, error]) ([]string, error) {
	if abi.HasOk[[]string, error](result) {
		return abi.OkValue[[]string, error](result, nil), nil
	}
	return nil, abi.ErrValue[[]string, error](result, nil)
}
