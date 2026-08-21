# Plan — Agent-native git-bug as a trusted core

**Date:** 2026-08-20 · **Session:** `agent-native-git-bug-xjz6vs` · **Status:** Wave 1a complete
**Branch (all six repos):** `claude/agent-native-git-bug-xjz6vs`

> **Relocated 2026-08-21.** This plan and its sibling finding were first committed to the
> `principles` repo (`provenance/plans/` and `05_findings/`). That was the wrong home: those
> lanes are for synthesis across Zencohen's Principles / Working Genius / ZenCon threads, and
> `principles/CLAUDE.md` routes engagement-specific content to the engagement's own repo. Both
> now live here, beside the code they concern. Paths below are repository-root relative.

**Shipped:**
- `git-bug@8ef9e97` — `CLAUDE.md`, verified against a real build and test run.
- `git-bug@873d7b1` — `doc/spec/spec_test.go`, the conformance harness. **Wave 1a.**

**Matt's gate, 2026-08-20:**
1. Wave 1 scope → **1a now, re-gate 1b** once we see whether spec and implementation agree.
2. Pre-work → **verdicts confirmed, but run ontomics anyway** as a check on the reasoning.
3. Upstream → **fork only for now**; address upstream later. *"If we can, let's try to make
   few or no changes to the code itself other than blocker bugfixes."*

**Interpretation of (3), applied throughout:** a new `_test.go` file and filling fixture
placeholders are additive and change no production code, so they are in scope. Editing
normative spec prose is not, and was not done — see §9. Wave 3 (`--format json` on write
commands) *is* a production-code change and is therefore now **blocked** on an explicit
call, not merely sequenced.

Claim tags: **[V]** verified this session (command + output) · **[A]** asserted from documentation · **[C]** conjecture.

---

## 1. The reframe

git-bug is not a bug tracker that we would make agent-native. It is a **generic
operation-based CRDT framework** (`entity/dag`) that ships with a bug tracker as its first
instance. **[A: `doc/design/data-model.md`, `doc/spec/dag-entity.md`]**

That substrate already has the properties a trusted core needs:

| Trusted-core property | git-bug mechanism |
|---|---|
| Provable read | IDs are content hashes: `id = hash(json(op))` |
| Provable authorship | OpenPGP-signed commits; identity entities with key rotation |
| No lost updates | Append-only operations; merge = set union |
| Deterministic state | Total order: Lamport clock, ties broken by pack ID → byte-identical snapshots |
| Inspectable | Plain git objects; ordinary git tooling reads it |
| Extensible without breaking peers | Unknown op types degrade to *degraded-but-valid* |

**So we do not need to build a trusted core. We need to (i) prove it, and (ii) expose it.**
That reframe is what the waves below follow.

---

## 2. Constraints (assigned, budgeted, managed)

| Constraint | Budget | Why |
|---|---|---|
| **Matt's attention** | One batched gate per wave | Scarcest input. Spend only on one-way doors + ROI verification. |
| **My context** | Orchestration context stays lean; detail pushed to files and subagents | Poisoning/distraction risk; separate orchestrate / act / verify |
| **Wall clock** | Commit + push every wave | Container is ephemeral and reclaimed on inactivity **[V]** |
| **Divergence** | Additive only — new files, new op types, new namespaces; no edits to core types | Merge cost compounds; the format is *designed* for additive extension **[A]** |
| **Trust** | Every claim carries [V]/[A]/[C] | "prove inputs and outputs from the trusted core" |
| **Reversibility** | Branch per wave, no force-push, worktrees for parallel work | Hard rule |
| **Model spend** | Cerebras for verifiable bulk work only; never for judgement | See §5 |

---

## 3. Evidence (all [V] — commands run this session)

| # | Finding | How verified |
|---|---|---|
| E1 | `go build ./...` fails repo-wide without `webui/dist` (`//go:embed all:dist` in `webui/handler.go:14`). A stub `webui/dist/index.html` unblocks all Go work; `dist` is gitignored so the tree stays clean. | Reproduced error; applied stub; build passed |
| E2 | Test baseline: **24 packages ok, 1 fail** — `bridge/github` (`TestValidateUsername`, `TestGithubImporter`), both from absent GitHub API access, not code defects. | `go test ./...`; isolated the failure |
| E3 | Golden-file flag is **`-Update`** (capital U). `CONTRIBUTING.md:249` documents `-update`, which fails to parse. | Ran both; `-update` FAIL, `-Update` ok |
| E4 | **Read path is already agent-native**: `--format json` emits full 64-char IDs + Lamport clocks; "Building cache…" correctly goes to stderr; pipes cleanly to `jq` on a cold cache. | Cold-cache run piped to `jq`, exit 0 |
| E5 | **Write path is not**: `bug new --format json` → `Error: unknown flag: --format`. `bug new` prints `3f65c24 created` — prose containing a **truncated 7-char** ID. An agent must string-parse, then re-query for the full ID. | Ran in a scratch repo |
| E6 | `--format json` exists on ~3 of ~38 commands (`bug`, `bug show`, `user`). | grep + `doc/md/` command inventory |
| E7 | **The spec conformance vectors are never executed.** `go test ./doc/spec/...` → `matched no packages`. `doc/` contains only `generate.go`. | Ran it |
| E8 | The two ID-derivation vectors — the only ones that would actually *verify* hashing — are placeholders: `<run: go test ./doc/spec/... -run TestVectors to generate>`, pointing at a test that was never written. The empty-input SHA-256 sanity vector does match. | Parsed all three fixture files |
| E9 | `spec-skills` has exactly **one** validator (`spec-skills:tools/verify_doc_freshness.py`) plus five grep-shaped CI gates. All **structural**; none check semantic consistency or validate a target against its spec. | `ls tools/`, read `spec-skills:.github/workflows/docs.yml` |
| E10 | Cerebras exposes **only** `gpt-oss-120b` and `gemma-4-31b`. **No embeddings endpoint.** | `GET /v1/models` |
| E11 | git-bug's doc corpus is **17,744 words (~26k tokens)** — fits in one context window. | `wc -w` over `doc/**/*.md` |

---

## 4. Pre-work verdicts — "consider why?"

### (a) ontomics map of git-bug — **DEFER** (confidence 0.85)

ontomics *infers* domain vocabulary. git-bug's vocabulary is **already formally specified**
with per-operation field tables in `doc/spec/bug.md` **[A]**. We would spend a heavy Rust +
candle build and ~GB-scale HuggingFace model downloads (reachability through the proxy
unverified **[C]**) to re-derive, less precisely, what humans already wrote down better.

*Revisit when:* we have enough of **our own** new code that naming-drift enforcement against
git-bug's established vocabulary becomes a real question. That is a Wave 4+ concern.

### (b) LightRAG over git-bug docs — **SKIP** (confidence 0.9)

RAG exists to make a corpus that *doesn't fit* addressable. This one fits **[E11]**.
Building it would insert a lossy retrieval layer in front of text we can read losslessly,
and **[E10]** Cerebras has no embeddings endpoint, so it would drag in a local embedding
dependency for negative value.

*Revisit when:* we need git-bug's **issue/PR history** — that is where design rationale
actually lives, it is genuinely large, and it is genuinely unaddressable. Trigger: a real
unanswered "why was it built this way" question, not a schedule.

### (c) spec-suite-init on git-bug — **RUN A SCOPED SUBSET** (confidence 0.8)

| Artifact | Call | Why |
|---|---|---|
| `SPEC.md` | **Do not generate** | `doc/spec/` is already normative with conformance requirements. A generated SPEC.md is a *second source of truth* — straight against "limit divergence." **Adopt `doc/spec/` by reference.** |
| `VISION`, `CORE_BELIEFS`, `STRATEGY`, `GOALS` | **Generate** | These do not exist and are about **our** bet, not git-bug's code. They are what gate every later wave. |
| `TECH_TRACKER` | **Generate — the sleeper** | git-bug's conformance requirements *are* invariants. TECH_TRACKER's `by-construction` vs `checked` tagging is precisely "correct by construction" + "Decidables over Booleans." |

---

## 5. The keystone: what a trusted core actually requires

**[E7][E8]** git-bug ships conformance vectors that nothing runs, with the verifying values
left blank. **[E9]** spec-skills' validators are structural only. The two gaps are the same
gap, and they solve each other.

The distinction that must drive the design:

> A test that generates its expected values from the implementation proves
> **self-consistency** — that behaviour has not changed. It does **not** prove conformance.
> Conformance is two *independent* parsers agreeing.

Conflating those two would manufacture exactly the false assurance this project exists to
avoid. So Wave 1 splits:

- **1a — Regression pin.** Wire the vectors into `doc/spec/spec_test.go`; generate the two
  missing expected values from the Go reference implementation; freeze them. Cheap, real,
  and *honest about what it proves*: drift detection.
- **1b — Real conformance.** An **independent decoder** that reads git objects from the
  *spec text* and cross-checks the Go implementation. Agreement is the proof. This is
  "parse, don't validate" applied literally, and it is the actual trusted core.

**Where Cerebras earns its place.** 1b is high-volume, mechanically checkable work — the one
shape where a weaker model is safe, because the harness adjudicates. Use `gpt-oss-120b` as an
**untrusted generator** (propose vectors, propose decoder cases) with the test harness as
**trusted verifier**. Untrusted generator + trusted verifier is the same architecture as the
trusted core itself, applied to model selection. Never for judgement calls.

---

## 6. Waves

| Wave | Deliverable | Gate | Reversibility |
|---|---|---|---|
| **0 — done** | `CLAUDE.md` for git-bug, verified against a real build/test run | — | Committed `8ef9e97`, pushed |
| **1 — keystone** | `doc/spec/spec_test.go` executing the vectors; two placeholders filled; 1a, or 1a+1b | **Scope: 1a or 1a+1b** | New file only; additive |
| **2** | Upper doc stack for our fork (VISION/CORE_BELIEFS/STRATEGY/GOALS/TECH_TRACKER); SPEC adopted by reference | **Matt owns VISION + STRATEGY** — normative, chosen not inferred | New files in our repo |
| **3** | Close the write-path parity gap **[E5][E6]**: `--format json` + full IDs on write commands | Design review before implementation | Additive flag; backwards-compatible |
| **4** | `/agent-native-review` against git-bug (dogfood); decide MCP surface vs CLI-JSON; consider an entity namespace for agent state | Gated on Wave 2 | Deferred by design |

**Why Wave 1 first — Value of Information.** Both outcomes are decision-relevant, which is
what makes it the highest-VOI action available:

- vectors **pass** → we have a working proof harness, cheaply, and the substrate is sound.
- vectors **fail** → we have found spec/implementation drift in the substrate *before*
  building on it. That is worth more than a pass.

There is no outcome where the effort is wasted. Nothing else in the backlog has that shape.

---

## 6b. Wave 1a result — the VOI bet paid out

The harness found a defect in the base-layer contract, which is the outcome the plan named as
the more valuable of the two. Full write-up with evidence:
[`2026-08-20_spec-entity-id-defect.md`](2026-08-20_spec-entity-id-defect.md).

**In one line:** `doc/spec/dag-entity.md` §5.2 and §7 say an entity's ID is the hash of its
whole `ops` blob. It is actually the hash of the **first operation** inside that blob. Measured
on a live repository: the spec's rule yields `436000bd…`, the real ref is `3f65c242…`. A
third-party reader built from the normative spec would find no entity at all.

Three other sources (the code comment at `entity/dag/entity.go:361`, `doc/design/data-model.md`,
and `doc/spec/bug.md`) all state the rule correctly. `dag-entity.md` is the lone outlier —
and it is the normative one.

**Why it survived:** the spec has a §12 Conformance section, a §13 Test vectors section, and
fixtures whose comment says *"Run 'go test ./doc/spec/... to verify"* — but the package never
existed, so `go test ./doc/spec/...` returned `matched no packages`. The apparatus of
verification was complete; only the verification was absent.

**Harness state:** 25 packages ok, 1 pre-existing network failure (`bridge/github`). Two
placeholder vectors filled through `entity.DeriveId` and frozen; two checks anchored on
externally verifiable constants so the suite is not self-referential.

**Consequence for 1b.** 1b was proposed as "an independent decoder, because two parsers
agreeing is the proof." Wave 1a raises its value: we now know the document a second
implementer would read is wrong in its most load-bearing rule, so the disagreement 1b is
designed to surface is not hypothetical. **Recommend proceeding to 1b** (confidence 0.85, up
from 0.75).

## 6c. Revised pre-work evidence

Two corrections to §4, both from actually running the thing rather than reasoning about it —
which is what Matt's "run ontomics anyway" was for:

- **My cost estimate was wrong.** `cargo build --release` took **4m44s**, not the 20–45 min
  I projected. The build-cost half of the defer argument does not hold. Recorded because an
  ROI verdict that survives only on a bad cost estimate should not survive.
- **But the capability gap is decisive, and I had missed it.** ontomics registers parsers for
  **Python, TypeScript, JavaScript, Rust only** (`ontomics:src/config.rs`) — there is **no Go
  parser**.
  git-bug is ~300 Go files plus a TypeScript webui, so an ontomics map would describe the
  React frontend and not the `entity/dag` CRDT substrate we care about.

So the **defer verdict stands, on stronger grounds than originally argued**: not "low value
for the cost" but "cannot perform the stated job." Running it was the right call — it replaced
a soft ROI argument with a hard capability fact, and corrected an error in my own reasoning.

**Confirmed by running it [V].** `ontomics health` against git-bug, complete run:

```
Language: TypeScript
Parsed 142 typescript files
Total: 142 parsed files
Found 362 concepts, 15 conventions · Built 221 entities
```

| | in repo | indexed |
|---|---|---|
| Go | 307 files, 73,870 LOC | **0** |
| TypeScript | 149 files | 142 |

It auto-detected TypeScript and mapped the webui. The entire trusted core — `entity/dag`,
`cache`, `repository`, `commands`, `bridge` — was not read at all. The concepts it surfaced
are `story`, `storybug`, `meta`, `metaprops`, `mock`, `withcommitsmock`, `makediffmock`:
an ontology of the frontend's **Storybook stories and test mocks**.

Cost of the check: ~5 min build, ~10 min run, 651 MB of models. Cheap, and it converted a
0.85-confidence judgement call into a settled fact. `.ontomics/index.db` was removed from the
git-bug tree afterwards; nothing was committed.

*Side benefit:* the run resolved an open conjecture — **HuggingFace is reachable through the
agent proxy**. That unblocks any future local-embedding work, including the deferred
LightRAG-over-issue-history option in §4(b).

## 7. Open decisions for Matt

The three gates of 2026-08-20 are answered and recorded in the header. What the Wave 1a
result newly puts in front of him:

1. **Proceed to 1b?** Recommended (0.85). The independent decoder now has a known
   disagreement to catch, not a hypothetical one.
2. **The spec correction — when, and to whom.** §5.2 and §7 of a public normative document
   are wrong. Options: hold it in the fork (current posture); open an upstream issue with the
   evidence; or send a spec-only PR. This is the outward-facing call, so it is his.
   *No action taken pending that call.*
3. **Does Wave 3 survive the no-code-changes constraint?** The write-path parity gap
   ([E5][E6]) is the concrete agent-native deficit, and closing it means editing production
   command code. Either Wave 3 is an exception to the constraint, or the agent-facing
   interface has to be built beside git-bug rather than inside it — which is a materially
   different architecture and worth deciding before Wave 2 sets direction.

## 9. Deliberately not done

- **Normative spec prose untouched.** `doc/spec/dag-entity.md` §5.2 and §7 still carry the
  wrong rule, as does a note in `doc/spec/testdata/bug.json`. Correcting a public normative
  document is outward-facing and unresolved — see §7.2. The evidence is staged so the fix can
  be one reviewed change when the call is taken.
- **No production code changed.** git-bug's divergence from upstream is exactly one new test
  file and two filled fixture placeholders.
- **LightRAG not built.** Per §4(b), unchanged.
- **`webui/dist` stub** was created locally to unblock Go builds. It is gitignored and was
  not committed; a shipped binary must be built with the real webui.
- **`.ontomics/`** appeared in the git-bug working tree from the ontomics run. Removed after
  the run; never committed. git-bug's tree carries no residue from the experiment.

---

## 8. Assumptions and known risks

- **[A]** `doc/spec/` accurately describes the current implementation. Wave 1 is precisely
  the test of that assumption — it may not hold, and that is the point.
- **[C]** The two placeholder vectors were left unfilled because the harness was never
  written, not because the values are contested. Unverified.
- **[V]** `bridge/*` tests cannot pass in this sandbox (no external API access). Any
  bridge-touching work is unverifiable here and must be gated differently.
- **[C]** HuggingFace reachability through the agent proxy is unverified — it is a blocker
  for ontomics and LightRAG both, and neither is on the critical path, so it stays untested.
- **[V]** `nix` is unavailable here, so `nix flake check` / `nix fmt` — the canonical
  format+lint gates — cannot run. Go-level fallbacks (`gofmt`, `golangci-lint`) are close
  but not identical to CI. Formatting-sensitive changes carry residual CI risk.
