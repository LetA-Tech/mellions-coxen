<!-- Mellions Engineer | Copyright © 2026 LetA Tech Ltd. | leta@letatech.ca -->

# Security Policy

Mellions Engineer is an open-source project licensed under Apache-2.0 and used in real software-engineering work with coding agents that may have meaningful repository and host authority.

Security reports should therefore be treated as production engineering issues rather than ordinary public bug reports.

## What the security model rests on

Mellions adds no second runtime permission system and no credential store of its own. Permissions, tools, sandboxing, credentials, model settings, and other runtime authorization remain the responsibility of the coding-agent runtime and operator environment.

Mellions provides engineering discipline and narrow deterministic protections where a mechanically checkable invariant needs them. It does not claim that a prompt, identity file, or partnership record is a security boundary.

Owner-reserved or destructive effects should be constrained by the environment that actually controls them: repository permissions, branch protection, runtime permissions, credentials, sandboxing, network policy, and other host/runtime controls.

The unattended configuration shipped with Mellions provides an additional runtime-applied deny surface for headless use. Operators remain responsible for reviewing unattended permissions before enabling autonomous work on their systems.

## Security-sensitive surfaces

Treat changes in these areas as security-sensitive:

- installation and runtime registration;
- session-start and lifecycle hooks;
- command parsing and shell construction;
- pre-tool guards and deterministic checks;
- unattended execution and shift runners;
- filesystem/worktree ownership and shared-tree protection;
- credential or secret-adjacent behavior;
- runtime permission or trust boundaries;
- any integration that can broaden the actions available to an agent.

Security-sensitive pull requests should include explicit negative/adversarial verification where practical, not only a green regression test.

## Reporting a vulnerability

Report suspected vulnerabilities privately to **LetA Tech Ltd.** at
[**leta@letatech.ca**](mailto:leta@letatech.ca).

**Do not open a public GitHub issue for an unpatched vulnerability.**

Please do not include secrets, tokens, private keys, customer/user data, exploit payloads against production systems, or sensitive infrastructure details in public discussions, issues, fixtures, benchmark material, or pull requests.

When practical, include:

- affected version or commit;
- runtime and version;
- operating system;
- the affected Mellions surface;
- configuration shape with secrets removed;
- the smallest safe reproduction you can provide;
- expected impact;
- any mitigation you have already confirmed.

We may ask for additional reproduction detail privately before determining whether and how to disclose the issue publicly.

## Coordinated disclosure

Please give maintainers a reasonable opportunity to reproduce, fix, verify, and release a correction before publishing exploit details for an unresolved vulnerability.

After remediation, durable public information may be recorded through release notes, a security advisory, documentation, or a public issue where appropriate and safe.

## Secrets

Do not commit populated environment files, tokens, private keys, production credentials, or private infrastructure configuration.

`.gitignore` can reduce accidental commits; it is not a security boundary. Review staged changes and test fixtures before pushing them to a public repository.

## What is not a security guarantee

Mellions improves engineering reliability. It does not guarantee that a frontier model will never make a bad decision or that an operator environment is secure.

The project does not claim:

- formal verification of agent behavior;
- isolation beyond the runtime/host controls actually configured;
- protection from credentials the runtime is already authorized to use;
- a security certification or independent security audit.

## Project ownership and license

Mellions Engineer is developed and maintained by **LetA Tech Ltd.**

Copyright © 2026 LetA Tech Ltd.

Licensed under the **Apache License, Version 2.0**. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).

Security and project contact: [**leta@letatech.ca**](mailto:leta@letatech.ca).
