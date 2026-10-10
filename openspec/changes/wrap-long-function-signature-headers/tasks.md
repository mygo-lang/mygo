# Tasks

## 1. Function Header Layout

- [ ] 1.1 Add formatter fixtures for over-width and short function declaration headers, including the requested multi-parameter layout, and verify the expected canonical output.
- [ ] 1.2 Implement whole-header width measurement and per-parameter multiline rendering for ordinary functions and implementation methods; verify both fixtures match the canonical layout.
- [ ] 1.3 Add fixture coverage for return types, comments, nested parameter types, and idempotence; verify formatted output preserves signature content and is unchanged on a second pass.

## 2. Integration

- [ ] 2.1 Run the formatter package test suite and verify existing golden layouts, parser validity, and idempotence checks pass.
