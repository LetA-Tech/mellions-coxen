#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
[ ! -e "$root/internal-docs" ] || {
  echo "internal-docs exists in the public candidate tree" >&2
  exit 1
}
grep -qxF '/internal-docs/' "$root/.gitignore" || {
  echo "internal-docs is not excluded from future public commits" >&2
  exit 1
}
scratch=$(mktemp -d "${TMPDIR:-/tmp}/mellions-public-root-test.XXXXXX")
scratch=$(cd "$scratch" && pwd -P)
trap 'rm -rf "$scratch"' EXIT
source_repo="$scratch/private source"
public_root="$scratch/public root"

mkdir -p "$source_repo"
git -C "$source_repo" init -q -b dev
git -C "$source_repo" config user.name test
git -C "$source_repo" config user.email test@example.invalid
printf 'private history probe\n' > "$source_repo/private-history-only.txt"
git -C "$source_repo" add private-history-only.txt
git -C "$source_repo" commit -q -m private
private_blob=$(git -C "$source_repo" rev-parse HEAD:private-history-only.txt)
rm "$source_repo/private-history-only.txt"
printf '# Public tree\n' > "$source_repo/README.md"
mkdir -p "$source_repo/scripts"
printf '#!/usr/bin/env bash\nexit 0\n' > "$source_repo/scripts/check.sh"
git -C "$source_repo" add -A
git -C "$source_repo" commit -q -m public
private_tip=$(git -C "$source_repo" rev-parse HEAD)

bash "$root/scripts/build-public-root.sh" \
  --source "$source_repo" --commit "$private_tip" --output "$public_root" \
  --author-name "Public Test" --author-email public@example.invalid >/dev/null

[ "$(git -C "$public_root" rev-list --all --count)" -eq 1 ] || {
  echo "public root has inherited commits" >&2
  exit 1
}
[ -z "$(git -C "$public_root" remote)" ] || {
  echo "public root inherited a remote" >&2
  exit 1
}
[ -f "$public_root/README.md" ] && [ -f "$public_root/scripts/check.sh" ] || {
  echo "public root does not contain the selected tree" >&2
  exit 1
}
if git -C "$public_root" cat-file -e "$private_blob^{blob}" 2>/dev/null; then
  echo "private history blob is reachable in the public root" >&2
  exit 1
fi
if git -C "$public_root" cat-file -e "$private_tip^{commit}" 2>/dev/null; then
  echo "private source commit is reachable in the public root" >&2
  exit 1
fi

mkdir "$scratch/existing"
if bash "$root/scripts/build-public-root.sh" \
  --source "$source_repo" --commit "$private_tip" \
  --output "$scratch/existing" --author-name "Public Test" \
  --author-email public@example.invalid >/dev/null 2>&1; then
  echo "builder accepted an existing output directory" >&2
  exit 1
fi

mkdir -p "$source_repo/internal-docs"
printf 'private current tree probe\n' > "$source_repo/internal-docs/probe.md"
git -C "$source_repo" add internal-docs/probe.md
git -C "$source_repo" commit -q -m private-current-tree
if bash "$root/scripts/build-public-root.sh" \
  --source "$source_repo" --commit HEAD \
  --output "$scratch/rejected-private-tree" --author-name "Public Test" \
  --author-email public@example.invalid >/dev/null 2>&1; then
  echo "builder accepted internal-docs in the candidate" >&2
  exit 1
fi

echo "public root tests passed"
