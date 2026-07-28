# symroom

`symroom` is the shared, verifiable work record of a project: who belongs to
it, what happened, and what was approved. It is a signed, append-only journal
of events (notes, decisions, membership changes, run lifecycle) — not a chat
system, not a policy engine, and not a Git host. See [AGENTS.md](AGENTS.md)
for the full boundary and convention rules.

## Status

Early scaffold. Implemented so far:

- Ed25519 identity creation/storage/resolution (`symroom identity`)
- Canonical event serialization and Ed25519 signing (`internal/event`)
- Room initialization: writes `room.toml`, a self-signed `room.created` event,
  and `.symroom/local.toml` (`symroom init`)
- Journal events as JSONL, one append-only file per author under `journal/`
- Membership/role state derived by replaying journal events
  (`internal/members`), with a fixed role matrix (owner/member/agent/observer)
- `symroom run` lifecycle (request, approve, deny, start, finish, fail, cancel, wait) and generic adapters
- `symroom checkpoint` (request, resolve, wait)
- `symroom brain-profile` — emit symbrain profile for agent members
- See [docs/approval-contract.md](docs/approval-contract.md) for the frozen approval backend contract.

## Installation

Requires Go 1.26+.

```sh
git clone <this repo>
cd symaira-room
make build
```

This builds `bin/symroom`. Run `make test` to run the test suite.

## Usage

```sh
# Create a signed Ed25519 identity
symroom identity create alice

# Initialize a room in the current directory, owned by that identity
symroom init . --identity alice --name "My Project"

# Post a signed journal note
symroom note "kicked off the project" --identity alice

# Record a signed decision, optionally referencing other event IDs
symroom decide "adopted the new API design" --refs ev_abc123 --identity alice
```

Both `note` and `decide` accept `--json` to print the full signed event
instead of just its ID.

Set a default identity in the config so `--identity` can be omitted:

```json
{
  "default_identity": "alice"
}
```

## Data layout

- `room.toml` — room ID, creation timestamp, root public key, root event ID
- `journal/<member_id>.jsonl` — one append-only, signed event log per author
- `.symroom/local.toml` — local, gitignored machine config (default identity)
