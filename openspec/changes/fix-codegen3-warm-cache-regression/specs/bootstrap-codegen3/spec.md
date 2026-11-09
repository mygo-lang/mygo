## REMOVED Requirements

### Requirement: No metadata artifact is emitted
**Reason**: The existing verified codegen3 cache and generated output already carry importer-facing package metadata; prohibiting all metadata artifacts conflicts with the implemented and specified warm-cache reuse behavior.
**Migration**: Replace the prohibition with the verified importer metadata requirement below; entries without trustworthy metadata use the source-based fallback.

## ADDED Requirements

### Requirement: Importer metadata is verified and safely staged

codegen3 SHALL derive importer-facing metadata from the compiled package and MAY emit it alongside generated output under the active contained build or cache root. The compiler SHALL use cached metadata for dependency declarations only after verifying its compatibility, integrity, source and dependency inputs; if metadata cannot be trusted or does not cover the requested inputs, it SHALL use the correct current source-based compilation path. Metadata reuse MUST preserve cross-package declarations, dynamic dispatch and diagnostics, and MUST NOT override editor overlays.

#### Scenario: Verified metadata supplies a dependency
- **WHEN** a dependency's generated output and importer metadata are compatible, complete and verified for the current inputs
- **THEN** an importer can obtain equivalent declarations and dynamic-dispatch information from that metadata

#### Scenario: Invalid metadata falls back to sources
- **WHEN** metadata is missing, corrupt, incompatible or stale
- **THEN** the compiler uses current source-derived declarations and does not consume the untrusted metadata

#### Scenario: Metadata remains contained
- **WHEN** codegen3 emits or stages metadata
- **THEN** it remains under the active build or cache output root and does not create generated files in the source checkout
