#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
# Builds a one-commit public candidate without changing the private source.
set -euo pipefail

source_repo=""
source_commit=""
output=""
author_name=""
author_email=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --source) source_repo=${2-}; shift 2 ;;
    --commit) source_commit=${2-}; shift 2 ;;
    --output) output=${2-}; shift 2 ;;
    --author-name) author_name=${2-}; shift 2 ;;
    --author-email) author_email=${2-}; shift 2 ;;
    *) echo "build-public-root: unknown argument: $1" >&2; exit 2 ;;
  esac
done

[ -n "$source_repo" ] && [ -n "$source_commit" ] && [ -n "$output" ] && \
  [ -n "$author_name" ] && [ -n "$author_email" ] || {
  echo "usage: build-public-root.sh --source <repo> --commit <commit> --output <new-directory> --author-name <name> --author-email <email>" >&2
  exit 2
}

source_repo=$(cd "$source_repo" && pwd -P) || exit 2
git -C "$source_repo" rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
  echo "build-public-root: --source is not a git worktree" >&2
  exit 2
}
resolved=$(git -C "$source_repo" rev-parse --verify "$source_commit^{commit}") || {
  echo "build-public-root: --commit does not resolve to a commit" >&2
  exit 2
}

output_parent=$(dirname "$output")
output_name=$(basename "$output")
output_parent=$(cd "$output_parent" && pwd -P) || {
  echo "build-public-root: the output parent does not exist" >&2
  exit 2
}
output="$output_parent/$output_name"
[ ! -e "$output" ] || {
  echo "build-public-root: output already exists: $output" >&2
  exit 2
}

scratch=$(mktemp -d "$output_parent/.mellions-public-root.XXXXXX")
cleanup() { [ -z "${scratch:-}" ] || rm -rf "$scratch"; }
trap cleanup EXIT

git -C "$source_repo" archive "$resolved" | tar -x -C "$scratch"
[ ! -e "$scratch/internal-docs" ] || {
  echo "build-public-root: candidate contains internal-docs/" >&2
  exit 2
}

git -C "$scratch" init -q -b main
git -C "$scratch" config user.name "$author_name"
git -C "$scratch" config user.email "$author_email"
git -C "$scratch" add -A
git -C "$scratch" commit -q -m "Mellions Engineer public release root"

[ "$(git -C "$scratch" rev-list --all --count)" -eq 1 ] || {
  echo "build-public-root: generated repository has more than one commit" >&2
  exit 2
}
[ -z "$(git -C "$scratch" remote)" ] || {
  echo "build-public-root: generated repository unexpectedly has a remote" >&2
  exit 2
}

mv "$scratch" "$output"
scratch=""
printf '%s\n' "$output"
