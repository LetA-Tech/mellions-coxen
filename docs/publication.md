<!-- Mellions Coxen | Copyright © 2026 LetA Tech Ltd. | leta@letatech.ca -->

# Initial public repository cutover

The access-controlled engineering repository contains private history even
when its current tree is clean. Changing that repository's visibility would
make the reachable history public. The initial Mellions public repository
therefore started from a snapshot in a new Git repository with one root commit;
it was not a visibility flip or an orphan branch inside the private object
database.

The permanent public repository was created through the procedure below. It is
retained as the cutover and recovery boundary, not as the day-to-day release
process: normal releases move reviewed work from `dev` to `main` in the public
repository.

Repository creation or renaming, visibility changes, tags, and credential
operations remain operator actions. The procedure creates and verifies only a
local, reversible candidate.

## Reproduce a fresh root

Run from the reviewed private candidate:

```bash
candidate=$(git rev-parse HEAD)
publication_parent=$(mktemp -d)
public_root="$publication_parent/mellions-coxen"
bash scripts/build-public-root.sh \
  --source "$(pwd -P)" \
  --commit "$candidate" \
  --output "$public_root" \
  --author-name "LetA Tech Ltd." \
  --author-email "leta@letatech.ca"
```

The builder exports the named commit with `git archive`, refuses a candidate
that contains `internal-docs/`, initializes a new repository, creates one root
commit, and configures no remote. It never changes the source repository.

## Verify reachability and contents

Run the project checks inside the fresh root, then verify the repository shape:

```bash
make -C "$public_root" check
test "$(git -C "$public_root" rev-list --all --count)" -eq 1
test -z "$(git -C "$public_root" remote)"
test ! -e "$public_root/internal-docs"
git -C "$public_root" fsck --full --no-reflogs
git -C "$public_root" ls-tree -r --name-only HEAD
git -C "$public_root" cat-file --batch-check --batch-all-objects
```

Inspect the file list and every object with the controlled private-term,
credential-shape, absolute-user-path, private-IP, and license/manifest checks
used for the release. Keep the private terms outside both repositories and run:

```bash
python3 "$public_root/scripts/check-public-privacy.py" \
  --root "$public_root" \
  --terms-file /path/to/private-release-terms.txt
```

Each detector must first find a planted positive control and leave a synthetic
negative control clear; `make check` exercises those structural controls and a
synthetic private term. Verify Markdown links with one known broken-link
control. Build and inspect the release archives from this root; an archive is
part of the release surface even when the repository tree is clean.

The private source tip must not exist in the fresh object database:

```bash
if git -C "$public_root" cat-file -e "$candidate^{commit}" 2>/dev/null; then
  echo "private candidate commit is reachable in the public root" >&2
  exit 1
fi
```

## Use a fresh root only for a new public destination

The existing public repository is authoritative. Do not rebuild its normal
releases from the access-controlled repository. Use this boundary only when a
new public destination must be created, and preserve the full private
repository under access control.

Only after the destination is confirmed empty, every credential exposure found
in source history is confirmed revoked or deleted, the reviewed candidate and
its intended merge result both pass `make check` locally, and the final
independent audit is clean should the operator add the public remote and push
the single-root `main` branch. Never add the private remote to the fresh root.
