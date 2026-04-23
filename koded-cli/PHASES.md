# 🧭 koded — Phased Execution Plan

Each phase:

* Has a **clear goal**
* Produces **working software**
* Can be tested manually
* Builds intuition incrementally

You should not move to the next phase until the current one feels solid.

---

## Phase 0 — Ground Rules & Setup (1–2 days) ✅

### Goal

Prepare a sane foundation without writing “real logic” yet.

### Tasks

* Initialize Go module
* Add Cobra CLI scaffold
* Decide binary name (`koded`)
* Define directory structure
* Add `.gitignore`
* Add basic `koded version` command
* Define config + state directories (`~/.koded/`)

### Exit criteria

```bash
koded version
koded help
```

Works cleanly.

---

## Phase 1 — Manifest System (2–3 days) ✅

### Goal

koded knows **what** it is installing before doing anything.

### Tasks

* Define manifest schema (YAML or JSON)
* Implement manifest loader
* Validate OS / arch resolution
* Add error handling for unsupported platforms
* Add `koded inspect <pkg>` command

### Deliverable

```bash
koded inspect rust
```

Outputs:

* Version
* Download URL
* Size (if available)
* Install target

### Exit criteria

* Incorrect platform fails loudly
* No downloading yet

---

## Phase 2 — Basic Downloader (No Resume) (2–3 days)

### Goal

Get a **reliable single-file downloader** working.

### Tasks

* HTTP download logic
* Progress display
* Save to cache directory
* Handle retries
* Handle network failure gracefully
* Add `koded download <pkg>`

### Exit criteria

* Large file downloads completely
* Clean failure message on interruption
* File integrity preserved

---

## Phase 3 — Chunked Download Engine (Core Phase) (4–6 days)

### Goal

Build the **heart of koded**.

### Tasks

* Split downloads using HTTP Range
* Concurrent chunk downloading
* Chunk file persistence
* Progress aggregation
* Chunk-level retry logic

### Exit criteria

* Downloads complete faster than Phase 2
* Chunks saved independently
* No data corruption

---

## Phase 4 — Persistent State & Resume (4–5 days)

### Goal

Make downloads **pauseable and resumable**.

### Tasks

* Design download state format
* Persist completed chunks
* Load state on restart
* Skip completed chunks
* Implement `koded pause`
* Implement `koded resume`

### Exit criteria

* Interrupting the process does not lose progress
* Resume continues exactly where it stopped
* No duplicate downloads

---

## Phase 5 — Verification & Assembly (2–3 days)

### Goal

Ensure **correctness before install**.

### Tasks

* SHA256 verification
* Chunk re-verification
* Assemble chunks into final artifact
* Cleanup corrupted chunks
* Fail safely on hash mismatch

### Exit criteria

* Corrupted downloads are detected
* Resume fixes corruption automatically

---

## Phase 6 — Installer (User-level) (3–4 days)

### Goal

Make tools usable.

### Tasks

* Archive extraction (`zip`, `tar.gz`)
* Binary detection
* Copy to install directory
* Handle permissions
* Implement `koded install <pkg>`

### Exit criteria

```bash
koded install rg
rg --version
```

Works.

---

## Phase 7 — UX, Status & Polish (2–3 days)

### Goal

Turn it from a hack into a **tool**.

### Tasks

* `koded status`
* Better progress UI
* Friendly error messages
* Dry-run mode
* Cache cleanup command

### Exit criteria

* Clear messaging
* No confusion about state

---

## Phase 8 — Platform Expansion (Optional)

### Goal

Prove portability.

### Tasks

* Windows installer logic
* Path handling
* Permission handling
* Platform-specific edge cases

---

# 📌 Rules to Follow (Important)

1. **Do not jump phases**
2. **Do not optimize early**
3. **Manual testing > benchmarks**
4. **Log everything**
5. **If resume works, you win**

---