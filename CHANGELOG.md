# Changelog

All notable changes to the Koded Stack are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### In Progress
- SEMI column dot-notation queries (`WHERE meta.age > 18`)
- Hybrid TCP/UDP fallover transport (`hybridProtocol/`)

### Planned
- KodedDB JOIN support
- `koded wallet` — Solana keypair, balance, sign, send
- `koded install` — install downloaded packages from cache
- Non-primary-key indexes in KodedDB
- HPACK dynamic table (currently static table only)
- KodedBrowser UI (Rust + iced, Firefox/Gecko fork)
- Web3 native layer (Solana adapter, MPC key sharding)

---

## [0.1.0] — 2025-04

### Added — HTTP/K.0 Protocol (Rust)

- **Phase 1 complete:**
  - 22-byte packet header codec (conn\_id, packet\_num, flags, frame\_type, payload\_len)
  - All frame types: HandshakeInit, HandshakeInitAck, HandshakeDone, Stream, Ack, Nak, PathChallenge, PathResponse, FecParity, Control, Close
  - ACK tracker with RFC 6298 EWMA RTT estimation (smoothed RTT + variance → RTO)
  - In-flight tracking with BTreeMap — retransmit on NAK or timeout
  - BBR-lite congestion control — Startup, Drain, ProbeBw (8-slot cycle), ProbeRtt phases
  - XOR parity FEC — 4+1 group encoding, single-loss recovery without retransmit

- **Phase 2 complete:**
  - TLS 1.3 handshake via rustls (memory-safe, ring backend)
  - Per-packet ChaCha20-Poly1305 AEAD encryption, stateless decryption
  - HPACK header compression — full RFC 7541 61-entry static table
  - Per-stream and connection-level credit-based flow control
  - 0-RTT session resumption via signed 24-hour session tickets
  - HTTP/K.0 application framing — HEADERS, DATA, TRAILERS, RESET, SETTINGS, PUSH, GOAWAY, PING

- **Phase 3 in progress:**
  - Hybrid TCP/UDP fallover transport (tcp.rs, udp.rs, client.rs — experimental)

### Added — KodedDB (Go)

- Storage engine:
  - Slotted page layout (4KB pages, header checksum, slot array, records)
  - Pager with 256-page LRU cache, dirty-page tracking, crash-safe writes
  - Write-Ahead Log (WAL) with sequence numbers and XOR checksums
  - Skip list MemTable — sorted O(log n) in-memory write buffer
  - B+ tree index — linked leaf nodes for O(n) range scans, upward split propagation
  - Schema store — table definitions persisted in the engine itself (WAL-backed)

- Query layer:
  - SQL lexer with full token set (keywords, literals, operators, punctuation)
  - Recursive descent parser → AST
  - Executor: CREATE TABLE, DROP TABLE, INSERT, SELECT (WHERE, ORDER BY, LIMIT), UPDATE, DELETE
  - SEMI column type — semistructured, per-row flexible shape, typed binary serialization

- TCP server:
  - Newline-delimited JSON protocol on port 6380
  - Request: `{"sql": "..."}` or `{"cmd": "ping|stats|tables|flush"}`
  - Response: `{"ok": true, "columns": [...], "rows": [...], "time_ms": ...}`
  - Concurrent client support

### Added — koded-cli (Go)

- Cobra CLI scaffold — binary name `koded`
- `koded version` — short, JSON, and default output modes
- `koded inspect <pkg>` — load and display manifest details
- `koded download <pkg>` — chunked parallel resumable downloader:
  - HTTP Range requests with 2MB chunks
  - 4 concurrent goroutines per download
  - Per-chunk retry (3 attempts with backoff)
  - JSON state file for resume across process restarts
  - SHA256 verification before assembly
  - Live progress bar with speed and ETA
- `koded db` — interactive SQL REPL connected to KodedDB over TCP:
  - Auto-reconnect on connection loss
  - Multi-line SQL (semicolon-terminated)
  - Dot commands: `.tables`, `.ping`, `.stats`, `.flush`, `.server`, `.clear`, `.help`, `.exit`
  - Tabular result rendering with column alignment
  - Offline fallback mode
- `koded protocol ping <addr>` — HTTP/K.0 INIT probes with RTT measurement
- `koded protocol bench <addr>` — latency benchmark with min/avg/max stats
- `koded protocol info` — full protocol specification summary

- Package manifests: `rust`, `go`, `bat`, `fd`
