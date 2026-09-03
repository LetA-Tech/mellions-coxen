# Mellions Engineer — Evidence & Benchmarks

> **What Mellions changes is not raw model intelligence. It changes engineering discipline: ownership, verification, continuity, coordination, and the probability that a capable agent actually finishes the job correctly.**

Mellions Engineer is developed through real, long-running software engineering with frontier coding agents, plus controlled comparisons where the same model and task can be run with and without a specific Mellions behavior.

This page is the public evidence layer. Private project names, internal architecture, proprietary workflows, and raw production traces are intentionally excluded.

## The fast answer

| Question | What the evidence says |
|---|---|
| **What problem does Mellions solve?** | Frontier agents are capable, but they still stop early, ask humans to make ordinary engineering decisions, trust green tests too easily, lose responsibility across interruptions, and collide with parallel work. |
| **What changes?** | Mellions pushes the agent toward end-to-end ownership, evidence before confidence, adversarial verification, explicit completion, durable responsibility, and fewer unnecessary escalations. |
| **How much did it improve?** | In one controlled same-model trial, resolution of an already-proven in-scope defect moved from **1/3 to 3/3**. In a nine-hour production retrospective, Claude estimated correct-completion probability at roughly **60–65% native-alone vs 85–90% with Mellions** for that task. |
| **What is the strongest mechanism?** | **Falsification / adversarial verification.** It repeatedly caught green tests that were green for the wrong reason and caused already-published conclusions to be retracted. |
| **Why should I care?** | The expensive failure is not an agent that crashes. It is an agent that confidently says the work is done when the engineering obligation is not actually satisfied. Mellions is designed around that failure mode. |

## Three results worth remembering

### 1. Established defect ownership: **1/3 → 3/3**

A controlled trial used the same frontier model, the same fixture, and the same attended-session framing. The agent was asked to fix one defect while another material defect existed in the same code path.

Before the responsibility correction, only **1 of 3** sessions fixed the second defect after proving it was real; the other two handed it back to the human as an optional decision.

With the Mellions responsibility method present, **3 of 3** sessions fixed the established in-scope defect instead of asking for permission.

**Why it matters:** once the agent has already established that a defect is real and belongs to the work it owns, asking the human to say “yes, fix it” adds management without adding engineering judgment.

---

### 2. Falsification changed a production outcome

In a long-running production implementation, the agent ran **13 isolated adversarial verification arms** rather than stopping at a green suite.

Those attacks caused it to:

- retract a conclusion it had already published;
- catch **two latent defects** that ordinary completion would have left behind;
- continue verifying until the claimed property, not merely the implementation, held.

Claude's own retrospective was explicit:

> “mellions-falsification alone justifies it.”

And on the counterfactual:

> “Without falsification I'd have shipped ... two latent defects in new production code.”

**Why it matters:** a frontier model can be both intelligent and convinced by its own inadequate test. Mellions makes “prove the fix can fail when neutralized” a normal engineering move rather than an exceptional one.

---

### 3. Completion claims became mechanically challengeable

A completion check was calibrated against **173 historical handoffs**:

- **0 honest handoffs** were challenged;
- **both known false-completion cases** were caught.

The point is not to create approval bureaucracy. The check is silent on ordinary work and intervenes only when an agent claims completion without showing what closed obligation set it reconciled.

**Why it matters:** autonomous engineering becomes much more useful when “done” has a technical meaning stronger than “the agent feels finished.”

## Feature evidence at a glance

| Mellions capability | Engineering problem | Observed behavior change | Public result | Evidence strength |
|---|---|---|---|---|
| **Falsification / adversarial verification** | Green tests can prove the wrong property | Agent attacks its own fix, checks whether mutations really landed, and treats suspiciously-green arms as findings | 13 production arms caught a false published conclusion + 2 latent defects; separate controlled treatment produced a stronger cross-process proof than the untreated run | **Strong** |
| **End-to-end ownership** | Agents prove a defect, then ask the human whether to fix it | Established in-scope findings become work the agent owns | **1/3 → 3/3** in the same-model attended trial | **Strong** |
| **Completion discipline** | Agents declare victory before reconciling the full obligation | Completion is challenged when the closed set was not established | 173 historical handoffs: 0 honest challenged; 2 known false completions caught | **Strong for the check** |
| **Self-correction** | Agents become committed to conclusions they already published | Adversarial evidence can trigger explicit retraction and remediation | Observed in production after a previously published conclusion failed its falsification arm | **Strong production evidence** |
| **Self-evolution** | Agent-support methods go stale or fail in new situations | Production failures can become tested improvements, installed and then judged by later real work | Full observe → repair → adversarial test → install → live verification loop exercised in production | **Strong capability evidence; longitudinal effect still accumulating** |
| **Continuity & recovery** | Responsibility spans compaction, restarts, and fresh sessions | Durable state preserves the engineering obligation while current reality is rechecked | One long production run estimated **20–30 minutes** of re-derivation avoided; native compaction still carried most context | **Moderate; incremental rather than transformational** |
| **Intelligent escalation** | Human presence turns ordinary engineering into “want me to continue?” | Agent investigates and acts inside delegation; genuine product/contract decisions still escalate | Controlled ownership result above + repeated production decision packages | **Moderate to strong** |
| **Autonomous work discovery** | Human has to select every next ticket | Agent surveys the engineering estate, chooses useful actionable work, and explains what it passed over | Repeated production use without a hand-selected task | **Good capability evidence; no matched quality benchmark yet** |
| **Parallel-work awareness** | Multiple agents duplicate or damage each other's work | Active responsibility and overlapping work become visible before action | Real multi-session use has prevented duplicate work after earlier collisions exposed the need | **Moderate production evidence** |
| **Independent perspectives / collaboration** | One agent's assumptions become the whole team's assumptions | A separate engineer can broaden scope or challenge the first reading | Production peer review materially expanded discovered verification scope | **Useful, but attribution is mixed with native runtime capabilities** |
| **Reporting** | Humans must replay hours of transcript to understand state | Reports emphasize established change, residuals, and real owner decisions | Repeated production use | **Qualitative; management-time savings not yet instrumented** |
| **Program awareness / raw discovery** | Agent needs context beyond one file | Can provide program context and situational signals | In one major production retrospective, program context added no measurable value and raw discovery uplift was not established | **Mixed / intentionally not overclaimed** |

## The overall case-study estimate

After one approximately nine-hour production implementation, Claude compared its likely outcome using native capabilities alone against the actual Mellions-assisted run.

| Retrospective estimate for that production task | Probability of correct completion |
|---|---:|
| Native agent + its normal memory/compaction | **~60–65%** |
| Agent with Mellions Engineer | **~85–90%** |

Claude estimated the net positive effect at roughly **25–30%**, after accounting for Mellions' context and operational cost. It attributed most of the difference to **falsification discipline and refusal to defer established defects**.

This is **not** a universal benchmark score, SWE-bench result, pass@1 measurement, or independent population estimate. It is a retrospective case-study estimate anchored to observable production failures that the Mellions-assisted run caught before completion.

That distinction matters: we think the result is meaningful enough to publish, but not broad enough to pretend it measures every coding task.

## What Claude said — including the inconvenient parts

The production retrospective was not an unconditional endorsement. That is useful evidence in itself.

On the strongest feature:

> “mellions-falsification alone justifies it.”

On implementation quality:

> “Without falsification I'd have shipped ... two latent defects in new production code.”

On continuity:

> “Continuity improvement: small. 20–30 minutes of re-derivation.”

That is the pattern we want to preserve publicly: **strong claims where the mechanism changed the outcome, conservative claims where native frontier-agent behavior already does most of the work.**

## What Mellions does *not* currently prove

The evidence does **not** support saying that Mellions makes the underlying model generally smarter.

We do not currently claim a demonstrated increase in:

- raw algorithmic intelligence;
- architecture reasoning ability;
- spontaneous defect-discovery radius;
- generic coding benchmark accuracy.

The strongest evidence is downstream of intelligence:

```text
capable model
    ↓
finds or receives engineering evidence
    ↓
Mellions changes how seriously that evidence is owned, attacked, preserved, coordinated, and completed
```

That is a narrower claim, and a more useful one for serious engineering teams.

## Why engineers should try it

Mellions is most interesting if you already like Claude Code, Codex, or another frontier coding agent but recognize one of these problems:

- **You became the agent's manager.** You keep choosing the next task, restating context, approving obvious follow-up work, and checking whether “done” means done.
- **Green tests are not enough.** You want the agent to actively try to disprove its own conclusion before it ships.
- **Work lasts longer than one context.** You need the responsibility to survive restarts, compaction, handoff, or a different engineer taking over.
- **You run agents in parallel.** You need awareness of overlapping responsibility without building a giant orchestrator.
- **You want unattended progress without unattended confidence theater.** The agent should keep working when delegated, but surface the few decisions that genuinely belong to a human.
- **You want the engineering system to learn from production mistakes.** Not by adding vague memory, but by turning reusable lessons into tested future behavior.

If those are not your problems, Mellions may not add much. If they are, the current evidence says the largest payoff is not more code generation — it is **more trustworthy engineering completion with less human orchestration**.

## Methodology

We separate three evidence types.

### Controlled behavior comparisons

Where possible, hold constant:

- model;
- task / fixture;
- prompt and session framing;
- available tools;
- runtime environment;

and vary the Mellions method or mechanism under test.

These support the strongest feature-level causal claims, but sample sizes are currently small.

### Production case studies

Long-running real engineering shows whether a method matters under the noise that fixtures omit: interruptions, ambiguous requirements, multiple repositories, parallel engineers, stale assumptions, and real consequences.

Production evidence is more ecologically valid but more confounded, so we avoid attributing every good outcome to Mellions.

### Deterministic replay and mutation

For mechanically checkable properties, we prefer replaying real command shapes, historical records, and deliberately broken variants. These are useful for measuring false positives and proving that a guard fails when the property it protects is removed.

## Public evidence vs private engineering records

The underlying production reports contain private repository names, internal issue identifiers, infrastructure details, and implementation-specific context. They remain part of the private engineering record and are **not** copied into this public benchmark.

The public layer keeps only what is necessary to evaluate the claim:

- the engineering behavior under test;
- treatment vs untreated shape where available;
- sample size;
- observed result;
- attribution strength;
- limitations.

Where a result can be reproduced safely without private context, we prefer a public controlled fixture or replay-style test over publishing private production traces.

## Help us challenge it

Mellions is open source because these claims should be challenged, not protected by a demo.

If you use it:

- run the same engineering task with and without a Mellions method;
- bring adversarial fixtures where the current method fails;
- measure unnecessary escalations and false completion;
- test long-running recovery and parallel-agent behavior;
- report cases where Mellions adds ceremony but no engineering value;
- contribute a smaller or stronger mechanism when the evidence supports it.

The project should get better by surviving skeptical engineers.

> **The goal is not to make an agent look autonomous. The goal is to make a strong agent more trustworthy when you give it real engineering responsibility.**
