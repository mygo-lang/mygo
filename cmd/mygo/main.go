package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
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
	legacy := false
	bootstrapTiming := false
	pipelineOptions := compiler.PipelineOptions{OptimizationLevel: 2}
	args := os.Args[1:]
	for len(args) > 0 {
		switch {
		case args[0] == "--no-prelude":
			noPrelude = true
			pipelineOptions.NoPrelude = true
			args = args[1:]
		case args[0] == "--bootstrap":
			bootstrap = true
			args = args[1:]
		case args[0] == "--legacy":
			legacy = true
			args = args[1:]
		case args[0] == "--bootstrap-timing":
			bootstrap = true
			bootstrapTiming = true
			args = args[1:]
		case args[0] == "--codegen3":
			pipelineOptions.Codegen3 = true
			args = args[1:]
		case args[0] == "--opt-level":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "--opt-level requires 0, 1, or 2")
				os.Exit(2)
			}
			level, err := strconv.Atoi(args[1])
			if err != nil {
				fmt.Fprintln(os.Stderr, "--opt-level requires 0, 1, or 2")
				os.Exit(2)
			}
			pipelineOptions.OptimizationLevel = level
			args = args[2:]
		case strings.HasPrefix(args[0], "--opt-level="):
			level, err := strconv.Atoi(strings.TrimPrefix(args[0], "--opt-level="))
			if err != nil {
				fmt.Fprintln(os.Stderr, "--opt-level requires 0, 1, or 2")
				os.Exit(2)
			}
			pipelineOptions.OptimizationLevel = level
			args = args[1:]
		case args[0] == "--ir3-dump":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "--ir3-dump requires a path")
				os.Exit(2)
			}
			pipelineOptions.IR3Dump = args[1]
			args = args[2:]
		case strings.HasPrefix(args[0], "--ir3-dump="):
			pipelineOptions.IR3Dump = strings.TrimPrefix(args[0], "--ir3-dump=")
			args = args[1:]
		case args[0] == "--codegen3-emit":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "--codegen3-emit requires a directory")
				os.Exit(2)
			}
			pipelineOptions.EmitDir = args[1]
			args = args[2:]
		case strings.HasPrefix(args[0], "--codegen3-emit="):
			pipelineOptions.EmitDir = strings.TrimPrefix(args[0], "--codegen3-emit=")
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
	if legacy && bootstrap {
		fmt.Fprintln(os.Stderr, "--legacy cannot be combined with --bootstrap or --bootstrap-timing")
		os.Exit(2)
	}
	if legacy && pipelineOptions.Codegen3 {
		fmt.Fprintln(os.Stderr, "--codegen3 cannot be combined with --legacy")
		os.Exit(2)
	}
	if legacy && args[0] != "sync" {
		fmt.Fprintln(os.Stderr, "--legacy is only supported for sync")
		os.Exit(2)
	}
	if err := compiler.PipelineOptionsValidate(pipelineOptions); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	bootstrap = !legacy

	switch args[0] {
	case "cache":
		if len(args) < 2 || args[1] != "clean" {
			usage()
			os.Exit(2)
		}
		runCacheClean()
	case "clean":
		if len(args) < 2 || args[1] != "-cache" {
			usage()
			os.Exit(2)
		}
		runCacheClean()
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
			if pipelineOptions.Codegen3 {
				written, err := compiler.SyncBootstrapWithOptions(root, pipelineOptions)
				must(err)
				for _, path := range written {
					fmt.Println(displayPath(path))
				}
				return
			}
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
	case "build", "test":
		command := args[0]
		root, buildArgs := commandRoot(args[1:])
		if pipelineOptions.Codegen3 {
			// BuildCodegen3 runs `go` with cmd.Dir = root, so a package
			// directory argument becomes "." relative to that root.
			targets := buildArgs
			if root != "." && len(buildArgs) > 0 {
				targets = append([]string{"."}, buildArgs[1:]...)
			}
			if buildErr := compiler.BuildCodegen3(root, pipelineOptions, command == "test", targets); buildErr != nil {
				fmt.Fprintln(os.Stderr, buildErr)
				os.Exit(1)
			}
			return
		}
		var err error
		if noPrelude {
			_, err = compiler.SyncBootstrapNoPrelude(root)
		} else if bootstrapTiming {
			_, err = compiler.SyncBootstrapWithTiming(root)
		} else {
			_, err = compiler.SyncBootstrap(root)
		}
		must(err)
		cmd := exec.Command("go", append([]string{command}, buildArgs...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()
		buildErr := cmd.Run()
		if buildErr != nil {
			var exited *exec.ExitError
			if errors.As(buildErr, &exited) {
				os.Exit(exited.ExitCode())
			}
			must(fmt.Errorf("%s", ast2.RenderDiagnostic(ast2.ContextDiagnostic(ast2.NewDiagnostic("go-"+command, "go-tool-start", "operation", buildErr.Error()), "go "+command))))
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: mygo [--legacy|--bootstrap] [--bootstrap-timing] [--no-prelude] [--codegen3] [--opt-level=0|1|2] [--ir3-dump=path] [--codegen3-emit=dir] <sync|build|test|fmt|cache clean|clean -cache> [path|go build/test args...]")
	fmt.Fprintln(os.Stderr, "  sync, build, and test use the bootstrap pipeline by default")
	fmt.Fprintln(os.Stderr, "  --legacy      use the legacy pipeline (sync only)")
	fmt.Fprintln(os.Stderr, "  --no-prelude  disable prelude auto-import (use when compiling prelude itself)")
	fmt.Fprintln(os.Stderr, "  --bootstrap   compatibility alias for the default parser2, typeinference2, and codegen2 pipeline")
	fmt.Fprintln(os.Stderr, "  --bootstrap-timing  print bootstrap stage durations to standard error")
	fmt.Fprintln(os.Stderr, "  --codegen3    select the IR-based backend (generated Go stays in build/mygo3)")
	fmt.Fprintln(os.Stderr, "  --opt-level   select the IR optimization level (0, 1, or 2)")
	fmt.Fprintln(os.Stderr, "  --ir3-dump    write the IR dump to the given path (reserved)")
	fmt.Fprintln(os.Stderr, "  --codegen3-emit  copy cached generated files into the given directory (debug/IDE)")
	fmt.Fprintln(os.Stderr, "  cache clean   remove the codegen3 cache entries and temporary overlays")
	fmt.Fprintln(os.Stderr, "  clean -cache  Go-compatible spelling of `cache clean`")
}

func runCacheClean() {
	root := compiler.Codegen3CacheRoot()
	if root == "" {
		fmt.Fprintln(os.Stdout, "codegen3 cache is disabled")
		return
	}
	removed, err := compiler.CleanCodegen3Cache()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "cleaned codegen3 cache %s: %d files removed\n", root, removed)
}

// commandRoot mirrors `go build`/`go test` package selection: a leading
// directory argument becomes the working directory, and the remaining
// arguments are interpreted relative to it.
func commandRoot(args []string) (string, []string) {
	if len(args) == 0 {
		return ".", args
	}
	info, err := os.Stat(args[0])
	if err != nil || !info.IsDir() {
		return ".", args
	}
	root := args[0]
	if !strings.HasPrefix(args[0], ".") && !strings.HasPrefix(args[0], "/") {
		args[0] = "./" + args[0]
	}
	return root, args
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
