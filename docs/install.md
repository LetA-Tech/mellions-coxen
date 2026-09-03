<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# Installing Mellions

From nothing to the first session in which the engineer is Mellions. Every
step here is what the product does today; where a runtime needs a step by
hand, it says so.

## What you are installing

Two things, and both are needed:

1. **The `mellions` binary** — a small Go program the sessions and the hooks
   call for facts a session cannot establish about itself: what is in flight,
   who else is here, what the estate needs, who it works with. It keeps its
   records outside every repository, under a *report root* (default
   `~/mellions`).
2. **The plugin** — the identity (`agents/mellions.md`), the hooks that deliver
   it and the situational context at every session start, the Skills, and the
   slash commands. It is one directory registered with Claude Code and with
   Codex as a plugin marketplace of one plugin, `mellions@mellions`.

The plugin's hooks run the binary. Installing the plugin without the binary
gives a session the identity and the Skills and nothing else; installing the
binary without the plugin gives you a CLI and a session that does not know it
is Mellions. `make install` does both.

## Requirements

- Go 1.26.1 or newer (`go version`).
- `git`, and `gh` authenticated against GitHub for the repositories
  (`gh auth status`). The survey, the claims and the assignment
  records read and write the tracker through `gh`.
- Claude Code, Codex, or both, already installed and signed in.
- macOS or Linux. The hooks are bash and the binary shells out to `git` and
  `gh`; there is no Windows build.
- `python3` on the host that will run unattended shifts (the shift follower is
  Python; interactive use needs none).

## 1. Install

```bash
git clone https://github.com/LetA-Tech/mellions-coxen
cd mellions-coxen
make install                       # /usr/local/bin/mellions; or:
make install PREFIX=~/.local       # ~/.local/bin/mellions, no sudo
```

`make install` builds `bin/mellions`, copies it to `$(PREFIX)/bin`, then runs
`mellions install -from .`, which registers this checkout with every runtime it
finds on the machine:

```
# Installing Mellions

Source: /home/you/mellions-coxen
        a local checkout — whether a runtime reads it in place or copies it is
        the runtime's own decision, and is reported below from its own records

## Claude Code — /home/you/.local/bin/claude
  ...
## Codex — /home/you/.local/bin/codex
  ...
## What the runtime will load next
  registry     mellions@mellions (0.1.0)
  loads from   /home/you/mellions-coxen
  read         in place — mellions is a directory marketplace
  inert copy   /home/you/.claude/plugins/cache/mellions/mellions/0.1.0
  enabled      yes, in /home/you/.claude/settings.json
  hooks        9 SessionStart hooks in .../hooks/hooks.json

A process launched from now on loads the plugin out of /home/you/mellions-coxen
itself, not out of a copy. What is committed there is what the next session
runs, so `git pull` there deploys hooks, Skills, commands and the agent —
everything except this binary, which `make install` puts on PATH. Running
this command again is not what deploys them, and does not have to.

A process already running loads neither: a runtime binds a session's hooks
when it launches, and neither /clear nor /compact re-resolves them.
```

**`loads from` is the line that matters, and it is not always the copy.** A
marketplace added from a local path is a `directory` source, and Claude Code
reads the plugin out of that directory: hooks, Skills, commands and the agent
all come from the checkout, and the copy under `~/.claude/plugins/cache/` is
written once by the installer and never read again. A marketplace added from
`owner/repo` or a git URL is fetched and copied, and there the copy is what
loads. `known_marketplaces.json` is the record that decides which;
`installed_plugins.json` names the copy either way and so cannot answer it.

The consequence for a checkout install is worth stating plainly: **`git pull`
in the checkout deploys everything except the `mellions` binary.** A commit
landing there changes what the next session on that host runs, with no install
step in between — which is what `mellions doctor`'s `load path commit` line
exists to make visible.

Read the last paragraph of the output literally. A Claude Code or Codex session
that was already open when you installed is not Mellions and cannot become
Mellions; start a new one. `mellions install` names the live sessions on the
machine that started before this registration, so you know which ones.

Useful forms:

```bash
mellions install -from .                 # again, after git pull
mellions install -runtime claude         # one runtime only
mellions install -dry-run                # print the runtime commands, run none
mellions install -from LetA-Tech/mellions-coxen   # from the published source
```

## 2. Configure

```bash
mellions config init                     # writes ~/.mellions/config.json
mellions config path                     # where it is
mellions config show                     # what it says
```

Edit the file. The keys that matter on day one:

```json
{
  "owner": "your-github-org",
  "repos": ["service-a", "service-b"],
  "work_root": "/home/you/workspace",
  "report_root": "/home/you/mellions"
}
```

| key | meaning |
|---|---|
| `owner` | the GitHub organisation or user the repositories live under |
| `repos` | the repositories the engineer is responsible for; every survey source collects over these |
| `work_root`, `work_roots` | where checkouts are found by name — `<work_root>/<repo>` |
| `checkouts` | an explicit path for a repository whose checkout is elsewhere, or that you work in but do not survey |
| `report_root` | where the engineer's records live: assignments, programs, partners, reports, sessions, shifts |
| `owner_labels` | issue labels that mean "waiting on the owner" |

`config/CONFIG.md` documents every key. Permissions, tools, hooks, MCP,
sandboxing, credentials and model settings are the runtime's, not Mellions';
`mellions config` refuses any key that would shadow them.

The config is found at `-config <file>`, `$MELLIONS_CONFIG`, `./mellions.json`,
or `~/.mellions/config.json`, in that order.

## 3. Connect it to your repositories

There is no per-repository setup. A repository is connected when:

- it is listed in `repos` (so the survey collects it and lanes can be opened on
  it), and
- a checkout of it sits under a `work_root` (or is named in `checkouts`), so a
  lane can be cut from it.

A session started in any directory is Mellions; one started inside a
repository's checkout knows which repository it is in, whether that tree is
somebody's lane, and who else is on the repository. Repositories keep their
own `CLAUDE.md` / `AGENTS.md`; Mellions adds nothing to them and reads them as
any engineer would.

## 4. Claude Code

`mellions install` did this:

```bash
claude plugin marketplace add /path/to/mellions-coxen     # the marketplace
claude plugin install mellions@mellions                      # the plugin
```

It also enables the plugin in `~/.claude/settings.json` (`enabledPlugins`).
Nothing else in Claude Code changes: permission mode, allowed tools, MCP
servers and model settings stay exactly as you had them. The plugin's hooks
appear under `/hooks` as `mellions`.

Start a session anywhere. The first thing you see is the SessionStart hooks
loading — "Loading who you are… how you think… how you establish what is
true… how you engineer… what you carry" — and the session's first words are
Mellions'. `/mellions:mellions` restates what it carries.

## 5. Codex

`mellions install` did this:

```bash
codex plugin marketplace add /path/to/mellions-coxen
codex plugin add mellions@mellions
```

Codex does not run a plugin's hooks until you have trusted them. **Start Codex
once and accept the Mellions hooks when it asks**, or open `/hooks` there and
trust them. Until then a Codex session has the Skills and the slash commands
but not the session-start context — it does not know it is Mellions.

Codex records that trust in `~/.codex/config.toml`, one entry per hook entry:

```toml
[hooks.state."/home/you/mellions-coxen/hooks/hooks.json:session_start:0:0"]
trusted_hash = "sha256:…"
```

`mellions doctor` counts those entries against the hooks the manifest declares
and prints `codex hooks: N of M trusted`. Short of `M`, a Codex session is not
Mellions, and no command here can close the gap — the prompt is Codex's and the
answer is yours. `codex --dangerously-bypass-hook-trust` runs them for one
invocation without persisting anything, which is useful for checking that Codex
finds the hooks at all and is not a substitute for trusting them.

Two Codex limits to know: Codex cuts a plugin Skill at 8,000 bytes and a
Skill description at 1,024, so every Skill in this repository is held under
those bounds by a build check; and the unattended shift script drives Claude
Code only (`CLAUDE_BIN`), so a Codex installation is interactive.

The Codex registration is verified by `mellions doctor`. The Codex path has
less production experience than the Claude Code path, so verify a fresh Codex
session instead of inferring success from registration alone.

## 6. Verify

```bash
mellions doctor
```

```
# mellions doctor — 0.1.0 (4328d7c)

binary                 present  /home/you/.local/bin/mellions
config                 present  /home/you/.mellions/config.json
checkouts              present  9 repositories under /home/you/workspace
assignments            present  0 open, /home/you/mellions/assignments
program                absent   none — mellions program discover drafts one
partnership            absent   none — mellions partner establish drafts one
git                    present
tracker (gh)           present  github.com
runtime claude         present  mellions@mellions (0.1.0) at ..., enabled, 9 SessionStart hooks
runtime codex          present  mellions@mellions installed, enabled 0.1.0 ...
load path              present  /home/you/mellions-coxen — read in place: known_marketplaces.json
                                records the mellions marketplace as a directory source, so a session
                                loads hooks, Skills, commands and the agent from there and `git pull`
                                there deploys them; the copy at ~/.claude/plugins/cache/... is written
                                by install and never read
load path commit       present  16b63c8 on dev, clean, up to date with origin/dev
codex hooks            partial  0 of 16 trusted — start codex once here and trust them, or a Codex
                                session is not Mellions
hooks                  present  9 SessionStart hooks declared in .../hooks/hooks.json
bearing skills         present  reasoning, deep research, falsification, self-learning under ...
```

`doctor` observes and configures nothing. Five things it establishes that
nothing else does:

- **where the plugin is actually loaded from** — `load path`, decided by the
  marketplace source in `known_marketplaces.json`, not by the copy
  `installed_plugins.json` names;
- **which commit that path stands at** — `load path commit`, when the load path
  is a git checkout: the commit, whether anything is uncommitted, and whether it
  is behind its upstream. On a checkout install this is the answer to "which
  Mellions is deployed on this host", and `UNCOMMITTED` or `N behind` means the
  host is running something no branch names;
- **whether Codex may run the hooks** — `codex hooks: N of M trusted`. Codex
  loads a plugin's Skills whether or not its hooks are trusted, so an untrusted
  installation reads as complete on every other line and hands you a session
  with every method that does not know it is Mellions. Trusting is Codex's own
  prompt to you and nothing here can grant it; the count is how you find out
  it is still owed. `unknown` means no `~/.codex/config.toml` to read, which is
  not the same as refused;
- **whether the next session will load the plugin** — from the runtime's own
  registry, settings and load path, made to agree;
- **whether *this* session loaded it** — run from inside a session, it reads
  that session's own transcript and says whether the identity reached it, and
  whether the installation it got is current or superseded by a reinstall;
- **unknown is not absent** — a probe that could not run says so and exits 0.
  Two states exit non-zero: a confirmed absence of something load-bearing, and
  a load path reported `STOPPED` — a checkout `git pull --ff-only` cannot
  deploy into today, so the host has stopped receiving what it installs. A
  checkout deliberately pinned to a tag is `STOPPED` by that definition and
  reports so for as long as it stays pinned.

A missing program and partnership are absences, not failures: the engineer
works without them and says so at every session start.

## 7. The first session

Start Claude Code (or Codex, after trusting the hooks) anywhere in the estate
and say:

> This installation has no program and no partnership. Run
> `mellions program discover` and draft the program; run
> `mellions partner establish <my email>` and draft my partnership; write the
> DECLARED sections as the questions you need answered; check both; adopt
> neither.

You get two files under the report root — `programs/<slug>.md` and
`partners/<name>.md` — with every section marked by its provenance:
`DISCOVERED` (cited evidence), `INFERRED` (the engineer's reading), `UNKNOWN`
(a named gap and what would settle it), and `DECLARED`, which the engineer may
not write and leaves as questions to you. Answer them in the files, in your
own words. Then:

```bash
mellions program check <slug>   && mellions program adopt <slug> -by "you"
mellions partner check <name>   && mellions partner adopt <name> -by "you"
```

Adoption records that you read what it wrote. From the next session on, both
reach the engineer at start, and it is told at its next prompt if either
changed underneath it.

Then give it work — `docs/playbook.md` walks through that.

## Unattended

`scripts/shift.sh` runs one shift without you; `scripts/shifts.sh` runs them
back to back — one runner per host, a self-update from the checkout between
shifts, a cooldown, a daily cap, `pause` and `stop` files. `deploy/README.md`
says how to start the runner at login on macOS or at boot on Linux without
root, and what the shift is and is not allowed to do while nobody is there.
`mellions doctor` says whether a runner is alive on this host.

## Upgrading

```bash
cd mellions-coxen && git pull --ff-only && make install
```

On a checkout install, **the fast-forward pull is the deployment** and `make install`
is there for the binary: the plugin's hooks, Skills, commands and agent are
read out of the checkout, so they are live for the next session the moment the
pull lands, and the `mellions` binary is the one part that has to be rebuilt
and copied onto PATH. Reinstalling is still correct — it re-registers and
re-reports — but it is not what deploys the plugin, and skipping it does not
hold a commit back.

The reverse is the thing to watch: a pull, a merge, or an uncommitted edit in
that checkout reaches every new session on the host immediately, with no
install step to gate it. `mellions doctor`'s `load path commit` line says what
the checkout is standing at, so a host that is dirty or behind is visible
rather than assumed.

Installing from `owner/repo` or a git URL instead makes the marketplace a
fetched one; there the runtime copies the plugin and keys the copy on the
`version` field, and reinstalling is the only thing that moves it.

Sessions already running keep the plugin they started with; `mellions install`
names them. On a machine with live sessions you would rather not disturb,
`CLAUDE.md` says how to prove a change in one throwaway session first
(`claude --settings <file> -p '…'`).

## Uninstalling

```bash
claude plugin uninstall mellions@mellions && claude plugin marketplace remove mellions
codex plugin remove mellions@mellions   && codex plugin marketplace remove mellions
rm "$(command -v mellions)"
```

The report root is yours: it holds every assignment record, program,
partnership and report. Nothing removes it.
