package compiler

import . "github.com/mygo-lang/mygo/prelude"

// Error is the Go boundary name for MyGO's Error type.
type Error = error

// CompileDirBootstrap exposes the self-hosted MyGO bootstrap compiler through
// the conventional Go ([]string, error) API.
func CompileDirBootstrap(dir string) ([]string, error) {
	return unwrapBootstrapResult(compileDirBootstrapEntry(dir))
}

// SyncBootstrap compiles every MyGO package below root through the self-hosted
// pipeline and exposes its Result value as a Go error return.
func SyncBootstrap(root string) ([]string, error) {
	return unwrapBootstrapResult(syncBootstrapMyGO(root))
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
