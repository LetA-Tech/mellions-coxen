<!-- Mellions Engineer | Copyright © 2026 LetA Tech Ltd. | leta@letatech.ca -->

# Contributing to Mellions Engineer

Mellions Engineer is used in real, long-running software-engineering work with frontier coding agents.

Contributions are welcome, but the standard is production engineering rather than feature accumulation.

The question for every change is:

> **Does this make the engineer more reliable, durable, disciplined, verifiable, or useful in real software engineering?**

Mellions Engineer is a project of **LetA Tech Ltd.** Project and maintainer contact: [**leta@letatech.ca**](mailto:leta@letatech.ca).

## Contribution workflow

The normal public contribution path is:

```text
find or reproduce a problem
        ↓
open / join an issue when useful
        ↓
fork or create a focused branch
        ↓
implement the smallest effective change
        ↓
run tests + adversarial / negative verification
        ↓
push the branch
        ↓
open a pull request into dev
        ↓
maintainer verification + peer review
        ↓
address findings and re-verify
        ↓
merge into dev
        ↓
release work is promoted to main by maintainers
```

### 1. Start from current `dev`

`dev` is the normal integration branch. `main` is the release branch.

If you are contributing from a fork:

```bash
git clone https://github.com/<your-user>/mellions-coxen.git
cd mellions-coxen
git remote add upstream https://github.com/LetA-Tech/mellions-coxen.git
git fetch upstream
git switch -c your-change upstream/dev
```

If you already have a checkout, refresh from `upstream/dev` before starting so the evidence you gather is against the current product.

### 2. Use an issue when it improves the engineering record

Open or join an issue before implementation when the change involves:

- a reproducible bug or regression;
- a benchmark reproduction or counterexample;
- a new runtime integration;
- a security-sensitive behavior;
- a meaningful product or architecture decision;
- work where independent discussion will improve the result.

Small documentation corrections and obviously local fixes do not require ceremonial issue creation.

Before filing something as new, search the existing issues and current code. An open issue may also describe a stale premise, so verify the present repository state rather than assuming the issue is still accurate.

### 3. Keep the branch focused

Prefer one engineering responsibility per branch and pull request.

Good branch names are descriptive rather than process-heavy, for example:

```text
fix/codex-hook-detection
bench/completion-reproduction
docs/runtime-integration-guide
```

Do not mix unrelated refactors, feature ideas, and documentation cleanup into a change that started as a specific defect.

### 4. Establish the problem before changing the product

For behavioral changes, record enough evidence to answer:

- What is actually wrong or missing?
- How can another engineer reproduce it?
- What is the root cause rather than only the symptom?
- Why does the correction belong in Mellions rather than the native coding-agent runtime or one repository?

A benchmark or behavioral claim should distinguish measured results from inference and retrospective estimates.

### 5. Implement and verify

Run the repository verification contract:

```bash
make check
```

For non-trivial behavior changes, add or update a regression test.

For session/runtime behavior, also exercise the relevant installed hook, CLI path, or fresh Claude Code/Codex session where practical.

For consequential fixes, `tests pass` is not the end of the proof. Try to falsify the claim:

1. establish the behavior before the fix;
2. implement the correction;
3. run the positive case;
4. neutralize or mutate the relevant correction where practical;
5. confirm the intended test/check fails for the intended reason;
6. restore the correction and confirm the evidence returns green.

If an adversarial arm stays green when it should fail, investigate the test before declaring success.

### 6. Push and open a pull request into `dev`

Push your branch and open a PR with **`dev` as the base branch**.

Using GitHub CLI:

```bash
git push -u origin your-change
gh pr create --base dev --fill
```

Or use GitHub's **Compare & pull request** UI and select `dev` as the base.

The repository PR template asks for:

- the engineering problem;
- root cause / evidence;
- what changed;
- verification;
- falsification / negative evidence;
- runtime impact;
- evidence boundary;
- remaining limitations;
- why the change belongs in Mellions.

Fill it with evidence, not ceremony.

### 7. Verification and review

Hosted CI is currently disabled. Pull-request authors run the repository gate
before requesting review, and maintainers reproduce it against the current
merge result:

```bash
make check
```

A green `make check` run is part of the evidence, not a substitute for reasoning
or adversarial verification. Record the commit tested and the observed exit;
the absence of a hosted check is never itself a pass.

Maintainers and contributors may challenge:

- the stated root cause;
- whether the test actually reaches the behavior;
- whether a simpler change would solve the problem;
- whether the capability now belongs in the native runtime;
- whether a benchmark claim overstates its evidence;
- whether the change adds unnecessary deterministic machinery.

Treat review findings as engineering evidence. Update the branch, rerun verification, and push the revisions to the same PR.

### 8. Merge and release

Normal contributor PRs merge into `dev` after review and verification.

`main` is the release branch. Promotion from `dev` to `main`, release tagging, package publication, and release notes are maintainer responsibilities unless explicitly delegated.

Do not open routine feature/fix PRs directly against `main` unless a maintainer requests it for a release-specific reason.

## Start here

Before changing anything substantial, read:

1. [`README.md`](README.md) — product positioning and public entry point
2. [`docs/benchmarks.md`](docs/benchmarks.md) — evidence standard and published benchmark boundary
3. [`docs/integrations.md`](docs/integrations.md) — current runtime support and adapter boundary
4. [`docs/architecture.md`](docs/architecture.md) — architecture and where mechanisms belong
5. [`docs/playbook.md`](docs/playbook.md) — how Mellions is used in real engineering work
6. the implementation and tests relevant to your change

Community participation follows [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
Installation and usage questions follow [`SUPPORT.md`](SUPPORT.md); suspected
vulnerabilities follow the private process in [`SECURITY.md`](SECURITY.md).

## Product principle

Mellions follows one architectural principle:

> **Frontier-model judgment + deterministic mechanisms.**

The frontier coding runtime already owns reasoning, tools, permissions, worktrees, subagents, sandboxing, messaging, model selection, and its native extension system.

Mellions should not rebuild those capabilities.

A new capability normally belongs in one of these places:

- the frontier runtime itself;
- a Skill or engineering method the model can apply with judgment;
- repository or tracker state;
- a runtime adapter;
- the Go core, when the property requires durable state or a mechanically checkable invariant.

Do not add deterministic machinery simply because something can be automated. Add it when evidence shows the property cannot be provided reliably enough by the model, repository, or native runtime alone.

## What we value

Strong contributions usually improve one of these areas:

- engineering responsibility and continuity;
- falsification and verification;
- recovery after interruption or compaction;
- multi-engineer coordination;
- autonomous work selection without hard-coded ranking;
- capability discovery without context inflation;
- unattended engineering safety and usefulness;
- owner reporting and reduction of unnecessary escalation;
- runtime compatibility;
- self-maintenance and self-evolution from real engineering evidence;
- documentation and onboarding clarity.

We especially value reports from real use that expose a concrete failure mode and make it reproducible.

## Engineering discipline

A change should not encourage the engineer to:

- accept an issue premise without verifying it;
- confuse a green suite with proof of the original claim;
- defer a defect discovered inside the current responsibility without establishing ownership;
- report a finding as new before checking existing project records where applicable;
- declare work complete without reconciling the actual obligation;
- treat remembered state as current truth;
- hide uncertainty behind confident language.

The expected behavior is to establish what is true, distinguish evidence from inference, and correct a claim when later evidence disproves it.

## Go and implementation quality

The deterministic surface is written in Go and is expected to remain straightforward to inspect, test, deploy, and reason about.

Prefer:

- clear data flow;
- standard-library solutions where sufficient;
- narrow interfaces;
- explicit state transitions;
- readable failure modes;
- deterministic tests;
- comments that explain current non-obvious truth rather than development history.

Avoid dependencies or abstraction layers without a concrete engineering benefit.

## Runtime integrations

A runtime integration is a real support claim, not a keyword.

A useful new adapter should include:

- reproducible installation;
- lifecycle/context delivery verification;
- confirmation that native permissions remain native;
- at least one real engineering scenario;
- `mellions doctor` or equivalent observability where appropriate;
- explicit limitations.

See [`docs/integrations.md`](docs/integrations.md).

## Documentation and public evidence

Public documentation should describe the product that exists today.

Do not publish private infrastructure, customer/user data, internal project identifiers, proprietary estate details, or historical material that is not needed to understand the public product.

For benchmark contributions, preserve the minimum defensible evidence:

- task/fixture shape;
- model/runtime where relevant;
- treatment and comparison condition;
- sample size;
- observed result;
- attribution strength;
- limitations.

See [`docs/benchmarks.md`](docs/benchmarks.md).

## Security

Mellions runs inside coding agents with meaningful repository and host authority.

Treat command parsing, credentials, hooks, unattended execution, installation, filesystem ownership, shared worktrees, and runtime permissions as security-sensitive.

Read [`SECURITY.md`](SECURITY.md) before modifying those areas.

Never include secrets, credentials, private host information, personal data, or internal infrastructure details in issues, tests, fixtures, documentation, or pull requests.

## Community

Useful community input includes:

- production retrospectives;
- failure reports with reproductions;
- benchmark reproductions and counterexamples;
- adversarial test cases;
- runtime compatibility findings;
- evidence that a method materially improves or harms engineering behavior;
- first-time installation feedback;
- proposals to remove complexity that no longer earns its place.

It is completely acceptable for the right contribution to delete a mechanism or document that a capability belongs in the native runtime instead of Mellions.

**Don't trust our benchmark. Reproduce it.**

## Copyright, contributions, and license

Copyright © 2026 **LetA Tech Ltd.**

Mellions Engineer is licensed under the **Apache License, Version 2.0**. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).

Unless you explicitly state otherwise, contributions intentionally submitted for inclusion in Mellions Engineer are provided under the terms of Apache-2.0 as described in Section 5 of the license.

Project and maintainer contact: [**leta@letatech.ca**](mailto:leta@letatech.ca).
