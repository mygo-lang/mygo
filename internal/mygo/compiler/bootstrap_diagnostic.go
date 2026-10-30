package compiler

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
)

// These methods provide the native Go error method sets for MyGO-owned values.
func (e BootstrapDiagnosticError) Error() string { return ast2.RenderDiagnostic(e.Diagnostic) }
func (e BootstrapDiagnosticError) Unwrap() error { return e.Cause }
func (e bootstrapFFILoadError) Error() string    { return e.Cause.Error() }
func (e bootstrapFFILoadError) Unwrap() error    { return e.Cause }
