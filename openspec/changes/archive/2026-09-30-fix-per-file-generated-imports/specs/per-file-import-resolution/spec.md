## Purpose

Defines how imports are visible to MyGO source files, how qualified types flow across declarations in one package, and how imports are selected for each generated Go file.

## ADDED Requirements

### Requirement: MyGO imports are visible only in their declaring source file
An explicit import declaration SHALL make its alias and imported names available during inference only to declarations originating in the same `.mygo` source file. Package declarations and implementations SHALL remain available across files. Two files MAY use the same alias for different import paths, while conflicting aliases within one file SHALL be rejected.

#### Scenario: Import alias does not leak to a sibling file
- **WHEN** `a.mygo` imports Go package `fmt` as `f` and `b.mygo` refers to `f.Sprint` without declaring that import
- **THEN** inference of `b.mygo` SHALL report the alias as unavailable

#### Scenario: Same alias can name different packages in separate files
- **WHEN** `a.mygo` imports package path `go:fmt` as `p` and `b.mygo` imports package path `go:strings` as `p`
- **THEN** each file's references to `p` SHALL resolve to its own import

### Requirement: Cross-file signatures preserve canonical imported type identity
When a package-local function or declaration has a type originating from an imported package, inference in another source file SHALL preserve that type's canonical package identity and associated method metadata. A caller SHALL be able to infer and use a returned imported type, including resolving applicable Go methods and MyGO typeclass or inherent implementations, without declaring the originating package import solely for that use.

#### Scenario: Go FFI return type carries through a package-local function
- **WHEN** `b.mygo` imports package `a`, declares `f` returning `a.A`, and `c.mygo` calls `f()` then calls a method available on `A` without importing `a`
- **THEN** inference SHALL resolve the result as the same canonical `a.A` type and accept the method call

#### Scenario: Typeclass implementation applies to a type returned across files
- **WHEN** one source file declares a function returning a type whose package-local typeclass implementation is declared in another file, and a third file calls that function then invokes the typeclass method
- **THEN** inference SHALL find the package implementation and resolve the call without requiring a duplicate import of the returned type's Go package

#### Scenario: Explicit package qualifier still requires a local import
- **WHEN** a source file directly writes a package-qualified name such as `a.A` or `a.NewA()` but does not import `a` in that file
- **THEN** inference SHALL reject the unresolved package qualifier even if another file imports the same package

### Requirement: Generated Go imports are selected per output file
Each generated Go file SHALL include imports required by its emitted Go declarations and expressions, including imported types in signatures and generated helper references. It SHALL omit imports that are not referenced by that file's emitted Go. Import selection SHALL preserve explicit aliases and account for compiler-synthesized imports.

#### Scenario: Import used only by an erased typeclass signature is omitted
- **WHEN** a `.mygo` file declares a typeclass signature using `mail.Message` but the typeclass declaration is erased and no emitted Go declaration or expression refers to `mail`
- **THEN** that generated Go file SHALL omit `net/mail`

#### Scenario: Returned imported value needs no caller import for method use
- **WHEN** one generated file declares `f() a.A` and another generated file calls `f()` and invokes a method on the result without otherwise naming a type from package `a`
- **THEN** the declaration file SHALL import `a` to express its signature and the caller file SHALL not gain an unused import of `a`

#### Scenario: File-local alias is preserved in generated references
- **WHEN** a generated declaration in one source file refers to an imported package through an explicit alias
- **THEN** that generated file SHALL use the corresponding import alias and SHALL not inherit aliases declared only by sibling files
