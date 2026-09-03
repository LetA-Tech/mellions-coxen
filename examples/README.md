# Examples

These examples show Mellions Engineer as a responsibility layer around a capable coding agent. They are intentionally generic and contain no private project assumptions.

## 1. Give the engineer a responsibility

Inside a configured repository, start Claude Code or Codex and say:

```text
Take issue #1428. Establish the current truth first, then carry the responsibility through implementation and verification. Escalate only if the remaining decision genuinely belongs to me.
```

Mellions supplies the durable engineering context. The frontier model still decides how to investigate and implement.

## 2. Let Mellions choose useful work

```text
/mellions:survey
```

Expected behavior:

```text
collect current engineering signals
→ distinguish actionable work from owner-gated work
→ choose a useful responsibility using frontier-model judgment
→ explain the choice
→ claim / carry the work
```

The survey collector does not hard-code engineering priority.

## 3. Challenge a green implementation

After a consequential fix appears complete:

```text
Use Mellions falsification discipline on this implementation. Identify the claims the current test evidence is supposed to establish, construct independent attempts to disprove them, and treat any arm that stays green unexpectedly as a finding rather than automatic success.
```

A useful falsification exercise may include neutralizing part of the fix, mutating the behavior, checking that the mutation actually landed, and verifying that the relevant test fails for the intended reason.

The objective is not mutation testing for its own sake. It is to answer:

> **Would the evidence notice if the claimed fix were actually absent or incomplete?**

## 4. Continue after interruption

```bash
mellions continue
mellions assign list
```

The new session should see recorded responsibility beside current external reality. It should re-establish what is still true instead of blindly trusting stale history.

## 5. Step away

```bash
mellions away -because "offline for the afternoon"
```

When you return:

```bash
mellions back
```

The target behavior is continued delegated engineering while the owner is unavailable, with genuinely human decisions preserved for the owner's return.

The current unattended shift runner is Claude Code-specific. Codex support is currently interactive; see [`../docs/integrations.md`](../docs/integrations.md).

## 6. Reproduce a benchmark

A useful treated/untreated comparison keeps as much as possible fixed:

```text
same model
same runtime
same repository fixture
same prompt
same tool access
same time budget
```

Then vary one Mellions behavior or mechanism.

Example behavioral question:

> The requested defect is fixed, but the agent discovers and proves a second material defect in the same code path. Does it resolve the established in-scope defect, or does it hand the obvious engineering decision back to the human?

Report both positive and negative results using the **Benchmark reproduction** GitHub issue form.

See [`../docs/benchmarks.md`](../docs/benchmarks.md) for the current evidence and methodology.

## 7. Evaluate a new runtime adapter

A runtime integration should prove more than "the plugin installed."

Verify that a fresh agent session receives the right Mellions context at the right lifecycle points:

```text
session start / resume
→ identity + relevant responsibility

engineering situation changes
→ relevant awareness reaches the agent

context renews / session restarts
→ responsibility survives and current reality is rechecked
```

Keep the native runtime responsible for its own permissions, tools, model selection, MCP, sandboxing, and agent-team features.

See [`../docs/integrations.md`](../docs/integrations.md) and use the **Runtime integration** issue template when proposing another runtime.
