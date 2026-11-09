## Why

Commit `cc5d576e9f` introduced verified warm-cache reuse, but a representative self-host build regressed from 27.9 seconds to a timeout after 150 seconds on a warm cache. Dependency inference can bypass the early-hit path, and shared dependency summaries and input files are repeatedly validated; the relative cost of these mechanisms still needs measurement.

## What Changes

- Make verified cached dependency declarations usable throughout codegen3 dependency discovery, including on-demand inference, while retaining safe fallback and overlay behavior.
- Separate generated-output reuse from typed-dependency reuse, and retain methods, instances and diagnostic locations in the data used by inference.
- Bound repeated validation and source hashing within a build, and avoid cache-specific input scans when caching is disabled without weakening invalidation.
- Investigate and remove remaining material warm-path costs, including metadata staging, loading and repeated FFI collection when measurements justify it.
- Add correctness, work-count, and representative cold, warm, disabled and incremental performance verification, including the self-host build/test workflow.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `codegen3-compilation-performance`: Extend verified reuse to on-demand dependencies, bound validation work on shared dependency graphs, and require representative self-host performance evidence.
- `bootstrap-codegen3`: Reconcile the obsolete prohibition on metadata artifacts with the existing verified cache metadata contract.

## Impact

The bootstrap compiler's dependency loading, codegen3 cache and input fingerprinting, metadata handling, tests, benchmarks, and performance documentation may change. Cache formats may require versioning; incompatible entries remain safe misses. Language semantics, generated ABI, output containment, and LSP overlay behavior remain unchanged.
