# koded-cli

```bash
koded
```

---

## 1. Product Vision

**koded** is a cross-platform CLI installer engine that provides **reliable, resumable, and controllable** downloads and installations for developer tools.

Unlike traditional package managers, **koded prioritizes control, transparency, and failure recovery** over abstraction.

> *“Install software without wasting bandwidth, time, or trust.”*

---

## 2. Problem Statement

Existing CLI package managers (Homebrew, apt, winget):

* Do not support **pause / resume**
* Waste bandwidth on failed installs
* Provide poor insight into download state
* Are opaque during failures
* Are tightly coupled to OS package systems

Developers want:

* Predictable installs
* Download control
* Resume after interruption
* Cross-platform consistency
* Minimal magic

---

## 3. Target Users

### Primary

* Software engineers
* Infra / backend developers
* Hackathon teams
* Developers in low / unstable bandwidth environments (important for your context)

### Secondary

* CI environments
* Internal tooling teams
* OSS contributors

---

## 4. Non-Goals (Very Important)

koded will **NOT**:

* Replace system package managers
* Resolve OS-level dependencies
* Install GUI applications
* Modify system-critical paths by default
* Act as a Linux distro manager

This keeps scope sane.

---

## 5. Core Features (MVP)

### 5.1 CLI Interface (Go + Cobra)

```bash
koded install <package>
koded download <package>
koded pause
koded resume
koded status
koded list
koded remove <package>
```

Flags:

```bash
--version
--dry-run
--force
--path
--arch
--os
```

---

### 5.2 Manifest-Driven Installation

All installs are defined by **manifests**.

#### Manifest responsibilities:

* Source URL(s)
* OS / arch mapping
* Archive type
* Install steps
* Binary paths
* Checksums

#### Example (conceptual):

```yaml
name: rust
version: 1.75.0

sources:
  darwin-arm64:
    url: https://static.rust-lang.org/...
    sha256: abc123
  linux-x64:
    url: https://static.rust-lang.org/...
    sha256: def456

install:
  type: archive
  bin:
    - rustc
    - cargo
```

---

### 5.3 Optimized Downloader (Core Differentiator)

#### Capabilities:

* HTTP Range requests
* Chunked downloads
* Concurrent goroutines per chunk
* Adaptive retry per chunk
* Backoff on failures
* Bandwidth-efficient resume

#### State persistence:

* Download progress
* Completed chunks
* Partial files
* Hash state

Stored locally:

```
~/.koded/state/
~/.koded/cache/
```

---

### 5.4 Pause / Resume

```bash
koded pause
koded resume
```

* Graceful interruption
* Resume without re-downloading completed chunks
* Survives terminal exit / system restart

---

### 5.5 Verification & Integrity

* SHA256 checksum validation
* Optional GPG support (future)
* Hash verification before install
* Abort on mismatch

---

### 5.6 Installation Targets

Default install location:

* macOS / Linux: `~/.local/bin`
* Windows: `%USERPROFILE%\.koded\bin`

User can override:

```bash
koded install rust --path /custom/bin
```

No sudo by default.

---

## 6. Platform Support (Phased)

### MVP

* macOS (arm64, x64)
* Linux (x64)

### Phase 2

* Windows (x64)

---

## 7. Architecture Overview

### High-Level Flow

```
CLI
 ↓
Manifest Resolver
 ↓
Downloader (chunked, resumable)
 ↓
Verifier
 ↓
Installer (OS-aware)
 ↓
State Update
```

---

### Directory Structure

```
cmd/
  root.go
  install.go
  download.go
  resume.go

core/
  manifest/
    loader.go
    resolver.go
  downloader/
    manager.go
    chunk.go
    resume.go
  installer/
    install.go
    linux.go
    darwin.go
    windows.go

state/
  state.go
  cache.go

utils/
  hash.go
  fs.go
  progress.go
```

---

## 8. State Management

### Persistent State (Local)

* Active downloads
* Installed packages
* Versions
* Cache metadata

Format:

* JSON or BoltDB (start with JSON)

---

## 9. UX / DX Principles

* Explicit > implicit
* Visible progress at all times
* Human-readable errors
* No silent failures
* Clear rollback on failure

Example error:

```
Download failed at chunk 7/16.
Saved progress. Run `koded resume` to continue.
```

---

## 10. Security Considerations

* No execution before verification
* No auto-sudo
* No arbitrary script execution in MVP
* All manifests reviewed (initially local)

---

## 11. Performance Requirements

* Resume must reuse ≥ 95% of previously downloaded data
* Concurrent downloads configurable
* Minimal memory footprint
* Fast startup (<100ms)

---

## 12. Metrics for Success

* Successful resume after network failure
* Zero re-download on resume
* Clear error messages
* Positive developer feedback
* Adoption in personal projects

---

## 13. Future Roadmap (Post-MVP)

* Plugin system
* Remote manifest registry
* Version pinning
* CI mode
* `koded doctor`
* GUI wrapper
* P2P mirror support
* Sandboxed installers

---

## 14. Why This Project Matters

This project demonstrates:

* Systems design
* Networking
* Concurrency
* Cross-platform engineering
* Infra maturity

This is **mid–senior level engineering**, not beginner stuff.