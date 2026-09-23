#!/usr/bin/env python3
"""Scan a public candidate for structural identifiers and private terms."""

from __future__ import annotations

import argparse
import ipaddress
import os
import pathlib
import re
import subprocess


USER_PATH = re.compile(r"/(Users|home)/([A-Za-z0-9._-]+)")
IP_ADDRESS = re.compile(r"(?<![0-9.])(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?![0-9.])")
EMAIL = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
SYNTHETIC_USERS = {"you"}
PUBLIC_EMAILS = {"leta@letatech.ca"}
SYNTHETIC_EMAIL_DOMAINS = {"example.com", "example.net", "example.org", "example.invalid"}
SSH_REMOTE_LOCAL_PART = "git"
PRIVATE_NETWORKS = tuple(
    ipaddress.ip_network(f"{first}.{second}.0.0/{prefix}")
    for first, second, prefix in ((10, 0, 8), (172, 16, 12), (192, 168, 16))
)


def arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--private-term", action="append", default=[])
    parser.add_argument("--terms-file", action="append", default=[])
    # A working tree also holds what .gitignore keeps out of every commit:
    # build output, and files tools running from the checkout write there.
    # --git scans what git would publish, tracked or not yet added.
    parser.add_argument("--git", action="store_true")
    return parser.parse_args()


def private_terms(args: argparse.Namespace) -> list[str]:
    terms = list(args.private_term)
    for name in args.terms_file:
        terms.extend(
            line.strip()
            for line in pathlib.Path(name).read_text().splitlines()
            if line.strip() and not line.lstrip().startswith("#")
        )
    return sorted(set(terms), key=str.casefold)


def candidates(root: pathlib.Path, git: bool):
    if not git:
        return sorted(p for p in root.rglob("*") if ".git" not in p.parts)
    listed = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        check=True, capture_output=True,
    ).stdout
    return sorted({root / name.decode() for name in listed.split(b"\0") if name})


def text_files(root: pathlib.Path, git: bool = False):
    for path in candidates(root, git):
        if path.is_symlink():
            raw = os.readlink(path).encode()
        elif path.is_file():
            raw = path.read_bytes()
        else:
            continue
        if b"\0" in raw:
            continue
        yield path, raw.decode(errors="replace")


def is_ssh_remote(line: str, match: re.Match[str]) -> bool:
    """Report whether an EMAIL match is the authority of an SSH clone URL.

    `git@github.com:Org/repo.git` and `ssh://git@github.com/Org/repo.git` both
    contain a token shaped like an address and addressed to nobody. The path
    separator that follows is what distinguishes them from a real address.
    """
    if match.group().split("@", 1)[0].casefold() != SSH_REMOTE_LOCAL_PART:
        return False
    return line[match.end() : match.end() + 1] in {":", "/"}


def findings(root: pathlib.Path, terms: list[str], git: bool = False) -> list[str]:
    found = []
    for path, body in text_files(root, git):
        relative = path.relative_to(root)
        lines = body.splitlines()
        for number, line in enumerate(lines, 1):
            for match in USER_PATH.finditer(line):
                if match.group(2).casefold() not in SYNTHETIC_USERS:
                    found.append(f"{relative}:{number}: absolute user path")
            for match in IP_ADDRESS.finditer(line):
                try:
                    address = ipaddress.ip_address(match.group())
                except ValueError:
                    continue
                if any(address in network for network in PRIVATE_NETWORKS):
                    found.append(f"{relative}:{number}: private IP address")
            for match in EMAIL.finditer(line):
                if is_ssh_remote(line, match):
                    continue
                email = match.group().casefold()
                domain = email.rsplit("@", 1)[1]
                if email not in PUBLIC_EMAILS and domain not in SYNTHETIC_EMAIL_DOMAINS:
                    found.append(f"{relative}:{number}: non-public email address")
            folded = line.casefold()
            for term in terms:
                if term.casefold() in folded:
                    found.append(f"{relative}:{number}: private term {term!r}")
    return sorted(set(found))


def main() -> int:
    args = arguments()
    root = pathlib.Path(args.root).resolve()
    if not root.is_dir():
        raise SystemExit(f"check-public-privacy: not a directory: {root}")
    found = findings(root, private_terms(args), args.git)
    if found:
        print("\n".join(found))
        return 1
    print(f"public privacy scan passed: {sum(1 for _ in text_files(root, args.git))} text files")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
