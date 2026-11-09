# selfhost-ci Specification

## Purpose

Defines the CI contract for the workflow that builds the compiler with codegen3,
runs the full test suite through it, and verifies that the checkout stays free of
generated Go.

## Requirements

### Requirement: The self-host workflow definition is accepted

The workflow definition SHALL be valid GitHub Actions YAML: every expression
SHALL reference only contexts available where it is written, and the definition
SHALL be queued whenever its triggers fire instead of being rejected.

#### Scenario: Pushing to a branch schedules the job

- **WHEN** a branch push or pull request matches the workflow triggers
- **THEN** GitHub accepts the definition and schedules the self-host job without reporting an invalid workflow file

#### Scenario: Job-scoped environment avoids step-only contexts

- **WHEN** the job-level environment block is evaluated
- **THEN** it references only contexts available at job scope, and any value that depends on the runner is declared on the steps that use it

### Requirement: The codegen3 cache root stays outside the checkout

The workflow SHALL direct the codegen3 cache to a directory outside the checked
out repository for every step that invokes the codegen3 compiler, and SHALL NOT
require a persistent cache across jobs.

#### Scenario: Compiler steps write outside the work tree

- **WHEN** a step builds the codegen3 compiler or runs the suite through it
- **THEN** the active cache root is outside the checkout, and the step does not create or modify generated sources inside the repository

### Requirement: Generated-source verification tolerates an empty set

The snapshot and comparison steps SHALL complete successfully when the
repository contains no generated Go files, and SHALL produce comparable,
order-independent output whether or not such files exist.

#### Scenario: Repository has no generated files

- **WHEN** the snapshot step finds no matching generated Go files
- **THEN** the step exits successfully, writes an empty snapshot, and the comparison step reports no difference

### Requirement: The self-host pipeline exercises the codegen3 backend end to end

The workflow SHALL build the bootstrap compiler, use it to build the codegen3
compiler, run the full test suite through that binary, and confirm that the
project module files are unchanged.

#### Scenario: Successful self-host run

- **WHEN** the job completes its build and test steps
- **THEN** the suite ran under the codegen3 backend, the before and after generated-source snapshots match, and the module files are unmodified

#### Scenario: A codegen3 failure fails the job

- **WHEN** the codegen3 build, the test suite, or the snapshot comparison fails
- **THEN** the job reports failure at that step rather than being skipped or treated as success
