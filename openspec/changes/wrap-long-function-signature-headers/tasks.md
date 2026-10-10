# Tasks

## 1. Function Header Layout

- [x] 1.1 Add formatter fixtures for over-width and short function declaration headers, including the requested multi-parameter layout, and verify the expected canonical output.
- [x] 1.2 Implement whole-header width measurement and per-parameter multiline rendering for ordinary functions and implementation methods; verify both fixtures match the canonical layout.
- [x] 1.3 Add fixture coverage for return types, comments, nested parameter types, and idempotence; verify formatted output preserves signature content and is unchanged on a second pass.

## 2. Function Call Layout

- [x] 2.1 Add a formatter fixture with an over-width one-line function call whose callee makes the complete expression exceed 100 columns although its argument group alone fits; verify fmt preserves the callee and emits canonical multiline arguments without manual source breaks.
- [x] 2.2 Implement full call-expression width measurement and callee-preserving argument wrapping; verify the new fixture and existing call-wrapping fixtures pass.
- [x] 2.3 Add short-call and repeated-format fixtures; verify calls under the width remain compact and wrapped calls are idempotent.

## 3. Integration

- [x] 3.1 Run the formatter package test suite and verify declaration headers, automatic call wrapping, existing golden layouts, parser validity, and idempotence checks pass.
