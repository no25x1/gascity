---
title: Pre-Launch Hooks Implementation Plan
status: Proposed
design: ./pre-launch-hooks.md
issues:
  - https://github.com/gastownhall/gascity/issues/845
  - https://github.com/gastownhall/gascity/issues/1052
---

# Pre-Launch Hooks Implementation Plan

## Overview

Implement the approved `pre_launch` design in small, testable slices. The
generic lifecycle hook lands first, with no agents opted in. The deterministic
claim helper lands after the hook engine is covered by tests. Pack opt-in is
last and only happens after the #845 drift case and #1052 duplicate-execution
race have regression coverage.

## Phase 1: Config And Fingerprint Plumbing

### Task 1: Add Config Fields

Add `pre_launch` and `pre_launch_append` to the same config surfaces as
`pre_start`:

- `config.Agent`
- `config.AgentPatch`
- `config.AgentOverride`
- patch and override application
- deep-copy helpers
- template resolution and startup hints
- runtime config

Acceptance criteria:

- `pre_launch` parses from agent TOML.
- `pre_launch_append` composes like `pre_start_append`.
- `pre_launch` is available in resolved template startup hints.
- Existing agents without `pre_launch` resolve unchanged.

Verification:

- `go test ./internal/config ./cmd/gc -run 'PreLaunch|AgentFieldSync|ExpandPacks|TemplateResolve|Pool'`

### Task 2: Add Fingerprint And Schema Coverage

Include declared `pre_launch` commands in startup fingerprints and generated
config docs.

Acceptance criteria:

- Changing `pre_launch` changes core fingerprint.
- Runtime hook result patches do not participate in fingerprints.
- Fingerprint breakdown and drift logging include `PreLaunch`.
- Generated schema/docs expose the field.

Verification:

- `go test ./internal/runtime ./cmd/gc -run 'Fingerprint|ConfigHash|Schema|PreLaunch'`
- `go run ./cmd/genschema`

## Phase 2: Generic Pre-Launch Engine

### Task 3: Define Result Contract And Validation

Add typed result parsing and validation for `pre_launch` command stdout.

Acceptance criteria:

- Empty stdout is `continue`.
- Unknown actions, malformed JSON, non-zero exit, invalid env, invalid
  metadata namespace, and oversized output fail closed.
- Bounds and failure stages match the design.
- Raw stdout and env values are not logged or persisted.

Verification:

- Unit tests for parser/validator failure stages.

### Task 4: Execute Pre-Launch Before Provider Start

Wire `pre_launch` into runtime startup after `pre_start` and before any provider
process creation.

Acceptance criteria:

- No-hook startup call order is unchanged.
- `pre_launch` runs after `pre_start` and before tmux `ensureFreshSession`.
- Prompt/nudge/env/metadata patches are applied before provider launch.
- `drain` archives without provider start.
- `abort` fails startup without provider start and records diagnostics.

Verification:

- `go test ./internal/runtime/tmux ./cmd/gc -run 'PreLaunch'`

## Phase 3: Claim Helper

### Task 5: Implement `gc work claim-next`

Add a deterministic helper that claims work for a concrete session using
existing bead semantics.

Acceptance criteria:

- Existing self-assigned work is returned before generic demand.
- Concurrent workers racing one bead produce one claim and one empty/conflict
  outcome.
- Claim conflict retries the next candidate.
- Retained `gc.routed_to` provenance does not reactivate generic demand after
  concrete assignment.

Verification:

- `go test ./cmd/gc ./internal/beads -run 'ClaimNext|PreLaunch|WorkQuery'`

## Phase 4: Docs, Examples, And Opt-In

### Task 6: Add Docs And Pack Example

Document `pre_launch`, add a safe example script, and avoid broad pack opt-in
until the generic hook and helper are green.

Acceptance criteria:

- Reference docs describe `pre_launch` and `pre_launch_append`.
- Example script sends diagnostics to stderr and emits one JSON object on stdout.
- PR description references #845 and #1052 explicitly.

Verification:

- Docs generated and checked in.

## Review Checkpoints

- Run local tests after each phase.
- After Phase 2, run `$review-pr --diff` against the staged generic hook change
  and fix blockers/majors.
- After Phase 3, run `$review-pr --diff` again, focused on claim helper and
  race behavior.
- Before PR creation, run full relevant test suite and `$review-pr` on the
  branch. Fix until no blockers or majors remain.

## Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Provider startup boundary is misplaced | Env or drain patches silently fail | Tests assert call ordering before provider creation |
| Claim helper is not truly atomic | #1052 remains possible | CAS/verify/retry tests with concurrent claim attempts |
| Drain is misclassified as crash | Reconciler resurrects intentionally drained sessions | Archive terminal-success state and metadata/event checks |
| Script output leaks secrets | Operator logs expose sensitive data | Bound stdout/stderr and persist sanitized metadata only |
| Feature becomes pool-specific | Poor abstraction and future churn | Keep claim helper separate from generic `pre_launch` |
