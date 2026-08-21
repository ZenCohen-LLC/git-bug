# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`git-bug` is a distributed issue tracker embedded in git. Issues, identities and their
full edit history are stored as native git objects under `refs/<namespace>/<id>` and
synchronised with ordinary `git push`/`git pull`. No server, no database, no separate
storage — the git object store *is* the database.

The interesting part is not the bug tracker. It is `entity/dag`: a **reusable,
generic, operation-based CRDT framework** for storing any versioned, signed,
mergeable entity in git. `bug` and `identity` are two instances of it. Read
`entity/dag/example_test.go` to see the framework used standalone.

> **Fork note.** This is the `zencohen-llc/git-bug` fork. Feature work goes on
> `claude/<topic>` branches, never directly on `main`. Keep divergence from upstream
> minimal and deliberate: prefer additive new entity namespaces and new operation types
> (the format is explicitly designed for this — see *Additive extensibility* below) over
> edits to existing core types.
>
> Fork-local planning and findings live in [`fork-notes/`](fork-notes/) — start there for
> why this fork exists and what is currently in flight. Note in particular
> [`fork-notes/2026-08-20_spec-entity-id-defect.md`](fork-notes/2026-08-20_spec-entity-id-defect.md):
> `doc/spec/dag-entity.md` §5.2 and §7 state the entity-ID derivation **incorrectly**, so do
> not implement an ID from that section without reading the finding first.

## Build, test, lint

The canonical dev environment is **nix** (`nix develop`, auto-activated via `direnv`).
It pins Go, golangci-lint, codespell, pinact, node and treefmt. If nix is unavailable,
the fallbacks below work — but formatting/lint results may drift from CI.

| Task | With nix | Without nix |
|---|---|---|
| Build binary | `make build` | `make build` (needs pnpm + go) |
| Full check suite | `nix flake check` | see individual commands below |
| Format everything | `nix fmt` (scope: `nix fmt <path>`) | `gofmt -w .` (Go only) |
| Lint Go | `nix flake check` | `golangci-lint run` |
| Spellcheck | `nix flake check` | `codespell --check-hidden` |
| Test | `make test` | `go test ./...` |

CI (`.github/workflows/build-and-test.yml`) runs exactly `make` then `make test`, on
Go 1.25.x + 1.26.x across Linux/macOS/Windows, plus a separate node job for `webui/`.

### The `webui/dist` build trap

`webui/handler.go` embeds the compiled frontend with `//go:embed all:dist`. **If
`webui/dist/` does not exist, every Go build fails** — including packages you did not
touch — with:

```
webui/handler.go:14:12: pattern all:dist: no matching files found
```

`commands/webui.go` is the only importer, but it pulls the failure into `./...`.

- **Real build:** `make build` → runs `cd webui && pnpm install && pnpm run build`,
  then `go generate`, then `go build`.
- **Go-only work** (no frontend change), to avoid a multi-minute `pnpm install`:
  ```bash
  mkdir -p webui/dist && printf '<!doctype html>\n' > webui/dist/index.html
  ```
  `webui/dist` is gitignored, so this leaves the tree clean. Do **not** commit it, and
  do not ship a binary built this way — the embedded UI would be the stub.

### Golden-file tests

CLI output is pinned by golden files in `commands/*/testdata/`. Regenerate after any
intentional change to command output:

```bash
go test ./commands/... -Update
```

The flag is **`-Update`** (capital U — it is declared as `flag.Bool("Update", …)` in
`commands/cmdtest/golden.go`). `CONTRIBUTING.md` documents it lowercase as `-update`;
that is wrong and fails with a flag-parse error. Always review the regenerated diff —
`-Update` will happily bless a regression.

### Code generation

Three `go:generate` directives, all run by `make build`; re-run `go generate` and commit
the result whenever you touch their inputs:

| Directive | Location | Regenerates |
|---|---|---|
| `go tool gqlgen generate` | `api/graphql/handler.go` | GraphQL resolvers/models from `*.graphql` |
| `go run doc/generate.go` | `main.go` | `doc/man/*.1` and `doc/md/*.md` from cobra commands |
| `go run misc/completion/generate.go` | `main.go` | shell completions in `misc/completion/` |

Generated trees (`doc/man/`, `doc/md/`, `misc/completion/`, `doc/spec/`) are excluded
from `treefmt` — never hand-edit them; change the source and regenerate.

## Architecture

Strictly layered. **Each layer talks only to the one directly below it.**

```
webui (React) ─┐
commands · bridge · termui · api/graphql
               ↓
             cache            ← BugCache/BugExcerpt, IdentityCache/IdentityExcerpt
               ↓
   entities/bug · entities/identity   ← concrete entities
               ↓
          entity/dag           ← generic CRDT framework
               ↓
          repository           ← git access (go-git), config, keyring, bleve index
```

The rule that bites: **`commands` must never reach into `entities/bug` or `repository`
to load or mutate data — it must go through `cache`.** The cache guarantees a single
in-memory instance per entity; bypassing it means concurrent copies and silent lost
writes. Using the *types* from lower packages is fine; retrieving and changing data is
not.

### The write path: operations, not state

Current state is **never stored**. An entity is an append-only series of `Operation`s;
the `Snapshot` is recomputed by replaying them (`Compile()`). This is what makes
distributed merges conflict-free — merging is set union over operations.

Storage shape, per edit session:

- `Operation`s → serialised as a JSON array into an `OperationPack`
- `OperationPack` → a git **blob** at tree path `ops`
- attached media → blobs under `media/`
- Lamport clocks → encoded **in tree entry names** (`create-clock-14`, `edit-clock-137`),
  each pointing at the empty blob so no network transfer is needed
- the tree → a git **commit**, chained into a DAG, published at `refs/<namespace>/<id>`

Namespaces: bugs are `refs/bugs/<id>`, identities `refs/identities/<id>`, remotes
mirror under `refs/remotes/<remote>/<namespace>/<id>`.

### Invariants you must not break

These are normative in `doc/spec/` and load-bearing for correctness. Changing any of
them is a format break requiring a version bump and migration tooling:

1. **IDs are content hashes.** An operation's ID is `hash(json(op))`; an entity's ID is
   the hash of its first operation as serialised on disk. Every operation carries a
   random `nonce` for entropy. Change the serialisation and you change every ID.
2. **Lamport clocks must respect the DAG.** A parent commit may not have a clock
   higher than or equal to its child. Violating commits are *refused*, not repaired.
3. **Ordering is deterministic and total:** Lamport clock first; ties (genuine
   concurrent edits) broken by lexicographic `OperationPack` ID. Two replicas with the
   same operation set must compile byte-identical snapshots.
4. **Unknown operations degrade, never fail.** A reader hitting an unimplemented
   operation type skips it and produces a *degraded-but-valid* snapshot
   (`entity/dag/op_unknown.go`). Never make an unknown type an error.
5. **Check the format version** in the commit tree before decoding (bug entity is
   version 4).
6. **Identity refs are fast-forward-only** — unlike DAG entities, they do not merge.

### Additive extensibility

The format is designed so new operation types and new entity namespaces can be added
without touching existing clients — old clients ignore what they don't know (invariant
4). This is the preferred way to extend git-bug, and the right shape for fork work.
Operation type integers are scoped per entity and namespaces are plain strings, so
independently chosen values can collide; upstream asks that broadly-used extensions be
coordinated on the issue tracker.

### Where the spec lives

`doc/spec/` is the **normative** data-format specification, with conformance
requirements for readers and writers, and JSON conformance fixtures in
`doc/spec/testdata/`. Treat it as the source of truth over any code comment:

- `doc/spec/dag-entity.md` — the base layer (refs, tree layout, pack JSON, clocks,
  ordering, the 5 merge scenarios, OpenPGP signing)
- `doc/spec/identity.md` — identity format and key lookup
- `doc/spec/bug.md` — the `bug` entity: every operation type with field tables
- `doc/design/data-model.md` — the *rationale* (why operations, why Lamport clocks)
- `doc/design/architecture.md` — the layer map above

### cache

More than a speed layer — it holds correctness guarantees:

- exactly one in-memory instance per bug/identity (prevents lost updates)
- `*Excerpt` types: small pre-digested records persisted to disk so the whole set can be
  queried/sorted/filtered without loading every entity
- a **lock file** in the repo's local storage, so two git-bug processes don't race
  (ordinary git commands are unaffected)
- `subcache.go` is the generic machinery; `bug_subcache.go` / `identity_subcache.go`
  specialise it. New entity types plug in here.

Bleve (`repository/index_bleve.go`) backs full-text search.

### commands

Cobra, with an explicit `execenv.Env` (`Ctx`, `Repo`, `Backend *cache.RepoCache`, and
injectable `In`/`Out`/`Err`). That injection is what makes golden-file testing possible —
new commands should take their IO from `Env`, never touch `os.Stdout` directly.
Structure mirrors the CLI: `commands/bug/`, `commands/user/`, `commands/bridge/`.
Conventions in `doc/design/cli-convention.md`.

### bridge

Import/export against GitHub, GitLab, Jira, Launchpad. `bridge/core` holds the shared
machinery (auth, config, rate limiting); each provider implements importer/exporter.
Bridges map external IDs into operation **metadata** (`SetMetadataOperation`) rather
than into entity fields — that is how round-tripping stays lossless.

Bridge tests hit live APIs and need credentials (`GITHUB_TOKEN`, `GITLAB_API_TOKEN`,
…); they are expected to fail locally without them.

**Verified baseline** (2026-08-20, Go 1.25.0, stub `webui/dist`): `go test ./...` →
24 packages `ok`, 1 package `FAIL` — `bridge/github`, on `TestValidateUsername` and
`TestGithubImporter`. Both fail because the GitHub API lookup returns empty without
network/credentials, not because of a code defect. Treat any *other* failure as real.

### Query language

`query/` parses the search syntax used by CLI and webui (`status:open sort:edit`,
`author:"René Descartes"`). Case-insensitive; ID arguments accept any unambiguous
prefix, like git hashes. Documented in `doc/usage/query-language.md`.

## Conventions

- **Commit messages are the source of truth** — see `.gitmessage` and
  `doc/contrib/commit-message-guidelines.md`. `CHANGELOG.md` is generated from them via
  git-cliff (`cliff.toml`), so a sloppy subject line becomes a sloppy release note.
- PRs are **squashed**; keep one PR to a singular unit of change.
- `pinact` pins GitHub Action SHAs — run it after adding any action.
- Go version floor is declared in `go.mod` (currently 1.25.0); CI also tests 1.26.x.
