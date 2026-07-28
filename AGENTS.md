# AGENTS.md

## Product Boundaries & Architecture

### What symroom is
`symroom` is the shared, verifiable work record of a project in which humans and agents are participants — who belongs to it, what happened, and what was approved.

### What symroom is not
`symroom` is **not**:
- A chat/channel system
- A second document database
- A Git host
- A policy engine
- A relay
- A system with a GUI of its own

---

## Boundaries Against Sibling Products

### Room ↔ Guard Boundary (Do Not Weaken)

| | Room | Guard |
|---|---|---|
| Question | Is this *operation* approved, and where is the record that it happened? | May this concrete *call* happen right now? |
| Unit | run, checkpoint, decision | single tool call |
| Position | beside the data path (bookkeeping) | in the data path (proxy) |
| Risk classification | none | core competence |
| Record | signed room journal | hash-chain call audit |

> Feature requests for per-call ask/deny, risk classes or schema pinning belong in `symaira-guard` and must be redirected there.

### Room ↔ Brain Boundary
Room-scoped agent rights are expressed as a `symbrain` profile that `symroom` *emits*; `symroom` never evaluates exposure policy and never imports `symbrain`.

### Room ↔ Desk Boundary
`symdesk` owns content, `symroom` owns references (relative path + sha256). `symroom` never writes into the vault.

---

## Conventions & Rules

- **Standalone-first**: No compile-time imports of sibling repos (`symaira-guard`, `symaira-brain`, `symaira-desk`). Runtime detection is performed via `exec.LookPath` with graceful degradation.
- **Zero stdio pollution**: When running in `mcp` mode, stdout is strictly reserved for JSON-RPC 2.0 protocol traffic. All log output goes to stderr or file sinks.
- **JSON Format**: `snake_case` field names for all serialized JSON types and event structures.
- **XDG Specification**: Configuration, data, and cache paths follow XDG specifications (`~/.config/symroom`, `~/.local/share/symroom`, `~/.cache/symroom`).
- **Environment Variables**: Environment overrides use the `SYMROOM_*` prefix.
- **Commits**: Follow Conventional Commits conventions.
- **Open Source Core**: No Pro/billing code in this repository.
