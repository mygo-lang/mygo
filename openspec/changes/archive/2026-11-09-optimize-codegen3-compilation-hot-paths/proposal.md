## Why

codegen3 warm builds still parse sources and collect Go FFI signatures before consulting the generated-output cache, while per-file emission repeatedly scans package-wide declarations. These costs can make unchanged builds and packages with many source files unnecessarily expensive.

## What Changes

- Introduce a validated early cache-hit path that avoids unnecessary front-end and FFI work while preserving dependency, toolchain, test, cache integrity, and diagnostic correctness.
- Prepare package-wide emission information once and group declarations by source file to avoid repeated whole-package scans.
- Establish repeatable cold, warm, incremental, and many-file measurements, with output and behavioral parity checks.
- Keep the implementation readable and functional in style, using suitable MyGO features and prelude collection operations; split long touched files by responsibility.
- Preserve cache portability, per-file outputs, source mappings, backend behavior, and existing CLI interfaces.

## Capabilities

### New Capabilities

- `codegen3-compilation-performance`: Observable warm-cache and multi-file emission work avoidance, safe fallback, and repeatable performance measurement.

### Modified Capabilities

None.

## Impact

The bootstrap build/cache driver, package and FFI metadata handling, codegen3 per-file emission, associated tests and benchmarks may change. Cache metadata may need versioning; existing entries must remain safe to ignore or rebuild. This change does not alter language semantics or generated ABI.
