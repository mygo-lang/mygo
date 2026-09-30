# Dynamic typeclass values

MyGO can package values implementing the same typeclass interface into one
runtime type. The compiler generates ordinary Go wrapper types, package-level
adapters for supported `impl` methods, and caller-side construction and
dispatch functions. This feature adds no MyGO syntax and leaves static `using`
dispatch unchanged.

For an interface named `Show`, a caller with a dependency on `backend` can use
generated functions like these:

```mygo
let first = MygoDynamicShowFromBackendWidgetShow(widget)
let second = MygoDynamicShowFromBackendGadgetShow(gadget)
let values: Slice[MygoDynamicShow] = [first, second]
let names = values.Map(func(value: MygoDynamicShow) -> String
  MygoDynamicShowCallShow(value)
end)
```

The compiler derives constructor names from the interface, producer package,
and `impl` name. Each constructor records the selected implementation with the
payload. The generated method function dispatches through that identity and
references the producer's adapter directly. The interface wrapper type is
shared by all concrete implementations of that interface.

Dynamic methods can use the packaged payload for one hidden-type parameter.
Methods that need a second hidden-type value or pass the hidden type through a
callback are not exposed dynamically. Generic methods are emitted as ordinary
generic caller-side functions and pass the caller's type arguments to the
producer adapter when the adapter ABI supports them.

## Compiled dependencies

Each compiled MyGO package emits `zz_mygo_metadata.gen.go`. Its delimited JSON
metadata is stored in Go source comments and describes exported MyGO
declarations, imports, implementation identities, adapter symbols, and
generated-source digests. Callers read this metadata while compiling and can
adopt an implementation package without its `.mygo` files or recompiling that
package.

Distribute the generated `.gen.go` sources, including the metadata file, with
the package. Go archives do not retain source comments, so binary-only
dependencies cannot provide this metadata and are not supported for dynamic
dispatch.
