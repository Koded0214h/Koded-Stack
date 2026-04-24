# The Koded Stack

> A complete alternative web stack — built from the transport protocol up to the browser — written in Rust and Go, by one person out of Lagos.

```
┌─────────────────────────────────────────────────────────┐
│  KodedBrowser  —  Firefox/Gecko fork, full UI rewrite   │
├──────────────────────────────────┬──────────────────────┤
│  Web3 Layer                      │  KodedDB             │
│  Native wallet · Solana · k0://  │  Semistructured DB   │
├──────────────────────────────────┴──────────────────────┤
│  HTTP/K.0  —  UDP, TLS 1.3, multiplexed streams         │
├─────────────────────────────────────────────────────────┤
│  koded-cli  —  unified console for the whole stack      │
└─────────────────────────────────────────────────────────┘
```

---

## What is this?

The Koded Stack is five interconnected systems that together replace the modern web infrastructure most people take for granted. Each piece stands on its own — but they are designed to work together from the ground up.

Chrome owns the browser. Google owns HTTP/3. AWS and Redis own database infrastructure. None of these were built with constraint in mind. The Koded Stack asks: what if you built the whole thing yourself? Not as a toy — as a real, working alternative you understand completely because you wrote every line.

---

## The Layers

### 1. HTTP/K.0 — The Protocol

Custom application-layer protocol built on UDP. Takes inspiration from QUIC (RFC 9000) but makes opinionated choices for gaming and streaming workloads. Phases 1 and 2 are complete and tested.

**Key capabilities:**
- Multiplexed independent streams — one dropped packet stalls one stream, not the connection
- Per-stream reliability flags (reliable = ACK + retransmit, partial = FEC only)
- ChaCha20-Poly1305 per-packet AEAD encryption with authenticated headers
- BBR-lite congestion control — reacts to RTT increase, not loss
- XOR parity FEC — recovers single-packet loss without a retransmit round-trip
- 0-RTT session resumption via signed session tickets
- PATH_CHALLENGE/RESPONSE for connection migration (WiFi → 4G)
- HPACK-lite header compression (RFC 7541 static table, ~80% reduction)
- Credit-based flow control per stream and per connection

**Location:** `HTTP-K.0-Browser/protocol/` — Rust + Tokio

---

### 2. KodedDB — The Database

A hybrid database built from scratch — typed columns for speed, `SEMI` columns for per-row flexible structure. Not a relational database with JSON bolted on. Not a document store pretending to have a schema.

**Storage engine:**
- Slotted page layout (4KB pages, checksum per page)
- Write-Ahead Log with sequence numbers and XOR checksums — crash-safe
- Skip list MemTable — O(log n) sorted in-memory write buffer
- B+ tree index — leaf nodes linked for O(n) range scans
- Page cache (256 pages, dirty-page-aware eviction)

**Query layer:**
- SQL lexer + recursive descent parser
- Supported: `CREATE TABLE`, `DROP TABLE`, `INSERT`, `SELECT` (WHERE, ORDER BY, LIMIT), `UPDATE`, `DELETE`
- `SEMI` column type — semistructured, different shape per row, serialized as typed binary

**Server:**
- TCP server on port `6380`
- Newline-delimited JSON protocol — `{"sql": "..."}` → `{"ok": true, "rows": [...]}`
- Commands: `ping`, `stats`, `tables`, `flush`

**Location:** `kodeddb-core/` — Go

---

### 3. KodedBrowser — The Browser

A Firefox fork (Gecko rendering engine) with the entire UI layer ripped out and rebuilt in Rust using `iced`. The goal is not a reskin. KodedDB replaces SQLite for all browser storage. HTTP/K.0 is wired into Gecko's network stack — `k0://` URLs resolve natively.

**Status:** Architecture designed. Not started. Planned as Phase 3.

---

### 4. Web3 Layer — Native, Not Bolted On

Solana adapter baked into KodedBrowser — no extensions, no Phantom, no MetaMask. The browser is the wallet. MPC key sharding via the Ika Network pattern. `k0://` URLs resolve to on-chain content on Arweave or IPFS.

**Status:** Architecture designed. Not started in the browser context.

---

### 5. koded-cli — The Console

The unified command-line tool that ties all layers together. Built on Cobra (Go). One terminal for the whole stack.

```bash
koded db                     # KodedDB interactive REPL
koded protocol ping <addr>   # HTTP/K.0 handshake probe
koded protocol bench <addr>  # RTT latency benchmark
koded protocol info          # Protocol specification
koded download <pkg>         # Resumable chunked download
koded inspect <pkg>          # Inspect package manifest
koded version                # Version info
```

**Location:** `koded-cli/` — Go

---

## Current Status

| Layer | Status | Language |
|---|---|---|
| HTTP/K.0 Protocol | Complete | Rust |
| KodedDB Storage Engine | Complete | Go |
| KodedDB Query Layer | Complete | Go |
| KodedDB TCP Server | Complete | Go |
| koded-cli core | Complete | Go |
| koded db REPL | Wired to engine | Go |
| koded protocol tools | Working | Go |
| SEMI column dot queries | In progress | Go |
| KodedDB JOIN support | Planned | Go |
| koded wallet | Planned | Go |
| KodedBrowser UI | Planned | Rust |
| Web3 native layer | Planned | Rust / Go |

---

## Repository Layout

```
next-up/
├── HTTP-K.0-Browser/        # HTTP/K.0 protocol (Rust)
│   ├── protocol/            # Core implementation
│   │   └── src/
│   │       ├── transport.rs     # Top-level UDP orchestrator
│   │       ├── connection.rs    # State machine + stream table
│   │       ├── packet.rs        # 22B wire format codec
│   │       ├── frame.rs         # HTTP/K.0 application frames
│   │       ├── ack.rs           # RTT estimation + retransmit
│   │       ├── congestion.rs    # BBR-lite
│   │       ├── fec.rs           # XOR parity FEC
│   │       ├── session.rs       # 0-RTT session tickets
│   │       ├── tls.rs           # TLS 1.3 + per-packet AEAD
│   │       ├── hpack.rs         # Header compression
│   │       ├── flow.rs          # Flow control windows
│   │       └── hybridProtocol/  # TCP/UDP fallover (in progress)
│   └── Network Stack/       # Learning / TCP-UDP primitives
│
├── kodeddb-core/            # KodedDB engine (Go)
│   ├── internal/
│   │   ├── storage/         # Page, pager, WAL, memtable, schema
│   │   ├── engine/          # B+ tree, engine facade
│   │   ├── query/           # Lexer, parser, executor
│   │   └── api/             # TCP server + JSON protocol
│   └── pkg/types/           # Shared type constants
│
├── koded-cli/               # Unified CLI (Go)
│   ├── cmd/                 # Cobra subcommands
│   │   ├── root.go
│   │   ├── db.go            # REPL connected to KodedDB
│   │   ├── protocol.go      # ping / bench / info
│   │   ├── download.go      # Chunked resumable downloader
│   │   ├── inspect.go       # Manifest inspector
│   │   └── version.go
│   ├── manifests/           # Package manifests (JSON)
│   └── pkg/types/           # Manifest + download types
│
├── docs/                    # Deep-dive documentation
└── LOOKING_INTO.md          # Full stack vision document
```

---

## Getting Started

### Prerequisites

| Tool | Version | Used by |
|---|---|---|
| Rust (stable) | 2021 edition+ | HTTP/K.0 |
| Go | 1.22+ | KodedDB, koded-cli |
| Cargo | bundled with Rust | HTTP/K.0 |

### HTTP/K.0 Protocol

```bash
cd HTTP-K.0-Browser/protocol

# Build
cargo build --release

# Run server (binds 0.0.0.0:9000)
cargo run -- server

# Connect as client
cargo run -- client

# RTT benchmark
cargo run -- bench

# Phase 2 feature demo (TLS, 0-RTT, FEC, HPACK, flow control)
cargo run -- demo

# Tests
cargo test
```

### KodedDB Server

```bash
cd kodeddb-core

# Run with defaults (data/ directory, port 6380)
go run .

# Custom address and data directory
go run . --addr 0.0.0.0:6380 --data /var/lib/kodeddb
```

### koded-cli

```bash
cd koded-cli

# Build binary
go build -o koded .

# Connect REPL to local KodedDB
./koded db

# Ping an HTTP/K.0 server
./koded protocol ping 127.0.0.1:9000

# Download a package
./koded download rust --dry-run
```

---

## Documentation

| Document | Contents |
|---|---|
| [Architecture](docs/architecture.md) | How all five layers connect and interact |
| [HTTP/K.0 Protocol](docs/http-k0.md) | Wire format, modules, design decisions |
| [KodedDB](docs/kodeddb.md) | Storage engine, query layer, TCP server internals |
| [koded-cli](docs/koded-cli.md) | Full command reference with examples |
| [Manifests](docs/manifests.md) | Package manifest format and authoring guide |
| [Roadmap](docs/roadmap.md) | Phased plan across all layers |

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Security

See [SECURITY.md](SECURITY.md).

---

*Built in Lagos. One person. From scratch.*
