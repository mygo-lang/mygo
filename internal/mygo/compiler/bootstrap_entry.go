package compiler

import . "github.com/mygo-lang/mygo/prelude"

// Error is the Go boundary name for MyGO's Error type.
type Error = error

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

// SyncBootstrapWithTiming is SyncBootstrap with parser2, import/FFI,
// typeinference2, codegen2, write, and total timings written to standard error.
func SyncBootstrapWithTiming(root string) ([]string, error) {
	return unwrapBootstrapResult(syncBootstrapMyGOWithTiming(root, true))
}

func unwrapBootstrapResult(result Result[[]string, error]) ([]string, error) {
	switch value := result.(type) {
	case ResultOk[[]string, error]:
		return value.F0, nil
	case ResultErr[[]string, error]:
		return nil, value.F0
	default:
		return nil, nil
	}
}
