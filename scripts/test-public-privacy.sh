#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/mellions-public-privacy.XXXXXX")
scratch=$(cd "$scratch" && pwd -P)
trap 'rm -rf "$scratch"' EXIT

mkdir -p "$scratch/bad" "$scratch/bad-ssh" "$scratch/bad-link" "$scratch/good"
python3 - "$scratch" <<'PY'
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
(root / "terms").write_text("confidential-service\n")
(root / "bad" / "paths.txt").write_text(
    "/Users/" + "private-user/project\n"
    + "10." + "23.4.5\n"
    + "person@" + "private.example\n"
    + "confidential-service\n"
)
# Sole finding in its own root, so the scan's verdict rests on this line alone:
# the SSH-clone-URL exemption is for the authority of a URL, never for a local
# part. An address that merely begins "git@" is still an address.
(root / "bad-ssh" / "address.txt").write_text("git@" + "private.example\n")
(root / "good" / "synthetic.txt").write_text(
    "/Users/you/project\n/home/you/project\n192.0.2.10\n"
    "leta@letatech.ca\ntest@example.invalid\n"
    # An SSH clone URL's authority is addressed to nobody.
    "git@github.com:LetA-Tech/mellions-coxen.git\n"
    "ssh://git@github.com/LetA-Tech/mellions-coxen.git\n"
)
(root / "bad-link" / "private-link").symlink_to("/Users/" + "private-user/project")
(root / "good" / "synthetic-link").symlink_to("/home/you/project")
PY

if python3 "$root/scripts/check-public-privacy.py" \
  --root "$scratch/bad" --terms-file "$scratch/terms" >/dev/null 2>&1; then
  echo "public privacy detector missed its planted controls" >&2
  exit 1
fi
if python3 "$root/scripts/check-public-privacy.py" \
  --root "$scratch/bad-ssh" >/dev/null 2>&1; then
  echo "public privacy detector exempted a git@ address that is not a clone URL" >&2
  exit 1
fi
if python3 "$root/scripts/check-public-privacy.py" \
  --root "$scratch/bad-link" >/dev/null 2>&1; then
  echo "public privacy detector missed a private symlink target" >&2
  exit 1
fi
python3 "$root/scripts/check-public-privacy.py" \
  --root "$scratch/good" --terms-file "$scratch/terms" >/dev/null
python3 "$root/scripts/check-public-privacy.py" --root "$root"
