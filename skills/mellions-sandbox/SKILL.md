---
name: mellions-sandbox
description: Load this when uncertainty can be settled by running something — reproducing behaviour, an integration test, a hypothesis, unfamiliar code, a clean build, a bounded test dependency — in a disposable gVisor sandbox rather than on the host. Triggers — "use the sandbox", "leta-sbx", "gVisor", "run it isolated", "try it in a container", "reproduce this safely".
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# mellions-sandbox

A sandbox is an engineering reasoning tool, not merely an isolation feature. Use it when an experiment can turn uncertainty into evidence.

Typical flow:

```text
read code / architecture / history
        ↓
form hypothesis
        ↓
run a bounded sandbox experiment
        ↓
inspect evidence
        ↓
refine or reject hypothesis
```

Do not ask the owner a technical question merely because the answer is not immediately visible. When a disposable experiment can answer it safely, investigate first.

## Select one sandbox implementation

Prefer a compatible sandbox already supplied by the host:

```bash
if command -v leta-sbx >/dev/null 2>&1; then
    SANDBOX="$(command -v leta-sbx)"
else
    # the bundled fallback sits beside this file: <this skill's directory>/scripts/leta-sbx
    SANDBOX="<this skill's directory>/scripts/leta-sbx"
fi

test -x "$SANDBOX" || {
    echo "No compatible host sandbox and the bundled fallback is unavailable" >&2
    exit 1
}
```

Use exactly the selected implementation for the task. Do not start a second sandbox layer when the host already provides the compatible capability.

The bundled wrapper ships in this installation and intentionally has the same CLI as the host implementation, so selecting between them changes nothing about how the sandbox is driven. Where it came from is recorded with the repository rather than in the corpus, which is provenance and not something this skill reads.

## Host prerequisite

Both paths require Docker with the `runsc` gVisor runtime registered:

```bash
docker info --format '{{json .Runtimes}}' | grep '"runsc"'
```

If `runsc` is absent, stop. Never silently degrade to `runc`; that changes the security boundary while pretending the experiment is still sandboxed.

On a supported Linux host where `runsc` is already installed but not registered, the worker implementation reports the repair explicitly:

```bash
sudo runsc install
sudo systemctl restart docker
```

Installing Docker or gVisor on an unmanaged host is an operator/environment action, not something to improvise during an engineering experiment.

## CLI

```bash
"$SANDBOX" [options] [--] [command ...]
```

| Option | Effect |
|---|---|
| `-i IMAGE` | image, default `ubuntu:24.04` |
| `-m MEM` | memory cap, default `2g`; swap pinned to the same value |
| `-c CPUS` | CPU cap, default `2` |
| `-n MODE` | network `none` by default, or `bridge` |
| `-r PATH` | bind `PATH` at `/work`, read-only |
| `-w` | make the mount writable |
| `-p PORT` | publish on `127.0.0.1` only; implies bridge networking |
| `-C` | keep default capabilities; otherwise all capabilities are dropped |
| `-N NAME` | container name |

Examples:

```bash
"$SANDBOX" -r "$PWD" -- go test ./...
"$SANDBOX" -i postgres:17-alpine -n bridge -p 5432 -- postgres
"$SANDBOX" -m 4g -c 4 -r "$PWD" -w -- make test
```

No command opens an interactive shell.

## Safety defaults

The proven wrapper applies:

- `--rm`;
- `--runtime=runsc`;
- memory and CPU caps;
- swap pinned to the memory cap;
- `--pids-limit 512`;
- `--network none` unless explicitly enabled;
- `--security-opt no-new-privileges`;
- `--cap-drop ALL` unless explicitly overridden;
- label `leta.sandbox=1`;
- read-only bind mounts by default;
- loopback-only published ports;
- git usable inside the mount: `safe.directory` is passed through git's
  environment configuration, since a read-only mount is read by container-root
  while the host path is the invoking user's. And where the repository's git
  directory lies outside the mounted path — a linked worktree, or any
  subdirectory of a repository — that whole directory is mounted read-only at
  its own absolute path so the `.git` pointer resolves. That carries every
  branch, every other worktree's state and every reflog of the parent
  repository, which is wider than the path named with `-r`, so it is announced
  on stderr rather than added silently. Mount a standalone clone instead where
  the sandboxed code should not see them.

A writable mount runs as the invoking uid/gid so writes work without restoring broad container capabilities and host files keep the correct ownership.

## When to use it

Use sandbox experimentation when it materially helps establish evidence, including:

- reproducing a defect without contaminating the host;
- verifying an integration assumption against a real dependency;
- comparing competing implementation hypotheses;
- mutation-testing an invariant;
- inspecting failure/recovery behavior;
- running unfamiliar or generated code;
- testing a dependency or provider behavior that static inspection cannot settle;
- validating an implementation before governing verification.

A sandbox result is evidence, not automatic authority. Record the command, inputs, relevant output, and what conclusion the experiment supports.

## Network discipline

Networking is off by default. Enable it only when the experiment requires it. Do not place credentials in a sandbox unless the experiment specifically requires them and the governing security rules permit it.

## Teardown

Ordinary runs use `--rm`. Inspect leftovers when a run is interrupted:

```bash
docker ps --filter label=leta.sandbox=1
```

For a test harness that genuinely needs service containers to outlive one command, label every container `leta.sandbox=1` and install teardown in the same script using an `EXIT INT TERM` trap. A start command without bounded cleanup is incomplete test infrastructure.

## Files

- `SKILL.md` — discovery and operating method.
- `scripts/leta-sbx` — the portable fallback this bundle carries.

The bundled script is intentionally small. This installation does not own a sandbox daemon, container scheduler, or second orchestration system.
