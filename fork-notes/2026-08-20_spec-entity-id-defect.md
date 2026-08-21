# Finding — git-bug's normative spec states the wrong entity-ID derivation

**Date:** 2026-08-20 · **Session:** `agent-native-git-bug-xjz6vs` · **Status:** verified, unreported upstream
**Context:** Wave 1a of [`2026-08-20_trusted-core-plan.md`](2026-08-20_trusted-core-plan.md)
**Repo under test:** this repository, `zencohen-llc/git-bug` @ `1a5e57d` (fork of `git-bug/git-bug`)

> All file paths below are relative to this repository's root.

## Summary

`doc/spec/dag-entity.md` — the **normative** base-layer specification — says an entity's ID is
the hash of its whole `ops` blob. The implementation, a live repository, and three other
documents all say it is the hash of the **first operation** inside that blob. The two values
differ, so a third-party reader built from the spec computes an ID that matches no ref on
disk and cannot locate any entity.

This is the single most load-bearing rule in the format: entity IDs *are* the git ref names
(`refs/bugs/<id>`). Getting it wrong is not a cosmetic doc bug — it is an interoperability
break for exactly the third-party implementations the spec invites
[Observed `doc/spec/README.md` §Third-party implementations and extensions].

## The contradiction

**What the normative spec says** [Observed `doc/spec/dag-entity.md` §7 ID derivation]:

| What | Input data |
|---|---|
| Operation ID | Raw JSON bytes of the operation object as stored in the `ops` array |
| Pack ID / **Entity ID (root pack)** | Raw JSON bytes of the full `ops` blob |

and [Observed `doc/spec/dag-entity.md` §5.2 Entity ID], in full: *"The entity's ID is the pack
ID of its first (root) commit's operation pack."*

Note the neighbouring [Observed `doc/spec/dag-entity.md` §5.1 Pack ID] is **correct** — *"The
pack's ID is the SHA-256 of the exact bytes written to the `ops` blob"* really is the pack ID.
The defect is confined to §5.2 equating that value with the entity ID, and to the §7 table row
that repeats the equation.

**What the implementation does** — `entity/dag/entity.go:361`:

```go
func (e *Entity) Id() entity.Id {
	// id is the id of the first operation
	return e.FirstOp().Id()
}
```

## Empirical proof

Built `git-bug` from the fork at `1a5e57d`, created an identity and a bug in a scratch
repository, then read the raw git objects:

```
ref created            refs/bugs/3f65c242ad5a4855b1749dbb4fefd5d109ae125222f4c149f596c63e5e6a3706
ops blob object        e6344c259b2ac30090e87eef00efea6b52add9f6   (224 bytes, exact)

sha256(exact ops blob bytes)   = 436000bd1ec2fa8b6a6ed8d877ac3021a038dbdec932b11bed401c94ab7216c6   ← what the spec says the entity ID is
sha256(ops[0] compact bytes)   = 3f65c242ad5a4855b1749dbb4fefd5d109ae125222f4c149f596c63e5e6a3706   ← the actual ref name
```

The second value matches the ref exactly. The first matches nothing. The blob bytes were
taken from `git cat-file blob` (not `-p`), so no newline handling is in question.

The identity rule, by contrast, **is** stated correctly — same method, clean match:

```
ref                     refs/identities/bf5aa2bf026a58aa8383c34aa9a1fbab963e063437a4b8f8097bdb832e179d38
sha256(version blob)  = bf5aa2bf026a58aa8383c34aa9a1fbab963e063437a4b8f8097bdb832e179d38
```

## Corroboration — three sources agree with the code, one does not

| Source | Says | Correct? |
|---|---|---|
| `entity/dag/entity.go:361` | ID is the first operation's ID | ✅ |
| [Observed `doc/design/data-model.md` §Entities and Operation's ID] | *"the hash of the first `Operation` of the entity, as serialized on disk"* | ✅ |
| [Observed `doc/spec/bug.md` §5.1 CreateOperation (type 1)] | *"Sets the bug ID (equal to this operation's ID)"* | ✅ |
| [Observed `doc/spec/bug.md` §7 Snapshot semantics] | *"ID … Derived from `CreateOperation` ID"* | ✅ |
| [Observed `doc/spec/dag-entity.md` §5.2 + §7] | Entity ID = pack ID = hash of full `ops` blob | ❌ |

So `dag-entity.md` is the outlier. Note the pack ID is itself a **real and necessary**
quantity — it is the tie-breaker for concurrent operations in the ordering algorithm
[Observed `doc/spec/README.md` §CRDT]. The defect is not that pack IDs are fictional; it is
that §7 *equates* the pack ID with the entity ID.

A secondary instance of the same conflation sits in the fixtures
[Observed `doc/spec/testdata/bug.json`]: *"The bug ID equals the ID of this CreateOperation,
which equals hex(sha256(ops_blob_bytes))."* The first clause is right, the second is wrong,
and the `expected_id` field is absent entirely.

## Why this was invisible

The conformance vectors that would have caught it were never executed. `doc/spec/testdata/`
carried a `_comment` telling the reader to *"Run 'go test ./doc/spec/...' to verify"*, but
no Go package existed at that path — `go test ./doc/spec/...` returned `matched no packages`.
Two `expected_id` values were left as the literal placeholder
`<run: go test ./doc/spec/... -run TestVectors to generate>`.

The spec closes with a §12 Conformance section and a §13 Test vectors section pointing at
those fixtures [Observed `doc/spec/dag-entity.md` §13 Test vectors]. The apparatus of
verification was fully present; only the verification was missing. That is precisely the
configuration that makes a wrong rule survive — the document reads as checked.

A specification with unexecuted conformance vectors is a specification nobody has checked.

## What was done

`git-bug@873d7b1` adds `doc/spec/spec_test.go` — the harness the fixtures referred to. It
derives through `entity.DeriveId` (the production function), freezes expected values in
testdata, regenerates them with `-Update` per the repo's golden-file convention, and anchors
two checks on externally verifiable constants (SHA-256 of empty input; git's empty-blob
SHA-1) so the suite is not purely self-referential.

`TestEntityIdIsFirstOperation` pins the distinction: it computes both derivations from every
fixture pack and fails if they ever collapse into the same value.

Result: 25 packages ok, 1 pre-existing network-dependent failure (`bridge/github`).

## What was deliberately not done

The normative prose in `doc/spec/dag-entity.md` §5 and §7 was **not edited**, nor was the
false note in `bug.json`. Per Matt's 2026-08-20 call — *fork only for now, few or no changes
to the code itself* — correcting an upstream normative document is an outward-facing decision
that has not been made. The evidence is recorded here so the correction can be proposed as a
single reviewed change when that call is taken.

## Bearing on the trusted-core thesis

The plan argued Wave 1 was the highest-VOI action available because both outcomes were
decision-relevant. That held: we found a defect in the base-layer contract *before* building
on it. Two durable lessons:

1. **An unexecuted conformance suite is worse than none** — it signals a verification that
   never happened. **[Conjecture, this conversation]** this pattern likely generalises to
   most "spec + fixtures" repos, and is worth a check whenever we adopt one.
2. **Corroboration beats authority.** The normative document was the wrong one; the design
   rationale, the entity spec, the code comment, and the bytes on disk were all right. A
   trusted core needs the *bytes* as arbiter, which is what Wave 1b (an independent decoder)
   is for.

## Verification

- **Anchor check** — every `[Observed …]` anchor resolves to a real heading in the cited file
  at `1a5e57d`; code citation `entity/dag/entity.go:361` quoted verbatim.
- **Semantic check** — the ID claim rests on measured bytes from a live repository, not on
  reading the code alone; both candidate derivations were computed and compared against the
  actual ref name.
- **Conjecture isolation** — one `[Conjecture]`, tagged inline, confined to the generalisation
  in Lesson 1. No conjecture appears in the Summary or Empirical proof.
- **Drift risk** — the empirical values come from a single bug and identity created with one
  build (Go 1.25.0). The derivation rule is deterministic, so a second sample would not add
  information, but the finding is pinned to `1a5e57d` and should be re-checked if upstream
  changes `Entity.Id()` or `DeriveId`.
- **Not verified** — whether upstream considers §7 a defect or an intentional simplification.
  Unasked; fork-only posture stands.
