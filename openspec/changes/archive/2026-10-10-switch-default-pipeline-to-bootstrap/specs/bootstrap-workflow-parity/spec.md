## ADDED Requirements

### Requirement: CLI compilation defaults to bootstrap
The CLI SHALL use the bootstrap pipeline for `sync` and the generation phase of `build` when no pipeline selection flag is supplied. Bootstrap errors SHALL be reported without automatically retrying through legacy. Default compilation SHALL honor `--no-prelude` and existing generated-file reporting behavior.

#### Scenario: Default sync
- **WHEN** `mygo sync <package>` is invoked
- **THEN** the CLI SHALL generate the package through bootstrap

#### Scenario: Default build
- **WHEN** `mygo build <package>` is invoked
- **THEN** the CLI SHALL generate through bootstrap and run Go build only after generation succeeds

#### Scenario: Default sync without Prelude
- **WHEN** `mygo --no-prelude sync <package>` is invoked for a package that does not use Prelude names
- **THEN** bootstrap SHALL compile without resolving or injecting Prelude

#### Scenario: Bootstrap compilation fails
- **WHEN** default sync or build encounters a bootstrap compilation error
- **THEN** the CLI SHALL report failure without falling back to legacy or invoking Go build

### Requirement: Legacy generation requires explicit sync selection
The CLI SHALL select the legacy pipeline for `mygo --legacy sync <package>`, including its existing generated-file reporting and `--no-prelude` behavior. `--legacy` SHALL be accepted only for `sync`; another command with that flag SHALL fail with a usage error before formatting, generation, or Go build occurs.

#### Scenario: Explicit legacy sync
- **WHEN** `mygo --legacy sync <package>` is invoked
- **THEN** the CLI SHALL generate through the legacy pipeline

#### Scenario: Legacy sync without Prelude
- **WHEN** `mygo --legacy --no-prelude sync <package>` is invoked
- **THEN** legacy generation SHALL disable automatic Prelude resolution and injection

#### Scenario: Unsupported legacy command
- **WHEN** `mygo --legacy build <package>` or `mygo --legacy fmt <file>` is invoked
- **THEN** the CLI SHALL report a usage error before performing command work

### Requirement: Pipeline flags have deterministic compatibility
The CLI SHALL retain `--bootstrap` as an explicit compatibility alias for default compilation and retain `--bootstrap-timing` for bootstrap stage timing. Combining `--legacy` with either bootstrap flag SHALL produce a usage error before generation, regardless of flag order. CLI help SHALL describe bootstrap as the default and `--legacy sync` as the legacy generation command.

#### Scenario: Existing bootstrap invocation
- **WHEN** `mygo --bootstrap sync <package>` or `mygo --bootstrap build <package>` is invoked
- **THEN** pipeline selection SHALL match the corresponding default command

#### Scenario: Timing remains available
- **WHEN** `mygo --bootstrap-timing sync <package>` is invoked with normal Prelude selection
- **THEN** bootstrap SHALL generate the package and print stage durations to standard error

#### Scenario: Conflicting selectors
- **WHEN** global flags contain `--legacy` and either `--bootstrap` or `--bootstrap-timing` in either order
- **THEN** the CLI SHALL report a usage error before generation

#### Scenario: Help explains migration
- **WHEN** CLI usage is displayed
- **THEN** it SHALL identify bootstrap as the default, the compatibility alias, and the sync-only legacy selector
