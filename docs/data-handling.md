<!-- Mellions Coxen | Copyright © 2026 LetA Tech Ltd. | leta@letatech.ca -->

# Data handling

Mellions Coxen is local engineering tooling, not a hosted Mellions service.
The Go binary contains no product telemetry client. That does not make a
Mellions session local-only: the configured coding-agent runtime, model
provider, repository host, and tools determine where engineering data goes.

## What is stored locally

Configuration chooses the work roots, assignment root, report root, partner
and program directories. Depending on the features used, Mellions writes:

- assignment records and their worktrees;
- reports, unattended-shift logs, replies, and runner state;
- partnership and program documents;
- session-presence and awareness-delivery records;
- owner attended/away state and digest markers.

The coding-agent runtime separately owns its session transcripts, caches,
credentials, trust records, and extension registration. Mellions may read the
runtime's registration, trust, process, and session metadata needed for install,
doctor, presence, continuity, and renewal behavior; those runtime files remain
the runtime's state.

## What reaches a model runtime

Session-start and awareness hooks can place identity, partnership, program,
assignments, survey findings, peer presence, and owner digest material into the
agent context. Anything in those records may therefore be sent by the runtime
to its configured model provider. Do not put a secret or unnecessary personal,
customer, or proprietary data in a record merely because the record is local.

The runtime's provider terms, retention controls, regional settings, and
enterprise policy govern that transmission. Mellions cannot strengthen or
erase a provider's retention after data has been sent.

## External effects

The binary has no direct hosted API. It invokes operator-installed tools such
as `git`, `gh`, Claude Code, and Codex. Those tools—and any tools the agent uses
through its runtime—can read repositories, contact external services, create
tracker records, or publish content according to their own configuration and
the authority available to the session.

Before attaching logs or reports to GitHub, review them for repository names,
paths, hostnames, user data, source excerpts, and operational details. Public
fixtures should be synthetic.

## Credentials and protections

Mellions has no credential store of its own. Credentials available to a coding
agent remain controlled by the runtime, operating system, environment, and
external tools. The credential-read pre-tool guard rejects a narrow class of
commands that would print credential-bearing files into a transcript. It is a
specific safeguard, not a general data-loss-prevention system, and it can be
disabled explicitly for a session.

Use runtime permissions, scoped credentials, repository access controls,
sandboxing, network policy, and separate test accounts where the effect must be
impossible. Review [`SECURITY.md`](../SECURITY.md) before unattended use.

## Retention and removal

Operators choose retention for the configured Mellions roots, source
repositories, runtime transcripts, and external services. Stop active sessions
and runners before removing local state, and verify the configured paths with
`mellions doctor` and the configuration file rather than assuming defaults.
Deleting local files does not delete tracker comments, git objects already
pushed, provider records, or copies held by collaborators.
