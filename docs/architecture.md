# Architecture — The Koded Stack

This document describes how the five layers of the Koded Stack fit together, what each one owns, and how they interact.

---

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     KodedBrowser                            │
│   Firefox/Gecko rendering engine                            │
│   Full UI rewrite in Rust (iced, GPU-accelerated)           │
│   Tree-style tabs · command palette · redesigned devtools   │
├──────────────────────────────────┬──────────────────────────┤
│  Web3 Layer                      │  KodedDB                 │
│  Solana adapter (native)         │  Hybrid structured/semi  │
│  MPC key sharding (Ika pattern)  │  4KB pages · WAL · B+tree│
│  k0:// → Arweave / IPFS          │  TCP server :6380        │
├──────────────────────────────────┴──────────────────────────┤
│  HTTP/K.0 Protocol                                          │
│  UDP · TLS 1.3 · BBR-lite · XOR FEC · 0-RTT · HPACK        │
├─────────────────────────────────────────────────────────────┤
│  koded-cli                                                  │
│  Cobra CLI · db REPL · protocol tools · package downloader  │
└─────────────────────────────────────────────────────────────┘
```

Each layer is independently deployable and testable, but they share interfaces:

| Interface | From | To | Protocol |
|---|---|---|---|
| SQL queries | koded-cli (`koded db`) | KodedDB | TCP / JSON newline-delimited |
| HTTP/K.0 ping/bench | koded-cli (`koded protocol`) | HTTP/K.0 server | UDP (K.0 wire format) |
| Browser storage | KodedBrowser | KodedDB | In-process Go FFI (planned) |
| Browser networking | KodedBrowser | HTTP/K.0 | Gecko network stack hook (planned) |
| Wallet state | Web3 layer | KodedDB | In-process (planned) |

---

## HTTP/K.0 Protocol — Internal Architecture

```
Application (send_request / send_response)
         │
         ▼
   HTTP/K.0 Framing (frame.rs)
   HEADERS · DATA · TRAILERS · RESET · SETTINGS · PUSH · GOAWAY · PING
         │
         ▼
   K0Transport (transport.rs)
   ┌─────────────────────────────────────────────────────┐
   │  Per-connection context (ConnectionCtx):            │
   │  ┌──────────────┐ ┌────────────┐ ┌───────────────┐  │
   │  │ TLS 1.3 +   │ │   Flow     │ │  0-RTT        │  │
   │  │ ChaCha20    │ │  Control   │ │  Sessions     │  │
   │  └──────────────┘ └────────────┘ └───────────────┘  │
   │  ┌──────────────┐ ┌────────────┐ ┌───────────────┐  │
   │  │ HPACK        │ │ BBR-lite   │ │  XOR FEC      │  │
   │  │ Compression  │ │ Congestion │ │  Recovery     │  │
   │  └──────────────┘ └────────────┘ └───────────────┘  │
   └─────────────────────────────────────────────────────┘
         │
         ▼
   K0Connection (connection.rs)
   Stream multiplexing table · ACK tracker (ack.rs)
         │
         ▼
   Packet Codec (packet.rs) — 22B header
         │
         ▼
   tokio::net::UdpSocket
```

**Inbound packet path:**

```
UDP receive → packet.decode() → crypto.open() → flow_control.check()
           → frame.decode() → ack.record() → deliver to stream
```

**Outbound packet path:**

```
application data → hpack.encode() (if headers) → frame.encode()
               → flow_control.consume() → bbr.pace()
               → crypto.seal() → packet.encode() → UDP send
```

---

## KodedDB — Internal Architecture

```
TCP Client
    │  {"sql": "SELECT ..."}
    ▼
api/server.go         ← accepts connections, spawns goroutine per client
    │
    ▼
query/lexer.go        ← tokenizes SQL string
    │
    ▼
query/parser.go       ← builds AST via recursive descent
    │
    ▼
query/executor.go     ← walks AST, calls engine methods
    │
    ▼
engine/engine.go      ← schema resolution, transaction coordination
    │  ┌──────────────────────────────────────────────────┐
    │  │ engine/btree.go                                  │
    │  │  B+ tree index over page IDs                     │
    │  │  Leaf nodes linked for range scans               │
    │  │  Splits propagate upward to parent               │
    │  └──────────────────────────────────────────────────┘
    │  ┌──────────────────────────────────────────────────┐
    │  │ storage/pager.go                                 │
    │  │  256-page LRU cache with dirty tracking          │
    │  │  Reads from disk on cache miss                   │
    │  │  Writes to WAL before writing to page file       │
    │  │                                                  │
    │  │ storage/page.go                                  │
    │  │  4KB slotted page layout                         │
    │  │  Header: checksum · slot count · free ptr        │
    │  │  Slot array grows down; records grow up          │
    │  │                                                  │
    │  │ storage/wal.go                                   │
    │  │  Append-only log with sequence numbers           │
    │  │  XOR checksum per entry                          │
    │  │  Replayed on startup for crash recovery          │
    │  │                                                  │
    │  │ storage/memtable.go                              │
    │  │  Skip list — O(log n) insert/lookup              │
    │  │  Flushed to B+ tree / page file on demand        │
    │  │                                                  │
    │  │ storage/schema_store.go                          │
    │  │  Table definitions stored inside the engine      │
    │  │  Persisted via WAL — survives restarts           │
    │  └──────────────────────────────────────────────────┘
    │
    ▼
Response JSON {"ok": true, "rows": [...]}
```

### Data Flow: INSERT

```
INSERT INTO users (id, name) VALUES (1, 'Koded')
         │
         ▼ executor resolves schema
         ▼ validates column types
         ▼ serializes row to bytes
         ▼ WAL.append(entry)         ← durable before visible
         ▼ memtable.insert(key, val) ← in-memory write buffer
         ▼ {"ok": true, "affected": 1}
```

### Data Flow: SELECT

```
SELECT * FROM users WHERE id = 1
         │
         ▼ executor resolves schema
         ▼ btree.lookup(key)          ← O(log n) via B+ tree
         ▼ pager.get_page(page_id)    ← cache hit or disk read
         ▼ page.scan() → matching rows
         ▼ {"ok": true, "columns": [...], "rows": [...]}
```

---

## koded-cli — Internal Architecture

```
koded (binary)
    │
    ▼
cmd/root.go         ← Cobra root, global flags
    │
    ├── cmd/db.go           ← SQL REPL
    │     ├── dialDB()      ← TCP connection to KodedDB :6380
    │     ├── dbClient{}    ← JSON newline send/recv
    │     └── runDB()       ← input loop, dot commands, result rendering
    │
    ├── cmd/protocol.go     ← HTTP/K.0 tools
    │     ├── ping          ← buildInitPacket() → UDP send/recv → RTT
    │     ├── bench         ← N probes, min/avg/max stats
    │     └── info          ← static protocol reference output
    │
    ├── cmd/download.go     ← Chunked resumable downloader
    │     ├── downloadWithResume()   ← state file + goroutine pool
    │     ├── downloadChunk()        ← HTTP Range request per chunk
    │     ├── mergeChunks()          ← assemble after all complete
    │     └── verifySHA256()         ← integrity check before use
    │
    ├── cmd/inspect.go      ← Manifest loader + pretty printer
    └── cmd/version.go      ← Version string
```

### Download State Machine

```
koded download rust
       │
       ▼ load manifest from manifests/rust.json
       ▼ resolve OS/arch → source URL + SHA256 + size
       ▼ load .koded/cache/rust-1.75.0.tar.gz.state.json (if exists)
       ▼ compute pending chunks (skip completed ones)
       ▼ spawn 4 goroutines via semaphore channel
       │   each goroutine:
       │     HTTP GET with Range: bytes=<start>-<end>
       │     write to rust-1.75.0.tar.gz.partN
       │     on success: update state.json atomically
       │     on failure: retry up to 3× with exponential backoff
       ▼ all goroutines done → mergeChunks()
       ▼ verifySHA256() → abort + report on mismatch
       ▼ cleanup .partN files and state.json
       ▼ report speed stats
```

---

## Cross-Layer Data Flow: Full Request

This is what a `koded db` session looks like end-to-end once KodedBrowser and HTTP/K.0 are fully integrated — tracing a single SQL query from the CLI through every layer.

```
Developer terminal
    koded> SELECT * FROM sessions WHERE user_id = 42;
         │
         ▼ koded-cli: dbClient.query()
         │ JSON: {"sql": "SELECT * FROM sessions WHERE user_id = 42"}
         │ TCP send to 127.0.0.1:6380
         │
         ▼ KodedDB api/server.go: reads JSON from conn
         ▼ query/lexer → parser → executor
         ▼ engine.Scan() → pager → page.scan()
         │ {"ok": true, "columns": ["user_id","token","expires"], "rows": [...]}
         │
         ▼ koded-cli: renderResponse() → tabular output
         │
  user_id  │  token         │  expires
  ─────────┼────────────────┼──────────────────
  42       │  eyJhbGci...   │  2025-05-01T00:00
```

---

## Design Principles

**Explicit over implicit.** Every layer makes its behavior visible — the WAL makes writes durable before they're acknowledged, the downloader saves state before marking a chunk complete, the protocol flags every packet with its reliability requirements.

**No head-of-line blocking.** HTTP/K.0's entire reason for existing is that TCP stalls all data when one packet is lost. Every design decision in the protocol layer traces back to this.

**Correctness before performance.** The WAL writes before the memtable updates. SHA256 is verified before a download is assembled. The B+ tree splits conservatively. Performance optimizations (page cache, concurrent downloads, BBR pacing) are layered on top of a correct foundation.

**No magic.** The CLI shows download progress with bytes and speed. The REPL shows query time in milliseconds. The protocol tools show raw RTT. Nothing is hidden.
