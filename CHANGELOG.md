# Changelog

All notable Mellions Engineer changes intended for public users are recorded here.

## Unreleased

No user-facing changes have been recorded since `0.1.0`.

## 0.1.0 — 2026-09-03

### First public release

- Repositioned Mellions Engineer around engineering responsibility, reliability, falsifiable completion, and long-running agent work.
- Added a fast-scan evidence section to the README.
- Added [`docs/benchmarks.md`](docs/benchmarks.md) with public methodology, feature-level evidence, limitations, and benchmark-reproduction guidance.
- Added [`docs/integrations.md`](docs/integrations.md) with an explicit Claude Code / Codex integration matrix and truthful future-runtime boundaries.
- Added LetA Tech branding to the repository landing page.
- Rewrote contribution guidance for sustained production use and evidence-backed community development.
- Added a complete fork/branch/pull-request contribution workflow targeting `dev`.
- Added GitHub issue forms and an evidence-focused pull-request template.
- Established `make check` as the pull-request verification contract. Hosted CI
  is currently disabled; authors and maintainers run the gate locally against
  the reviewed commit and merge result.
- Adopted the **Apache License, Version 2.0**, with LetA Tech Ltd. attribution and `leta@letatech.ca` as the project contact.
- Published the source at [`LetA-Tech/mellions-coxen`](https://github.com/LetA-Tech/mellions-coxen) with a clean public history and protected `dev` and `main` branches.

### Engineering behavior

Current public capabilities include:

- durable engineering identity and responsibility;
- program and partnership context;
- autonomous engineering-state survey and work selection;
- responsibility continuity across sessions;
- engineering falsification and adversarial verification methods;
- intelligent escalation inside delegated boundaries;
- parallel-engineer awareness and claims;
- attended and unattended operation;
- evidence-backed learning and self-evolution;
- Claude Code and Codex plugin integrations.

### Distribution and verification

- Added reproducible release archives for macOS and Linux on `amd64` and `arm64`, with SHA-256 checksums.
- Verified the complete local gate, public-document links, privacy controls, release archive contents, and installation from a fresh public clone.
- Kept public release notes focused on product behavior without exposing private project history, internal issue identifiers, infrastructure, or estate-specific production records.

Codex users must review and trust Mellions hooks in `/hooks`; see [`docs/integrations.md`](docs/integrations.md) for the current runtime support boundaries.
