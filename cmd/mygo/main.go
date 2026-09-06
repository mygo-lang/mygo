package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/compiler"
	"github.com/mygo-lang/mygo/internal/mygo/formatter"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	noPrelude := false
	bootstrap := false
	bootstrapTiming := false
	args := os.Args[1:]
	for len(args) > 0 {
		switch args[0] {
		case "--no-prelude":
			noPrelude = true
			args = args[1:]
		case "--bootstrap":
			bootstrap = true
			args = args[1:]
		case "--bootstrap-timing":
			bootstrap = true
			bootstrapTiming = true
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
	case "fmt":
		if err := runFmt(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "sync":
		root := "."
		if len(args) > 1 {
			root = args[1]
		}
		if bootstrap {
			if noPrelude {
				_, err := compiler.SyncBootstrapNoPrelude(root)
				must(err)
				return
			}
			if bootstrapTiming {
				_, err := compiler.SyncBootstrapWithTiming(root)
				must(err)
				return
			}
			_, err := compiler.SyncBootstrap(root)
			must(err)
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
			if noPrelude {
				written, err = compiler.SyncBootstrapNoPrelude(root)
			} else if bootstrapTiming {
				written, err = compiler.SyncBootstrapWithTiming(root)
			} else {
				written, err = compiler.SyncBootstrap(root)
			}
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
	fmt.Fprintln(os.Stderr, "usage: mygo [--bootstrap] [--bootstrap-timing] [--no-prelude] <sync|build|fmt> [path|go build args...]")
	fmt.Fprintln(os.Stderr, "  --no-prelude  disable prelude auto-import (use when compiling prelude itself)")
	fmt.Fprintln(os.Stderr, "  --bootstrap   use parser2, typeinference2, and codegen2")
	fmt.Fprintln(os.Stderr, "  --bootstrap-timing  print bootstrap stage durations to standard error")
}

func runFmt(args []string) error {
	check := false
	var paths []string
	for _, arg := range args {
		if arg == "--check" {
			check = true
		} else {
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		formatted, err := formatter.Format("<stdin>", string(data))
		if err != nil {
			return err
		}
		_, err = os.Stdout.WriteString(formatted)
		return err
	}
	changed := false
	var failures []error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
			continue
		}
		formatted, err := formatter.Format(path, string(data))
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if string(data) == formatted {
			continue
		}
		changed = true
		if check {
			fmt.Fprintln(os.Stdout, path)
			continue
		}
		if err := writeFormattedFile(path, []byte(formatted)); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
			continue
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	if check && changed {
		return fmt.Errorf("files are not formatted")
	}
	return nil
}

func writeFormattedFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mygo-fmt-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
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
