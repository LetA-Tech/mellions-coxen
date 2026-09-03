#!/usr/bin/env bash
# Mellions Engineer | LetA Tech Ltd. | leta@letatech.ca
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/mellions-release-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

version=$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' "$root/.claude-plugin/plugin.json" | head -1)
commit=$(git -C "$root" rev-parse --short HEAD)
epoch=$(git -C "$root" log -1 --format=%ct)

build() {
	make -s -C "$root" release DIST_DIR="$1" SOURCE_DATE_EPOCH="$epoch" >/dev/null
}

build "$tmp/one"
(cd "$tmp/one" && shasum -a 256 -c checksums.txt >/dev/null)

count=$(find "$tmp/one" -maxdepth 1 -type f -name '*.tar.gz' | wc -l | tr -d ' ')
[ "$count" = 4 ] || { echo "release has $count archives, want 4" >&2; exit 1; }

host_os=$(go env GOOS)
host_arch=$(go env GOARCH)
for target in darwin_arm64 darwin_amd64 linux_amd64 linux_arm64; do
	archive="$tmp/one/mellions_${version}_${target}.tar.gz"
	[ -f "$archive" ] || { echo "missing $archive" >&2; exit 1; }
	pkg="mellions_${version}_${target}"
	cat >"$tmp/want.list" <<EOF
$pkg/
$pkg/LICENSE
$pkg/NOTICE
$pkg/README.md
$pkg/config/
$pkg/config/mellions.example.json
$pkg/mellions
EOF
	tar -tzf "$archive" >"$tmp/got.list"
	diff -u "$tmp/want.list" "$tmp/got.list"
	mkdir "$tmp/extract"
	tar -xzf "$archive" -C "$tmp/extract"
	bin="$tmp/extract/$pkg/mellions"
	[ -x "$bin" ] || { echo "$target binary is not executable" >&2; exit 1; }
	cmp "$root/LICENSE" "$tmp/extract/$pkg/LICENSE"
	cmp "$root/NOTICE" "$tmp/extract/$pkg/NOTICE"
	cmp "$root/README.md" "$tmp/extract/$pkg/README.md"
	cmp "$root/config/mellions.example.json" "$tmp/extract/$pkg/config/mellions.example.json"
	go version -m "$bin" >"$tmp/buildinfo"
	grep -Fq -- '-trimpath=true' "$tmp/buildinfo" || {
		echo "$target binary was not built with -trimpath" >&2; exit 1;
	}
	if LC_ALL=C grep -aFq "$root" "$bin" || LC_ALL=C grep -aFq "$tmp" "$bin"; then
		echo "$target binary contains a local build path" >&2; exit 1
	fi
	os=${target%_*}; arch=${target#*_}
	grep -Fq "GOOS=$os" "$tmp/buildinfo"
	grep -Fq "GOARCH=$arch" "$tmp/buildinfo"
	if [ "$os" = "$host_os" ] && [ "$arch" = "$host_arch" ]; then
		"$bin" version | grep -Fq "$version ($commit)"
	fi
	rm -rf "$tmp/extract"
done

build "$tmp/two"
for archive in "$tmp/one"/*.tar.gz; do
	cmp "$archive" "$tmp/two/$(basename "$archive")"
done
cmp "$tmp/one/checksums.txt" "$tmp/two/checksums.txt"

echo "release archive checks passed"
