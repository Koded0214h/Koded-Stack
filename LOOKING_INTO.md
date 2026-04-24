# The Koded Stack

> A complete alternative web stack — built from the transport protocol up to the browser — written in Rust and Go, by one person out of Lagos.

---

## What is this?

The Koded Stack is five interconnected systems that together form a full alternative to the modern web infrastructure most people take for granted. Each piece is the best version of itself, designed to work together — but independent enough to be useful on its own.

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

## The Five Layers

### 1. HTTP/K.0 — The Protocol
*"UDP speed. HTTPS security. Better than HTTP/3."*

HTTP/K.0 is a custom transport protocol built on UDP, inspired by QUIC but designed without Google's constraints. It solves the head-of-line blocking problem that plagues TCP-based protocols — if one packet is lost, only that stream stalls, not the entire connection.

**What it does:**
- Multiplexed independent streams on a single UDP connection
- Per-stream reliability flags — reliable (ACK + retransmit) or partial (FEC, no retransmit)
- ChaCha20-Poly1305 per-packet encryption with AAD-authenticated headers
- BBR-lite congestion control — reacts to RTT increase, not packet loss (stays fast for games and streaming)
- XOR parity FEC — recovers a lost packet without a round trip
- Session ticket 0-RTT resumption — skip the handshake on repeat connections
- PATH_CHALLENGE/RESPONSE — connection migrates if your IP changes (WiFi → 4G)
- HPACK-lite header compression — 61-entry static table, ~80% reduction on common headers
- Credit-based flow control per stream and per connection

**Status:** Complete. Handshake, streams, ACK engine, FEC, BBR, TLS framing, HTTP/K.0 application frames, 0-RTT — all built and tested.

**Language:** Rust + Tokio

---

### 2. KodedDB — The Database
*"Redis + PostgreSQL + MongoDB — but semistructured and actually fast."*

KodedDB is not another relational database with JSON bolted on as an afterthought. It is built from the ground up as a hybrid — typed columns for speed and constraints, SEMI columns for flexibility per row.

A `SEMI` column stores a different shape per row. Row 1 can have `meta.age = 25`. Row 2 can have `meta.company.name = "Teenovatex"`. Same column, different structure. No schema migration needed.

**Storage engine:**
- Slotted page layout (4KB pages, checksum per page)
- Write-Ahead Log (WAL) with sequence numbers and XOR checksums — crash-safe
- Skip list MemTable — sorted in-memory write buffer, O(log n) ops
- B+ tree index — leaf nodes linked for O(n) range scans, splits propagate upward
- Page cache (256 pages, dirty-page-aware eviction)

**Query layer:**
- SQL lexer + recursive descent parser
- AST → executor → engine calls
- Supported: `CREATE TABLE`, `DROP TABLE`, `INSERT`, `SELECT` (WHERE, ORDER BY, LIMIT), `UPDATE`, `DELETE`
- SEMI column type — semistructured, any shape per row, serialized as typed binary
- Schema store — table definitions persisted inside the engine itself (WAL-backed)

**Server:**
- TCP query server on port 6380
- Newline-delimited JSON protocol: `{"sql": "..."}` → `{"ok": true, "rows": [...]}`
- Special commands: `ping`, `stats`, `tables`, `flush`
- Multiple simultaneous clients

**Status:** Storage engine done. Query layer done. TCP server done. REPL connected. Missing: SEMI column dot-notation queries (`WHERE meta.age > 18`), non-primary-key indexes, JOIN support.

**Language:** Go

---

### 3. KodedBrowser — The Browser
*"Firefox core. Completely new soul."*

KodedBrowser is a fork of Mozilla Firefox (Gecko rendering engine) with the entire UI layer ripped out and rebuilt. The goal is not a reskin — it is a ground-up reimplementation of everything the user sees and touches, while keeping the battle-tested Gecko engine underneath.

**What changes:**
- Full UI rewrite in Rust using `iced` (GPU-accelerated, native)
- Dark terminal aesthetic — fits the Koded brand
- Tree-style tabs, command palette (Arc/Zen-style)
- Dev tools redesigned from scratch
- KodedDB replaces SQLite for all browser storage — history, bookmarks, session state, IndexedDB
- HTTP/K.0 wired into Gecko's network stack — `k0://` URLs resolve natively
- HTTPS fallback for legacy sites

**Status:** Not started. Requires Firefox source download (~250MB). Planned as Phase 3.

**Language:** Rust (UI layer), C++ (Gecko, untouched)

---

### 4. Web3 Layer — Native, Not Bolted On
*"The browser is the wallet."*

Web3 on the internet today requires browser extensions that users have to install, trust, and maintain. KodedBrowser makes the wallet core infrastructure — as native as the address bar.

**What it includes:**
- Solana adapter baked into the browser (no MetaMask, no Phantom extension)
- MPC key sharding via the Ika Network pattern (same approach used in Janus Protocol)
- KodedDB stores wallet state, transaction history, and signed sessions
- `k0://` URLs can resolve to on-chain content hosted on Arweave or IPFS — decentralized websites
- dApp permission model that is actually sane — not "approve everything on first load"

**Status:** Architecture designed. Not started in the browser context. Solana experience from Janus Protocol and Fog of War will feed directly into this.

---

### 5. koded-cli — The Console
*"One terminal for the whole stack."*

koded-cli is the unified command-line tool that ties all five layers together. Built on Cobra, it gives developers a single interface to operate every part of the Koded Stack.

**Current subcommands:**
```
koded db                  KodedDB interactive REPL (connected to live server)
koded protocol ping       Send HTTP/K.0 INIT probes, measure RTT
koded protocol bench      Latency benchmark against a K.0 server
koded protocol info       Print the full protocol specification
koded download            Download packages from manifest (resumable, parallel)
koded inspect             Inspect package manifests
koded version             Print CLI version
```

**Planned subcommands:**
```
koded wallet              Solana keypair, balance, sign, send
koded browser             Launch / control KodedBrowser headlessly
koded install             Install downloaded packages
```

**Status:** Core subcommands done. `koded db` fully wired to live KodedDB engine over TCP. Protocol tools functional against a running HTTP/K.0 server.

**Language:** Go

---

## Why Build This?

Chrome owns the browser. Google owns HTTP/3 (QUIC). AWS and Redis own database infrastructure. None of these were built with developers like Koded in mind — they were built by large teams at large companies with large compute budgets.

The Koded Stack is the answer to: what if you built the whole thing yourself? Not as a toy. As a real, working alternative — protocol, database, browser, wallet, CLI — that you understand completely because you wrote every line.

It is also a career document. Every layer is a technical proof that the person who built it understands systems deeply: how packets move, how data persists, how browsers render, how keys stay secure.

---

## Current Status

| Layer | Status | Language |
|---|---|---|
| HTTP/K.0 Protocol | ✅ Complete | Rust |
| KodedDB Storage Engine | ✅ Complete | Go |
| KodedDB Query Layer | ✅ Complete | Go |
| KodedDB TCP Server | ✅ Complete | Go |
| koded-cli core | ✅ Complete | Go |
| koded db REPL | ✅ Wired to engine | Go |
| koded protocol tools | ✅ Working | Go |
| SEMI column dot queries | 🔧 In progress | Go |
| KodedDB JOIN support | 📋 Planned | Go |
| koded wallet | 📋 Planned | Go |
| KodedBrowser UI | 📋 Planned | Rust |
| Web3 native layer | 📋 Planned | Rust / Go |

---

## Repositories

| Repo | Contents |
|---|---|
| `HTTP-K.0-Browser` | HTTP/K.0 protocol (Rust) |
| `Koded-Stack` / `kodeddb-core` | KodedDB engine, query layer, TCP server (Go) |
| `koded-cli` | Unified CLI tool (Go) |

---

*Built in Lagos. One person. From scratch.*