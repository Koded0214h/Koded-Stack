# Security Policy

## Scope

This policy covers the following components of the Koded Stack:

| Component | Security Surface |
|---|---|
| HTTP/K.0 protocol | Wire encryption, authentication, session tickets |
| KodedDB | Data integrity, WAL crash safety, TCP server |
| koded-cli | Download verification, manifest trust, file integrity |

---

## HTTP/K.0 Security Model

### Per-Packet Encryption

Every UDP packet payload is independently sealed with **ChaCha20-Poly1305** using the packet number as a nonce. This is stateless — packets can be decrypted out of order. A packet with modified header flags fails authentication even if the payload is intact, because the Additional Authenticated Data (AAD) covers the connection ID, flags, and frame type.

Nonce: `[0x00 × 4 | packet_num (8B big-endian)]`  
AAD: `[conn_id (8B) | flags (1B) | frame_type (1B)]`

### TLS 1.3

The handshake uses **rustls** — a memory-safe TLS implementation. TLS 1.3 is enforced; TLS 1.2 is disabled in the default configuration. Key material for per-packet encryption is derived from TLS exporter material after the handshake completes.

### 0-RTT Replay Risk

0-RTT session resumption carries inherent replay risk. The implementation mitigates this by restricting 0-RTT to **idempotent methods only** (`GET`, `HEAD`, `OPTIONS`). `POST`, `PUT`, and `DELETE` wait for the 1-RTT confirmation before being sent.

Session tickets expire after 24 hours (`SESSION_LIFETIME_SECS`).

### Connection Migration

PATH_CHALLENGE/RESPONSE validates new network paths before migrating a connection. A challenge is a random 64-bit value — the connection only migrates when a matching response is received from the new address.

---

## KodedDB Security Model

KodedDB is designed to run as a local or trusted-network service. It has **no authentication layer** in the current implementation. Do not expose port `6380` to untrusted networks.

**WAL integrity:** Each WAL entry carries an XOR checksum. Replaying a truncated or corrupted WAL will detect mismatches and stop at the last consistent entry — preventing partial state from being applied after a crash.

**Page checksums:** Each 4KB page stores a `uint32` checksum in its header. The pager verifies this on every read from disk.

---

## koded-cli Download Security

Downloads are verified with **SHA256** before the file is used. If the checksum does not match, the download is aborted and the file is not assembled. The state file is cleaned up.

Manifests are currently local files only — no remote manifest registry exists yet. All manifests in `koded-cli/manifests/` are reviewed before inclusion.

`koded` does **not** execute downloaded files automatically. Installation steps are defined in the manifest's `install` block and require explicit invocation. No auto-sudo.

---

## Reporting a Vulnerability

If you find a security issue — particularly in the HTTP/K.0 encryption layer, the WAL integrity logic, or the download verification path — please report it privately before disclosing publicly.

**Contact:** Open a GitHub issue marked `[SECURITY]` with limited detail, and we will coordinate a private channel for the full report.

Please include:
- Which component is affected
- A description of the vulnerability
- Steps to reproduce or a proof-of-concept
- Your assessment of severity and exploitability

We aim to respond within 72 hours and provide a fix or mitigation timeline within 14 days of confirmation.

---

## Known Limitations

- KodedDB has no authentication — intended for local/trusted use only
- The hybrid TCP/UDP transport is experimental and has not been security-reviewed
- GPG signature verification for manifests is planned but not implemented
- The 0-RTT window is restricted to safe methods, but ticket storage is in-memory only — tickets do not survive a server restart
