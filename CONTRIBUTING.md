# Contributing to the Koded Stack

Thanks for your interest. This is a systems engineering project — protocol, database, browser, CLI — built with strong opinions about correctness and control. Contributions should respect that philosophy.

---

## Before You Start

Read [LOOKING_INTO.md](LOOKING_INTO.md) and the relevant [docs/](docs/) before opening a PR. Understand what each layer is trying to be and why before suggesting changes to it.

---

## What We Welcome

- Bug reports with reproduction steps
- Fixes for correctness issues (protocol correctness, data integrity, wrong behavior)
- Performance improvements backed by measurements
- Documentation improvements — clearer explanations, better examples
- New package manifests (`koded-cli/manifests/`)
- Tests for untested paths

## What We Don't Welcome (Right Now)

- Large refactors without prior discussion
- Abstractions that don't earn their weight
- Dependencies that solve problems we've intentionally solved ourselves
- Changes to the HTTP/K.0 wire format without an RFC-style justification
- "make it more idiomatic" PRs that don't fix a real problem

---

## Development Setup

### HTTP/K.0 (Rust)

```bash
cd HTTP-K.0-Browser/protocol
cargo build
cargo test
```

All major modules have unit tests. Run `cargo test -- --nocapture` for verbose output.

### KodedDB (Go)

```bash
cd kodeddb-core
go test ./...
```

Tests live alongside the packages they cover (`engine_test.go`, `storage_test.go`, `query_test.go`, `server_test.go`).

### koded-cli (Go)

```bash
cd koded-cli
go build -o koded .
./koded --help
```

There are no automated tests for the CLI yet. Test manually against a running KodedDB server and HTTP/K.0 server.

---

## Code Style

### Rust

- Standard `rustfmt` formatting — run `cargo fmt` before committing
- `clippy` warnings should be clean — run `cargo clippy`
- No `unwrap()` in non-test code without a comment explaining why it cannot fail
- Async code stays in `tokio` — no mixing runtimes

### Go

- Standard `gofmt` formatting — enforced by the toolchain
- Errors returned, not panicked — `fmt.Errorf` wrapping with context
- Keep packages small and focused — `internal/` for packages not meant to be imported externally
- No global mutable state outside of explicitly documented server singletons

### Markdown

- One blank line between sections
- Code blocks always specify a language
- Tables for structured comparisons, not bullet walls

---

## Commit Messages

```
<type>(<scope>): <short description>

<optional body — explain WHY, not what>
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`

Scopes: `protocol`, `db`, `cli`, `storage`, `query`, `api`, `fec`, `ack`, `congestion`, `tls`

Examples:
```
feat(protocol): add PATH_CHALLENGE retransmit on timeout
fix(storage): prevent WAL sequence number wrap on long-running servers
docs(cli): add manifest authoring guide
perf(query): avoid allocating AST node for WHERE-less SELECT
```

---

## Pull Request Process

1. Open an issue first for anything non-trivial — discuss the approach before writing code
2. Keep PRs focused — one concern per PR
3. Update relevant docs in `docs/` if your change affects behavior or the API
4. All existing tests must pass
5. New behavior must have tests

---

## Adding a Package Manifest

The simplest way to contribute is adding a manifest for a tool not yet in `koded-cli/manifests/`.

See [docs/manifests.md](docs/manifests.md) for the format. At minimum a manifest needs:
- `name`, `version`, `size`
- At least one source entry with a real download URL and SHA256
- A valid `install` block

Test your manifest:
```bash
./koded inspect <yourpkg>
./koded download <yourpkg> --dry-run
```

---

## Questions

Open an issue. Label it `question`.
