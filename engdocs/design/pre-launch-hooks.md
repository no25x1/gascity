---
title: Pre-Launch Hooks
status: Proposed
issues:
  - https://github.com/gastownhall/gascity/issues/845
  - https://github.com/gastownhall/gascity/issues/1052
---

# Pre-Launch Hooks

## Summary

Add an agent-level `pre_launch` lifecycle hook that runs after Gas City has
created or selected a concrete session bead and prepared its runtime
environment, but before the provider process starts and before any LLM
inference can occur.

`pre_launch` is a general deterministic startup extension point. It is not a
pool-worker primitive. A hook may inspect the prepared session context, perform
mechanical setup, and return a small structured patch that changes the initial
runtime configuration or stops startup before the model process launches.

The motivating use case is issue #845: pool workers sometimes need a
deterministic claim-or-drain cascade before inference starts. The same
mechanism also addresses issue #1052: two concurrently spawned pool sessions can
observe the same unassigned `gc.routed_to` bead and both execute it if claim
verification happens only inside prompt-following behavior. With `pre_launch`,
that behavior becomes a reusable script layered on top of the general hook:
query, atomically claim, inject the claimed bead into the first prompt, or drain
without launching the model when no work is available.

## Goals

- Provide a reusable pre-inference lifecycle boundary named `pre_launch`.
- Let scripts patch the initial prompt, nudge, environment, and session
  metadata using a typed JSON result contract.
- Let scripts stop startup with a clear controller-visible action instead of
  starting an idle or incorrectly scoped model process.
- Preserve existing behavior by default. Agents without `pre_launch` continue
  to start exactly as they do today.
- Keep bead-claiming as a script/helper use case, not a controller-special
  pool primitive.
- Make startup failures diagnosable from logs and session bead metadata.

## Non-Goals

- Do not replace `pre_start`. `pre_start` remains target-filesystem setup before
  provider session creation, such as worktree preparation.
- Do not add a new bead-store primitive. Claiming continues to use existing
  `bd update --claim` compare-and-swap behavior.
- Do not change the ownership meaning of `gc.routed_to`. It remains generic
  routing/provenance. Concrete ownership remains `assignee`.
- Do not run arbitrary semantic decision-making in Go. Scripts own their policy;
  Gas City only executes the declared lifecycle hook and applies the structured
  result.
- Do not support arbitrary command mutation in v1. Command rewriting would
  create a second provider-resolution layer and is deliberately excluded.

## Current Behavior

The current startup path resolves an agent template into a runtime config, then
starts the configured provider. Existing startup extension points are:

- `pre_start`: shell commands run before session creation on the target
  filesystem. Failures abort startup.
- `session_setup`: shell commands run after provider session creation and
  readiness. Failures are warnings.
- `session_setup_script`: a script run after `session_setup`.
- `session_live`: idempotent commands run at startup and re-applied on live-only
  config drift.
- Provider hooks such as `gc hook --inject`, which run after the model process
  exists and depend on the provider's hook mechanism.

For pool work, `EffectiveWorkQuery()` can return unassigned work routed by
`gc.routed_to=<template>`. `gc hook --inject` currently reminds the model to
claim that work. The atomic claim happens inside model-followed instructions,
not before inference starts.

That is acceptable for templates that want prompt-level ownership. It is a poor
fit for deterministic startup plumbing: a model can drift from the FIRST ACTION
block, and two sessions can both observe the same unclaimed bead before either
claim has persisted.

## Proposed Configuration

Add an agent field:

```toml
[[agent]]
name = "worker"
pre_launch = [
  "{{.ConfigDir}}/scripts/claim-next-work.sh"
]
```

`pre_launch` is a list of shell command templates, expanded with the same
context as `pre_start` and `session_setup`:

- `{{.Session}}`
- `{{.Agent}}`
- `{{.AgentBase}}`
- `{{.Rig}}`
- `{{.RigRoot}}`
- `{{.CityRoot}}`
- `{{.CityName}}`
- `{{.WorkDir}}`
- `{{.ConfigDir}}`

The field should also be available in agent patches and rig overrides:

```toml
[patches.agent.worker]
pre_launch_append = [
  "{{.ConfigDir}}/scripts/startup-policy.sh"
]
```

`pre_launch_append` has exactly the same replace-then-append semantics as
`pre_start_append`. Within one patch, `pre_launch = [...]` replaces the current
list, then `pre_launch_append = [...]` appends to the resulting list. Across
layered pack overrides and patches, ordering follows the existing patch
application order used by `pre_start`.

The initial implementation supports `pre_launch` and `pre_launch_append`
wherever existing `pre_start` and `pre_start_append` are supported:

- `config.Agent`
- `config.AgentPatch`
- `config.AgentOverride`
- pack expansion and patch application
- deep-copy paths used by pool/template resolution
- runtime startup hints

Future config nesting such as `[agent.lifecycle]` can be added as syntax sugar
if the project standardizes a lifecycle block later. The public name remains
`pre_launch`.

## Execution Point

Run `pre_launch` after all of the following are true:

1. The session bead exists or has been selected for wake.
2. `preWakeCommit` has written the new generation and instance token for a
   wake.
3. The resolved runtime config includes the concrete session environment:
   `GC_SESSION_ID`, `GC_SESSION_NAME`, `GC_ALIAS`, `GC_TEMPLATE`,
   `GC_SESSION_ORIGIN`, `GC_INSTANCE_TOKEN`, generation, and continuation
   epoch.
4. The final command, prompt, nudge, work directory, overlay paths, and
   startup hints have been resolved.
5. The provider process has not started.

This places the hook at a controller-owned deterministic boundary: scripts can
act with concrete session identity, but no LLM turn has occurred.

### Placement In Runtime Providers

For the tmux provider, `pre_launch` runs inside `doStartSession()` immediately
after `runPreStart()` returns and before `ensureFreshSession()` is called.

The ordering is:

1. `runPreStart()`
2. `runPreLaunch()`
3. `ensureFreshSession()`
4. `setRemainOnExit()`
5. readiness waits, dialog handling, `session_setup`, nudge, and
   `session_live`

This placement is required because `ensureFreshSession()` creates the tmux pane
and commits the launch command and environment. Any later placement would make
`env`, prompt, and drain patches partially or fully ineffective.

For exec-style providers, the equivalent anchor is after target preparation
(`pre_start`) and before finalizing `exec.Cmd.Env`, command arguments, container
spec, pod spec, or provider-specific start payload. A provider must not create a
process, pane, pod, container, ACP child, or remote session before `pre_launch`
has returned `continue`.

The following provider-side actions have not happened when `pre_launch` runs:
tmux `new-session`, `setRemainOnExit`, `waitForCommand`, startup dialog
handling, readiness polling, `session_setup`, startup nudge, `session_live`, and
provider process creation.

`pre_launch` runs even when no other startup hints are present. It must not be
skipped by the existing no-hints fire-and-forget path.

## Hook Environment

Each `pre_launch` command runs with the runtime config environment plus
controller context:

- `GC_CITY_PATH`
- `GC_CITY_NAME`
- `GC_WORK_DIR`
- `GC_AGENT`
- `GC_TEMPLATE`
- `GC_ALIAS`
- `GC_SESSION_ID`
- `GC_SESSION_NAME`
- `GC_SESSION_ORIGIN`
- `GC_INSTANCE_TOKEN`
- `GC_RUNTIME_EPOCH`
- `GC_CONTINUATION_EPOCH`
- rig-specific bead-store variables already used by hooks and work-query probes
  when the agent belongs to a rig
- `GC_SESSION`, as a compatibility alias for `GC_SESSION_NAME`

Commands run in the resolved work directory when present. If the work directory
is empty, they run from the city directory.

Command stdin is closed. Scripts must not wait for interactive input. Commands
execute through the same shell model as `pre_start` and use the same
per-command setup timeout plus the total timeout described below.

## Result Contract

Each command may write JSON to stdout. Empty stdout means:

```json
{"action": "continue"}
```

The JSON object shape is:

```json
{
  "action": "continue",
  "prompt_prepend": "",
  "prompt_append": "",
  "nudge_prepend": "",
  "nudge_append": "",
  "env": {
    "KEY": "value"
  },
  "metadata": {
    "key": "value"
  },
  "reason": "human-readable context"
}
```

Allowed `action` values:

- `continue`: apply the patch and continue to the next `pre_launch` command or
  provider start.
- `drain`: apply metadata, do not start the provider process, and transition the
  session into the normal drain/idle completion path.
- `abort`: fail startup. The reconciler records a wake failure and retries or
  rolls back according to existing startup failure rules.

Unknown actions are treated as `abort`. Malformed JSON is treated as `abort`.
Non-zero exit status always aborts, even when stdout contains valid JSON.
Scripts that intentionally return `drain` must exit 0. This avoids treating a
crashed script that happened to print parseable output as a successful drain.

Stdout must contain exactly one JSON object after trimming surrounding
whitespace. Empty stdout is equivalent to `{"action":"continue"}`. Unknown JSON
fields are ignored for forward compatibility. `null` values for string fields
are treated as empty. `env` and `metadata` must be JSON objects with string
values.

Patch semantics:

- `prompt_prepend` and `prompt_append` modify the resolved prompt for providers
  whose first turn is delivered as a command argument or prompt flag.
- `nudge_prepend` and `nudge_append` modify the startup nudge for providers
  using `PromptMode=none`.
- `env` overlays the runtime environment for the provider process and subsequent
  `pre_launch` commands.
- `metadata` writes user metadata before the provider starts. Empty values clear
  keys, matching existing metadata batch semantics.
- `reason` is diagnostic text recorded in logs and, for `drain` or `abort`,
  session metadata.

The controller applies `prompt_*`, `nudge_*`, `env`, and `metadata` patches
after each command so later commands can observe environment changes from
earlier commands. Metadata writes should be batched per command.

### Provider Mode Matrix

`pre_launch` has two delivery families:

- Prompt-capable launch: the provider has a structured first-turn prompt in the
  resolved runtime configuration.
- Nudge-only launch: the provider starts first and receives initial text through
  `Nudge`.

The implementation adds a distinct `runtime.Config.Prompt` field so prompt
patches are applied before command assembly. Provider command strings are
composed from `Command` plus `Prompt` after `pre_launch` runs. `pre_launch` does
not directly rewrite `Command`.

| Provider/start mode | `prompt_*` | `nudge_*` | `env` | `metadata` |
|---|---|---|---|---|
| tmux command-arg or prompt-flag launch | applied to `Config.Prompt` before command assembly | applied to startup nudge if present | applied to provider process and later hooks | applied before provider start |
| tmux nudge-only (`PromptMode=none`) | rejected with `failure_stage=unsupported_prompt_patch` | applied to startup nudge | applied to provider process and later hooks | applied before provider start |
| exec provider | serialized in start JSON as `prompt` or command-composed prompt, depending on provider capability | serialized as `nudge` | included in start JSON env | applied before provider start |
| k8s/container provider | applied before pod/container command/env finalization | applied only if provider supports startup nudge | included in pod/container env | applied before provider start |
| attach=false fresh launch | same as the provider's launch mode | same as the provider's launch mode | applied | applied |
| resume of an already-running provider process | `pre_launch` does not run | `pre_launch` does not run | not applicable | not applicable |

`pre_launch` is a launch hook, not a turn hook. It runs only when the controller
is about to start a provider process. It does not run when submitting follow-up
messages to an already-running session.

Prompt and nudge patches are bounded. Large prompt/nudge content should be
written to a file and referenced from the prompt, or delivered through the same
paste-buffer path used by large hook injections. Direct `prompt_*` and
`nudge_*` patches over the configured byte limit abort with
`failure_stage=patch_too_large`.

### Environment Overlay Rules

Environment patches apply to a per-start deep copy of `runtime.Config.Env`.
They never mutate agent config, template params, shared runtime config, or other
sessions.

Later `pre_launch` commands inherit earlier `env` patches. The provider process,
`session_setup`, and `session_live` inherit the final environment after all
successful `pre_launch` commands.

Environment keys must match:

```text
^[A-Z_][A-Z0-9_]{0,127}$
```

Keys or values containing NUL or newline are rejected. Values are capped by the
limits below.

Scripts may not override controller-owned identity variables:

- `GC_SESSION_ID`
- `GC_SESSION_NAME`
- `GC_SESSION`
- `GC_ALIAS`
- `GC_AGENT`
- `GC_TEMPLATE`
- `GC_SESSION_ORIGIN`
- `GC_INSTANCE_TOKEN`
- `GC_RUNTIME_EPOCH`
- `GC_CONTINUATION_EPOCH`
- `GC_CITY_PATH`
- `GC_CITY_NAME`
- `GC_WORK_DIR`

Scripts also may not set security-sensitive process-injection keys or prefixes:

- `PATH`
- `LD_*`
- `DYLD_*`
- `BASH_ENV`
- `ENV`
- `PYTHONPATH`
- `NODE_OPTIONS`
- `GODEBUG`

Rejected env patches abort before provider launch with
`failure_stage=env_validation`.

### Metadata Rules

Controller diagnostics use the reserved `gc.pre_launch.*` metadata namespace.
Scripts may not write that namespace.

Scripts may write user metadata under `pre_launch.user.*`. Other metadata keys
are rejected in v1 to prevent collisions with controller fields such as
`state`, `session_name`, `assignee`, `gc.routed_to`, `pending_create_claim`,
and `started_config_hash`.

The controller records the command index on every drain or abort:

- `gc.pre_launch.action`
- `gc.pre_launch.reason`
- `gc.pre_launch.at`
- `gc.pre_launch.command_index`
- `gc.pre_launch.failure_stage`
- `gc.pre_launch.provider_started=false`
- `gc.pre_launch.stderr_tail`

## Claim-Or-Drain Example

A pool worker that wants deterministic startup ownership can opt into:

```toml
pre_launch = [
  "gc work claim-next --template {{.Agent}} --assignee {{.Session}} --json"
]
```

`gc work claim-next --json` writes the single `pre_launch` result JSON object to
stdout. All diagnostics go to stderr.

Bead bodies can contain untrusted or instruction-like text. Scripts that inject
bead content into prompts should fence it as data and include explicit
"work only this claimed bead" instructions. Future helpers may provide a
standard fenced representation.

### `gc work claim-next` Contract

The first Gas City-provided claim-or-drain helper is `gc work claim-next`.
It is layered on existing work query and bead update semantics but is in the
same implementation milestone as `pre_launch` because it is the concrete fix
path for #845 and #1052.

Inputs:

- `--template <qualified-template>` selects the template whose
  `EffectiveWorkQuery()` should be used.
- `--assignee <session-id-or-name>` is the concrete owner to claim for.
- `--json` emits machine-readable output.

Algorithm:

1. Check for an existing in-progress bead already assigned to the requested
   assignee. If present, return that bead first. This makes retry after crash or
   abort idempotent.
2. Run the template's effective work query in session context.
3. Consider only generic-demand candidates with no assignee.
4. Attempt an atomic claim using bd compare-and-swap semantics:
   "set assignee to `<assignee>` and transition to in-progress only if assignee
   is empty and the bead is still claimable."
5. Reload the bead and verify `assignee == <assignee>`.
6. If claim failed because another worker won, retry the next candidate.
7. Return no claim only after the candidate set is exhausted.

JSON output:

When an existing self-assigned bead is found, or a new bead is claimed:

```json
{
  "action": "continue",
  "env": {
    "GC_WORK_BEAD": "ga-123"
  },
  "nudge_append": "\n\nClaimed work bead: ga-123",
  "metadata": {
    "pre_launch.user.claimed_work_bead": "ga-123"
  }
}
```

When no work is available:

```json
{
  "action": "drain",
  "reason": "no_work"
}
```

If the helper cannot query, claim, or verify work, it exits non-zero. The
`pre_launch` runner treats that as an abort with `failure_stage=command_failed`
or a more specific stage when available.

Exit codes:

- `0`: successful claim or empty queue
- `1`: invalid arguments or config
- `2`: bead store unavailable
- `3`: claim verification failed for reasons other than ordinary contention

The helper must not derive ownership from `gc.routed_to`. `gc.routed_to` may be
retained as provenance after claim; generic-demand accounting must exclude any
bead with concrete `assignee` regardless of retained provenance.

## Interaction With Existing Lifecycle

### `pre_start`

`pre_start` still runs where it does today. It prepares filesystems and runtime
targets. It has no structured output and does not patch prompt or metadata.

### `pre_launch`

`pre_launch` runs after session identity exists and before provider process
creation. It can patch initial prompt/nudge/env/metadata or choose not to
launch.

### `session_setup` and `session_live`

These still run only after a provider session exists. If `pre_launch` returns
`drain` or `abort`, neither runs because no provider process was started.

### Config Fingerprints

The declared `pre_launch` command list is part of the stable startup config and
must be included in the core session fingerprint. Runtime results from
`pre_launch` are not part of the fingerprint because they are per-start facts.

If a `pre_launch` command changes, existing sessions follow the same restart
path as a `pre_start` change.

`ConfigFingerprint` and `CoreFingerprint` are computed from the declared
resolved `pre_launch` list before any command executes. Runtime patches returned
by `pre_launch` for env, prompt, nudge, or metadata are process-start facts and
never re-enter fingerprint computation. This remains true even if a hook returns
an allowlisted `GC_*` env value such as `GC_WORK_BEAD`.

`PreLaunch` must appear in fingerprint breakdown and drift logging alongside
`PreStart`.

### Draining

`action=drain` means no provider process was launched. A pre-launch drain is a
terminal-success launch outcome, not a provider crash and not a retryable wake
failure.

The controller archives the session bead immediately with:

- `state=archived`
- `gc.pre_launch.action=drain`
- `gc.pre_launch.reason=<bounded reason>`
- `gc.pre_launch.at=<RFC3339 timestamp>`
- `gc.pre_launch.command_index=<zero-based index>`
- `gc.pre_launch.provider_started=false`

No provider stop, interrupt, runtime drain acknowledgment, readiness wait,
dialog handling, `session_setup`, startup nudge, `session_live`, or
`setRemainOnExit` is attempted. There is no pane or process to preserve, so the
forensic surface is the bounded metadata and typed events.

Pool and capacity accounting must not count archived pre-launch drains as
running or in-flight sessions. The reconciler must not resurrect an archived
pre-launch drain unless a new explicit wake/materialization request is created.

`drain` preserves continuation epoch because no model turn was produced.

If a script claims a bead and then returns `drain`, the script must release or
requeue its claim before returning. The controller-owned `gc work claim-next`
path does not return `drain` after a successful claim.

### Abort And Retry

`action=abort`, non-zero exit, timeout, malformed JSON, invalid patch, metadata
write failure, or oversized output aborts startup before provider launch.

Abort writes:

- `gc.pre_launch.action=abort`
- `gc.pre_launch.reason=<bounded reason>`
- `gc.pre_launch.failure_stage=<closed enum>`
- `gc.pre_launch.at=<RFC3339 timestamp>`
- `gc.pre_launch.command_index=<zero-based index>`
- `gc.pre_launch.provider_started=false`

Failure stages:

- `script_exit`
- `timeout`
- `stdout_too_large`
- `stderr_too_large`
- `json_parse`
- `unknown_action`
- `env_validation`
- `metadata_validation`
- `metadata_write`
- `unsupported_prompt_patch`
- `patch_too_large`
- `context_canceled`

Abort retry uses the existing wake failure machinery, but the failure is tagged
as pre-launch rather than provider-start. A retry must be idempotent. The
claim-next helper first returns any self-assigned bead for the session before it
queries generic demand, so a controller crash after claim and before provider
start relaunches against the already-owned bead instead of claiming a second
one.

If a later `pre_launch` command fails after an earlier command claimed work via
`gc work claim-next`, retry idempotence handles the claim. Scripts that perform
their own external claims must either be idempotent in the same way or release
their claim before aborting.

## Error Handling

- Command timeout uses the existing session setup timeout.
- The pre-launch list also has a total timeout budget, defaulting to the larger
  of the setup timeout and the sum of per-command timeouts capped by daemon
  startup timeout. Exceeding the total budget aborts with
  `failure_stage=timeout`.
- Non-zero command exit status always aborts startup.
- Malformed JSON aborts startup and records the raw parse error.
- Metadata write failure aborts startup before provider creation.
- Environment keys must match the same conservative variable-name rules used by
  config env overlays. Invalid keys abort startup.
- Large stdout should be rejected with a bounded size limit before JSON parsing
  to avoid unbounded memory use.

Concrete limits for v1:

- stdout: 64 KiB, enforced while reading with a limit reader
- stderr: 16 KiB retained tail, enforced while reading
- `reason`: 1 KiB after sanitization
- each metadata key: 128 bytes
- each metadata value: 4 KiB
- total script metadata patch: 16 KiB
- each env value: 8 KiB
- total env patch: 32 KiB
- `prompt_prepend` + `prompt_append`: 64 KiB total per command
- `nudge_prepend` + `nudge_append`: 16 KiB total per command

On stdout or stderr overflow, the controller terminates the command, aborts
startup, and persists only the byte count, command index, failure stage, and a
sanitized error. Raw stdout, raw prompt/nudge patch contents, full env, env
values, and secret-bearing output are never logged or persisted.

## Security

`pre_launch` executes configured shell commands. It has the same trust model as
`pre_start`, `session_setup`, and pack scripts: city and pack authors are
trusted code providers.

Additional safeguards:

- The result contract is data, not shell. Prompt/nudge/env/metadata patches are
  applied structurally by Go code.
- Command mutation is excluded in v1.
- `prompt_*` patches mutate a structured prompt field before provider command
  assembly; they do not perform shell string rewriting.
- Secret-bearing environment values should not be serialized into logs.
- Result parse errors should include command index and action context, but not
  dump full environment.

## Observability

For each command, log:

- session name
- template name
- command index
- action
- duration
- reason, when present

For `drain` and `abort`, persist metadata such as:

- `gc.pre_launch.action`
- `gc.pre_launch.reason`
- `gc.pre_launch.at`

Scripts can persist additional domain-specific metadata through the structured
`metadata` patch.

The event bus emits typed events with registered payloads:

- `session.pre_launch_started`
- `session.pre_launch_command_completed`
- `session.pre_launch_drained`
- `session.pre_launch_aborted`

Payloads include session ID, session name, template, command index, action,
duration, failure stage, and sanitized reason. `gc trace` should surface
pre-launch as a distinct startup phase.

Because no tmux pane exists on pre-launch drain or abort, `setRemainOnExit`
cannot provide forensics. The substitute forensic surface is the event stream,
bounded stderr tail, and `gc.pre_launch.*` metadata.

## Implementation Checklist

- Add `PreLaunch` and `PreLaunchAppend` to `internal/config/config.go` agent,
  patch, and override structs.
- Wire patch and override application in `internal/config/patch.go` and pack
  expansion.
- Deep-copy fields in pool/template resolution helpers.
- Add `PreLaunch` to startup hints and `runtime.Config`.
- Add `Prompt` to `runtime.Config` so prompt patches do not rewrite `Command`.
- Add `pre_launch` and `prompt` to exec provider JSON only when supported.
- Add `PreLaunch` to `runtime.CoreFingerprint`,
  `CoreFingerprintBreakdown`, canonical config hash, and drift logging.
- Update config schema generation and generated config docs.
- Update field-sync tests and any patch-only exclusions deliberately.
- Register typed event payloads for all new pre-launch events.
- Expose fields in API/OpenAPI/dashboard typed config views if agent config
  views include lifecycle fields.
- Implement `gc work claim-next` with the CAS/verify/retry/idempotence contract
  above.
- Add docs and a pack-local example script only after the generic lifecycle hook
  tests pass.

## Rollout

1. Add config fields and docs with no agents opted in.
2. Implement generic runtime support and hook-level tests for continue, drain,
   abort, malformed output, timeout, invalid env, metadata failure, no-hook
   compatibility, prompt/nudge/env/metadata patches, and fingerprint behavior.
3. Add `gc work claim-next` with tests for claim success, claim conflict retry,
   existing self-assigned idempotence, empty queue, store outage, and ownership
   verification failure.
4. Add an example script in the relevant pack.
5. Opt in the first pool worker template only after tests cover claim success,
   claim-lost, empty-drain, malformed hook result paths, the #1052 concurrent
   two-sessions-one-bead race, and rollback by removing the config field.

Rollback for the first pool-worker opt-in is removing `pre_launch` from the
template config. Because the feature is additive and declared in config, rollback
does not require data migration.

## Test Plan

Tests should use fake runtime providers or `fakeStartOps`; no test should
require real LLM inference.

- `TestPreLaunchNoHookStartsExactlyAsBefore`: no `pre_launch` produces the same
  start call order and runtime config as current startup.
- `TestPreLaunchRunsAfterPreStartBeforeEnsureFreshSession`: verifies tmux start
  operation ordering.
- `TestPreLaunchPromptPatchAppliesBeforeCommandAssembly`: prompt append affects
  `Config.Prompt` and provider command composition.
- `TestPreLaunchRejectsPromptPatchForNudgeOnlyProvider`: unsupported prompt
  patch aborts with diagnostic metadata.
- `TestPreLaunchNudgePatchAppliesToPromptModeNone`: nudge append reaches startup
  nudge.
- `TestPreLaunchEnvPatchVisibleToLaterCommandAndProvider`: env overlay is
  per-start and cumulative.
- `TestPreLaunchRejectsReservedEnvOverride`: reserved `GC_*`, `PATH`, and
  `LD_*` keys fail closed.
- `TestPreLaunchMetadataPatchWritesUserNamespaceOnly`: user metadata writes and
  reserved namespace rejection.
- `TestPreLaunchDrainArchivesWithoutProviderStart`: drain records metadata and
  skips provider-side steps.
- `TestPreLaunchAbortDoesNotStartProvider`: malformed JSON, non-zero exit,
  timeout, oversized stdout, invalid env, and metadata write failure each abort.
- `TestPreLaunchAbortRecordsFailureStage`: every failure stage is visible in
  metadata/events.
- `TestPreLaunchFingerprintIncludesDeclaredCommands`: command list changes hash.
- `TestPreLaunchRuntimePatchDoesNotAffectFingerprint`: env/prompt runtime patch
  does not change core hash.
- `TestPreLaunchAppendMatchesPreStartAppendSemantics`: patch and override
  ordering matches existing lifecycle append behavior.
- `TestWorkClaimNextClaimsOneOfConcurrentWorkers`: two sessions race one bead;
  only one claims and the other returns empty.
- `TestWorkClaimNextRetriesOnClaimConflict`: conflict on first candidate tries
  the next candidate.
- `TestWorkClaimNextReturnsExistingSelfAssignedFirst`: retry after crash uses
  existing ownership.
- `TestWorkClaimNextRetainsRoutedToAsProvenanceWithoutGenericDemand`: assigned
  work with retained `gc.routed_to` is excluded from generic demand.
- `TestPreLaunchAbortAfterClaimRecovery`: retry after abort returns the
  self-assigned bead instead of claiming another.
- `TestPreLaunchCrashAfterClaimRecovery`: controller restart between claim and
  provider launch resumes the self-assigned bead.

## Acceptance Criteria

- Agents without `pre_launch` start exactly as before.
- A `pre_launch` script can append to the initial prompt before provider start.
- A `pre_launch` script can append to the startup nudge before provider start.
- A `pre_launch` script can add provider environment variables visible to the
  launched process.
- A `pre_launch` script can write session metadata before provider start.
- `action=drain` prevents provider process launch and leaves a diagnosable
  session bead state.
- `action=abort` and malformed output fail startup without launching the
  provider process.
- The declared `pre_launch` command list participates in config drift
  fingerprinting.
- Claim-or-drain can be implemented with `gc work claim-next` using existing
  work-query and bead-claim semantics, addressing #845 and #1052 without giving
  `gc.routed_to` ownership semantics.
- The first implementation includes the two issue references in PR text and
  validates both Stephanie's #845 drift case and Riley's #1052 duplicate
  execution race.

## Open Questions

- Should `pre_launch` be top-level on `[[agent]]` for symmetry with
  `pre_start`, or should a future `[agent.lifecycle]` block also be introduced
  for grouping? This design chooses top-level now for the smallest consistent
  change.
- Should a future `gc hook emit` helper generate result JSON for pack authors?
  The v1 example is POSIX-only, but a helper would improve ergonomics.
- Should `pre_launch` eventually support a structured claim field in its own
  JSON result so the controller can own rollback of local bead claims? V1 relies
  on idempotent helpers and explicit script rollback for external systems.
