## Why

Go permits a value of one interface type to be passed where another interface is expected when its method set satisfies the target interface. Bootstrap inference currently treats imported Go interface types nominally, rejecting valid calls such as passing `io.ReadCloser` to a parameter of type `io.Reader`.

## What Changes

- Define Go interface assignability for function arguments using Go method-set satisfaction.
- Support named imported interfaces and interfaces that include methods through embedding.
- Preserve rejection when the source interface lacks any method required by the target interface.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-ffi-boundary`: specify assignability between imported Go interface types at call arguments.

## Impact

This changes bootstrap type inference at Go FFI call boundaries. It affects calls to imported Go functions and methods that accept interface parameters; generated Go continues to use the original values and relies on Go's interface conversion semantics.
