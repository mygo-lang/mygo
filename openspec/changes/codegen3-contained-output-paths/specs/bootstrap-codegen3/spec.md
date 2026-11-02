# Spec Delta

## MODIFIED Requirements

### Requirement: Generated output builds outside the source tree
The complete set of generated Go files for a package SHALL compile with the project's supported Go toolchain and SHALL not depend on generated files being checked into the repository. When `codegen3` is enabled the compiler SHALL write generated Go only under the active codegen3 build/cache output root and SHALL invoke the Go toolchain directly (`go build` / `go test`) through a temporary overlay to produce binaries, test binaries, and test results. The compiler SHALL NOT create `zz_*.gen.go`, `zz_*.gen_test.go`, or a persistent project-local generated directory such as `build/.mygo3` inside the repository. Project module inputs (`go.mod`, and `go.sum` when present) SHALL remain in place, and Go dependencies SHALL be downloaded through `GOPROXY`. Output paths for packages outside the current module SHALL use deterministic import-path-based directories. Before writing, the compiler SHALL verify that the resolved output directory is contained within the active output root; it SHALL fail with a diagnostic without writing if containment cannot be established.

#### Scenario: Package build succeeds
- **WHEN** a package and its dependencies are compiled by the bootstrap pipeline
- **THEN** the generated Go files form a buildable package inside the active codegen3 build/cache output root that the temporary overlay exposes

#### Scenario: No checked-in generated files required
- **WHEN** generated files are produced only at build time
- **THEN** the package still compiles and no generated file appears under the source tree

#### Scenario: No project-local generated directory
- **WHEN** a package is built or tested with `codegen3`
- **THEN** the repository contains no `build/.mygo3` directory, and the temporary overlay exists only inside the cache's temporary area

#### Scenario: Direct Go toolchain invocation
- **WHEN** `codegen3` builds or tests a package
- **THEN** the compiler runs `go build` or `go test -overlay <overlay>` against generated output without writing `zz_*.gen.go` or `zz_*.gen_test.go` beside the `.mygo` sources, and a binary is written to the requested `-o` path or to the same default location `go build` would use

#### Scenario: Module context and dependency download
- **WHEN** the generated package requires a Go module dependency
- **THEN** the build uses the project's `go.mod`/`go.sum` and resolves the dependency through `GOPROXY`

#### Scenario: External MyGO dependency output stays contained
- **WHEN** an imported MyGO package is resolved from the Go module cache or a local replacement outside the current module root
- **THEN** its generated files are written under a deterministic import-path-based directory within the active codegen3 build/cache output root, and the dependency source directory remains unchanged

#### Scenario: Output path traversal is rejected
- **WHEN** an output path for any package, including an external test package, resolves outside the active codegen3 build/cache output root
- **THEN** codegen3 reports an output-path diagnostic before creating directories or files
