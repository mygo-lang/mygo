package mygo

import compilerpkg "github.com/mygo-lang/mygo/internal/mygo/compiler"

func CompileDirBootstrap(dir string) ([]string, error) {
	return compilerpkg.CompileDirBootstrap(dir)
}
func SyncBootstrap(root string) ([]string, error) { return compilerpkg.SyncBootstrap(root) }
func CompileDirBootstrapWithTiming(dir string) ([]string, error) {
	return compilerpkg.CompileDirBootstrapWithTiming(dir)
}
func SyncBootstrapWithTiming(root string) ([]string, error) {
	return compilerpkg.SyncBootstrapWithTiming(root)
}
