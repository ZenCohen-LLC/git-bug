# fork-notes

Working notes for the `zencohen-llc/git-bug` fork. **Nothing here comes from upstream**, and
nothing here is normative for git-bug — the normative data-format specification is
[`doc/spec/`](../doc/spec/).

This directory exists so that planning and findings live next to the code they concern. It is
deliberately a single flat, dated lane rather than a structure, and it is trivially strippable
if this fork is ever rebased or a change is offered upstream.

| Convention | |
|---|---|
| Naming | `YYYY-MM-DD_<slug>.md` |
| Paths inside a note | relative to the **repository root**, unless prefixed `repo:path` (e.g. `ontomics:src/config.rs`) |
| Status | each note states its own status in its header; this README never asserts one |

## Contents

- [`2026-08-20_trusted-core-plan.md`](2026-08-20_trusted-core-plan.md) — the plan for building
  an agent-native git-bug as a trusted core: assigned constraints, measured evidence, the
  pre-work ROI verdicts, and the wave sequence with its approval gates.
- [`2026-08-20_spec-entity-id-defect.md`](2026-08-20_spec-entity-id-defect.md) — verified
  finding: `doc/spec/dag-entity.md` §5.2 and §7 state the wrong entity-ID derivation. The
  implementation, a live repository, and three other documents all disagree with it. No fix
  applied; the upstream posture is an open decision recorded in the plan.

## Related

`doc/spec/spec_test.go` executes the conformance vectors in `doc/spec/testdata/`, which had
never been run. That harness is what surfaced the finding above.
