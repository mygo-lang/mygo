## Context

See proposal.md for motivation. `internal/mygo` contains hand-authored `.mygo` implementation sources and generated Go counterparts. Existing code already uses `Option[T]` and explicit `Some`/`None` matching, so the refactor can use established language patterns without changing MyGO syntax.

## Goals / Non-Goals

**Goals:**
- Make absence explicit where indexed or recursive helpers currently invent placeholder values for missing elements.
- Propagate `Option[T]` through helper boundaries and handle `None` at the call site that has enough context to decide the correct behavior.
- Preserve intentionally chosen defaults and externally observable behavior for valid source programs.
- Regenerate generated Go from the authoritative MyGO sources.

**Non-Goals:**
- Ban `UnwrapOr` or mechanically replace every occurrence.
- Change collection indexing semantics, MyGO syntax, or user-facing diagnostics unless required to correctly handle an impossible internal absence.
- Manually edit generated Go as the source of truth.

## Decisions

1. **Classify each occurrence by intent before changing it.** Replace only defaults that mask an absent indexed element in traversal/helper logic. Retain explicit semantic fallbacks, accumulator initialization, and other defaults whose absence is a valid condition. This avoids a broad textual rewrite changing behavior.
2. **Propagate `Option[T]` across the helper boundary.** Helpers that can fail to retrieve an element return `Option[T]`; callers use explicit `Some`/`None` branches. Prefer handling absence at the nearest layer that can preserve existing behavior or report a meaningful failure, rather than introducing another fabricated placeholder.
3. **Work from `.mygo` sources and regenerate.** Generated `.gen.go` files are derived outputs. Update source files throughout `internal/mygo`, then run the repository's established generation workflow and review the generated diff.
4. **Keep the change broad but staged by package.** Audit all MyGO sources under `internal/mygo`, then implement and compile in dependency order (shared utilities and AST foundations before parser/compiler consumers). This keeps intermediate type errors tractable while maintaining one repository-wide contract.

## Risks / Trade-offs

- [Many helpers may assume equal slice lengths or non-empty inputs] -> Make the owning caller decide whether absence is a recoverable `None`, an existing error result, or a violated invariant; do not silently invent a value.
- [Changing helper signatures may reveal latent mismatches across package boundaries] -> Propagate `Option[T]` through the full call chain and compile/regenerate each dependency layer before proceeding.
- [A retained `UnwrapOr` may be misclassified] -> Review context and intent, not just the default literal; document retained defaults when their purpose is not obvious.
- [Generated files may drift from source] -> Use the canonical generation command and verify the generated changes correspond to edited `.mygo` sources.

## Migration Plan

1. Inventory `UnwrapOr` uses in all `internal/mygo/**/*.mygo` sources and classify each as an absent-index fallback or intentional default.
2. Refactor absence-masking helpers and their callers in dependency order, preserving intentional fallback behavior.
3. Regenerate derived Go files, then run the repository's relevant compiler/build checks and review remaining candidate occurrences.
4. No data migration or rollback procedure is needed; reverting the source and generated-file commit restores prior internal behavior.
