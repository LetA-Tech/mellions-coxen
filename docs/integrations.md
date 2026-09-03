# Integrations

> **Bring your frontier coding agent. Mellions Coxen adds the engineering discipline around it.**

Mellions is designed around engineering runtimes, not around one model vendor. The model remains responsible for reasoning, implementation, tool use, and technical judgment. Mellions adds durable responsibility, situational context, engineering methods, coordination, verification discipline, and recovery around that runtime.

The important distinction is between a **model** and an **integration**. A model can benefit from Mellions only when it is running inside a runtime that Mellions can actually reach.

## Support today

| Runtime / model | Status | How Mellions connects | Notes |
|---|---|---|---|
| **Claude Code** | **First-class, production-used** | Native plugin registration, Skills, slash commands, session lifecycle hooks, pre-tool awareness, prompt awareness, and pre-compaction renewal | Deepest production use. Current unattended shift runner drives Claude Code. |
| **Codex** | **Implemented, interactive** | Plugin marketplace registration, Skills, slash commands, and the Mellions hook manifest | Codex requires explicit hook trust. The current unattended shift runner does not drive Codex. This path has less production mileage than Claude Code. |
| **Models used inside an integrated runtime** | **Inherited from the runtime** | No model-specific Mellions adapter is required when the runtime already exposes the model through the supported integration | Mellions is intentionally not coupled to one model generation. |
| **Opus 5** | **Runtime-dependent** | When used through Claude Code or another future supported runtime, it inherits that runtime's Mellions integration | This is not a separate direct model integration. |
| **Fable 5** | **Integration target / experimental usage context** | No dedicated first-class adapter is shipped today | Do not interpret references to Fable experiments as a public runtime-support claim. |
| **Kimi 3** | **Future/community integration target** | No dedicated first-class adapter is shipped today | A compatible engineering runtime needs an adapter before Mellions can automatically supply lifecycle context. |
| **Other CLI / coding-agent runtimes** | **Adapter-ready conceptually; not automatically integrated** | The `mellions` CLI and shell-based mechanisms are reusable building blocks, but the repository does not ship a universal lifecycle adapter | Community integrations are welcome when they preserve the runtime's native capabilities rather than hiding them behind a lowest-common-denominator wrapper. |

## Claude Code integration

The Claude Code path is the most mature integration in the repository.

`mellions install` registers Mellions as a Claude Code plugin. The plugin supplies Skills and commands and uses lifecycle hooks to surface the engineering context at the moments it matters.

Current hook surfaces include:

| Claude lifecycle surface | Mellions use |
|---|---|
| `SessionStart` | Restore identity, engineering methods, partnership/program context where relevant, work in flight, and owner-facing state |
| `UserPromptSubmit` | Surface relevant situational awareness before the next turn |
| `PreToolUse` | Add narrow awareness and deterministic protection around relevant operations |
| `PreCompact` | Preserve the durable engineering responsibility before context renewal |

Mellions does not replace Claude Code permissions, tools, worktrees, subagents, teams, MCP configuration, sandboxing, or model selection. Those remain Claude Code's responsibility.

## Codex integration

Codex is also implemented as a plugin path.

`mellions install` registers the Mellions marketplace and plugin with Codex. Skills and commands are available through the plugin, while lifecycle hooks require Codex's own explicit trust mechanism before they run.

That trust boundary is important: Mellions does not bypass it or create a second permission system.

Run:

```bash
mellions doctor
```

to see whether Codex is installed and whether the hooks required by the current manifest are trusted.

The current Codex path is interactive. Mellions' unattended shift runner is currently Claude Code-specific.

## What a future runtime integration needs

A new runtime does **not** need to reimplement Mellions.

A useful adapter needs enough lifecycle access to make the same engineering semantics available while leaving the runtime itself in control. Depending on the runtime, that usually means equivalents for:

1. **session start / resume** — deliver durable identity, relevant context, and work in flight;
2. **situational awareness** — surface facts when the engineering situation changes rather than loading everything permanently;
3. **engineering methods** — make Mellions Skills or equivalent reusable methods discoverable;
4. **tool/action awareness** — allow narrow checks where a deterministic property genuinely needs protection;
5. **context renewal / recovery** — preserve responsibility across compaction, restart, or a fresh session;
6. **native permissions** — inherit the runtime's authorization model rather than inventing another one.

The adapter should expose the runtime's strengths instead of reducing every coding agent to the same generic API.

## Model-agnostic does not mean integration-agnostic

Mellions is deliberately model-agnostic in its engineering philosophy:

```text
frontier engineering intelligence
            +
Mellions engineering discipline
```

But automatic integration is a concrete engineering claim.

We therefore distinguish:

- **supported now** — the repository contains and verifies the integration;
- **inherited through a supported runtime** — the model is selected by that runtime;
- **future/community target** — the product philosophy applies, but an adapter still needs to be built and verified.

If you build an adapter for another capable engineering runtime, include a reproducible installation path, lifecycle verification, and at least one real engineering scenario showing that Mellions context and methods reached the agent at the correct time.

See [`benchmarks.md`](benchmarks.md) for how we evaluate behavioral claims and [`../CONTRIBUTING.md`](../CONTRIBUTING.md) for the contribution standard.
