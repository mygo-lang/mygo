package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/compiler"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	noPrelude := false
	bootstrap := false
	args := os.Args[1:]
	for len(args) > 0 {
		switch args[0] {
		case "--no-prelude":
			noPrelude = true
			args = args[1:]
		case "--bootstrap":
			bootstrap = true
			args = args[1:]
		default:
			goto parsedFlags
		}
	}

parsedFlags:
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
		if bootstrap {
			written, err := compiler.SyncBootstrap(root)
			must(err)
			for _, path := range written {
				fmt.Println(displayPath(path))
			}
		} else if noPrelude {
			written, err := compiler.SyncNoPrelude(root)
			must(err)
			for _, path := range written {
				fmt.Println(displayPath(path))
			}
		} else {
			written, err := compiler.Sync(root)
			must(err)
			for _, path := range written {
				fmt.Println(displayPath(path))
			}
		}
	case "build":
		root := "."
		buildArgs := args[1:]
		if len(buildArgs) > 0 {
			if info, err := os.Stat(buildArgs[0]); err == nil && info.IsDir() {
				root = buildArgs[0]
				if !strings.HasPrefix(buildArgs[0], ".") && !strings.HasPrefix(buildArgs[0], "/") {
					buildArgs[0] = "./" + buildArgs[0]
				}
			}
		}
		var written []string
		var err error
		if bootstrap {
			written, err = compiler.SyncBootstrap(root)
		} else if noPrelude {
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
	fmt.Fprintln(os.Stderr, "usage: mygo [--bootstrap] [--no-prelude] <sync|build> [path|go build args...]")
	fmt.Fprintln(os.Stderr, "  --no-prelude  disable prelude auto-import (use when compiling prelude itself)")
	fmt.Fprintln(os.Stderr, "  --bootstrap   use parser2, typeinference2, and codegen2 (currently no MyGO package import resolution)")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func displayPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	wd, err := os.Getwd()
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(wd, abs)
	if err != nil {
		return abs
	}
	return rel
}
