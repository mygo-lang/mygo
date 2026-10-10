## Context

See proposal.md for motivation. `cmd/mygo/main.go` parses global flags before the subcommand and currently initializes bootstrap selection to false. Both sync and build dispatch to separate legacy and bootstrap Go boundary APIs. Bootstrap prints written paths inside its writer; legacy sync relies on the CLI to print its returned paths. The formatter dispatch is independent.

This design is needed because changing the default requires explicit compatibility and migration decisions.

## Goals / Non-Goals

**Goals:** Make command selection deterministic, validate selectors before side effects, and preserve each pipeline's output and Prelude handling.

**Non-Goals:** Changing Go API meanings, rewriting compiler stages, expanding legacy support to build or fmt, or changing formatter behavior. Timing combined with `--no-prelude` retains existing behavior; adding a timed no-Prelude API is outside this change.

## Decisions

### Resolve selectors after parsing

Record whether legacy, explicit bootstrap, and bootstrap timing were requested. Validate incompatible combinations after global flag parsing, then choose bootstrap by default. Preserve the global-before-subcommand flag grammar. Using the last flag as the winner was rejected because selection would depend on argument order.

### Restrict legacy selection to sync

Interpret the requested escape hatch literally as `--legacy sync`. Reject legacy build and fmt with exit status 2 before command work. A legacy build remains possible by explicitly generating with legacy sync and then invoking `go build`. Allowing legacy build would expand the requested compatibility surface.

### Preserve compatibility and existing dispatch boundaries

Keep `--bootstrap` as an accepted alias so existing scripts continue working. Route default sync and build to the existing bootstrap APIs, selecting the no-Prelude or timing variant with the current precedence. Legacy sync continues using the existing legacy APIs. Keep generated-path printing at its existing owner to prevent duplicated bootstrap output. Avoid changing `compiler.Sync` or `CompileDir`, since callers may intentionally depend on legacy semantics.

### Surface bootstrap failures directly

Do not retry through legacy when bootstrap fails. Automatic fallback would make default compilation depend on source content and obscure bootstrap errors. Build runs Go build only after successful generation.

## Risks / Trade-offs

- [Plain sync/build can encounter bootstrap gaps or different output] -> Document the breaking default and the explicit legacy sync plus Go build recovery workflow; use focused CLI acceptance fixtures.
- [Conflicting flags previously had no explicit validation] -> Validate before generation and cover both argument orders.
- [Help and workflow instructions still call legacy production/default] -> Update README and compiler pipeline documentation together and audit other default claims.
- [Whole-tree regeneration creates unrelated generated changes] -> Validate with isolated temporary packages; this change does not require repository-wide regeneration.

## Migration Plan

1. Introduce CLI selection and validation with focused regression coverage.
2. Update help and documentation to use plain sync/build for bootstrap and `--legacy sync` for legacy generation.
3. Verify default, explicit bootstrap, legacy, no-Prelude, timing, conflict, and generation-failure paths using isolated fixtures.
4. Users needing legacy output migrate to `mygo --legacy sync <path>` followed by Go build when necessary. Existing explicit bootstrap commands continue to work.
5. Rollback restores the previous CLI default and matching documentation without changing the compiler APIs or requiring bulk regeneration.
