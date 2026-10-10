## Purpose

Define a reliable standard LSP connection for MyGO editors, including message framing, lifecycle, document synchronization and source locations across Unicode text.

## ADDED Requirements

### Requirement: Standard message transport and JSON-RPC
The server SHALL read exact Content-Length byte counts using a persistent input stream, reserve stdout for framed protocol output, preserve integer and string request IDs, encode protocol fields as standard JSON values, and respond exactly once to each request and never to a notification. Successful empty results MUST include result: null; errors MUST contain an error object without a result field.

#### Scenario: Consecutive messages without body newlines
- **WHEN** multiple framed messages arrive together or in partial reads with UTF-8 bodies lacking trailing newlines
- **THEN** every complete message is decoded once with its original ID and no logging bytes enter stdout

#### Scenario: Invalid request and unknown method
- **WHEN** a complete frame contains malformed JSON, an invalid request, or an unsupported request method
- **THEN** the server returns the applicable JSON-RPC error and remains able to process subsequent valid messages

#### Scenario: Notification and null response
- **WHEN** a client sends initialized or a hover request with no available information
- **THEN** initialized produces no response and hover produces a response containing the original ID and result: null

### Requirement: Server lifecycle
The server SHALL initialize before serving language requests, reject requests before initialization, support shutdown followed by exit, and advertise only implemented capabilities. Exit after shutdown MUST terminate successfully; exit without shutdown MUST terminate unsuccessfully.

#### Scenario: Ordered lifecycle
- **WHEN** a client sends initialize, initialized, shutdown, and exit
- **THEN** initialize returns capabilities, shutdown returns null, and exit terminates without a notification response

### Requirement: Versioned full document synchronization
The server SHALL advertise full synchronization, read didOpen.textDocument.text, accept strictly newer didChange document versions, and use the latest complete text. didClose MUST remove the overlay and clear published diagnostics for that URI.

#### Scenario: Out of order edit
- **WHEN** an open document at version 4 receives changes at versions 5 and then 3
- **THEN** subsequent language requests use version 5 and the older update is ignored

#### Scenario: Close document
- **WHEN** an edited document closes
- **THEN** its diagnostics are cleared and subsequent workspace analysis uses available disk content

### Requirement: Unicode positions and canonical file URIs
The server SHALL interpret and return UTF-16 line positions, preserve indentation, convert escaped file URIs correctly, and avoid duplicated URI schemes. Invalid positions MUST be handled without process failure.

#### Scenario: Unicode and escaped path
- **WHEN** a symbol follows Chinese text or a supplementary Unicode character in a file whose path contains spaces
- **THEN** hover and navigation ranges identify the exact symbol using UTF-16 positions and a valid escaped file URI
