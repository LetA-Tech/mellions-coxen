<p align="center">
  <img src="assets/letatech-logo.svg" alt="LetA Tech" width="180" />
</p>

<h1 align="center">Mellions Engineer</h1>

<p align="center"><strong>Turn any capable coding agent into a real software engineer.</strong></p>

<p align="center"><strong>Model- and agent-agnostic by design. Mellions Engineer adds engineering discipline, durable responsibility, falsifiable verification, continuity, and self-evolution.</strong></p>

<p align="center">Built for long-running, production-grade software engineering.</p>

---

Mellions Engineer is built for capable coding agents and models without coupling its engineering model to one vendor. The underlying agent and runtime remain responsible for reasoning, tools, permissions, worktrees, subagents, sandboxing, and native execution.

Mellions adds what a transient coding-agent session does not reliably preserve on its own: **responsibility, continuity, engineering discipline, coordination, proof, and useful autonomy over time.**

> **Frontier-model judgment + deterministic mechanisms.**

## Why Mellions

Using a strong coding agent can still leave the human acting as its engineering manager:

- choosing every next task;
- re-explaining context;
- approving obvious follow-up work;
- checking whether a green suite really proves the fix;
- recovering interrupted work;
- coordinating parallel sessions;
- deciding whether “done” actually means done.

Mellions is designed to reduce that management load.

Instead of operating each session step by step, give the agent engineering responsibility:

```text
Take #1428 and carry it through.
```

Or let it inspect the engineering estate and choose useful work:

```text
/mellions:survey
```

When you step away:

```bash
mellions away
```

When you return:

```bash
mellions back
```

The model remains the engineer. Mellions helps that engineer stay responsible for the work.

## Evidence in 30 seconds

Mellions is developed through sustained production use plus controlled same-model comparisons where a specific behavior can be isolated.

| Result | Observed outcome | Why it matters |
|---|---:|---|
| **Established in-scope defect ownership** | **1/3 → 3/3** sessions resolved the already-proven defect instead of handing it back to the human | Removes approval-seeking after the engineering answer is already known |
| **Completion challenge calibration** | **173 historical handoffs:** 0 honest handoffs challenged; both known false-completion cases caught | Makes “done” stronger than “my patch passed” without adding ceremony to normal work |
| **Long production case study** | Claude retrospectively estimated **~60–65%** correct-completion probability native-alone vs **~85–90%** with Mellions for that task | The largest observed value came from verification depth and ownership discipline |

The strongest measured Mellions behavior so far is **falsification / adversarial verification**. In one long production implementation, the agent ran 13 isolated adversarial verification arms, retracted a conclusion it had already published, and caught two latent defects that ordinary completion would have left behind.

Claude's own retrospective summarized the value this way:

> “mellions-falsification alone justifies it.”

And on the implementation risk it changed:

> “Without falsification I'd have shipped ... two latent defects in new production code.”

The ~60–65% vs ~85–90% figures are a **production case-study estimate**, not a universal coding benchmark, SWE-bench score, or independent population statistic. The underlying behavior changes were observable; the percentage is not generalized beyond that production case.

### Don't trust our benchmark. Reproduce it.

Run the same engineering task with and without Mellions. Bring us the cases where it helps, does nothing, or makes the agent worse. The benchmark should survive skeptical engineers, not marketing copy.

**[Read the public methodology, feature-level evidence, limitations, and reproduction challenge →](docs/benchmarks.md)**

## What changes in practice

| Without a responsibility layer | With Mellions Engineer |
|---|---|
| Finds another real defect, then asks whether to fix it | Establishes whether the defect is in scope and owns it when it is |
| Green tests become the stopping point | Important conclusions are actively falsified |
| “Done” can mean “my patch passed” | Completion is reconciled against the engineering obligation |
| Session state is mostly conversational | Durable responsibility survives interruption and handoff |
| Parallel agents can unknowingly duplicate work | Active responsibility and overlapping work are visible |
| Human presence encourages approval-seeking | Ordinary engineering proceeds inside delegation; real owner decisions still escalate |
| Failures become anecdotes | Reusable failures can become tested improvements for later sessions |

## Model- and agent-agnostic by design

Mellions' engineering model is not tied to Claude, Codex, or a particular LLM family. **The engineering discipline is portable; automatic lifecycle integration is provided through runtime adapters.**

Today:

| Runtime / model | Current support |
|---|---|
| **Claude Code** | **First-class and production-used.** Native plugin, Skills, slash commands, lifecycle hooks, awareness hooks, recovery/renewal, and unattended runner support. |
| **Codex** | **Implemented and interactive.** Plugin, Skills, commands, and hook manifest are present; Codex must explicitly trust the hooks. Current unattended runner is Claude-only. |
| **Opus 5** | Supported when selected through an integrated runtime such as Claude Code. Mellions does not require a separate Opus-specific adapter. |
| **Fable 5** | Used in evaluation/review contexts, but **no dedicated first-class runtime adapter is shipped today**. |
| **Kimi 3** | **Future/community integration target.** No dedicated first-class adapter is shipped today. |
| **Other capable coding-agent runtimes** | The architecture is designed to accept runtime adapters, but automatic lifecycle support must be implemented and verified per runtime. |

A model name is not the same thing as an integration claim. Mellions can be model- and agent-agnostic while remaining precise about which runtimes have automatic integration today.

**[See the integration matrix, hook surfaces, and adapter requirements →](docs/integrations.md)**

## How Mellions hooks into engineering work

Mellions uses the runtime's own extension points rather than replacing the runtime.

For Claude Code today, the plugin uses lifecycle surfaces such as:

| Runtime moment | What Mellions contributes |
|---|---|
| **Session start / resume** | Durable identity, engineering methods, relevant program/partnership context, work in flight, and owner state |
| **Prompt submission** | Situational awareness that became relevant since the last turn |
| **Before tool use** | Narrow awareness and deterministic protection where an invariant genuinely needs it |
| **Before compaction** | Preserve the engineering responsibility before context renewal |

Codex receives the same Mellions plugin concepts through its supported plugin path, subject to Codex's own hook-trust boundary.

Mellions does not replace native permissions, MCP, tool configuration, sandboxing, model selection, worktrees, or agent teams.

## Quick start

### Requirements

- Go 1.26+
- Git
- GitHub CLI (`gh`), authenticated for repository surveys and claims
- Claude Code and/or Codex
- macOS or Linux

### Install

```bash
git clone https://github.com/LetA-Tech/mellions-coxen.git
cd mellions-coxen
make install

mellions config init
mellions doctor
```

Then establish the program and working relationship:

```bash
mellions program discover
mellions partner establish you@example.com
```

Review the human-declared sections, then adopt them:

```bash
mellions program adopt -by "you"
mellions partner adopt you -by "you"
```

See [`docs/install.md`](docs/install.md) for the complete walkthrough.

## The engineering discipline

Mellions is opinionated about serious engineering while leaving technical judgment to the frontier model.

A Mellions engineer is expected to:

- establish the actual problem before changing code;
- distinguish evidence from inference and unknowns;
- pursue root cause rather than symptom suppression;
- own material findings that belong to the responsibility already being carried;
- challenge tests that may prove the wrong property;
- try to disprove consequential conclusions;
- correct or retract claims when later evidence defeats them;
- verify the original obligation before declaring completion.

The goal is not more ceremony. It is making confident-but-wrong completion harder.

## Falsification is first-class

Passing tests are evidence. They are not automatically proof.

For consequential work, Mellions encourages the engineer to attack the claimed fix: neutralize important parts, mutate the behavior, isolate experiments, verify the mutation actually landed, and inspect any result that stays green when it should have failed.

A green adversarial arm can itself be the finding: perhaps the test never reached the behavior, the assertion proved something else, or a different check fired first.

That is where Mellions has shown its clearest value so far.

## What Mellions adds

### Durable engineering identity

Every session begins with the same engineering character: evidence-first, technically rigorous, production-minded, independent, and responsible for the engineering problem rather than merely the wording of a prompt.

### Partnership and intelligent escalation

Mellions carries how the owner wants to work and what has been delegated. Ordinary engineering should not become an approval request just because a human is available. Genuine product, architecture, contract, credential, or otherwise reserved decisions still belong to the human.

### Program and situational awareness

A session can understand what the work is for, what is already in flight, which repositories matter, what changed, and what needs attention. `mellions survey` gathers the situation; the frontier model still decides what matters.

### Responsibility that survives sessions

Assignments preserve the engineering responsibility and its established state. A later session sees recorded history beside current external reality and re-establishes what is still true rather than blindly replaying stale state.

### Multi-engineer coordination

Mellions makes active responsibility, worktrees, and relevant peers visible so several frontier coding-agent engineers can work in parallel without becoming a centrally orchestrated swarm.

### Engineering methods

Reusable methods cover reasoning, research, falsification, bug audit, remediation, closure, delegation, continuity, safe experimentation, and self-learning. They are discoverable on demand rather than forcing the entire toolbox into every context.

```bash
mellions skills
mellions skills "a green test run is not proof the fix holds"
```

### Attended ↔ unattended engineering

`mellions away` and `mellions back` make owner availability explicit. Delegated work can continue while the owner is away; real human decisions become durable decision packages instead of questions sent to an empty room.

### Self-evolution from production evidence

When real use exposes a Mellions weakness, the intended loop is:

```text
production weakness
→ establish root cause
→ make the smallest effective correction
→ adversarially verify it
→ update the working installation
→ validate the improvement in later real work
```

A changed prompt, Skill, or mechanism is not automatically considered an improvement. Later engineering behavior is the held-out test.

## Architecture

Mellions is intentionally focused.

```text
Human engineering leader
        │
        │ intent, delegation, consequential decisions
        ▼
Mellions Engineer
responsibility · continuity · discipline · coordination · proof
        │
        ├────────────────┬────────────────┐
        ▼                ▼                ▼
   Claude Code         Codex        future frontier
                                      coding runtimes
```

The runtime keeps ownership of model reasoning, terminal and filesystem tools, permissions, worktrees, subagents, sandboxing, messaging, scheduling, and plugin execution.

Mellions adds deterministic mechanisms only where durable state, coordination, or a mechanically checkable invariant needs them.

The deterministic surface is written in **Go** and is designed to remain compact, inspectable, testable, portable, and operationally simple. The current Go core uses the standard library only.

## Use Mellions

### Give it a responsibility

```text
Take #1428. Establish what is true now and carry it through.
```

### Let it choose useful work

```text
/mellions:survey
```

### Continue after interruption

```bash
mellions continue
mellions assign list
```

### Leave and come back

```bash
mellions away -because "stepping out"
# ... later
mellions back
```

## Contribute

Mellions is developed in the open through reproducible engineering evidence.

The normal contribution path is:

```text
issue / reproduction when useful
→ focused branch from dev
→ implement + make check + adversarial verification
→ open PR into dev
→ maintainer verification + review
→ revise and re-verify
→ merge to dev
→ maintainers promote releases to main
```

For a fork-based contribution:

```bash
git switch -c your-change upstream/dev
# make the change
make check
git push -u origin your-change
gh pr create --base dev --fill
```

Hosted CI is currently disabled. Pull-request authors run `make check`, and
maintainers reproduce it against the current merge result before merging. See
[`CONTRIBUTING.md`](CONTRIBUTING.md) for the complete workflow, evidence
standard, and Apache-2.0 contribution terms.

## Documentation

- [`docs/benchmarks.md`](docs/benchmarks.md) — public evidence, methodology, feature benchmarks, limitations, and reproduction challenge
- [`docs/integrations.md`](docs/integrations.md) — Claude Code, Codex, model/runtime support, hooks, and future adapters
- [`docs/install.md`](docs/install.md) — installation and first session
- [`docs/playbook.md`](docs/playbook.md) — day-to-day work with Mellions
- [`docs/cli.md`](docs/cli.md) — command reference
- [`docs/architecture.md`](docs/architecture.md) — architecture and design boundaries
- [`docs/community.md`](docs/community.md) — GitHub/Discord community model and benchmark challenge
- [`docs/data-handling.md`](docs/data-handling.md) — local state, model/runtime disclosure, external effects, and operator controls
- [`docs/publication.md`](docs/publication.md) — the fresh-root release boundary and history verification
- [`examples/README.md`](examples/README.md) — usage and benchmark-reproduction examples
- [`config/CONFIG.md`](config/CONFIG.md) — configuration reference
- [`deploy/README.md`](deploy/README.md) — unattended operation
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — contributing and pull-request workflow
- [`SECURITY.md`](SECURITY.md) — security model and private reporting
- [`SUPPORT.md`](SUPPORT.md) — installation and usage support
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) — community conduct and enforcement
- [`CHANGELOG.md`](CHANGELOG.md) — public release history

## Development

```bash
make build
make check
make install
make release
```

`make check` runs the Go verification suite plus the hook and Skill checks that protect the runtime contract.

## Production use

Mellions Engineer is shaped by real, long-running, multi-repository engineering rather than demonstration tasks. Production failures are used to remove dead weight, strengthen methods that materially improve engineering quality, and repair Mellions itself before later sessions encounter the same class of problem.

That feedback loop is part of the product.

## Community

Mellions is for engineers already pushing frontier coding agents beyond isolated coding tasks.

We welcome production experience reports, benchmark reproductions and counterexamples, adversarial fixtures, runtime compatibility findings, verified runtime adapters, engineering-method improvements, onboarding improvements, and evidence-backed simplifications.

**Don't trust our benchmark. Reproduce it.**

If Mellions adds ceremony without engineering value, report it. If a frontier runtime now provides a capability better natively, help us remove the redundant layer. If a method fails, bring the failure case.

The standard is not “more automation.” The standard is:

> **Does this make the engineer more reliable, durable, disciplined, and useful while requiring less human orchestration?**

GitHub is the durable engineering record. Fast community conversation can happen elsewhere, but findings that matter should come back as issues, benchmark cases, documentation, or code.

## License, ownership, and contact

Mellions Engineer is open source under the **Apache License, Version 2.0**. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).

Copyright © 2026 **LetA Tech Ltd.**

Project, maintainer, and general communication contact: [**leta@letatech.ca**](mailto:leta@letatech.ca).

For security vulnerabilities, follow [`SECURITY.md`](SECURITY.md) and report privately rather than opening a public issue.

---

<p align="center">
  <strong>Less prompting. Less re-explaining. Less agent management.<br />More engineering carried through to completion.</strong>
</p>
