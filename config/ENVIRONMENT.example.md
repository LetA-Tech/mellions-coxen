<!-- Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca -->

# This installation's host

Copy to `config/ENVIRONMENT.md`, which is not committed, and record only what
`mellions doctor` cannot establish on its own. Everything a probe can answer
belongs to the probe: a version written here is stale the first time somebody
upgrades and does not edit this file.

Delete every line that does not apply. An empty file is a correct file.

## Host

    <name>, <distribution and release>, <cores>, <memory>, <root disk>

## Where the repositories are

    Checkouts live under <path>/<repo>.
    The working branch is <branch>; <branch> is the release branch.

The repository's own `CLAUDE.md` remains authoritative for both names.

## Access

    `gh` is authenticated as <login>; git over <https|ssh>.

Record the login, never a token, a key or a password.

## Isolation

    <container runtime and version>, runtimes: <list>
    This host already provides <sandbox command>, so the portable sandbox
    capability reuses it rather than starting a second implementation.

## Anything else a session would otherwise have to discover twice

    <internal hostname and what it serves, and how it is reached>
    <a tool that is installed but not configured, and what is missing>
