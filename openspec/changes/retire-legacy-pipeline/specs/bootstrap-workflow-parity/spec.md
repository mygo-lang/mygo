## MODIFIED Requirements

### Requirement: CLI compilation defaults to bootstrap
The CLI SHALL use the bootstrap pipeline for `sync` and the generation phase of `build`. Bootstrap errors SHALL be reported without invoking Go build. Default compilation SHALL honor `--no-prelude` and existing generated-file reporting behavior.

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
- **THEN** the CLI SHALL report failure without invoking Go build

## REMOVED Requirements

### Requirement: Legacy generation requires explicit sync selection
**Reason**: The legacy `--legacy sync` pipeline is removed, so no command selects it and no generated-file reporting path remains for it.
**Migration**: Use `mygo sync` or `mygo --no-prelude sync`, which generate through bootstrap. Regenerate previously legacy-generated packages with the default command.

### Requirement: Pipeline flags have deterministic compatibility
**Reason**: The requirement exists to define compatibility among the bootstrap flags and the removed `--legacy` selector. Its conflict scenarios no longer have two pipelines to conflict.
**Migration**: Use `--bootstrap` or `--bootstrap-timing` with the default pipeline. `--legacy` is rejected with the normal usage error.

## ADDED Requirements

### Requirement: Bootstrap flags remain available
The CLI SHALL retain `--bootstrap` as an explicit compatibility alias for default compilation and retain `--bootstrap-timing` for bootstrap stage timing. CLI help SHALL describe one default bootstrap pipeline and the compatibility and timing flags.

#### Scenario: Existing bootstrap invocation
- **WHEN** `mygo --bootstrap sync <package>` or `mygo --bootstrap build <package>` is invoked
- **THEN** pipeline selection SHALL match the corresponding default command

#### Scenario: Timing remains available
- **WHEN** `mygo --bootstrap-timing sync <package>` is invoked with normal Prelude selection
- **THEN** bootstrap SHALL generate the package and print stage durations to standard error

#### Scenario: Removed selector is rejected
- **WHEN** `mygo --legacy sync <package>` is invoked
- **THEN** the CLI SHALL reject the invocation with a usage error before generating or invoking Go build

#### Scenario: Help describes the default pipeline
- **WHEN** CLI usage is displayed
- **THEN** it SHALL identify bootstrap as the default and describe the compatibility and timing flags
