# Community

Mellions Engineer is being prepared for a public engineering community built around one principle:

> **If a claim matters, make it reproducible. If a finding matters, bring it back to GitHub.**

## Where conversation belongs

### GitHub — durable project authority

Use GitHub for anything that should survive chat history:

- bugs and regressions;
- benchmark reproductions;
- runtime integration proposals;
- design and implementation decisions;
- documentation corrections;
- security reports through the process in [`../SECURITY.md`](../SECURITY.md);
- pull requests;
- release notes;
- evidence that changes a public claim.

GitHub is the engineering record.

### Discord — fast community conversation

A lightweight Mellions Engineer Discord presence is planned for:

- installation and usage questions;
- Claude Code and Codex integration discussion;
- experiments with other capable models and coding-agent runtimes;
- benchmark reproduction;
- interesting agent-behavior findings;
- falsification and adversarial-verification discussion;
- contributor help;
- early improvement ideas.

The initial structure should stay deliberately simple:

```text
Mellions Engineer
├── community / general
└── benchmarks-and-experiments
```

Threads are enough for focused integration or model experiments until community volume proves more structure is useful.

No Discord invite is published in the repository yet. Add it here and to the README only after the community server/channel is actually created and ready for public use.

## The authority split

> **GitHub = durable project authority and engineering record**  
> **Discord = fast community conversation and collaboration**

A useful Discord conversation should not disappear in Discord if it establishes something durable.

Promote material outcomes to GitHub as appropriate:

```text
interesting conversation
        ↓
reproducible finding
        ↓
GitHub issue / benchmark case / documentation / pull request
        ↓
reviewable project knowledge
```

## Challenge the project

Mellions is explicitly interested in evidence that contradicts its own claims.

Useful community work includes:

- reproduce the published benchmarks;
- run treated vs untreated agent sessions;
- find cases where a Mellions method adds ceremony without value;
- test whether a native runtime now makes a Mellions mechanism redundant;
- contribute adversarial fixtures;
- compare engineering behavior across models while keeping the runtime and task controlled where possible;
- test long-running recovery, owner escalation, and parallel-agent behavior;
- build and verify integrations for additional coding-agent runtimes.

**Don't trust our benchmark. Reproduce it.**

See [`benchmarks.md`](benchmarks.md) for the public evidence standard and [`../CONTRIBUTING.md`](../CONTRIBUTING.md) for contribution requirements.
