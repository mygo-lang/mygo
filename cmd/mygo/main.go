package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mygo-lang/mygo/internal/mygo/compiler"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	noPrelude := false
	args := os.Args[1:]
	// Parse --no-prelude flag before the subcommand.
	if len(args) > 0 && args[0] == "--no-prelude" {
		noPrelude = true
		args = args[1:]
	}
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "sync":
		root := "."
		if len(args) > 1 {
			root = args[1]
		}
		if noPrelude {
			written, err := compiler.SyncNoPrelude(root)
			must(err)
			for _, path := range written {
				fmt.Println(path)
			}
		} else {
			written, err := compiler.Sync(root)
			must(err)
			for _, path := range written {
				fmt.Println(path)
			}
		}
	case "build":
		root := "."
		buildArgs := args[1:]
		var written []string
		var err error
		if noPrelude {
			written, err = compiler.SyncNoPrelude(root)
		} else {
			written, err = compiler.Sync(root)
		}
		must(err)
		_ = written
		cmd := exec.Command("go", append([]string{"build"}, buildArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()
		must(cmd.Run())
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: mygo [--no-prelude] <sync|build> [path|go build args...]")
	fmt.Fprintln(os.Stderr, "  --no-prelude  disable prelude auto-import (use when compiling prelude itself)")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
