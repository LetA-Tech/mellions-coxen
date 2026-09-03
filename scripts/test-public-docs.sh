#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
python3 - "$root" <<'PY'
import pathlib
import re
import sys
import tempfile
import urllib.parse

LINK = re.compile(r"!?\[[^]]*\]\(([^)]+)\)")


def target(raw):
    raw = raw.strip()
    if raw.startswith("<") and ">" in raw:
        return raw[1 : raw.index(">")]
    return raw.split(maxsplit=1)[0] if raw else ""


def markdown_links(path):
    fenced = False
    marker = ""
    for number, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
        stripped = line.lstrip()
        if stripped.startswith(("```", "~~~")):
            current = stripped[:3]
            if not fenced:
                fenced, marker = True, current
            elif current == marker:
                fenced, marker = False, ""
            continue
        if fenced:
            continue
        for match in LINK.finditer(line):
            yield number, target(match.group(1))


def broken(root, files):
    findings = []
    for path in files:
        for number, value in markdown_links(path):
            parsed = urllib.parse.urlparse(value)
            if not value or value.startswith("#") or parsed.scheme or parsed.netloc:
                continue
            relative = urllib.parse.unquote(value.split("#", 1)[0])
            if not relative or relative.startswith("/"):
                continue
            destination = (path.parent / relative).resolve()
            if not destination.exists():
                findings.append(f"{path.relative_to(root)}:{number}: {value}")
    return findings


with tempfile.TemporaryDirectory() as directory:
    control = pathlib.Path(directory)
    (control / "exists.md").write_text("present\n")
    good = control / "good.md"
    good.write_text("[present](exists.md)\n")
    bad = control / "bad.md"
    bad.write_text("[missing](not-there.md)\n")
    assert broken(control, [good]) == []
    assert broken(control, [bad]) == ["bad.md:1: not-there.md"]

root = pathlib.Path(sys.argv[1]).resolve()
files = sorted(
    path
    for path in root.rglob("*.md")
    if ".git" not in path.parts and "internal-docs" not in path.parts
)
findings = broken(root, files)
if findings:
    print("broken public Markdown links:", file=sys.stderr)
    print("\n".join(findings), file=sys.stderr)
    raise SystemExit(1)
print(f"public Markdown links passed: {len(files)} files")
PY
