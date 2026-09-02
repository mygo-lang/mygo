## MODIFIED Requirements

### Requirement: Shared language conformance
The production compiler and the bootstrap compiler SHALL accept or reject the
same in-scope MyGO source programs and SHALL preserve the same observable
source semantics. Differences in generated Go identifiers or internal lowering
strategies SHALL NOT constitute a language-level difference. This includes
qualified construction of enum variants whose unqualified name is reused by
another enum in the same package.

#### Scenario: A shared conformance fixture is compiled
- **WHEN** an in-scope fixture is compiled by both pipelines
- **THEN** both compilations SHALL either succeed with equivalent runtime
  behavior or fail with a diagnostic for the same source-language violation

#### Scenario: Both compilers accept a shared named variant name
- **WHEN** a package defines `AgentState.Running { Loop: ContextWindow }` and
  a separate enum also defines `Running`, then constructs
  `AgentState.Running { Loop: ctx }`
- **THEN** both compilation paths SHALL accept the program and infer the
  construction as `AgentState`

#### Scenario: Both compilers retain ordinary qualified construction
- **WHEN** a package constructs a named enum variant with no colliding variant
  names
- **THEN** both compilation paths SHALL retain the existing construction
  behavior
