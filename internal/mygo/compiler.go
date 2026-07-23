package mygo

import compilerpkg "github.com/mygo-lang/mygo/internal/mygo/compiler"

func CompileDir(dir string) ([]string, error) { return compilerpkg.CompileDir(dir) }
func Sync(root string) ([]string, error)      { return compilerpkg.Sync(root) }
func CompileDirBootstrap(dir string) ([]string, error) {
	return compilerpkg.CompileDirBootstrap(dir)
}
func SyncBootstrap(root string) ([]string, error) { return compilerpkg.SyncBootstrap(root) }
