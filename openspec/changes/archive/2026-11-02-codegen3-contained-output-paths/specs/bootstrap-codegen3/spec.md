# Spec Delta

## MODIFIED Requirements

### Requirement: Generated output builds outside the source tree
The complete set of generated Go files for a package SHALL compile with the project's supported Go toolchain and SHALL not depend on generated files being checked into the repository. When `codegen3` is enabled the compiler SHALL write generated Go only under the active per-build output root in `MYGO_CACHE` (or the system temporary directory when caching is disabled) and SHALL invoke the Go toolchain directly (`go build` / `go test`), using temporary overlays for in-module packages and staged module copies for external MyGO packages. The compiler SHALL NOT create `zz_*.gen.go`, `zz_*.gen_test.go`, or a persistent project-local generated directory inside the repository. Project module inputs (`go.mod`, and `go.sum` when present) SHALL remain unchanged; Go dependencies SHALL be downloaded through `GOPROXY`. Output paths for packages outside the current module SHALL use deterministic import-path-based directories. Before writing or copying, the compiler SHALL verify that the resolved destination is contained within the active output root; it SHALL fail with a diagnostic without writing if containment cannot be established.

#### Scenario: Package build succeeds
- **WHEN** a package and its dependencies are compiled by the bootstrap pipeline
- **THEN** generated Go files are staged under the active codegen3 build/cache output root and the Go toolchain builds them using the configured temporary overlay and module file

#### Scenario: No checked-in generated files required
- **WHEN** generated files are produced only at build time
- **THEN** the package still compiles and no generated file appears under the source tree

#### Scenario: No project-local generated directory
- **WHEN** a package is built or tested with `codegen3`
- **THEN** the repository contains no persistent generated directory, and the temporary overlay exists only inside the `MYGO_CACHE` temporary area (or system temporary area when caching is disabled)

#### Scenario: Direct Go toolchain invocation
- **WHEN** `codegen3` builds or tests a package
- **THEN** the compiler runs `go build` or `go test` with the temporary overlay and, when external modules are staged, the temporary `-modfile`, without writing generated Go beside MyGO sources; a binary is written to the requested `-o` path or the same default location `go build` would use

#### Scenario: Module context and dependency download
- **WHEN** the generated package requires a Go module dependency
- **THEN** the build uses the project's `go.mod`/`go.sum` and resolves the dependency through `GOPROXY`

#### Scenario: External MyGO dependency output stays contained
- **WHEN** an imported MyGO package is resolved from the Go module cache or a local replacement outside the current module root
- **THEN** its source module is staged under a deterministic import-path-based directory within the active codegen3 build/cache output root, generated files are written only in that staged copy, and the dependency source directory remains unchanged

#### Scenario: Go module cache is not an overlay target
- **WHEN** codegen3 builds a MyGO package whose source is under `GOMODCACHE`
- **THEN** the Go overlay contains no replacement target beneath `GOMODCACHE`; a per-build temporary module file redirects the external module to its staged copy, and the project's `go.mod` and `go.sum` remain unchanged

#### Scenario: Staged module preserves ordinary Go sources
- **WHEN** an external MyGO module also contains ordinary Go files or other packages
- **THEN** the staged module preserves the source files needed to compile its imported Go packages alongside generated MyGO output

#### Scenario: Output path traversal is rejected
- **WHEN** an output or staged module path for any package, including an external test package, resolves outside the active codegen3 build/cache output root
- **THEN** codegen3 reports an output-path diagnostic before creating directories or files
