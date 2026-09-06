---
name: mellions-sandbox
description: Load this when uncertainty can be settled by running something — reproducing behaviour, an integration test, a hypothesis, unfamiliar code, a clean build, a bounded test dependency — in a disposable isolated environment rather than on the host. Triggers — "use the sandbox", "leta-mac-sbx", "leta-linux-sbx", "run it isolated", "reproduce this safely".
---
<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# mellions-sandbox

## The invariant

**Use a disposable, isolated, reproducible execution environment to reproduce
defects, test implementations and run adversarial or falsification
experiments, without contaminating the host.**

That is the requirement, and no isolation technology is. A method that demands
one mechanism everywhere refuses to run experiments on a host whose isolation
exists under another name, and the refusal then gets reported as a blocker.

A sandbox is evidence machinery, not merely a security wrapper. What a
disposable experiment can answer is not settled by speculation and not asked
of the owner: run it.

## Which one

```bash
case "$(uname -s)" in
    Darwin) SBX=leta-mac-sbx   ;;   # Colima VM
    Linux)  SBX=leta-linux-sbx ;;   # Docker + gVisor
esac
command -v "$SBX" >/dev/null || SBX="<this skill's directory>/scripts/$SBX"
```

They take the same options, so nothing after this point is platform-specific:

```bash
"$SBX" [-i IMAGE] [-m MEM] [-c CPUS] [-n none|bridge] [-r PATH] [-w] [-p PORT] [-C] [-N NAME] [--] [cmd ...]
"$SBX" status     # what is up
"$SBX" down       # destroy it, and say what survived
```

Defaults: `ubuntu:24.04`, 2g memory with swap pinned to it, 2 CPUs, 512 pids,
`--rm`, network **none**, every capability dropped, `-r` mounted read-only at
`/work`, `-p` published on loopback only. `-w` makes the mount writable and
runs as your uid so host files keep their ownership.

```bash
"$SBX" -r "$PWD" -- go test ./...
"$SBX" -i postgres:17-alpine -n bridge -p 5432 -- postgres
"$SBX" -m 4g -c 4 -r "$PWD" -w -- make test
```

The two boundaries differ, and the evidence says which one produced it.
gVisor interposes a syscall boundary where containers share the **host**
kernel — a Linux-host problem. On macOS Docker already runs inside the Colima
VM, so the VM is that boundary and `runsc` would be a second one inside an
existing one: **its absence there is the design, not a broken host.** Read it
as breakage and you stop on a blocker that is not there. Never weaken a
boundary while claiming the one you did not get: where `runsc` *is*
registered, dropping to `runc` is a silent downgrade.

On macOS, mount only what Colima shares — `$HOME`, `/tmp/colima`. A bind
outside them does not fail, it mounts **empty**, so the run fails for missing
sources and reads as a broken tree. Stage under `$HOME`, never in a session
scratchpad under `/private/tmp`; `leta-mac-sbx` refuses an unshared path
rather than let the experiment lie.

## Where neither helper exists

Run it under plain Docker with the same properties set explicitly, and record
that the evidence came from the generic path:

```bash
docker run --rm --network none --memory 2g --memory-swap 2g --cpus 2 \
  --pids-limit 512 --cap-drop ALL --security-opt no-new-privileges \
  --label leta.sandbox=1 --mount type=bind,src="$PWD",dst=/work,readonly=true \
  -w /work -i ubuntu:24.04 <command>
```

Windows carries no helper here: Docker Desktop over WSL2 runs that command
unchanged, and the WSL2 VM is its boundary. Anything genuinely disposable and
isolated is acceptable; naming which it was is not optional.

## The properties, whatever the mechanism

- **disposable** — destroyed after the experiment, nothing depending on it surviving;
- **bounded** — memory, CPU and pid caps set before it starts, not discovered under load;
- **minimally mounted** — the one worktree the experiment needs, read-only unless it must write;
- **minimally networked** — off unless the experiment requires it, published ports on loopback;
- **uncredentialed** — no real secret unless the experiment is about one and the partnership permits it;
- **non-contaminating** — nothing it does reaches host state a later run reads;
- **deterministically torn down** — in the turn that provisioned it, proved by an inventory.

A mechanism that cannot give one of these gives less than "run it in a
sandbox" implies. Name which one, rather than let the phrase stand for it.

## The run

Establish the host and its isolation → select the helper → mount only the
worktree the experiment needs → provision only its dependencies → run the
reproduction, the test, or the falsification arm → capture the command, the
inputs, the output and the boundary they came from → tear it down and verify.

Reach for it wherever it materially establishes evidence: reproducing a defect
off the host, verifying an integration assumption against a real dependency,
separating competing hypotheses, mutation-testing an invariant, exercising
failure and recovery, running unfamiliar or generated code, validating an
implementation before the governing gate. A sandbox result is evidence, never
authority on its own.

## Teardown

`--rm` covers an ordinary run, anonymous volumes included. `"$SBX" down`
destroys nothing: on macOS it gives the VM back, and only once no container is
left holding it.

**`leta.sandbox=1` says findable for teardown, never whose.** Repository
harnesses apply it to their own test databases, so reaping by it removes
another lane's running database — and a suite whose container vanishes
mid-run can report a false pass as easily as a false failure. Ownership is
the second label, `leta.sandbox.owner`, which `LETA_SBX_OWNER` sets: filter
on that, or remove by name what you started. Any removal takes `docker rm
-v`, since without it the image's declared volume is orphaned and the next
container silently attaches stale contents; `docker volume ls -qf
dangling=true` is what proves a teardown, not `docker ps` alone. A harness needing a service to outlive one command labels it
and installs teardown in the same script on an `EXIT INT TERM` trap. A start
command without bounded cleanup is incomplete test infrastructure, and an
experiment whose environment is still up is unfinished, not finished.

## Files

- `SKILL.md` — the method.
- `scripts/leta-linux-sbx` — Docker + gVisor.
- `scripts/leta-mac-sbx` — Colima, the same CLI, the VM as the boundary.

Both are small on purpose. This installation does not own a sandbox daemon, a
container scheduler, or a second orchestration system.
