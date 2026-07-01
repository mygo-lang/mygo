package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mygo-lang/mygo/internal/mygo"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "sync":
		root := "."
		if len(os.Args) > 2 {
			root = os.Args[2]
		}
		written, err := mygo.Sync(root)
		must(err)
		for _, path := range written {
			fmt.Println(path)
		}
	case "build":
		root := "."
		args := os.Args[2:]
		written, err := mygo.Sync(root)
		must(err)
		_ = written
		cmd := exec.Command("go", append([]string{"build"}, args...)...)
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
	fmt.Fprintln(os.Stderr, "usage: mygo <sync|build> [path|go build args...]")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
