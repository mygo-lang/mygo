## ADDED Requirements

### Requirement: Cache locking is portable across supported release platforms

The codegen3 compiler SHALL provide cache locking on every platform in the
project's supported release matrix, including Windows and macOS, while
preserving shared locking for builds and exclusive locking for cache cleaning.
The compiler SHALL wait for a bounded period when a compatible lock is held and
SHALL fail without deleting or reading partially written cache data when the
lock cannot be acquired before the deadline.

#### Scenario: Windows release build compiles
- **WHEN** the compiler is built for `windows/amd64`
- **THEN** the generated compiler sources compile without unresolved Unix-only lock symbols

#### Scenario: macOS release builds compile
- **WHEN** the compiler is built for `darwin/amd64` or `darwin/arm64`
- **THEN** the generated compiler sources compile and retain the same shared and exclusive cache-lock behavior

#### Scenario: Concurrent builds share the cache on every supported platform
- **WHEN** multiple codegen3 builds use the same cache root on Windows, macOS, or Linux
- **THEN** each build acquires a compatible shared lock, cache entries remain complete, and builds are not unnecessarily serialized against one another

#### Scenario: Cleaning cannot race an active build on every supported platform
- **WHEN** cache cleaning requests an exclusive lock while a build holds a shared lock
- **THEN** cleaning waits for the bounded timeout or exits with a busy diagnostic without deleting active cache data
